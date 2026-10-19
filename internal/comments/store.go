package comments

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/eskelinenantti/review-my-slop/internal/appinfo"
)

const (
	maxCommentBytes = 64 << 10
	maxPendingBytes = 16 << 20
)

// Store persists pending review comments in a database at Path.
type Store struct {
	Path string
}

// DefaultPath returns the database path in the application data directory.
func DefaultPath() (string, error) {
	data, err := appinfo.DataDir()
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
	now := time.Now()
	if comment.ID == "" {
		comment.ID = fmt.Sprintf("%d", now.UnixNano())
	}
	if comment.CreatedAt.IsZero() {
		comment.CreatedAt = now.UTC()
	}
	data, err := encodeComment(comment)
	if err != nil {
		return Comment{}, err
	}
	err = s.transact(func(bucket *bolt.Bucket) error {
		return addComment(bucket, comment.ID, data)
	})
	return comment, err
}

func addComment(bucket *bolt.Bucket, id string, data []byte) error {
	if bucket.Get([]byte(id)) != nil {
		return errors.New("comment ID already exists")
	}
	if err := checkPendingSize(bucket, 0, len(data)); err != nil {
		return err
	}
	return bucket.Put([]byte(id), data)
}

// List returns pending comments for the repository.
func (s Store) List(repository string) ([]Comment, error) {
	var pendingComments []Comment
	err := s.view(func(bucket *bolt.Bucket) error {
		visit := func(_ []byte, comment Comment) bool {
			pendingComments = append(pendingComments, comment)
			return true
		}
		return forEachRepositoryComment(bucket, repository, visit)
	})
	return pendingComments, err
}

// Update replaces an existing comment in the repository.
func (s Store) Update(comment Comment) error {
	err := validateCommentIdentity(comment.Repository, comment.ID)
	if err != nil {
		return err
	}
	data, err := encodeComment(comment)
	if err != nil {
		return err
	}
	return s.transact(
		func(bucket *bolt.Bucket) error {
			return replaceComment(bucket, comment, data)
		},
	)
}

// Delete removes the identified comment from the repository.
func (s Store) Delete(repository, id string) error {
	if err := validateCommentIdentity(repository, id); err != nil {
		return err
	}
	return s.transact(func(bucket *bolt.Bucket) error {
		key, err := findCommentKey(bucket, repository, id)
		if err != nil {
			return err
		}
		return bucket.Delete(key)
	})
}

// Acknowledge removes the identified comments after their feedback is consumed.
func (s Store) Acknowledge(repository string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	wantedIDs := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wantedIDs[id] = struct{}{}
	}
	return s.transact(func(bucket *bolt.Bucket) error {
		keys, err := findPendingKeys(bucket, repository, wantedIDs)
		if err != nil {
			return err
		}
		return deleteKeys(bucket, keys)
	})
}

func deleteKeys(bucket *bolt.Bucket, keys [][]byte) error {
	for _, key := range keys {
		if err := bucket.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

func pendingBytes(bucket *bolt.Bucket) int {
	var total int
	cursor := bucket.Cursor()
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		total += len(value)
	}
	return total
}

func checkPendingSize(
	bucket *bolt.Bucket,
	replacedBytes, addedBytes int,
) error {
	if pendingBytes(bucket)-replacedBytes+addedBytes > maxPendingBytes {
		return fmt.Errorf("pending feedback exceeds %d bytes", maxPendingBytes)
	}
	return nil
}

func replaceComment(bucket *bolt.Bucket, comment Comment, data []byte) error {
	key, err := findCommentKey(bucket, comment.Repository, comment.ID)
	if err != nil {
		return err
	}
	oldSize := len(bucket.Get(key))
	if err := checkPendingSize(bucket, oldSize, len(data)); err != nil {
		return err
	}
	if !bytes.Equal(key, []byte(comment.ID)) {
		if err := bucket.Delete(key); err != nil {
			return err
		}
	}
	return bucket.Put([]byte(comment.ID), data)
}

func findCommentKey(
	bucket *bolt.Bucket,
	repository, id string,
) ([]byte, error) {
	var found []byte
	visit := func(key []byte, comment Comment) bool {
		if comment.ID == id {
			found = bytes.Clone(key)
			return false
		}
		return true
	}
	err := forEachRepositoryComment(bucket, repository, visit)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, errors.New("comment is no longer in the comments")
	}
	return found, nil
}

func findPendingKeys(
	bucket *bolt.Bucket,
	repository string,
	wantedIDs map[string]struct{},
) ([][]byte, error) {
	var keys [][]byte
	visit := func(key []byte, comment Comment) bool {
		if _, ok := wantedIDs[comment.ID]; ok {
			keys = append(keys, bytes.Clone(key))
		}
		return true
	}
	err := forEachRepositoryComment(bucket, repository, visit)
	return keys, err
}

func forEachRepositoryComment(
	bucket *bolt.Bucket,
	repository string,
	visit func(key []byte, comment Comment) bool,
) error {
	cursor := bucket.Cursor()
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		comment, err := decodeComment(value)
		if err != nil {
			return err
		}
		if comment.Repository == repository && !visit(key, comment) {
			return nil
		}
	}
	return nil
}
