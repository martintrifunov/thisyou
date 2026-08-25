package bookmarks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/maco/thisyou/internal/reddit"
)

func TestStoreTogglePersistsAndRemovesBookmark(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "bookmarks.json")
	store := NewStoreAt(path)
	item := reddit.Item{Kind: reddit.Post, ID: "abc", Author: "alice", Title: "saved post"}

	saved, entries, err := store.Toggle(item)
	if err != nil {
		t.Fatal(err)
	}
	if !saved || len(entries) != 1 || entries[0].Item.ID != item.ID {
		t.Fatalf("saved = %v, entries = %+v", saved, entries)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("bookmark permissions = %o, want 600", info.Mode().Perm())
	}

	reloaded, err := NewStoreAt(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded) != 1 || reloaded[0].Item.Title != item.Title {
		t.Fatalf("reloaded entries = %+v", reloaded)
	}

	saved, entries, err = store.Toggle(item)
	if err != nil {
		t.Fatal(err)
	}
	if saved || len(entries) != 0 {
		t.Fatalf("saved = %v, entries = %+v after removal", saved, entries)
	}
}

func TestStoreSortsNewestAndDeduplicatesOnLoad(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "bookmarks.json")
	data := `{
  "version": 1,
  "entries": [
    {"item":{"Kind":"comment","ID":"old"},"saved_at":"2025-01-01T00:00:00Z"},
    {"item":{"Kind":"post","ID":"new"},"saved_at":"2025-01-02T00:00:00Z"},
    {"item":{"Kind":"post","ID":"new"},"saved_at":"2025-01-03T00:00:00Z"},
    {"item":{"Kind":"post","ID":""},"saved_at":"2025-01-04T00:00:00Z"}
  ]
}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := NewStoreAt(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Item.ID != "new" || entries[1].Item.ID != "old" {
		t.Fatalf("loaded entries = %+v", entries)
	}
}

func TestStoreRejectsInvalidRecordsAndFiles(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "bookmarks.json")
	store := NewStoreAt(path)
	if _, _, err := store.Toggle(reddit.Item{Kind: reddit.Post}); err == nil {
		t.Fatal("invalid record was bookmarked")
	}
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("invalid bookmark file was accepted")
	}
}
