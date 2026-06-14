package inbox

import (
	"encoding/binary"
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

var messagesBucket = []byte("messages")

type Store struct {
	Path string
}

type Message struct {
	ID              string         `json:"id"`
	Repository      string         `json:"repository"`
	DiffFingerprint string         `json:"diff_fingerprint"`
	CreatedAt       time.Time      `json:"created_at"`
	Comment         review.Comment `json:"comment"`
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

func (s Store) Put(message Message) error {
	if message.Repository == "" {
		return errors.New("message requires a repository")
	}
	if len(message.Comment.Body) == 0 {
		return errors.New("comment body is empty")
	}
	if len(message.Comment.Body) > maxCommentBytes {
		return fmt.Errorf("comment exceeds %d bytes", maxCommentBytes)
	}
	if message.ID == "" {
		message.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now().UTC()
	}
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode inbox message: %w", err)
	}
	return s.update(func(bucket *bolt.Bucket) error {
		var pending int
		cursor := bucket.Cursor()
		for _, value := cursor.First(); value != nil; _, value = cursor.Next() {
			pending += len(value)
		}
		if pending+len(data) > maxPendingBytes {
			return fmt.Errorf("pending feedback exceeds %d bytes", maxPendingBytes)
		}
		key, err := bucket.NextSequence()
		if err != nil {
			return err
		}
		var encoded [8]byte
		binary.BigEndian.PutUint64(encoded[:], key)
		return bucket.Put(encoded[:], data)
	})
}

func (s Store) ListComments(repository string) ([]review.StoredComment, error) {
	taken, err := s.Peek(repository)
	if err != nil {
		return nil, err
	}
	comments := make([]review.StoredComment, 0, len(taken.Messages))
	for _, message := range taken.Messages {
		comments = append(comments, review.StoredComment{
			ID:      message.ID,
			Comment: message.Comment,
		})
	}
	return comments, nil
}

func (s Store) UpdateComment(repository string, stored review.StoredComment) error {
	if repository == "" || stored.ID == "" {
		return errors.New("repository and message ID are required")
	}
	if len(stored.Comment.Body) == 0 {
		return errors.New("comment body is empty")
	}
	if len(stored.Comment.Body) > maxCommentBytes {
		return fmt.Errorf("comment exceeds %d bytes", maxCommentBytes)
	}
	return s.update(func(bucket *bolt.Bucket) error {
		cursor := bucket.Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			var message Message
			if err := json.Unmarshal(value, &message); err != nil {
				return fmt.Errorf("decode inbox message: %w", err)
			}
			if message.Repository != repository || message.ID != stored.ID {
				continue
			}
			message.Comment = stored.Comment
			data, err := json.Marshal(message)
			if err != nil {
				return fmt.Errorf("encode inbox message: %w", err)
			}
			return bucket.Put(key, data)
		}
		return errors.New("comment is no longer in the inbox")
	})
}

func (s Store) DeleteComment(repository string, stored review.StoredComment) error {
	if repository == "" || stored.ID == "" {
		return errors.New("repository and message ID are required")
	}
	return s.update(func(bucket *bolt.Bucket) error {
		cursor := bucket.Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			var message Message
			if err := json.Unmarshal(value, &message); err != nil {
				return fmt.Errorf("decode inbox message: %w", err)
			}
			if message.Repository != repository || message.ID != stored.ID {
				continue
			}
			return bucket.Delete(key)
		}
		return errors.New("comment is no longer in the inbox")
	})
}

type Taken struct {
	Messages []Message
	keys     [][]byte
}

func (s Store) Peek(repository string) (Taken, error) {
	var result Taken
	err := s.view(func(bucket *bolt.Bucket) error {
		return bucket.ForEach(func(key, value []byte) error {
			var message Message
			if err := json.Unmarshal(value, &message); err != nil {
				return fmt.Errorf("decode inbox message: %w", err)
			}
			if message.Repository != repository {
				return nil
			}
			result.Messages = append(result.Messages, message)
			result.keys = append(result.keys, append([]byte(nil), key...))
			return nil
		})
	})
	return result, err
}

func (s Store) Delete(taken Taken) error {
	if len(taken.keys) == 0 {
		return nil
	}
	return s.update(func(bucket *bolt.Bucket) error {
		for _, key := range taken.keys {
			if err := bucket.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s Store) update(fn func(*bolt.Bucket) error) error {
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
