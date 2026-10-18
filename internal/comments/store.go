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

// Store persists pending review comments in a database at Path.
type Store struct {
	Path string
}

// DefaultPath returns the database path in the application data directory.
func DefaultPath() (string, error) {
	data, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(data, "comments.db"), nil
}

// OpenDefault returns a Store configured with the default database path.
func OpenDefault() (Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return Store{}, err
	}
	return Store{Path: path}, nil
}

// Add validates and stores a comment, assigning an ID and timestamp when
// absent.
func (s Store) Add(comment Comment) (Comment, error) {
	if comment.Repository == "" {
		return Comment{}, errors.New("comment requires a repository")
	}
	if comment.ID == "" {
		comment.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if comment.CreatedAt.IsZero() {
		comment.CreatedAt = time.Now().UTC()
	}
	data, err := encodeComment(comment)
	if err != nil {
		return Comment{}, err
	}
	err = s.transact(func(bucket *bolt.Bucket) error {
		return addComment(bucket, comment.ID, data)
	})
	return comment, err
}

func addComment(bucket *bolt.Bucket, id string, data []byte) error {
	if pendingBytes(bucket)+len(data) > maxPendingBytes {
		return fmt.Errorf("pending feedback exceeds %d bytes", maxPendingBytes)
	}
	if bucket.Get([]byte(id)) != nil {
		return errors.New("comment ID already exists")
	}
	return bucket.Put([]byte(id), data)
}

// List returns pending comments for the repository.
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

// Update replaces an existing comment in the repository.
func (s Store) Update(comment Comment) error {
	identityErr := validateCommentIdentity(
		comment.Repository,
		comment.ID,
	)
	if identityErr != nil {
		return identityErr
	}
	data, err := encodeComment(comment)
	if err != nil {
		return err
	}
	return s.transact(
		func(bucket *bolt.Bucket) error {
			return replaceComment(bucket, comment, data)
		},
	)
}

// Delete removes the identified comment from the repository.
func (s Store) Delete(repository, id string) error {
	if err := validateCommentIdentity(repository, id); err != nil {
		return err
	}
	return s.transact(func(bucket *bolt.Bucket) error {
		key, err := findCommentKey(bucket, repository, id)
		if err != nil {
			return err
		}
		if key == nil {
			return errors.New("comment is no longer in the comments")
		}
		return bucket.Delete(key)
	})
}

// Acknowledge removes the identified comments after their feedback is consumed.
func (s Store) Acknowledge(repository string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	return s.transact(func(bucket *bolt.Bucket) error {
		keys, err := findPendingKeys(bucket, repository, wanted)
		if err != nil {
			return err
		}
		return deleteKeys(bucket, keys)
	})
}

func deleteKeys(bucket *bolt.Bucket, keys [][]byte) error {
	for _, key := range keys {
		if err := bucket.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

func pendingBytes(bucket *bolt.Bucket) int {
	var total int
	cursor := bucket.Cursor()
	for _, value := cursor.First(); value != nil; _, value = cursor.Next() {
		total += len(value)
	}
	return total
}

func replaceComment(bucket *bolt.Bucket, comment Comment, data []byte) error {
	key, err := findCommentKey(bucket, comment.Repository, comment.ID)
	if err != nil {
		return err
	}
	if key == nil {
		return errors.New("comment is no longer in the comments")
	}
	if !bytes.Equal(key, []byte(comment.ID)) {
		if err := bucket.Delete(key); err != nil {
			return err
		}
	}
	return bucket.Put([]byte(comment.ID), data)
}

func findCommentKey(
	bucket *bolt.Bucket,
	repository, id string,
) ([]byte, error) {
	cursor := bucket.Cursor()
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		comment, err := decodeComment(value)
		if err != nil {
			return nil, err
		}
		if comment.Repository == repository && comment.ID == id {
			return append([]byte(nil), key...), nil
		}
	}
	return nil, nil
}

func findPendingKeys(
	bucket *bolt.Bucket,
	repository string,
	wanted map[string]struct{},
) ([][]byte, error) {
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

func validateCommentIdentity(repository, id string) error {
	if repository == "" || id == "" {
		return errors.New("repository and comment ID are required")
	}
	return nil
}

func encodeComment(comment Comment) ([]byte, error) {
	if err := validateComment(comment); err != nil {
		return nil, err
	}
	data, err := json.Marshal(comment)
	if err != nil {
		return nil, fmt.Errorf("encode comment: %w", err)
	}
	return data, nil
}

type legacyComment struct {
	Anchor Anchor `json:"anchor"`
	Body   string `json:"body"`
}

type legacyRecord struct {
	ID         string         `json:"id"`
	Repository string         `json:"repository"`
	CreatedAt  time.Time      `json:"created_at"`
	Comment    *legacyComment `json:"comment"`
}

func decodeComment(data []byte) (Comment, error) {
	var legacy legacyRecord
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

func (s Store) transact(fn func(*bolt.Bucket) error) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(messagesBucket)
		if err != nil {
			return err
		}
		return fn(bucket)
	})
}

func (s Store) view(fn func(*bolt.Bucket) error) error {
	db, err := s.open()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer db.Close()
	return db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(messagesBucket)
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
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return nil, fmt.Errorf("create comments directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(s.Path), 0o700); err != nil {
		return nil, fmt.Errorf("secure comments directory: %w", err)
	}
	db, err := bolt.Open(s.Path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open comments: %w", err)
	}
	if err := os.Chmod(s.Path, 0o600); err != nil {
		db.Close()
		return nil, fmt.Errorf("secure comments database: %w", err)
	}
	return db, nil
}
