package comments

import (
	"bytes"
	"errors"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

func addComment(bucket *bolt.Bucket, id string, data []byte) error {
	if pendingCommentBytes(bucket)+len(data) > maxPendingBytes {
		return fmt.Errorf("pending feedback exceeds %d bytes", maxPendingBytes)
	}
	if bucket.Get([]byte(id)) != nil {
		return errors.New("comment ID already exists")
	}
	return bucket.Put([]byte(id), data)
}

func replaceComment(bucket *bolt.Bucket, comment Comment, data []byte) error {
	key, err := findCommentKey(bucket, comment.Repository, comment.ID)
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

func deleteCommentKeys(bucket *bolt.Bucket, keys [][]byte) error {
	for _, key := range keys {
		if err := bucket.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

func findCommentKey(
	bucket *bolt.Bucket,
	repository, id string,
) ([]byte, error) {
	cursor := bucket.Cursor()
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		comment, err := decodeCommentRecord(value)
		if err != nil {
			return nil, err
		}
		if comment.Repository == repository && comment.ID == id {
			return append([]byte(nil), key...), nil
		}
	}
	return nil, nil
}

func findPendingCommentKeys(
	bucket *bolt.Bucket,
	repository string,
	wanted map[string]struct{},
) ([][]byte, error) {
	var keys [][]byte
	err := bucket.ForEach(func(key, value []byte) error {
		comment, err := decodeCommentRecord(value)
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
}

func pendingCommentBytes(bucket *bolt.Bucket) int {
	var total int
	cursor := bucket.Cursor()
	for _, value := cursor.First(); value != nil; _, value = cursor.Next() {
		total += len(value)
	}
	return total
}

func validateComment(comment Comment) error {
	if len(comment.Body) == 0 {
		return errors.New("comment body is empty")
	}
	if len(comment.Body) > maxCommentBytes {
		return fmt.Errorf("comment exceeds %d bytes", maxCommentBytes)
	}
	return nil
}
