package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAtCreatesAndReloadsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	first, err := LoadAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if first != Default() {
		t.Fatalf("created config = %#v, want defaults", first)
	}
	first.DefaultSort = "score"
	first.CacheEnabled = false
	if err := SaveAt(path, first); err != nil {
		t.Fatal(err)
	}
	second, err := LoadAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("loaded config = %#v, want %#v", second, first)
	}
}

func TestLoadAtRejectsInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"default_sort":"random"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAt(path); err == nil {
		t.Fatal("invalid config was accepted")
	}
}
