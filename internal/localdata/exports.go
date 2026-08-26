package localdata

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Export struct {
	Name     string
	Path     string
	Bytes    int64
	Modified time.Time
}

type ExportStore struct {
	directory string
}

func NewExportStore(directory string) *ExportStore {
	return &ExportStore{directory: directory}
}

func (s *ExportStore) List() ([]Export, error) {
	files, err := os.ReadDir(s.directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	entries := make([]Export, 0, len(files))
	for _, file := range files {
		if file.IsDir() || !isExportName(file.Name()) {
			continue
		}
		info, err := file.Info()
		if err != nil {
			return nil, err
		}
		entries = append(entries, Export{
			Name: file.Name(), Path: filepath.Join(s.directory, file.Name()),
			Bytes: info.Size(), Modified: info.ModTime(),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if !entries[i].Modified.Equal(entries[j].Modified) {
			return entries[i].Modified.After(entries[j].Modified)
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

func (s *ExportStore) Delete(name string) error {
	if !isExportName(name) || filepath.Base(name) != name {
		return fmt.Errorf("invalid export name")
	}
	err := os.Remove(filepath.Join(s.directory, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func isExportName(name string) bool {
	if !strings.HasPrefix(name, "thisyou-") {
		return false
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".csv", ".json", ".md", ".sqlite", ".db":
		return true
	default:
		return false
	}
}
