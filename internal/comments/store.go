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

type Store struct {
	Path string
}

func DefaultPath() (string, error) {
	data, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(data, "comments.db"), nil
}

func OpenDefault() (Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return Store{}, err
	}
	return Store{Path: path}, nil
}

func (s Store) Add(comment Comment) (Comment, error) {
	if comment.Repository == "" {
		return Comment{}, errors.New("comment requires a repository")
	}
	if err := validateComment(comment); err != nil {
		return Comment{}, err
	}
	comment = initializeComment(comment)
	data, err := json.Marshal(comment)
	if err != nil {
		return Comment{}, fmt.Errorf("encode comment: %w", err)
	}
	err = s.update(func(bucket *bolt.Bucket) error { return addComment(bucket, comment.ID, data) })
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

func pendingBytes(bucket *bolt.Bucket) int {
	var pending int
	cursor := bucket.Cursor()
	for _, value := cursor.First(); value != nil; _, value = cursor.Next() {
		pending += len(value)
	}
	return pending
}

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

func (s Store) Update(comment Comment) error {
	if missingIdentity(comment.Repository, comment.ID) {
		return errors.New("repository and comment ID are required")
	}
	if err := validateComment(comment); err != nil {
		return err
	}
	data, err := json.Marshal(comment)
	if err != nil {
		return fmt.Errorf("encode comment: %w", err)
	}
	return s.update(func(bucket *bolt.Bucket) error { return updateComment(bucket, comment, data) })
}

func findCommentKey(bucket *bolt.Bucket, repository, id string) ([]byte, error) {
	cursor := bucket.Cursor()
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		stored, err := decodeComment(value)
		if err != nil {
			return nil, err
		}
		if matchesComment(stored, repository, id) {
			return append([]byte(nil), key...), nil
		}
	}
	return nil, errors.New("comment is no longer in the comments")
}

func matchesComment(comment Comment, repository, id string) bool {
	return comment.Repository == repository && comment.ID == id
}

func replaceComment(bucket *bolt.Bucket, oldKey []byte, id string, data []byte) error {
	if !bytes.Equal(oldKey, []byte(id)) {
		if err := bucket.Delete(oldKey); err != nil {
			return err
		}
	}
	return bucket.Put([]byte(id), data)
}

func (s Store) Delete(repository, id string) error {
	if repository == "" || id == "" {
		return errors.New("repository and comment ID are required")
	}
	return s.update(func(bucket *bolt.Bucket) error {
		key, err := findCommentKey(bucket, repository, id)
		if err != nil {
			return err
		}
		return bucket.Delete(key)
	})
}

func (s Store) Acknowledge(repository string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	return s.update(func(bucket *bolt.Bucket) error {
		keys, err := acknowledgedKeys(bucket, repository, wanted)
		if err != nil {
			return err
		}
		return deleteKeys(bucket, keys)
	})
}

func acknowledgedKeys(bucket *bolt.Bucket, repository string, wanted map[string]struct{}) ([][]byte, error) {
	var keys [][]byte
	err := bucket.ForEach(func(key, value []byte) error {
		comment, err := decodeComment(value)
		if err != nil {
			return err
		}
		if wantedComment(comment, repository, wanted) {
			keys = append(keys, append([]byte(nil), key...))
		}
		return nil
	})
	return keys, err
}

func wantedComment(comment Comment, repository string, wanted map[string]struct{}) bool {
	_, ok := wanted[comment.ID]
	return comment.Repository == repository && ok
}

func deleteKeys(bucket *bolt.Bucket, keys [][]byte) error {
	for _, key := range keys {
		if err := bucket.Delete(key); err != nil {
			return err
		}
	}
	return nil
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
		ID         string    `json:"id"`
		Repository string    `json:"repository"`
		CreatedAt  time.Time `json:"created_at"`
		Comment    *struct {
			Anchor Anchor `json:"anchor"`
			Body   string `json:"body"`
		} `json:"comment"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	if legacy.Comment != nil {
		return Comment{ID: legacy.ID, Repository: legacy.Repository, CreatedAt: legacy.CreatedAt, Anchor: legacy.Comment.Anchor, Body: legacy.Comment.Body}, nil
	}
	var comment Comment
	if err := json.Unmarshal(data, &comment); err != nil {
		return Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	return comment, nil
}

func (s Store) update(fn func(*bolt.Bucket) error) error {
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
	if err := s.prepareDirectory(); err != nil {
		return nil, err
	}
	db, err := bolt.Open(s.Path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open comments: %w", err)
	}
	return secureDatabase(db, s.Path)
}

func secureDatabase(db *bolt.DB, path string) (*bolt.DB, error) {
	if err := os.Chmod(path, 0o600); err != nil {
		db.Close()
		return nil, fmt.Errorf("secure comments database: %w", err)
	}
	return db, nil
}

func initializeComment(comment Comment) Comment {
	if comment.ID == "" {
		comment.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if comment.CreatedAt.IsZero() {
		comment.CreatedAt = time.Now().UTC()
	}
	return comment
}

func (s Store) prepareDirectory() error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("create comments directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("secure comments directory: %w", err)
	}
	return nil
}

func missingIdentity(repository, id string) bool { return repository == "" || id == "" }

func updateComment(bucket *bolt.Bucket, comment Comment, data []byte) error {
	key, err := findCommentKey(bucket, comment.Repository, comment.ID)
	if err != nil {
		return err
	}
	return replaceComment(bucket, key, comment.ID, data)
}
