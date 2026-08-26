package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

const fileVersion = 1

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type Theme struct {
	Accent     string `json:"accent"`
	Muted      string `json:"muted"`
	Foreground string `json:"foreground"`
	Border     string `json:"border"`
}

type Config struct {
	Version               int    `json:"version"`
	ExportDirectory       string `json:"export_directory"`
	DefaultSort           string `json:"default_sort"`
	RequestTimeoutSeconds int    `json:"request_timeout_seconds"`
	CacheEnabled          bool   `json:"cache_enabled"`
	StartupHealthCheck    bool   `json:"startup_health_check"`
	Theme                 Theme  `json:"theme"`
}

func Default() Config {
	return Config{
		Version:               fileVersion,
		ExportDirectory:       ".",
		DefaultSort:           "newest",
		RequestTimeoutSeconds: 25,
		CacheEnabled:          true,
		StartupHealthCheck:    true,
		Theme: Theme{
			Accent:     "#ff4500",
			Muted:      "#818384",
			Foreground: "#d7dadc",
			Border:     "#343536",
		},
	}
}

func Load() (Config, string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return Config{}, "", err
	}
	path := filepath.Join(directory, "thisyou", "config.json")
	value, err := LoadAt(path)
	return value, path, err
}

func LoadAt(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		value := Default()
		if err := SaveAt(path, value); err != nil {
			return Config{}, err
		}
		return value, nil
	}
	if err != nil {
		return Config{}, err
	}
	value := Default()
	if err := json.Unmarshal(data, &value); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if err := value.Validate(); err != nil {
		return Config{}, err
	}
	return value, nil
}

func SaveAt(path string, value Config) error {
	if err := value.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".config-*")
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
	if err := encoder.Encode(value); err != nil {
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
	return os.Rename(temporaryPath, path)
}

func (c Config) Validate() error {
	if c.Version != fileVersion {
		return fmt.Errorf("unsupported config version %d", c.Version)
	}
	switch c.DefaultSort {
	case "newest", "oldest", "score", "subreddit":
	default:
		return fmt.Errorf("invalid default_sort %q", c.DefaultSort)
	}
	if c.ExportDirectory == "" {
		return fmt.Errorf("export_directory cannot be empty")
	}
	if c.RequestTimeoutSeconds < 1 || c.RequestTimeoutSeconds > 300 {
		return fmt.Errorf("request_timeout_seconds must be between 1 and 300")
	}
	colors := map[string]string{
		"accent": c.Theme.Accent, "muted": c.Theme.Muted,
		"foreground": c.Theme.Foreground, "border": c.Theme.Border,
	}
	for name, value := range colors {
		if !colorPattern.MatchString(value) {
			return fmt.Errorf("theme.%s must be a #RRGGBB color", name)
		}
	}
	return nil
}
