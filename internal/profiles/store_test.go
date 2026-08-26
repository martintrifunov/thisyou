package profiles

import (
	"path/filepath"
	"testing"
)

func TestStoreTogglesAndPersistsProfiles(t *testing.T) {
	store := NewStoreAt(filepath.Join(t.TempDir(), "profiles.json"))
	saved, entries, err := store.Toggle("u/Alice")
	if err != nil || !saved || len(entries) != 1 || entries[0].Username != "alice" {
		t.Fatalf("save = %v, entries = %#v, err = %v", saved, entries, err)
	}
	loaded, err := store.Load()
	if err != nil || len(loaded) != 1 || loaded[0].Username != "alice" {
		t.Fatalf("load = %#v, err = %v", loaded, err)
	}
	saved, entries, err = store.Toggle("alice")
	if err != nil || saved || len(entries) != 0 {
		t.Fatalf("remove = %v, entries = %#v, err = %v", saved, entries, err)
	}
}
