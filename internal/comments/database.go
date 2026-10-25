package comments

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

var commentsBucket = []byte("messages")

type databaseMode uint8

const (
	writeDatabase databaseMode = iota
	readDatabase
)

func (s Store) transact(fn func(commentBucket) error) error {
	db, err := s.openDatabase(writeDatabase)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(commentsBucket)
		if err != nil {
			return err
		}
		return fn(commentBucket{bucket: bucket})
	})
}

func (s Store) view(fn func(commentBucket) error) error {
	db, err := s.openDatabase(readDatabase)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer db.Close()
	return db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(commentsBucket)
		if bucket == nil {
			return nil
		}
		return fn(commentBucket{bucket: bucket})
	})
}

func (s Store) openDatabase(mode databaseMode) (*bolt.DB, error) {
	if err := s.preparePath(mode); err != nil {
		return nil, err
	}
	db, err := bolt.Open(s.Path, 0o600, &bolt.Options{
		Timeout:  2 * time.Second,
		ReadOnly: mode == readDatabase,
	})
	if err != nil {
		return nil, fmt.Errorf("open comments: %w", err)
	}
	if err := os.Chmod(s.Path, 0o600); err != nil {
		db.Close()
		return nil, fmt.Errorf("secure comments database: %w", err)
	}
	return db, nil
}

func (s Store) preparePath(mode databaseMode) error {
	if s.Path == "" {
		return errors.New("comments path is empty")
	}
	if mode == readDatabase {
		if _, err := os.Stat(s.Path); err != nil {
			return fmt.Errorf("open comments: %w", err)
		}
	} else if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("create comments directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("secure comments directory: %w", err)
	}
	return nil
}
