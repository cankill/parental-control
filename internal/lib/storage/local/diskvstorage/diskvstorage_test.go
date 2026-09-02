package diskvstorage

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSplitBucket(t *testing.T) {
	paths, file := splitBucket("2026-07-15T10/com.google.Chrome")
	if !reflect.DeepEqual(paths, []string{"2026-07-15T10"}) || file != "com.google.Chrome" {
		t.Fatalf("splitBucket = %v, %q", paths, file)
	}
	// Доменный bucket с префиксом.
	paths, file = splitBucket("dom/2026-07-15T10/youtube.com")
	if !reflect.DeepEqual(paths, []string{"dom", "2026-07-15T10"}) || file != "youtube.com" {
		t.Fatalf("split domain bucket = %v, %q", paths, file)
	}
}

func TestBucketTransformRoundTrip(t *testing.T) {
	key := "2026-07-15T10/com.apple.Safari"
	pk := withBucketTransform(key)
	if got := inverseWithBucketTransform(pk); got != key {
		t.Fatalf("round-trip: got %q, want %q", got, key)
	}
}

func TestSaveGetAndListBuckets(t *testing.T) {
	s := OpenStorage(t.TempDir())
	s.SaveValue("2026-07-15T10", "com.google.Chrome", "12345")
	s.SaveValue("2026-07-15T11", "com.apple.Terminal", "6789")

	if v := s.GetValue("2026-07-15T10", "com.google.Chrome"); v != "12345" {
		t.Errorf("GetValue = %q, want 12345", v)
	}
	// Несуществующий ключ — пустая строка.
	if v := s.GetValue("2026-07-15T10", "nonexistent"); v != "" {
		t.Errorf("missing value = %q, want empty", v)
	}

	values := s.GetValues("2026-07-15T10")
	if len(values) != 1 || values["com.google.Chrome"] != "12345" {
		t.Errorf("GetValues = %v", values)
	}

	buckets := s.ListBuckets()
	if len(buckets) != 2 || buckets[0] != "2026-07-15T10" || buckets[1] != "2026-07-15T11" {
		t.Errorf("ListBuckets = %v, want sorted [T10 T11]", buckets)
	}
}

func TestGetValuesOnlyWalksRequestedBucket(t *testing.T) {
	root := t.TempDir()
	s := OpenStorage(root)
	s.SaveValue("target", "com.google.Chrome", "12345")

	// A root-wide filepath.Walk would enter this sibling before "target" and
	// fail. A correctly scoped KeysPrefix starts directly in target/.
	blocked := filepath.Join(root, "000-blocked")
	if err := os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(blocked, 0700); err != nil {
			t.Errorf("restore blocked directory permissions: %v", err)
		}
	})
	if _, err := os.ReadDir(blocked); err == nil {
		t.Skip("test process can read mode-000 directories")
	}

	values := s.GetValues("target")
	if len(values) != 1 || values["com.google.Chrome"] != "12345" {
		t.Fatalf("GetValues(target) = %v", values)
	}
}
