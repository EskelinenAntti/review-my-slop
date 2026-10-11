package comments

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	maxCommentBytes = 64 << 10
	maxPendingBytes = 16 << 20
)

var messagesBucket = []byte("messages")

// Store persists review comments in a local database.
type Store struct {
	Path string
}

// DefaultPath resolves the default comments database path.
func DefaultPath() (string, error) {
	data, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(data, "comments.db"), nil
}

// OpenDefault constructs a store at the default database path.
func OpenDefault() (Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return Store{}, err
	}
	return Store{Path: path}, nil
}

// Add stores a new comment and returns its assigned identity.
func (s Store) Add(comment Comment) (Comment, error) {
	if comment.Repository == "" {
		return Comment{}, errors.New("comment requires a repository")
	}
	if err := validateComment(comment); err != nil {
		return Comment{}, err
	}
	if comment.ID == "" {
		comment.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if comment.CreatedAt.IsZero() {
		comment.CreatedAt = time.Now().UTC()
	}
	data, err := json.Marshal(comment)
	if err != nil {
		return Comment{}, fmt.Errorf("encode comment: %w", err)
	}
	err = s.writeComments(
		func(bucket *bolt.Bucket) error {
			return insertComment(bucket, comment.ID, data)
		},
	)
	return comment, err
}

func insertComment(bucket *bolt.Bucket, id string, data []byte) error {
	var pending int
	cursor := bucket.Cursor()
	for _, value := cursor.First(); value != nil; _, value = cursor.Next() {
		pending += len(value)
	}
	if pending+len(data) > maxPendingBytes {
		return fmt.Errorf(
			"pending feedback exceeds %d bytes",
			maxPendingBytes,
		)
	}
	if bucket.Get([]byte(id)) != nil {
		return errors.New("comment ID already exists")
	}
	return bucket.Put([]byte(id), data)
}

// List loads the comments belonging to a repository.
func (s Store) List(repository string) ([]Comment, error) {
	var comments []Comment
	err := s.view(func(bucket *bolt.Bucket) error {
		return bucket.ForEach(func(_, value []byte) error {
			comment, err := decodeComment(value)
			if err != nil {
				return err
			}
			if comment.Repository == repository {
				comments = append(comments, comment)
			}
			return nil
		})
	})
	return comments, err
}

// Update replaces an existing comment with the same identity.
func (s Store) Update(comment Comment) error {
	if comment.Repository == "" || comment.ID == "" {
		return errors.New("repository and comment ID are required")
	}
	if err := validateComment(comment); err != nil {
		return err
	}
	data, err := json.Marshal(comment)
	if err != nil {
		return fmt.Errorf("encode comment: %w", err)
	}
	return s.writeComments(func(bucket *bolt.Bucket) error {
		return replaceStoredComment(bucket, comment, data)
	})
}

func storedKey(bucket *bolt.Bucket, repository, id string) ([]byte, error) {
	cursor := bucket.Cursor()
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		stored, err := decodeComment(value)
		if err != nil {
			return nil, err
		}
		if stored.Repository == repository && stored.ID == id {
			return append([]byte(nil), key...), nil
		}
	}
	return nil, errors.New("comment is no longer in the comments")
}

// Delete removes the identified comment from a repository.
func (s Store) Delete(repository, id string) error {
	if repository == "" || id == "" {
		return errors.New("repository and comment ID are required")
	}
	return s.writeComments(func(bucket *bolt.Bucket) error {
		key, err := storedKey(bucket, repository, id)
		if err != nil {
			return err
		}
		return bucket.Delete(key)
	})
}

// Acknowledge removes the comments successfully exported to the agent.
func (s Store) Acknowledge(repository string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	return s.writeComments(func(bucket *bolt.Bucket) error {
		keys, err := acknowledgedKeys(bucket, repository, wanted)
		if err != nil {
			return err
		}
		return removeKeys(bucket, keys)
	})
}

func acknowledgedKeys(
	bucket *bolt.Bucket,
	repository string,
	wanted map[string]struct{},
) (
	[][]byte,
	error,
) {
	var keys [][]byte
	err := bucket.ForEach(func(key, value []byte) error {
		comment, err := decodeComment(value)
		if err != nil {
			return err
		}
		if comment.Repository != repository {
			return nil
		}
		if _, ok := wanted[comment.ID]; ok {
			keys = append(keys, append([]byte(nil), key...))
		}
		return nil
	})
	return keys, err
}

func validateComment(comment Comment) error {
	if len(comment.Body) == 0 {
		return errors.New("comment body is empty")
	}
	if len(comment.Body) > maxCommentBytes {
		return fmt.Errorf("comment exceeds %d bytes", maxCommentBytes)
	}
	return nil
}

func decodeComment(data []byte) (Comment, error) {
	var legacy struct {
		ID         string      `json:"id"`
		Repository string      `json:"repository"`
		CreatedAt  time.Time   `json:"created_at"`
		Comment    *legacyBody `json:"comment"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	if legacy.Comment != nil {
		return Comment{
			ID:         legacy.ID,
			Repository: legacy.Repository,
			CreatedAt:  legacy.CreatedAt,
			Anchor:     legacy.Comment.Anchor,
			Body:       legacy.Comment.Body,
		}, nil
	}
	var comment Comment
	if err := json.Unmarshal(data, &comment); err != nil {
		return Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	return comment, nil
}

func (s Store) writeComments(fn func(*bolt.Bucket) error) error {
	return s.updateBucket(messagesBucket, fn)
}

func (s Store) updateBucket(name []byte, fn func(*bolt.Bucket) error) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(name)
		if err != nil {
			return err
		}
		return fn(bucket)
	})
}

func (s Store) view(fn func(*bolt.Bucket) error) error {
	return s.viewBucket(messagesBucket, fn)
}

func (s Store) viewBucket(name []byte, fn func(*bolt.Bucket) error) error {
	db, err := s.open()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer db.Close()
	return db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(name)
		if bucket == nil {
			return nil
		}
		return fn(bucket)
	})
}

func (s Store) open() (*bolt.DB, error) {
	if s.Path == "" {
		return nil, errors.New("comments path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), privateDirMode); err != nil {
		return nil, fmt.Errorf("create comments directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(s.Path), privateDirMode); err != nil {
		return nil, fmt.Errorf("secure comments directory: %w", err)
	}
	db, err := bolt.Open(
		s.Path,
		privateFileMode,
		&bolt.Options{Timeout: 2 * time.Second},
	)
	if err != nil {
		return nil, fmt.Errorf("open comments: %w", err)
	}
	if err := os.Chmod(s.Path, privateFileMode); err != nil {
		db.Close()
		return nil, fmt.Errorf("secure comments database: %w", err)
	}
	return db, nil
}

type legacyBody struct {
	Anchor Anchor `json:"anchor"`
	Body   string `json:"body"`
}

func replaceStoredComment(
	bucket *bolt.Bucket,
	comment Comment,
	data []byte,
) error {
	key, err := storedKey(bucket, comment.Repository, comment.ID)
	if err != nil {
		return err
	}
	if !bytes.Equal(key, []byte(comment.ID)) {
		if err := bucket.Delete(key); err != nil {
			return err
		}
	}
	return bucket.Put([]byte(comment.ID), data)
}

func removeKeys(bucket *bolt.Bucket, keys [][]byte) error {
	for _, key := range keys {
		if err := bucket.Delete(key); err != nil {
			return err
		}
	}
	return nil
}
