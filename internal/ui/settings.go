package ui

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// UISettings holds configurable UI preferences.
type UISettings struct {
	OutDir string `json:"out_dir"`
	// Database settings
	DatabasePath      string `json:"database_path"`
	EnableFileBackups bool   `json:"enable_file_backups"`
	// Local AI settings
	UseLocalAI   bool    `json:"use_local_ai"`
	LlamaTemp    float64 `json:"llama_temp"`
	LlamaContext int     `json:"llama_context"`
	LlamaModel   string  `json:"llama_model"`
	LlamaAPIKey  string  `json:"llama_api_key"`
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
		s.settings = UISettings{
			OutDir:            "./out",
			DatabasePath:      "./data/blackbox.db",
			EnableFileBackups: true,
			UseLocalAI:        false,
			LlamaTemp:         0.1,
			LlamaContext:      32000,
			LlamaModel:        "",
			LlamaAPIKey:       "",
		}
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
	// Set defaults for new fields if not present
	if cfg.LlamaTemp == 0 {
		cfg.LlamaTemp = 0.1
	}
	if cfg.LlamaContext == 0 {
		cfg.LlamaContext = 32000
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
	if newSettings.DatabasePath == "" {
		newSettings.DatabasePath = "./data/blackbox.db"
	}
	// Set defaults for new fields if not present
	if newSettings.LlamaTemp == 0 {
		newSettings.LlamaTemp = 0.1
	}
	if newSettings.LlamaContext == 0 {
		newSettings.LlamaContext = 32000
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
