package historycache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/maco/thisyou/internal/reddit"
)

func TestStoreSavesAndLoadsSnapshot(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	store := NewStoreAt(directory)
	snapshot := Snapshot{
		Username: "alice",
		SavedAt:  time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC),
		Posts:    []reddit.Item{{Kind: reddit.Post, ID: "post", Title: "cached"}},
		Comments: []reddit.Item{{Kind: reddit.Comment, ID: "comment", Body: "cached"}},
	}
	if err := store.Save(snapshot); err != nil {
		t.Fatal(err)
	}
	loaded, found, err := NewStoreAt(directory).Load("alice")
	if err != nil {
		t.Fatal(err)
	}
	if !found || loaded.Username != "alice" || len(loaded.Posts) != 1 || len(loaded.Comments) != 1 || !loaded.SavedAt.Equal(snapshot.SavedAt) {
		t.Fatalf("loaded snapshot = %+v, found = %v", loaded, found)
	}
	info, err := os.Stat(filepath.Join(directory, "alice.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("cache permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestStoreReportsMissingAndInvalidSnapshots(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	store := NewStoreAt(directory)
	if _, found, err := store.Load("missing"); err != nil || found {
		t.Fatalf("missing cache = found %v, error %v", found, err)
	}
	if err := os.WriteFile(filepath.Join(directory, "broken.json"), []byte("bad json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Load("broken"); err == nil {
		t.Fatal("invalid cache was accepted")
	}
}

func TestStoreRejectsUnsafeUsername(t *testing.T) {
	t.Parallel()
	store := NewStoreAt(t.TempDir())
	if err := store.Save(Snapshot{Username: "../outside"}); err == nil {
		t.Fatal("unsafe username was saved")
	}
	if _, _, err := store.Load("../outside"); err == nil {
		t.Fatal("unsafe username was loaded")
	}
}
