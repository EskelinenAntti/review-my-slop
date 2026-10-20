package comments

import (
	"bytes"
	"errors"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

const maxPendingBytes = 16 << 20

func addComment(bucket *bolt.Bucket, id string, data []byte) error {
	if bucket.Get([]byte(id)) != nil {
		return errors.New("comment ID already exists")
	}
	if err := checkPendingSize(bucket, 0, len(data)); err != nil {
		return err
	}
	return bucket.Put([]byte(id), data)
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

func deleteComment(bucket *bolt.Bucket, repository, id string) error {
	key, err := findCommentKey(bucket, repository, id)
	if err != nil {
		return err
	}
	return bucket.Delete(key)
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

func acknowledgeComments(
	bucket *bolt.Bucket,
	repository string,
	ids []string,
) error {
	wantedIDs := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wantedIDs[id] = struct{}{}
	}
	// Collect keys before mutating the bucket so cursor iteration stays valid.
	var keys [][]byte
	visit := func(key []byte, comment Comment) bool {
		if _, ok := wantedIDs[comment.ID]; ok {
			keys = append(keys, bytes.Clone(key))
		}
		return true
	}
	err := forEachRepositoryComment(bucket, repository, visit)
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
