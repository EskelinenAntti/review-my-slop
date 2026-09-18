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
	messagesBucket  = "messages"
	settingsBucket  = "settings"
	sideBySideKey   = "side-by-side"
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

func (s Store) Add(comment review.Comment) (review.Comment, error) {
	if comment.Repository == "" {
		return review.Comment{}, errors.New("comment requires a repository")
	}
	comment, data, err := prepareComment(comment)
	if err != nil {
		return review.Comment{}, err
	}
	err = s.updateMessages(func(bucket *bolt.Bucket) error {
		if pendingBytes(bucket)+len(data) > maxPendingBytes {
			return fmt.Errorf("pending feedback exceeds %d bytes", maxPendingBytes)
		}
		if key, err := findCommentKey(bucket, comment.Repository, comment.ID); err != nil {
			return err
		} else if key != nil || bucket.Get([]byte(comment.ID)) != nil {
			return errors.New("comment ID already exists")
		}
		return bucket.Put([]byte(comment.ID), data)
	})
	return comment, err
}

func (s Store) List(repository string) ([]review.Comment, error) {
	var comments []review.Comment
	err := s.viewMessages(func(bucket *bolt.Bucket) error {
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
	data, err := encodeComment(comment)
	if err != nil {
		return err
	}
	return s.updateMessages(func(bucket *bolt.Bucket) error {
		key, err := findCommentKey(bucket, comment.Repository, comment.ID)
		if err != nil {
			return err
		}
		if key == nil {
			return errors.New("comment is no longer in the inbox")
		}
		if !bytes.Equal(key, []byte(comment.ID)) {
			if err := bucket.Delete(key); err != nil {
				return err
			}
		}
		return bucket.Put([]byte(comment.ID), data)
	})
}

func (s Store) Delete(repository, id string) error {
	if repository == "" || id == "" {
		return errors.New("repository and comment ID are required")
	}
	return s.updateMessages(func(bucket *bolt.Bucket) error {
		key, err := findCommentKey(bucket, repository, id)
		if err != nil {
			return err
		}
		if key == nil {
			return errors.New("comment is no longer in the inbox")
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
	return s.updateMessages(func(bucket *bolt.Bucket) error {
		keys, err := findCommentKeys(bucket, repository, wanted)
		if err != nil {
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

func prepareComment(comment review.Comment) (review.Comment, []byte, error) {
	if err := validateComment(comment); err != nil {
		return review.Comment{}, nil, err
	}
	if comment.ID == "" {
		comment.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if comment.CreatedAt.IsZero() {
		comment.CreatedAt = time.Now().UTC()
	}
	data, err := encodeComment(comment)
	if err != nil {
		return review.Comment{}, nil, err
	}
	return comment, data, nil
}

func encodeComment(comment review.Comment) ([]byte, error) {
	data, err := json.Marshal(comment)
	if err != nil {
		return nil, fmt.Errorf("encode comment: %w", err)
	}
	return data, nil
}

func pendingBytes(bucket *bolt.Bucket) int {
	var total int
	cursor := bucket.Cursor()
	for _, value := cursor.First(); value != nil; _, value = cursor.Next() {
		total += len(value)
	}
	return total
}

func findCommentKey(bucket *bolt.Bucket, repository, id string) ([]byte, error) {
	wanted := map[string]struct{}{id: {}}
	keys, err := findCommentKeys(bucket, repository, wanted)
	if len(keys) == 0 {
		return nil, err
	}
	return keys[0], err
}

func findCommentKeys(bucket *bolt.Bucket, repository string, wanted map[string]struct{}) ([][]byte, error) {
	var matches [][]byte
	err := bucket.ForEach(func(key, value []byte) error {
		comment, err := decodeComment(value)
		if err != nil {
			return err
		}
		if comment.Repository != repository {
			return nil
		}
		if _, ok := wanted[comment.ID]; ok {
			matches = append(matches, append([]byte(nil), key...))
		}
		return nil
	})
	return matches, err
}

func decodeComment(data []byte) (review.Comment, error) {
	var envelope struct {
		Comment json.RawMessage `json:"comment"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return review.Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	if len(envelope.Comment) > 0 && string(envelope.Comment) != "null" {
		return decodeLegacyComment(data)
	}
	var comment review.Comment
	if err := json.Unmarshal(data, &comment); err != nil {
		return review.Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	return comment, nil
}

func decodeLegacyComment(data []byte) (review.Comment, error) {
	var legacy legacyComment
	if err := json.Unmarshal(data, &legacy); err != nil {
		return review.Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	return legacy.toComment(), nil
}

type legacyComment struct {
	ID         string        `json:"id"`
	Repository string        `json:"repository"`
	CreatedAt  time.Time     `json:"created_at"`
	Comment    legacyDetails `json:"comment"`
}

type legacyDetails struct {
	Anchor review.Anchor `json:"anchor"`
	Body   string        `json:"body"`
}

func (legacy legacyComment) toComment() review.Comment {
	return review.Comment{
		ID:         legacy.ID,
		Repository: legacy.Repository,
		CreatedAt:  legacy.CreatedAt,
		Anchor:     legacy.Comment.Anchor,
		Body:       legacy.Comment.Body,
	}
}

func (s Store) SideBySideEnabled() (bool, error) {
	var enabled bool
	err := s.viewBucket(settingsBucket, func(bucket *bolt.Bucket) error {
		value := bucket.Get([]byte(sideBySideKey))
		enabled = len(value) == 1 && value[0] == 1
		return nil
	})
	return enabled, err
}

func (s Store) SetSideBySide(enabled bool) error {
	value := byte(0)
	if enabled {
		value = 1
	}
	return s.updateBucket(settingsBucket, func(bucket *bolt.Bucket) error {
		return bucket.Put([]byte(sideBySideKey), []byte{value})
	})
}

func (s Store) updateMessages(fn func(*bolt.Bucket) error) error {
	return s.updateBucket(messagesBucket, fn)
}

func (s Store) updateBucket(name string, fn func(*bolt.Bucket) error) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(name))
		if err != nil {
			return err
		}
		return fn(bucket)
	})
}

func (s Store) viewMessages(fn func(*bolt.Bucket) error) error {
	return s.viewBucket(messagesBucket, fn)
}

func (s Store) viewBucket(name string, fn func(*bolt.Bucket) error) error {
	db, err := s.open()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer db.Close()
	return db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(name))
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
	directory := filepath.Dir(s.Path)
	if err := ensureInboxDirectory(directory); err != nil {
		return nil, err
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

func ensureInboxDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create inbox directory: %w", err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("secure inbox directory: %w", err)
	}
	return nil
}
