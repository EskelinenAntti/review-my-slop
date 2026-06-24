package inbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/eskelinenantti/review-my-slop/internal/review"
	"github.com/eskelinenantti/review-my-slop/internal/xdg"
)

const (
	maxCommentBytes = 64 << 10
	maxPendingBytes = 16 << 20
)

var (
	messagesBucket  = []byte("messages")
	settingsBucket  = []byte("settings")
	sideBySideKey   = []byte("side-by-side")
	enabledSetting  = []byte{1}
	disabledSetting = []byte{0}
)

type Store struct {
	Path string
}

func DefaultPath() (string, error) {
	data, err := xdg.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(data, "inbox.db"), nil
}

func OpenDefault() (Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return Store{}, err
	}
	return Store{Path: path}, nil
}

// Save adds a new comment or updates an existing comment in a repository.
func (s Store) Save(comment review.Comment, repository string) (review.Comment, error) {
	comment.Repository = repository
	if comment.ID == "" {
		return s.Add(comment)
	}
	return comment, s.Update(comment)
}

func (s Store) Add(comment review.Comment) (review.Comment, error) {
	if comment.Repository == "" {
		return review.Comment{}, errors.New("comment requires a repository")
	}
	if err := validateComment(comment); err != nil {
		return review.Comment{}, err
	}
	if comment.ID == "" {
		comment.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if comment.CreatedAt.IsZero() {
		comment.CreatedAt = time.Now().UTC()
	}
	data, err := json.Marshal(comment)
	if err != nil {
		return review.Comment{}, fmt.Errorf("encode comment: %w", err)
	}
	err = s.update(func(bucket *bolt.Bucket) error {
		var pending int
		cursor := bucket.Cursor()
		for _, value := cursor.First(); value != nil; _, value = cursor.Next() {
			pending += len(value)
		}
		if pending+len(data) > maxPendingBytes {
			return fmt.Errorf("pending feedback exceeds %d bytes", maxPendingBytes)
		}
		if bucket.Get([]byte(comment.ID)) != nil {
			return errors.New("comment ID already exists")
		}
		return bucket.Put([]byte(comment.ID), data)
	})
	return comment, err
}

func (s Store) List(repository string) ([]review.Comment, error) {
	var comments []review.Comment
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

func (s Store) Update(comment review.Comment) error {
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
	return s.update(func(bucket *bolt.Bucket) error {
		cursor := bucket.Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			stored, err := decodeComment(value)
			if err != nil {
				return err
			}
			if stored.Repository != comment.Repository || stored.ID != comment.ID {
				continue
			}
			oldKey := append([]byte(nil), key...)
			if !bytes.Equal(oldKey, []byte(comment.ID)) {
				if err := bucket.Delete(oldKey); err != nil {
					return err
				}
			}
			return bucket.Put([]byte(comment.ID), data)
		}
		return errors.New("comment is no longer in the inbox")
	})
}

func (s Store) Delete(repository, id string) error {
	if repository == "" || id == "" {
		return errors.New("repository and comment ID are required")
	}
	return s.update(func(bucket *bolt.Bucket) error {
		cursor := bucket.Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			comment, err := decodeComment(value)
			if err != nil {
				return err
			}
			if comment.Repository != repository || comment.ID != id {
				continue
			}
			return bucket.Delete(key)
		}
		return errors.New("comment is no longer in the inbox")
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

func validateComment(comment review.Comment) error {
	if len(comment.Body) == 0 {
		return errors.New("comment body is empty")
	}
	if len(comment.Body) > maxCommentBytes {
		return fmt.Errorf("comment exceeds %d bytes", maxCommentBytes)
	}
	return nil
}

func decodeComment(data []byte) (review.Comment, error) {
	var legacy struct {
		ID         string    `json:"id"`
		Repository string    `json:"repository"`
		CreatedAt  time.Time `json:"created_at"`
		Comment    *struct {
			Anchor review.Anchor `json:"anchor"`
			Body   string        `json:"body"`
		} `json:"comment"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return review.Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	if legacy.Comment != nil {
		return review.Comment{ID: legacy.ID, Repository: legacy.Repository, CreatedAt: legacy.CreatedAt, Anchor: legacy.Comment.Anchor, Body: legacy.Comment.Body}, nil
	}
	var comment review.Comment
	if err := json.Unmarshal(data, &comment); err != nil {
		return review.Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	return comment, nil
}

func (s Store) SideBySide() (bool, error) {
	var enabled bool
	err := s.viewBucket(settingsBucket, func(bucket *bolt.Bucket) error {
		enabled = bytes.Equal(bucket.Get(sideBySideKey), enabledSetting)
		return nil
	})
	return enabled, err
}

func (s Store) SetSideBySide(enabled bool) error {
	value := disabledSetting
	if enabled {
		value = enabledSetting
	}
	return s.updateBucket(settingsBucket, func(bucket *bolt.Bucket) error {
		return bucket.Put(sideBySideKey, value)
	})
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
		return nil, errors.New("inbox path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return nil, fmt.Errorf("create inbox directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(s.Path), 0o700); err != nil {
		return nil, fmt.Errorf("secure inbox directory: %w", err)
	}
	db, err := bolt.Open(s.Path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open inbox: %w", err)
	}
	if err := os.Chmod(s.Path, 0o600); err != nil {
		db.Close()
		return nil, fmt.Errorf("secure inbox database: %w", err)
	}
	return db, nil
}
