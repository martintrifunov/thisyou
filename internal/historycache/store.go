package historycache

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

	"github.com/maco/thisyou/internal/reddit"
)

const fileVersion = 1

type Snapshot struct {
	Username string        `json:"username"`
	SavedAt  time.Time     `json:"saved_at"`
	Posts    []reddit.Item `json:"posts"`
	Comments []reddit.Item `json:"comments"`
	Errors   []string      `json:"errors,omitempty"`
	Warnings []string      `json:"warnings,omitempty"`
}

type Store struct {
	directory string
	pathErr   error
	mu        sync.Mutex
}

type Entry struct {
	Username string
	SavedAt  time.Time
	Posts    int
	Comments int
	Bytes    int64
}

type fileData struct {
	Version  int      `json:"version"`
	Snapshot Snapshot `json:"snapshot"`
}

func NewStore() *Store {
	directory, err := os.UserCacheDir()
	if err != nil {
		return &Store{pathErr: err}
	}
	return NewStoreAt(filepath.Join(directory, "thisyou", "history"))
}

func NewStoreAt(directory string) *Store {
	return &Store{directory: directory}
}

func (s *Store) Load(username string) (Snapshot, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pathErr != nil {
		return Snapshot{}, false, s.pathErr
	}
	normalized, err := normalizeUsername(username)
	if err != nil {
		return Snapshot{}, false, err
	}
	username = normalized
	data, err := os.ReadFile(s.path(username))
	if errors.Is(err, os.ErrNotExist) {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, err
	}
	var stored fileData
	if err := json.Unmarshal(data, &stored); err != nil {
		return Snapshot{}, false, fmt.Errorf("decode cached history: %w", err)
	}
	if stored.Version != fileVersion {
		return Snapshot{}, false, fmt.Errorf("unsupported history cache version %d", stored.Version)
	}
	if !strings.EqualFold(stored.Snapshot.Username, username) {
		return Snapshot{}, false, fmt.Errorf("cached username does not match u/%s", username)
	}
	stored.Snapshot.Username = username
	return stored.Snapshot, true, nil
}

func (s *Store) Save(snapshot Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pathErr != nil {
		return s.pathErr
	}
	if snapshot.Username == "" {
		return fmt.Errorf("cached username is empty")
	}
	normalized, err := normalizeUsername(snapshot.Username)
	if err != nil {
		return err
	}
	snapshot.Username = normalized
	if snapshot.SavedAt.IsZero() {
		snapshot.SavedAt = time.Now().UTC()
	}
	if err := os.MkdirAll(s.directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(s.directory, ".history-*")
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
	if err := encoder.Encode(fileData{Version: fileVersion, Snapshot: snapshot}); err != nil {
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
	return os.Rename(temporaryPath, s.path(snapshot.Username))
}

func (s *Store) List() ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pathErr != nil {
		return nil, s.pathErr
	}
	files, err := os.ReadDir(s.directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(files))
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.directory, file.Name()))
		if err != nil {
			return nil, err
		}
		var stored fileData
		if err := json.Unmarshal(data, &stored); err != nil || stored.Version != fileVersion {
			continue
		}
		username, err := normalizeUsername(stored.Snapshot.Username)
		if err != nil || username+".json" != file.Name() {
			continue
		}
		entries = append(entries, Entry{
			Username: username, SavedAt: stored.Snapshot.SavedAt,
			Posts: len(stored.Snapshot.Posts), Comments: len(stored.Snapshot.Comments), Bytes: int64(len(data)),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if !entries[i].SavedAt.Equal(entries[j].SavedAt) {
			return entries[i].SavedAt.After(entries[j].SavedAt)
		}
		return entries[i].Username < entries[j].Username
	})
	return entries, nil
}

func (s *Store) Delete(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pathErr != nil {
		return s.pathErr
	}
	normalized, err := normalizeUsername(username)
	if err != nil {
		return err
	}
	err = os.Remove(s.path(normalized))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Store) path(username string) string {
	return filepath.Join(s.directory, username+".json")
}

func normalizeUsername(username string) (string, error) {
	normalized, err := reddit.NormalizeUsername(username)
	if err != nil || normalized != strings.ToLower(username) {
		return "", fmt.Errorf("invalid cached username")
	}
	return normalized, nil
}
