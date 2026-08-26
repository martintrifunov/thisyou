package localdata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExportStoreListsAndSafelyDeletesExports(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"thisyou-alice-20260101.json", "notes.json"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	store := NewExportStore(directory)
	entries, err := store.List()
	if err != nil || len(entries) != 1 || entries[0].Name != "thisyou-alice-20260101.json" {
		t.Fatalf("entries = %#v, err = %v", entries, err)
	}
	if err := store.Delete("../notes.json"); err == nil {
		t.Fatal("unsafe export name was accepted")
	}
	if err := store.Delete(entries[0].Name); err != nil {
		t.Fatal(err)
	}
}
