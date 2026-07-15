package diskvstorage

import (
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
