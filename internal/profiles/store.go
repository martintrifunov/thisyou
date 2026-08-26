package profiles

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/martintrifunov/thisyou/internal/reddit"
)

const fileVersion = 1

type Entry struct {
	Username string    `json:"username"`
	SavedAt  time.Time `json:"saved_at"`
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
	return NewStoreAt(filepath.Join(directory, "thisyou", "profiles.json"))
}

func NewStoreAt(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *Store) Toggle(username string) (bool, []Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	normalized, err := reddit.NormalizeUsername(username)
	if err != nil {
		return false, nil, err
	}
	entries, err := s.load()
	if err != nil {
		return false, nil, err
	}
	for index, entry := range entries {
		if strings.EqualFold(entry.Username, normalized) {
			entries = append(entries[:index], entries[index+1:]...)
			if err := s.save(entries); err != nil {
				return false, nil, err
			}
			return false, entries, nil
		}
	}
	entries = append(entries, Entry{Username: normalized, SavedAt: time.Now().UTC()})
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
		return nil, fmt.Errorf("decode saved profiles: %w", err)
	}
	if stored.Version != fileVersion {
		return nil, fmt.Errorf("unsupported saved-profile file version %d", stored.Version)
	}
	entries := make([]Entry, 0, len(stored.Entries))
	seen := make(map[string]struct{}, len(stored.Entries))
	for _, entry := range stored.Entries {
		username, err := reddit.NormalizeUsername(entry.Username)
		if err != nil {
			continue
		}
		if _, exists := seen[username]; exists {
			continue
		}
		seen[username] = struct{}{}
		entry.Username = username
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
	temporary, err := os.CreateTemp(directory, ".profiles-*")
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

func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if !entries[i].SavedAt.Equal(entries[j].SavedAt) {
			return entries[i].SavedAt.After(entries[j].SavedAt)
		}
		return entries[i].Username < entries[j].Username
	})
}
