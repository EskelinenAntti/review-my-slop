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

	"github.com/anttieskelinen/review-my-slop/internal/review"
)

const (
	maxCommentBytes = 64 << 10
	maxBatchBytes   = 1 << 20
	maxPendingBytes = 16 << 20
)

var batchesBucket = []byte("batches")

type Store struct {
	Path string
}

func DefaultPath() (string, error) {
	if home := os.Getenv("REVIEW_MY_SLOP_HOME"); home != "" {
		return filepath.Join(home, "inbox.db"), nil
	}
	state, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(state, "review-my-slop", "inbox.db"), nil
}

func OpenDefault() (Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return Store{}, err
	}
	return Store{Path: path}, nil
}

func (s Store) Put(batch review.Batch) error {
	if batch.Repository == "" || len(batch.Comments) == 0 {
		return errors.New("batch requires a repository and at least one comment")
	}
	for _, comment := range batch.Comments {
		if len(comment.Body) == 0 {
			return errors.New("comment body is empty")
		}
		if len(comment.Body) > maxCommentBytes {
			return fmt.Errorf("comment exceeds %d bytes", maxCommentBytes)
		}
	}
	if batch.ID == "" {
		batch.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if batch.CreatedAt.IsZero() {
		batch.CreatedAt = time.Now().UTC()
	}
	data, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("encode feedback batch: %w", err)
	}
	if len(data) > maxBatchBytes {
		return fmt.Errorf("feedback batch exceeds %d bytes", maxBatchBytes)
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
	var comments []review.StoredComment
	for _, batch := range taken.Batches {
		for index, comment := range batch.Comments {
			comments = append(comments, review.StoredComment{
				BatchID: batch.ID,
				Index:   index,
				Comment: comment,
			})
		}
	}
	return comments, nil
}

func (s Store) UpdateComment(repository string, stored review.StoredComment) error {
	if repository == "" || stored.BatchID == "" {
		return errors.New("repository and batch ID are required")
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
			var batch review.Batch
			if err := json.Unmarshal(value, &batch); err != nil {
				return fmt.Errorf("decode feedback batch: %w", err)
			}
			if batch.Repository != repository || batch.ID != stored.BatchID {
				continue
			}
			if stored.Index < 0 || stored.Index >= len(batch.Comments) {
				return fmt.Errorf("comment index %d is out of range", stored.Index)
			}
			batch.Comments[stored.Index] = stored.Comment
			data, err := json.Marshal(batch)
			if err != nil {
				return fmt.Errorf("encode feedback batch: %w", err)
			}
			if len(data) > maxBatchBytes {
				return fmt.Errorf("feedback batch exceeds %d bytes", maxBatchBytes)
			}
			return bucket.Put(key, data)
		}
		return errors.New("comment is no longer in the inbox")
	})
}

func (s Store) DeleteComment(repository string, stored review.StoredComment) error {
	if repository == "" || stored.BatchID == "" {
		return errors.New("repository and batch ID are required")
	}
	return s.update(func(bucket *bolt.Bucket) error {
		cursor := bucket.Cursor()
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			var batch review.Batch
			if err := json.Unmarshal(value, &batch); err != nil {
				return fmt.Errorf("decode feedback batch: %w", err)
			}
			if batch.Repository != repository || batch.ID != stored.BatchID {
				continue
			}
			if stored.Index < 0 || stored.Index >= len(batch.Comments) {
				return fmt.Errorf("comment index %d is out of range", stored.Index)
			}
			batch.Comments = append(batch.Comments[:stored.Index], batch.Comments[stored.Index+1:]...)
			if len(batch.Comments) == 0 {
				return bucket.Delete(key)
			}
			data, err := json.Marshal(batch)
			if err != nil {
				return fmt.Errorf("encode feedback batch: %w", err)
			}
			return bucket.Put(key, data)
		}
		return errors.New("comment is no longer in the inbox")
	})
}

type Taken struct {
	Batches []review.Batch
	keys    [][]byte
}

func (s Store) Peek(repository string) (Taken, error) {
	var result Taken
	err := s.view(func(bucket *bolt.Bucket) error {
		return bucket.ForEach(func(key, value []byte) error {
			var batch review.Batch
			if err := json.Unmarshal(value, &batch); err != nil {
				return fmt.Errorf("decode feedback batch: %w", err)
			}
			if batch.Repository != repository {
				return nil
			}
			result.Batches = append(result.Batches, batch)
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
		bucket, err := tx.CreateBucketIfNotExists(batchesBucket)
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
		bucket := tx.Bucket(batchesBucket)
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
