package comments

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/eskelinenantti/review-my-slop/internal/appinfo"
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

// Save adds a comment without an ID or updates an existing comment with an ID.
// An ID that is no longer stored in the repository is an error, not an insert.
func (s Store) Save(comment Comment) (Comment, error) {
	if comment.ID == "" {
		return s.Add(comment)
	}
	if err := s.Update(comment); err != nil {
		return Comment{}, err
	}
	return comment, nil
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
	err = s.transact(func(bucket commentBucket) error {
		return bucket.add(comment.ID, data)
	})
	return comment, err
}

// List returns pending comments for the repository.
func (s Store) List(repository string) ([]Comment, error) {
	var pendingComments []Comment
	err := s.view(func(bucket commentBucket) error {
		var err error
		pendingComments, err = bucket.list(repository)
		return err
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
	return s.transact(func(bucket commentBucket) error {
		return bucket.replace(comment, data)
	})
}

// Delete removes the identified comment from the repository.
func (s Store) Delete(repository, id string) error {
	if err := validateCommentIdentity(repository, id); err != nil {
		return err
	}
	return s.transact(func(bucket commentBucket) error {
		return bucket.delete(repository, id)
	})
}

// Acknowledge removes the identified comments after their feedback is consumed.
func (s Store) Acknowledge(repository string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return s.transact(func(bucket commentBucket) error {
		return bucket.acknowledge(repository, ids)
	})
}
