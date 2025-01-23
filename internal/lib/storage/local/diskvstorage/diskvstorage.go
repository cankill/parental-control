package diskvstorage

import (
	"fmt"
	"strings"

	"github.com/peterbourgon/diskv/v3"
)

const sdb_extension = ".sdb"

type LocalStorage struct {
	db *diskv.Diskv
}

func OpenStorage(dbFolder string) *LocalStorage {
	db := diskv.New(diskv.Options{
		BasePath:          dbFolder,
		AdvancedTransform: withBucketTransform,
		InverseTransform:  inverseWithBucketTransform,
		CacheSizeMax:      1024 * 1024,
	})

	return &LocalStorage{db: db}
}

func (s *LocalStorage) SaveValue(bucketName, key string, value string) {
	s.db.WriteString(bucketName+"/"+key, value)
}

func (s *LocalStorage) GetValues(bucketName string) map[string]string {
	result := make(map[string]string)
	cancel := make(<-chan struct{})
	keys := s.db.KeysPrefix(bucketName, cancel)
	for key := range keys {
		value := s.db.ReadString(key)
		_, file := splitBucket(key)
		result[file] = value
	}

	return result
}

func (s *LocalStorage) GetValue(bucketName string, key string) string {
	return s.db.ReadString(bucketName + "/" + key)
}

func (s *LocalStorage) DumpBuckets() {
	fmt.Print("Buckets: ")
	cancel := make(<-chan struct{})
	keys := s.db.Keys(cancel)
	buckets := map[string]bool{}
	for key := range keys {
		paths, _ := splitBucket(key)
		buckets[strings.Join(paths, "/")] = true
	}

	for bucket := range buckets {
		fmt.Printf("%s, ", bucket)
	}

	fmt.Println()
}

func (s *LocalStorage) DumpBucket(bucketName string) {
	values := s.GetValues(bucketName)
	for key, value := range values {
		fmt.Printf("%s: %s=%s\n", bucketName, key, value)
	}
}

func withBucketTransform(key string) *diskv.PathKey {
	paths, file := splitBucket(key)
	return &diskv.PathKey{
		Path:     paths,
		FileName: file + sdb_extension,
	}
}

func splitBucket(key string) ([]string, string) {
	path := strings.Split(key, "/")
	last := len(path) - 1
	return path[:last], path[last]
}

func inverseWithBucketTransform(pathKey *diskv.PathKey) (key string) {
	if len(pathKey.Path) < 1 || len(pathKey.FileName) < 4 {
		return
	}
	extension := pathKey.FileName[len(pathKey.FileName)-4:]
	if extension != sdb_extension {
		return
	}
	return strings.Join(pathKey.Path, "/") + "/" + pathKey.FileName[:len(pathKey.FileName)-4]
}
