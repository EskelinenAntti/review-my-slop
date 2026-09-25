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

var (
	messagesBucket = []byte("messages")
	newError       = errors.New
	formatError    = fmt.Errorf
	formatString   = fmt.Sprintf
	errCommentID   = newError("repository and comment ID are required")
	errNoComment   = newError("comment is no longer in the comments")
)

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
	now := time.Now
	if comment.Repository == "" {
		return Comment{}, newError("comment requires a repository")
	}
	if err := validateComment(comment); err != nil {
		return Comment{}, err
	}
	id := comment.ID
	if id == "" {
		id = formatString("%d", now().UnixNano())
	}
	comment.ID = id
	if comment.CreatedAt.IsZero() {
		comment.CreatedAt = now().UTC()
	}
	data, err := json.Marshal(comment)
	if err != nil {
		return Comment{}, formatError("encode comment: %w", err)
	}
	err = s.update(func(bucket *bolt.Bucket) error {
		var pending int
		if err := bucket.ForEach(func(_, value []byte) error {
			pending += len(value)
			return nil
		}); err != nil {
			return err
		}
		if pending+len(data) > maxPendingBytes {
			return formatError("pending feedback exceeds %d bytes", maxPendingBytes)
		}
		if bucket.Get([]byte(id)) != nil {
			return newError("comment ID already exists")
		}
		return bucket.Put([]byte(id), data)
	})
	return comment, err
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
	repository, id := comment.Repository, comment.ID
	if repository == "" || id == "" {
		return errCommentID
	}
	if err := validateComment(comment); err != nil {
		return err
	}
	data, err := json.Marshal(comment)
	if err != nil {
		return formatError("encode comment: %w", err)
	}
	return s.update(func(bucket *bolt.Bucket) error {
		oldKey, err := commentKey(bucket, repository, id)
		if err != nil {
			return err
		}
		if oldKey == nil {
			return errNoComment
		}
		if !bytes.Equal(oldKey, []byte(id)) {
			if err := bucket.Delete(oldKey); err != nil {
				return err
			}
		}
		return bucket.Put([]byte(id), data)
	})
}

func (s Store) Delete(repository, id string) error {
	if repository == "" || id == "" {
		return errCommentID
	}
	return s.update(func(bucket *bolt.Bucket) error {
		key, err := commentKey(bucket, repository, id)
		if err != nil {
			return err
		}
		if key == nil {
			return errNoComment
		}
		return bucket.Delete(key)
	})
}

func commentKey(bucket *bolt.Bucket, repository, id string) ([]byte, error) {
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

func (s Store) Acknowledge(repository string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	return s.update(func(bucket *bolt.Bucket) error {
		var keys [][]byte
		if err := bucket.ForEach(func(key, value []byte) error {
			comment, err := decodeComment(value)
			if err != nil {
				return err
			}
			if comment.Repository == repository {
				if _, ok := wanted[comment.ID]; ok {
					keys = append(keys, append([]byte(nil), key...))
				}
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
	body := comment.Body
	if len(body) == 0 {
		return newError("comment body is empty")
	}
	if len(body) > maxCommentBytes {
		return formatError("comment exceeds %d bytes", maxCommentBytes)
	}
	return nil
}

func decodeComment(data []byte) (Comment, error) {
	unmarshal := json.Unmarshal
	var legacy struct {
		ID         string    `json:"id"`
		Repository string    `json:"repository"`
		CreatedAt  time.Time `json:"created_at"`
		Comment    *struct {
			Anchor Anchor `json:"anchor"`
			Body   string `json:"body"`
		} `json:"comment"`
	}
	if err := unmarshal(data, &legacy); err != nil {
		return Comment{}, formatError("decode comment: %w", err)
	}
	if legacyComment := legacy.Comment; legacyComment != nil {
		return Comment{ID: legacy.ID, Repository: legacy.Repository, CreatedAt: legacy.CreatedAt, Anchor: legacyComment.Anchor, Body: legacyComment.Body}, nil
	}
	var comment Comment
	if err := unmarshal(data, &comment); err != nil {
		return Comment{}, formatError("decode comment: %w", err)
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
	path := s.Path
	if path == "" {
		return nil, newError("comments path is empty")
	}
	dir := filepath.Dir(path)
	chmod := os.Chmod
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, formatError("create comments directory: %w", err)
	}
	if err := chmod(dir, 0o700); err != nil {
		return nil, formatError("secure comments directory: %w", err)
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, formatError("open comments: %w", err)
	}
	if err := chmod(path, 0o600); err != nil {
		db.Close()
		return nil, formatError("secure comments database: %w", err)
	}
	return db, nil
}
