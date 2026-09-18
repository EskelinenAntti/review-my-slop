package comments

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	maxCommentBytes = 64 << 10
	maxPendingBytes = 16 << 20
	databaseName    = "inbox-v2.db"
)

var (
	commentsBucket = []byte("comments")
	settingsBucket = []byte("settings")
	sideBySideKey  = []byte("side-by-side")
	enabledValue   = []byte{1}
	disabledValue  = []byte{0}
)

// Store is a small bbolt-backed inbox.
type Store struct {
	path string
}

func NewStore(path string) Store { return Store{path: path} }

func (s Store) Path() string { return s.path }

// DefaultPath resolves the XDG data directory and the fresh v2 database name.
func DefaultPath() (string, error) {
	root := os.Getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(root) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home directory: %w", err)
		}
		root = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(root, "review-my-slop", databaseName), nil
}

func OpenDefault() (Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return Store{}, err
	}
	return NewStore(path), nil
}

func (s Store) Add(comment Comment) (Comment, error) {
	if err := validateComment(comment); err != nil {
		return Comment{}, err
	}
	if comment.CreatedAt.IsZero() {
		comment.CreatedAt = time.Now().UTC()
	}
	generatedID := comment.ID == ""
	if generatedID {
		comment.ID = strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	err := s.update(func(bucket *bolt.Bucket) error {
		for sequence := int64(0); ; sequence++ {
			if bucket.Get([]byte(comment.ID)) != nil {
				if !generatedID {
					return errors.New("comment ID already exists")
				}
				comment.ID = fmt.Sprintf("%s-%d", comment.ID, sequence+1)
				continue
			}
			data, marshalErr := json.Marshal(comment)
			if marshalErr != nil {
				return fmt.Errorf("encode comment: %w", marshalErr)
			}
			if err := pendingLimit(bucket, len(data)); err != nil {
				return err
			}
			return bucket.Put([]byte(comment.ID), data)
		}
	})
	return comment, err
}

func (s Store) List(repository string) ([]Comment, error) {
	snapshot, err := s.Snapshot(repository)
	if err != nil {
		return nil, err
	}
	return snapshot.Comments(), nil
}

// Snapshot reads pending comments and retains their exact encoded values for a
// later conditional acknowledgement.
func (s Store) Snapshot(repository string) (Snapshot, error) {
	snapshot := Snapshot{repository: repository}
	err := s.view(func(bucket *bolt.Bucket) error {
		return bucket.ForEach(func(key, value []byte) error {
			comment, err := decodeComment(value)
			if err != nil {
				return err
			}
			if comment.Repository != repository {
				return nil
			}
			snapshot.items = append(snapshot.items, snapshotItem{
				comment: cloneComment(comment),
				encoded: append([]byte(nil), value...),
			})
			return nil
		})
	})
	if err != nil {
		return Snapshot{}, err
	}
	sort.SliceStable(snapshot.items, func(i, j int) bool {
		left, right := snapshot.items[i].comment, snapshot.items[j].comment
		if left.CreatedAt.Equal(right.CreatedAt) {
			return left.ID < right.ID
		}
		return left.CreatedAt.Before(right.CreatedAt)
	})
	return snapshot, nil
}

// Acknowledge removes only unchanged comments from snapshot.
func (s Store) Acknowledge(snapshot Snapshot) error {
	if len(snapshot.items) == 0 {
		return nil
	}
	return s.update(func(bucket *bolt.Bucket) error {
		for _, item := range snapshot.items {
			current := bucket.Get([]byte(item.comment.ID))
			if !bytes.Equal(current, item.encoded) {
				continue
			}
			if err := bucket.Delete([]byte(item.comment.ID)); err != nil {
				return err
			}
		}
		return nil
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
	return s.update(func(bucket *bolt.Bucket) error {
		old := bucket.Get([]byte(comment.ID))
		if old == nil {
			return errors.New("comment is no longer in the inbox")
		}
		var stored Comment
		if err := json.Unmarshal(old, &stored); err != nil {
			return fmt.Errorf("decode comment: %w", err)
		}
		if stored.Repository != comment.Repository {
			return errors.New("comment is no longer in the inbox")
		}
		if err := pendingLimitForUpdate(bucket, len(old), len(data)); err != nil {
			return err
		}
		return bucket.Put([]byte(comment.ID), data)
	})
}

func (s Store) Delete(repository, id string) error {
	if repository == "" || id == "" {
		return errors.New("repository and comment ID are required")
	}
	return s.update(func(bucket *bolt.Bucket) error {
		value := bucket.Get([]byte(id))
		if value == nil {
			return errors.New("comment is no longer in the inbox")
		}
		var comment Comment
		if err := json.Unmarshal(value, &comment); err != nil {
			return fmt.Errorf("decode comment: %w", err)
		}
		if comment.Repository != repository {
			return errors.New("comment is no longer in the inbox")
		}
		return bucket.Delete([]byte(id))
	})
}

func (s Store) SideBySide() (bool, error) {
	var enabled bool
	err := s.viewBucket(settingsBucket, func(bucket *bolt.Bucket) error {
		enabled = bytes.Equal(bucket.Get(sideBySideKey), enabledValue)
		return nil
	})
	return enabled, err
}

func (s Store) SetSideBySide(enabled bool) error {
	value := disabledValue
	if enabled {
		value = enabledValue
	}
	return s.updateBucket(settingsBucket, func(bucket *bolt.Bucket) error {
		return bucket.Put(sideBySideKey, value)
	})
}

func validateComment(comment Comment) error {
	if comment.Repository == "" {
		return errors.New("comment requires a repository")
	}
	if len(comment.Body) == 0 {
		return errors.New("comment body is empty")
	}
	if len(comment.Body) > maxCommentBytes {
		return fmt.Errorf("comment exceeds %d bytes", maxCommentBytes)
	}
	return nil
}

func decodeComment(data []byte) (Comment, error) {
	var comment Comment
	if err := json.Unmarshal(data, &comment); err != nil {
		return Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	if err := validateComment(comment); err != nil {
		return Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	return comment, nil
}

func pendingLimit(bucket *bolt.Bucket, added int) error {
	var pending int
	if err := bucket.ForEach(func(_, value []byte) error {
		pending += len(value)
		return nil
	}); err != nil {
		return err
	}
	if pending+added > maxPendingBytes {
		return fmt.Errorf("pending feedback exceeds %d bytes", maxPendingBytes)
	}
	return nil
}

func pendingLimitForUpdate(bucket *bolt.Bucket, oldSize, newSize int) error {
	var pending int
	if err := bucket.ForEach(func(_, value []byte) error {
		pending += len(value)
		return nil
	}); err != nil {
		return err
	}
	if pending-oldSize+newSize > maxPendingBytes {
		return fmt.Errorf("pending feedback exceeds %d bytes", maxPendingBytes)
	}
	return nil
}

func (s Store) update(fn func(*bolt.Bucket) error) error {
	return s.updateBucket(commentsBucket, fn)
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
	return s.viewBucket(commentsBucket, fn)
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
	if s.path == "" {
		return nil, errors.New("inbox path is empty")
	}
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create inbox directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return nil, fmt.Errorf("secure inbox directory: %w", err)
	}
	db, err := bolt.Open(s.path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open inbox: %w", err)
	}
	if err := os.Chmod(s.path, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("secure inbox database: %w", err)
	}
	return db, nil
}
