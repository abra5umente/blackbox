package ui

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// UISettings holds configurable UI preferences. Only OutDir is used for now.
type UISettings struct {
	OutDir string `json:"out_dir"`
	// Future fields may be added here. Keep JSON stable.
}

type SettingsStore struct {
	mu       sync.RWMutex
	path     string
	settings UISettings
}

// NewSettingsStore initialises the store and loads from disk or defaults.
func NewSettingsStore(configPath string) (*SettingsStore, error) {
	store := &SettingsStore{path: configPath}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

// load reads settings from disk. If not found, sets defaults and ensures directory exists.
func (s *SettingsStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.path == "" {
		return errors.New("settings path not set")
	}
	if _, err := os.Stat(s.path); err != nil {
		// Default settings
		s.settings = UISettings{OutDir: "./out"}
		// Ensure directory exists for first save
		_ = os.MkdirAll(filepath.Dir(s.path), 0755)
		return nil
	}
	b, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var cfg UISettings
	if err := json.Unmarshal(b, &cfg); err != nil {
		return err
	}
	if cfg.OutDir == "" {
		cfg.OutDir = "./out"
	}
	s.settings = cfg
	return nil
}

// Save persists the settings to disk, creating parent directories as needed.
func (s *SettingsStore) Save(newSettings UISettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if newSettings.OutDir == "" {
		newSettings.OutDir = "./out"
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(newSettings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path, b, 0644); err != nil {
		return err
	}
	s.settings = newSettings
	return nil
}

// Get returns a copy of the current settings.
func (s *SettingsStore) Get() UISettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}
