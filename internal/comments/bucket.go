package comments

import (
	"bytes"
	"errors"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

const maxPendingBytes = 16 << 20

// commentBucket owns record keys, repository filtering, and storage limits
// within a single transaction. Keys may differ from IDs for legacy records.
type commentBucket struct {
	bucket *bolt.Bucket
}

func (b commentBucket) add(id string, data []byte) error {
	if b.bucket.Get([]byte(id)) != nil {
		return errors.New("comment ID already exists")
	}
	if err := b.checkPendingSize(0, len(data)); err != nil {
		return err
	}
	return b.bucket.Put([]byte(id), data)
}

func (b commentBucket) list(repository string) ([]Comment, error) {
	var pending []Comment
	err := b.forEach(repository, func(_ []byte, comment Comment) bool {
		pending = append(pending, comment)
		return true
	})
	return pending, err
}

func (b commentBucket) replace(comment Comment, data []byte) error {
	key, err := b.findKey(comment.Repository, comment.ID)
	if err != nil {
		return err
	}
	oldSize := len(b.bucket.Get(key))
	if err := b.checkPendingSize(oldSize, len(data)); err != nil {
		return err
	}
	if !bytes.Equal(key, []byte(comment.ID)) {
		if err := b.bucket.Delete(key); err != nil {
			return err
		}
	}
	return b.bucket.Put([]byte(comment.ID), data)
}

func (b commentBucket) delete(repository, id string) error {
	key, err := b.findKey(repository, id)
	if err != nil {
		return err
	}
	return b.bucket.Delete(key)
}

func (b commentBucket) acknowledge(repository string, ids []string) error {
	wantedIDs := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wantedIDs[id] = struct{}{}
	}
	keys, err := b.findPendingKeys(repository, wantedIDs)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err := b.bucket.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

func (b commentBucket) pendingBytes() int {
	var total int
	cursor := b.bucket.Cursor()
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		total += len(value)
	}
	return total
}

func (b commentBucket) checkPendingSize(replacedBytes, addedBytes int) error {
	if b.pendingBytes()-replacedBytes+addedBytes > maxPendingBytes {
		return fmt.Errorf("pending feedback exceeds %d bytes", maxPendingBytes)
	}
	return nil
}

func (b commentBucket) findKey(repository, id string) ([]byte, error) {
	var found []byte
	err := b.forEach(repository, func(key []byte, comment Comment) bool {
		if comment.ID == id {
			found = bytes.Clone(key)
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, errors.New("comment is no longer in the comments")
	}
	return found, nil
}

func (b commentBucket) findPendingKeys(
	repository string,
	wantedIDs map[string]struct{},
) ([][]byte, error) {
	var keys [][]byte
	err := b.forEach(repository, func(key []byte, comment Comment) bool {
		if _, ok := wantedIDs[comment.ID]; ok {
			keys = append(keys, bytes.Clone(key))
		}
		return true
	})
	return keys, err
}

func (b commentBucket) forEach(
	repository string,
	visit func(key []byte, comment Comment) bool,
) error {
	cursor := b.bucket.Cursor()
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
