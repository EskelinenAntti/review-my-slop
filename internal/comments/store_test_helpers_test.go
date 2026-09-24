package comments

import bolt "go.etcd.io/bbolt"

func (s Store) update(fn func(*bolt.Bucket) error) error {
	return s.transact(true, fn)
}

func (s Store) view(fn func(*bolt.Bucket) error) error {
	return s.transact(false, fn)
}
