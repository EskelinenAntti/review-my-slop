package comments

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

func OpenDefault() (Store, error) {
	root := os.Getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(root) {
		home, err := os.UserHomeDir()
		if err != nil {
			return Store{}, fmt.Errorf("resolve user home directory: %w", err)
		}
		root = filepath.Join(home, ".local", "share")
	}
	return Store{filepath.Join(root, "review-my-slop", "comments.db")}, nil
}

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
	key := []byte(comment.ID)
	return comment, s.transact(true, func(bucket *bolt.Bucket) error {
		var pending int
		bucket.ForEach(func(_, value []byte) error {
			pending += len(value)
			return nil
		})
		if pending+len(data) > maxPendingBytes {
			return fmt.Errorf("pending feedback exceeds %d bytes", maxPendingBytes)
		}
		if bucket.Get(key) != nil {
			return errors.New("comment ID already exists")
		}
		return bucket.Put(key, data)
	})
}

func (s Store) List(repository string) ([]Comment, error) {
	var comments []Comment
	return comments, s.transact(false, func(bucket *bolt.Bucket) error {
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
}

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
	key := []byte(comment.ID)
	return s.transact(true, func(bucket *bolt.Bucket) error {
		oldKey, err := findComment(bucket, comment.Repository, comment.ID)
		if err != nil {
			return err
		}
		if !bytes.Equal(oldKey, key) {
			if err := bucket.Delete(oldKey); err != nil {
				return err
			}
		}
		return bucket.Put(key, data)
	})
}

func (s Store) Delete(repository, id string) error {
	if repository == "" || id == "" {
		return errors.New("repository and comment ID are required")
	}
	return s.transact(true, func(bucket *bolt.Bucket) error {
		key, err := findComment(bucket, repository, id)
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
	return s.transact(true, func(bucket *bolt.Bucket) error {
		var keys [][]byte
		if err := bucket.ForEach(func(key, value []byte) error {
			comment, err := decodeComment(value)
			if err != nil {
				return err
			}
			if comment.Repository == repository && slices.Contains(ids, comment.ID) {
				keys = append(keys, bytes.Clone(key))
			}
			return nil
		}); err != nil {
			return err
		}
		for _, key := range keys {
			if err := bucket.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})
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

func findComment(bucket *bolt.Bucket, repository, id string) ([]byte, error) {
	cursor := bucket.Cursor()
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		comment, err := decodeComment(value)
		if err != nil {
			return nil, err
		}
		if comment.Repository == repository && comment.ID == id {
			return bytes.Clone(key), nil
		}
	}
	return nil, errors.New("comment is no longer in the comments")
}

func decodeComment(data []byte) (Comment, error) {
	var stored struct {
		Comment
		Legacy *Comment `json:"comment"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	if stored.Legacy != nil {
		stored.Anchor, stored.Body = stored.Legacy.Anchor, stored.Legacy.Body
	}
	return stored.Comment, nil
}

func (s Store) transact(write bool, fn func(*bolt.Bucket) error) error {
	db, err := s.open()
	if err != nil {
		if !write && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer db.Close()
	transaction := db.View
	if write {
		transaction = db.Update
	}
	return transaction(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(messagesBucket)
		if write {
			bucket, err = tx.CreateBucketIfNotExists(messagesBucket)
			if err != nil {
				return err
			}
		} else if bucket == nil {
			return nil
		}
		return fn(bucket)
	})
}

func (s Store) open() (*bolt.DB, error) {
	path := s.Path
	if path == "" {
		return nil, errors.New("comments path is empty")
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create comments directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return nil, fmt.Errorf("secure comments directory: %w", err)
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open comments: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		db.Close()
		return nil, fmt.Errorf("secure comments database: %w", err)
	}
	return db, nil
}
