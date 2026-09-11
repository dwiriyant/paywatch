package cache_test

import (
	"path/filepath"
	"testing"

	"github.com/dwiriyant/paywatch/internal/cache"
)

func TestFileStore_roundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	s := cache.NewFileStore(path)

	type blob struct {
		Token string `json:"token"`
	}
	if err := s.Save(blob{Token: "abc"}); err != nil {
		t.Fatal(err)
	}
	var got blob
	if err := s.Load(&got); err != nil {
		t.Fatal(err)
	}
	if got.Token != "abc" {
		t.Fatalf("%+v", got)
	}
}

func TestFileStore_missingIsOK(t *testing.T) {
	s := cache.NewFileStore(filepath.Join(t.TempDir(), "missing.json"))
	var got map[string]string
	if err := s.Load(&got); err != nil {
		t.Fatal(err)
	}
}
