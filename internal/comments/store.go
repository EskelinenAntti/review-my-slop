package comments

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	maxCommentBytes = 64 << 10
	maxPendingBytes = 16 << 20
)

var messagesBucket = []byte("messages")

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
	if err := func() error {
		if len(comment.Body) == 0 {
			return errors.New("comment body is empty")
		}
		if len(comment.Body) > maxCommentBytes {
			return fmt.Errorf("comment exceeds %d bytes", maxCommentBytes)
		}
		return nil
	}(); err != nil {
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
	err = func() error {
		db, err := func() (*bolt.DB, error) {
			if s.Path == "" {
				return nil, errors.New("comments path is empty")
			}
			if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
				return nil, fmt.Errorf("create comments directory: %w", err)
			}
			if err := os.Chmod(filepath.Dir(s.Path), 0o700); err != nil {
				return nil, fmt.Errorf("secure comments directory: %w", err)
			}
			db, err := bolt.Open(s.Path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
			if err != nil {
				return nil, fmt.Errorf("open comments: %w", err)
			}
			if err := os.Chmod(s.Path, 0o600); err != nil {
				db.Close()
				return nil, fmt.Errorf("secure comments database: %w", err)
			}
			return db, nil
		}()
		if err != nil {
			return err
		}
		defer db.Close()
		return db.Update(func(tx *bolt.Tx) error {
			bucket, err := tx.CreateBucketIfNotExists(messagesBucket)
			if err != nil {
				return err
			}
			return func(bucket *bolt.Bucket) error {
				var id string = comment.ID
				if func() int {
					var total int
					cursor := bucket.Cursor()
					for _, value := cursor.First(); value != nil; _, value = cursor.Next() {
						total += len(value)
					}
					return total
				}()+len(data) > maxPendingBytes {
					return fmt.Errorf("pending feedback exceeds %d bytes", maxPendingBytes)
				}
				if bucket.Get([]byte(id)) != nil {
					return errors.New("comment ID already exists")
				}
				return bucket.Put([]byte(id), data)
			}(bucket)
		})
	}()
	return comment, err
}

// List returns pending comments for the repository.
func (s Store) List(repository string) ([]Comment, error) {
	var comments []Comment
	err := func() error {
		db, err := func() (*bolt.DB, error) {
			if s.Path == "" {
				return nil, errors.New("comments path is empty")
			}
			if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
				return nil, fmt.Errorf("create comments directory: %w", err)
			}
			if err := os.Chmod(filepath.Dir(s.Path), 0o700); err != nil {
				return nil, fmt.Errorf("secure comments directory: %w", err)
			}
			db, err := bolt.Open(s.Path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
			if err != nil {
				return nil, fmt.Errorf("open comments: %w", err)
			}
			if err := os.Chmod(s.Path, 0o600); err != nil {
				db.Close()
				return nil, fmt.Errorf("secure comments database: %w", err)
			}
			return db, nil
		}()
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
			return func(bucket *bolt.Bucket) error {
				return bucket.ForEach(func(_, value []byte) error {
					comment, err := func() (Comment, error) {
						var legacy legacyRecord
						if err := json.Unmarshal(value, &legacy); err != nil {
							return Comment{}, fmt.Errorf("decode comment: %w", err)
						}
						if legacy.Comment != nil {
							return Comment{ID: legacy.ID, Repository: legacy.Repository, CreatedAt: legacy.CreatedAt, Anchor: legacy.Comment.Anchor, Body: legacy.Comment.Body}, nil
						}
						var comment Comment
						if err := json.Unmarshal(value, &comment); err != nil {
							return Comment{}, fmt.Errorf("decode comment: %w", err)
						}
						return comment, nil
					}()
					if err != nil {
						return err
					}
					if comment.Repository == repository {
						comments = append(comments, comment)
					}
					return nil
				})
			}(bucket)
		})
	}()
	return comments, err
}

// Update replaces an existing comment in the repository.
func (s Store) Update(comment Comment) error {
	if comment.Repository == "" || comment.ID == "" {
		return errors.New("repository and comment ID are required")
	}
	if err := func() error {
		if len(comment.Body) == 0 {
			return errors.New("comment body is empty")
		}
		if len(comment.Body) > maxCommentBytes {
			return fmt.Errorf("comment exceeds %d bytes", maxCommentBytes)
		}
		return nil
	}(); err != nil {
		return err
	}
	data, err := json.Marshal(comment)
	if err != nil {
		return fmt.Errorf("encode comment: %w", err)
	}
	{
		db, err := func() (*bolt.DB, error) {
			if s.Path == "" {
				return nil, errors.New("comments path is empty")
			}
			if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
				return nil, fmt.Errorf("create comments directory: %w", err)
			}
			if err := os.Chmod(filepath.Dir(s.Path), 0o700); err != nil {
				return nil, fmt.Errorf("secure comments directory: %w", err)
			}
			db, err := bolt.Open(s.Path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
			if err != nil {
				return nil, fmt.Errorf("open comments: %w", err)
			}
			if err := os.Chmod(s.Path, 0o600); err != nil {
				db.Close()
				return nil, fmt.Errorf("secure comments database: %w", err)
			}
			return db, nil
		}()
		if err != nil {
			return err
		}
		defer db.Close()
		return db.Update(func(tx *bolt.Tx) error {
			bucket, err := tx.CreateBucketIfNotExists(messagesBucket)
			if err != nil {
				return err
			}
			return func(bucket *bolt.Bucket) error {
				key, err := func() ([]byte, error) {
					var repository, id string = comment.Repository, comment.ID
					cursor := bucket.Cursor()
					for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
						comment, err := func() (Comment, error) {
							var legacy legacyRecord
							if err := json.Unmarshal(value, &legacy); err != nil {
								return Comment{}, fmt.Errorf("decode comment: %w", err)
							}
							if legacy.Comment != nil {
								return Comment{ID: legacy.ID, Repository: legacy.Repository, CreatedAt: legacy.CreatedAt, Anchor: legacy.Comment.Anchor, Body: legacy.Comment.Body}, nil
							}
							var comment Comment
							if err := json.Unmarshal(value, &comment); err != nil {
								return Comment{}, fmt.Errorf("decode comment: %w", err)
							}
							return comment, nil
						}()
						if err != nil {
							return nil, err
						}
						if comment.Repository == repository && comment.ID == id {
							return append([]byte(nil), key...), nil
						}
					}
					return nil, nil
				}()
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
			}(bucket)
		})
	}
}

// Delete removes the identified comment from the repository.
func (s Store) Delete(repository, id string) error {
	if repository == "" || id == "" {
		return errors.New("repository and comment ID are required")
	}
	db, err := func() (*bolt.DB, error) {
		if s.Path == "" {
			return nil, errors.New("comments path is empty")
		}
		if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
			return nil, fmt.Errorf("create comments directory: %w", err)
		}
		if err := os.Chmod(filepath.Dir(s.Path), 0o700); err != nil {
			return nil, fmt.Errorf("secure comments directory: %w", err)
		}
		db, err := bolt.Open(s.Path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
		if err != nil {
			return nil, fmt.Errorf("open comments: %w", err)
		}
		if err := os.Chmod(s.Path, 0o600); err != nil {
			db.Close()
			return nil, fmt.Errorf("secure comments database: %w", err)
		}
		return db, nil
	}()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(messagesBucket)
		if err != nil {
			return err
		}
		return func(bucket *bolt.Bucket) error {
			key, err := func() ([]byte, error) {
				cursor := bucket.Cursor()
				for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
					comment, err := func() (Comment, error) {
						var legacy legacyRecord
						if err := json.Unmarshal(value, &legacy); err != nil {
							return Comment{}, fmt.Errorf("decode comment: %w", err)
						}
						if legacy.Comment != nil {
							return Comment{ID: legacy.ID, Repository: legacy.Repository, CreatedAt: legacy.CreatedAt, Anchor: legacy.Comment.Anchor, Body: legacy.Comment.Body}, nil
						}
						var comment Comment
						if err := json.Unmarshal(value, &comment); err != nil {
							return Comment{}, fmt.Errorf("decode comment: %w", err)
						}
						return comment, nil
					}()
					if err != nil {
						return nil, err
					}
					if comment.Repository == repository && comment.ID == id {
						return append([]byte(nil), key...), nil
					}
				}
				return nil, nil
			}()
			if err != nil {
				return err
			}
			if key == nil {
				return errors.New("comment is no longer in the comments")
			}
			return bucket.Delete(key)
		}(bucket)
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
	db, err := func() (*bolt.DB, error) {
		if s.Path == "" {
			return nil, errors.New("comments path is empty")
		}
		if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
			return nil, fmt.Errorf("create comments directory: %w", err)
		}
		if err := os.Chmod(filepath.Dir(s.Path), 0o700); err != nil {
			return nil, fmt.Errorf("secure comments directory: %w", err)
		}
		db, err := bolt.Open(s.Path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
		if err != nil {
			return nil, fmt.Errorf("open comments: %w", err)
		}
		if err := os.Chmod(s.Path, 0o600); err != nil {
			db.Close()
			return nil, fmt.Errorf("secure comments database: %w", err)
		}
		return db, nil
	}()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(messagesBucket)
		if err != nil {
			return err
		}
		return func(bucket *bolt.Bucket) error {
			keys, err := func() ([][]byte, error) {
				var keys [][]byte
				err := bucket.ForEach(func(key, value []byte) error {
					comment, err := func() (Comment, error) {
						var legacy legacyRecord
						if err := json.Unmarshal(value, &legacy); err != nil {
							return Comment{}, fmt.Errorf("decode comment: %w", err)
						}
						if legacy.Comment != nil {
							return Comment{ID: legacy.ID, Repository: legacy.Repository, CreatedAt: legacy.CreatedAt, Anchor: legacy.Comment.Anchor, Body: legacy.Comment.Body}, nil
						}
						var comment Comment
						if err := json.Unmarshal(value, &comment); err != nil {
							return Comment{}, fmt.Errorf("decode comment: %w", err)
						}
						return comment, nil
					}()
					if err != nil {
						return err
					}
					if comment.Repository != repository {
						return nil
					}
					if _, ok := wanted[comment.ID]; ok {
						keys = append(keys, append([]byte(nil), key...))
					}
					return nil
				})
				return keys, err
			}()
			if err != nil {
				return err
			}
			for _, key := range keys {
				if err := bucket.Delete(key); err != nil {
					return err
				}
			}
			return nil
		}(bucket)
	})
}

type legacyComment struct {
	Anchor Anchor `json:"anchor"`
	Body   string `json:"body"`
}

type legacyRecord struct {
	ID         string         `json:"id"`
	Repository string         `json:"repository"`
	CreatedAt  time.Time      `json:"created_at"`
	Comment    *legacyComment `json:"comment"`
}
