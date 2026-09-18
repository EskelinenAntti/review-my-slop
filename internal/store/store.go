package store

import (
	"bytes"
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
	data, err := encodeComment(item)
	if err != nil {
		return comment.Comment{}, err
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
	data, err := encodeComment(item)
	if err != nil {
		return err
	}
	return s.updateMessages(func(bucket *bbolt.Bucket) error {
		key := []byte(item.ID)
		stored := bucket.Get(key)
		if stored == nil {
			return errors.New("comment is no longer in the inbox")
		}
		current, err := decodeComment(stored)
		if err != nil {
			return err
		}
		if current.Repository != item.Repository {
			return errors.New("comment is no longer in the inbox")
		}
		return bucket.Put(key, data)
	})
}

func (s Store) Delete(repository, id string) error {
	if repository == "" || id == "" {
		return errors.New("repository and comment ID are required")
	}
	return s.updateMessages(func(bucket *bbolt.Bucket) error {
		key := []byte(id)
		stored := bucket.Get(key)
		if stored == nil {
			return errors.New("comment is no longer in the inbox")
		}
		item, err := decodeComment(stored)
		if err != nil {
			return err
		}
		if item.Repository != repository {
			return errors.New("comment is no longer in the inbox")
		}
		return bucket.Delete(key)
	})
}

func (s Store) Acknowledge(repository string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return s.updateMessages(func(bucket *bbolt.Bucket) error {
		for _, id := range ids {
			key := []byte(id)
			value := bucket.Get(key)
			if value == nil {
				continue
			}
			item, err := decodeComment(value)
			if err != nil {
				return err
			}
			if item.Repository != repository {
				continue
			}
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

func pendingBytes(bucket *bbolt.Bucket) int {
	total := 0
	cursor := bucket.Cursor()
	for _, value := cursor.First(); value != nil; _, value = cursor.Next() {
		total += len(value)
	}
	return total
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
