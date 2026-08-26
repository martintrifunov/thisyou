package bookmarks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/martintrifunov/thisyou/internal/reddit"
)

const fileVersion = 1

type Entry struct {
	Item    reddit.Item `json:"item"`
	SavedAt time.Time   `json:"saved_at"`
}

type Store struct {
	path    string
	pathErr error
	mu      sync.Mutex
}

type fileData struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}

func NewStore() *Store {
	directory, err := os.UserConfigDir()
	if err != nil {
		return &Store{pathErr: err}
	}
	return NewStoreAt(filepath.Join(directory, "thisyou", "bookmarks.json"))
}

func NewStoreAt(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *Store) Toggle(item reddit.Item) (bool, []Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if item.ID == "" || (item.Kind != reddit.Post && item.Kind != reddit.Comment) {
		return false, nil, fmt.Errorf("record cannot be bookmarked")
	}
	entries, err := s.load()
	if err != nil {
		return false, nil, err
	}
	key := itemKey(item)
	for i, entry := range entries {
		if itemKey(entry.Item) == key {
			entries = append(entries[:i], entries[i+1:]...)
			if err := s.save(entries); err != nil {
				return false, nil, err
			}
			return false, entries, nil
		}
	}
	entries = append(entries, Entry{Item: item, SavedAt: time.Now().UTC()})
	sortEntries(entries)
	if err := s.save(entries); err != nil {
		return false, nil, err
	}
	return true, entries, nil
}

func (s *Store) load() ([]Entry, error) {
	if s.pathErr != nil {
		return nil, s.pathErr
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var stored fileData
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("decode bookmarks: %w", err)
	}
	if stored.Version != fileVersion {
		return nil, fmt.Errorf("unsupported bookmark file version %d", stored.Version)
	}
	entries := make([]Entry, 0, len(stored.Entries))
	seen := make(map[string]struct{}, len(stored.Entries))
	for _, entry := range stored.Entries {
		if entry.Item.ID == "" || (entry.Item.Kind != reddit.Post && entry.Item.Kind != reddit.Comment) {
			continue
		}
		key := itemKey(entry.Item)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		entries = append(entries, entry)
	}
	sortEntries(entries)
	return entries, nil
}

func (s *Store) save(entries []Entry) error {
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".bookmarks-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(fileData{Version: fileVersion, Entries: entries}); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, s.path)
}

func itemKey(item reddit.Item) string {
	return string(item.Kind) + ":" + item.ID
}

func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if !entries[i].SavedAt.Equal(entries[j].SavedAt) {
			return entries[i].SavedAt.After(entries[j].SavedAt)
		}
		return itemKey(entries[i].Item) < itemKey(entries[j].Item)
	})
}
