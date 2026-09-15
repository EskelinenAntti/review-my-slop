package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bbolt "go.etcd.io/bbolt"

	"github.com/eskelinenantti/review-my-slop/internal/comment"
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

func OpenDefault() (Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return Store{}, err
	}
	return Store{Path: path}, nil
}

func (s Store) Add(item comment.Comment) (comment.Comment, error) {
	if item.Repository == "" {
		return comment.Comment{}, errors.New("comment requires a repository")
	}
	if err := validateComment(item); err != nil {
		return comment.Comment{}, err
	}
	if item.ID == "" {
		item.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	}
	data, err := json.Marshal(item)
	if err != nil {
		return comment.Comment{}, fmt.Errorf("encode comment: %w", err)
	}
	err = s.updateMessages(func(bucket *bbolt.Bucket) error {
		pending := pendingBytes(bucket)
		if pending+len(data) > maxPendingBytes {
			return fmt.Errorf("pending feedback exceeds %d bytes", maxPendingBytes)
		}
		if bucket.Get([]byte(item.ID)) != nil {
			return errors.New("comment ID already exists")
		}
		return bucket.Put([]byte(item.ID), data)
	})
	return item, err
}

func (s Store) List(repository string) ([]comment.Comment, error) {
	var result []comment.Comment
	err := s.viewMessages(func(bucket *bbolt.Bucket) error {
		return bucket.ForEach(func(_, value []byte) error {
			item, err := decodeComment(value)
			if err != nil {
				return err
			}
			if item.Repository == repository {
				result = append(result, item)
			}
			return nil
		})
	})
	return result, err
}

func (s Store) Update(item comment.Comment) error {
	if item.Repository == "" || item.ID == "" {
		return errors.New("repository and comment ID are required")
	}
	if err := validateComment(item); err != nil {
		return err
	}
	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("encode comment: %w", err)
	}
	return s.updateMessages(func(bucket *bbolt.Bucket) error {
		oldKey, found, err := findComment(bucket, item.Repository, item.ID)
		if err != nil {
			return err
		}
		if !found {
			return errors.New("comment is no longer in the inbox")
		}
		newKey := []byte(item.ID)
		if !bytes.Equal(oldKey, newKey) {
			if bucket.Get(newKey) != nil {
				return errors.New("comment ID already exists")
			}
			if err := bucket.Delete(oldKey); err != nil {
				return err
			}
		}
		return bucket.Put(newKey, data)
	})
}

func (s Store) Delete(repository, id string) error {
	if repository == "" || id == "" {
		return errors.New("repository and comment ID are required")
	}
	return s.updateMessages(func(bucket *bbolt.Bucket) error {
		key, found, err := findComment(bucket, repository, id)
		if err != nil {
			return err
		}
		if found {
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
	return s.updateMessages(func(bucket *bbolt.Bucket) error {
		var keys [][]byte
		if err := bucket.ForEach(func(key, value []byte) error {
			item, err := decodeComment(value)
			if err != nil {
				return err
			}
			if item.Repository == repository {
				if _, ok := wanted[item.ID]; ok {
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

func (s Store) SideBySide() (bool, error) {
	var enabled bool
	err := s.viewBucket(settingsBucket, func(bucket *bbolt.Bucket) error {
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
	return s.updateBucket(settingsBucket, func(bucket *bbolt.Bucket) error {
		return bucket.Put(sideBySideKey, value)
	})
}

func validateComment(item comment.Comment) error {
	if len(item.Body) == 0 {
		return errors.New("comment body is empty")
	}
	if len(item.Body) > maxCommentBytes {
		return fmt.Errorf("comment exceeds %d bytes", maxCommentBytes)
	}
	return nil
}

func decodeComment(data []byte) (comment.Comment, error) {
	var legacy struct {
		ID         string    `json:"id"`
		Repository string    `json:"repository"`
		CreatedAt  time.Time `json:"created_at"`
		Comment    *struct {
			Anchor comment.Anchor `json:"anchor"`
			Body   string         `json:"body"`
		} `json:"comment"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return comment.Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	if legacy.Comment != nil {
		return comment.Comment{ID: legacy.ID, Repository: legacy.Repository, CreatedAt: legacy.CreatedAt, Anchor: legacy.Comment.Anchor, Body: legacy.Comment.Body}, nil
	}
	var item comment.Comment
	if err := json.Unmarshal(data, &item); err != nil {
		return comment.Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	return item, nil
}

func pendingBytes(bucket *bbolt.Bucket) int {
	total := 0
	cursor := bucket.Cursor()
	for _, value := cursor.First(); value != nil; _, value = cursor.Next() {
		total += len(value)
	}
	return total
}

func findComment(bucket *bbolt.Bucket, repository, id string) ([]byte, bool, error) {
	cursor := bucket.Cursor()
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		item, err := decodeComment(value)
		if err != nil {
			return nil, false, err
		}
		if item.Repository == repository && item.ID == id {
			return append([]byte(nil), key...), true, nil
		}
	}
	return nil, false, nil
}

func (s Store) updateMessages(fn func(*bbolt.Bucket) error) error {
	return s.updateBucket(messagesBucket, fn)
}

func (s Store) updateBucket(name []byte, fn func(*bbolt.Bucket) error) error {
	database, err := s.openDatabase()
	if err != nil {
		return err
	}
	defer database.Close()
	return database.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(name)
		if err != nil {
			return err
		}
		return fn(bucket)
	})
}

func (s Store) viewMessages(fn func(*bbolt.Bucket) error) error {
	return s.viewBucket(messagesBucket, fn)
}

func (s Store) viewBucket(name []byte, fn func(*bbolt.Bucket) error) error {
	database, err := s.openDatabase()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer database.Close()
	return database.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(name)
		if bucket == nil {
			return nil
		}
		return fn(bucket)
	})
}

func (s Store) openDatabase() (*bbolt.DB, error) {
	if s.Path == "" {
		return nil, errors.New("inbox path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return nil, fmt.Errorf("create inbox directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(s.Path), 0o700); err != nil {
		return nil, fmt.Errorf("secure inbox directory: %w", err)
	}
	database, err := bbolt.Open(s.Path, 0o600, &bbolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open inbox: %w", err)
	}
	if err := os.Chmod(s.Path, 0o600); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("secure inbox database: %w", err)
	}
	return database, nil
}
