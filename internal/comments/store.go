package comments

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

// WritePending emits repository feedback and acknowledges only emitted
// comments.
func (s Store) WritePending(w io.Writer, repository string) error {
	pending, err := s.List(repository)
	if err != nil {
		return err
	}
	if err := writePendingComments(w, pending); err != nil {
		return err
	}
	return s.acknowledge(repository, commentIDs(pending))
}

const (
	maxCommentBytes = 64 << 10
	maxPendingBytes = 16 << 20
)

var messagesBucket = []byte("messages")

// Store persists comments in a local database at Path.
type Store struct {
	Path string
}

// Add validates and stores a comment, assigning its ID and timestamp if absent.
func (s Store) Add(comment Comment) (Comment, error) {
	if comment.Repository == "" {
		return Comment{}, errors.New("comment requires a repository")
	}
	if err := validateCommentBody(comment.Body); err != nil {
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
	db, err := s.open()
	if err != nil {
		return Comment{}, err
	}
	defer db.Close()
	err = db.Update(func(tx *bolt.Tx) error {
		return addStoredComment(tx, comment, data)
	})
	return comment, err
}

// List returns pending comments for a repository in creation order.
func (s Store) List(repository string) ([]Comment, error) {
	var comments []Comment
	db, err := s.open()
	if errors.Is(err, os.ErrNotExist) {
		return comments, nil
	}
	if err != nil {
		return nil, err
	}
	defer db.Close()
	err = db.View(func(tx *bolt.Tx) error {
		var err error
		comments, err = repositoryComments(tx, repository)
		return err
	})
	return comments, err
}

// Update replaces an existing comment with the same repository and ID.
func (s Store) Update(comment Comment) error {
	if comment.Repository == "" || comment.ID == "" {
		return errors.New("repository and comment ID are required")
	}
	if err := validateCommentBody(comment.Body); err != nil {
		return err
	}
	data, err := json.Marshal(comment)
	if err != nil {
		return fmt.Errorf("encode comment: %w", err)
	}
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bolt.Tx) error {
		return updateStoredComment(tx, comment, data)
	})
}

// Delete removes the comment identified by repository and ID.
func (s Store) Delete(repository, id string) error {
	if repository == "" || id == "" {
		return errors.New("repository and comment ID are required")
	}
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bolt.Tx) error {
		return deleteStoredComment(tx, repository, id)
	})
}

func (s Store) acknowledge(repository string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bolt.Tx) error {
		return acknowledgeStoredComments(tx, repository, wanted)
	})
}

func addStoredComment(tx *bolt.Tx, comment Comment, data []byte) error {
	bucket, err := tx.CreateBucketIfNotExists(messagesBucket)
	if err != nil {
		return err
	}
	total := 0
	cursor := bucket.Cursor()
	for _, value := cursor.First(); value != nil; _, value = cursor.Next() {
		total += len(value)
	}
	if total+len(data) > maxPendingBytes {
		return fmt.Errorf(
			"pending feedback exceeds %d bytes",
			maxPendingBytes,
		)
	}
	if bucket.Get([]byte(comment.ID)) != nil {
		return errors.New("comment ID already exists")
	}
	return bucket.Put([]byte(comment.ID), data)
}

func repositoryComments(
	tx *bolt.Tx,
	repository string,
) ([]Comment, error) {
	bucket := tx.Bucket(messagesBucket)
	if bucket == nil {
		return nil, nil
	}
	var comments []Comment
	err := bucket.ForEach(func(_, value []byte) error {
		comment, err := decodeStoredComment(value)
		if err != nil {
			return err
		}
		if comment.Repository == repository {
			comments = append(comments, comment)
		}
		return nil
	})
	return comments, err
}

func updateStoredComment(
	tx *bolt.Tx,
	comment Comment,
	data []byte,
) error {
	bucket, err := tx.CreateBucketIfNotExists(messagesBucket)
	if err != nil {
		return err
	}
	key, err := findStoredCommentKey(bucket, comment.Repository, comment.ID)
	if err != nil {
		return err
	}
	if key == nil {
		return errors.New("comment is no longer in the comments")
	}
	if !bytes.Equal(key, []byte(comment.ID)) {
		if err := bucket.Delete(key); err != nil {
			return err
		}
	}
	return bucket.Put([]byte(comment.ID), data)
}

func deleteStoredComment(tx *bolt.Tx, repository, id string) error {
	bucket, err := tx.CreateBucketIfNotExists(messagesBucket)
	if err != nil {
		return err
	}
	key, err := findStoredCommentKey(bucket, repository, id)
	if err != nil {
		return err
	}
	if key == nil {
		return errors.New("comment is no longer in the comments")
	}
	return bucket.Delete(key)
}

func acknowledgeStoredComments(
	tx *bolt.Tx,
	repository string,
	wanted map[string]struct{},
) error {
	bucket, err := tx.CreateBucketIfNotExists(messagesBucket)
	if err != nil {
		return err
	}
	keys, err := acknowledgedCommentKeys(bucket, repository, wanted)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err := bucket.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

func acknowledgedCommentKeys(
	bucket *bolt.Bucket,
	repository string,
	wanted map[string]struct{},
) ([][]byte, error) {
	var keys [][]byte
	err := bucket.ForEach(func(key, value []byte) error {
		comment, err := decodeStoredComment(value)
		if err != nil {
			return err
		}
		if comment.Repository != repository {
			return nil
		}
		if _, ok := wanted[comment.ID]; !ok {
			return nil
		}
		keys = append(keys, append([]byte(nil), key...))
		return nil
	})
	return keys, err
}

func (s Store) open() (*bolt.DB, error) {
	if s.Path == "" {
		return nil, errors.New("comments path is empty")
	}
	directory := filepath.Dir(s.Path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create comments directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return nil, fmt.Errorf("secure comments directory: %w", err)
	}
	db, err := bolt.Open(s.Path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open comments: %w", err)
	}
	if err := os.Chmod(s.Path, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("secure comments database: %w", err)
	}
	return db, nil
}

func validateCommentBody(body string) error {
	if len(body) == 0 {
		return errors.New("comment body is empty")
	}
	if len(body) > maxCommentBytes {
		return fmt.Errorf("comment exceeds %d bytes", maxCommentBytes)
	}
	return nil
}

func decodeStoredComment(value []byte) (Comment, error) {
	var legacy legacyRecord
	if err := json.Unmarshal(value, &legacy); err != nil {
		return Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	if legacy.Comment != nil {
		return Comment{
			ID:         legacy.ID,
			Repository: legacy.Repository,
			CreatedAt:  legacy.CreatedAt,
			Anchor:     legacy.Comment.Anchor,
			Body:       legacy.Comment.Body,
		}, nil
	}
	var comment Comment
	if err := json.Unmarshal(value, &comment); err != nil {
		return Comment{}, fmt.Errorf("decode comment: %w", err)
	}
	return comment, nil
}

func findStoredCommentKey(
	bucket *bolt.Bucket,
	repository, id string,
) ([]byte, error) {
	cursor := bucket.Cursor()
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		comment, err := decodeStoredComment(value)
		if err != nil {
			return nil, err
		}
		if matchesStoredComment(comment, repository, id) {
			return append([]byte(nil), key...), nil
		}
	}
	return nil, nil
}

func matchesStoredComment(comment Comment, repository, id string) bool {
	return comment.Repository == repository && comment.ID == id
}

func formatAnchorSide(side string, start, end int) string {
	if end == 0 || end == start {
		return fmt.Sprintf("%s line %d", side, start)
	}
	return fmt.Sprintf("%s lines %d-%d", side, start, end)
}
