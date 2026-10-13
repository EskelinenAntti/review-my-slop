package comments

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	maxCommentBytes = 64 << 10
	maxPendingBytes = 16 << 20
)

var commentsBucket = []byte("messages")

// Store persists pending review comments in a database at Path.
type Store struct {
	Path string
}

// DefaultPath returns the database path in the application data directory.
func DefaultPath() (string, error) {
	data, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(data, "comments.db"), nil
}

// OpenDefault returns a Store configured with the default database path.
func OpenDefault() (Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return Store{}, err
	}
	return Store{Path: path}, nil
}

// Add validates and stores a comment, assigning an ID and timestamp when
// absent.
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
	err = s.withWriteBucket(func(bucket *bolt.Bucket) error {
		return addComment(bucket, comment.ID, data)
	})
	return comment, err
}

// List returns pending comments for the repository.
func (s Store) List(repository string) ([]Comment, error) {
	var comments []Comment
	err := s.withReadBucket(func(bucket *bolt.Bucket) error {
		return bucket.ForEach(func(_, value []byte) error {
			comment, err := decodeCommentRecord(value)
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

// Update replaces an existing comment in the repository.
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
	return s.withWriteBucket(
		func(bucket *bolt.Bucket) error {
			return replaceComment(bucket, comment, data)
		},
	)
}

// Delete removes the identified comment from the repository.
func (s Store) Delete(repository, id string) error {
	if repository == "" || id == "" {
		return errors.New("repository and comment ID are required")
	}
	return s.withWriteBucket(func(bucket *bolt.Bucket) error {
		key, err := findCommentKey(bucket, repository, id)
		if err != nil {
			return err
		}
		if key == nil {
			return errors.New("comment is no longer in the comments")
		}
		return bucket.Delete(key)
	})
}

// Acknowledge removes the identified comments after their feedback is consumed.
func (s Store) Acknowledge(repository string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	return s.withWriteBucket(func(bucket *bolt.Bucket) error {
		keys, err := findPendingCommentKeys(bucket, repository, wanted)
		if err != nil {
			return err
		}
		return deleteCommentKeys(bucket, keys)
	})
}
