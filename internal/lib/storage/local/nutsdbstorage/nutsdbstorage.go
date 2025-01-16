package nutsdbstorage

import (
	"fmt"

	"github.com/nutsdb/nutsdb"
)

type LocalStorage struct {
	db *nutsdb.DB
}

func New(dbFolder string) (*LocalStorage, error) {
	const op = "storage.local.nutsdbstorage.New"

	db, err := nutsdb.Open(
		nutsdb.DefaultOptions,
		nutsdb.WithDir(dbFolder),
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &LocalStorage{db: db}, nil
}

func (s *LocalStorage) Close() {
	s.db.Close()
}

func (s *LocalStorage) NewBucket(name string) error {
	err := s.db.Update(func(tx *nutsdb.Tx) error {
		return tx.NewBucket(nutsdb.DataStructureBTree, name)
	})

	return err
}

func (s *LocalStorage) FindBucket(bucketName string) (found bool, err error) {
	found = false
	err = s.db.View(
		func(tx *nutsdb.Tx) error {
			return tx.IterateBuckets(nutsdb.DataStructureBTree, bucketName, func(bucket string) bool {
				found = found || (bucket == bucketName)
				return true
			})
		})

	return
}

func (s *LocalStorage) GetValue(bucketName, key string) (result []byte, err error) {
	err = s.db.View(
		func(tx *nutsdb.Tx) error {
			result, err = tx.Get(bucketName, []byte(key))
			if err != nil {
				return err
			}
			// fmt.Println("val:", string(result))

			return nil
		})

	return
}

func (s *LocalStorage) GetValues(bucketName string) (result map[string][]byte, err error) {
	result = make(map[string][]byte)
	err = s.db.View(
		func(tx *nutsdb.Tx) error {
			keys, values, err := tx.GetAll(bucketName)
			if err != nil {
				return err
			}

			for idx, key := range keys {
				appName := string(key)
				result[appName] = values[idx]
			}

			return nil
		})

	return
}

func (s *LocalStorage) SaveValue(bucketName, key string, value []byte) error {
	return s.db.Update(
		func(tx *nutsdb.Tx) error {
			err := tx.Put(bucketName, []byte(key), value, 0)
			if err != nil {
				return err
			}

			return nil
		})
}
