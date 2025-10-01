package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSettingsStoreLoadsDefaultsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ui.json")

	store, err := NewSettingsStore(path)
	if err != nil {
		t.Fatalf("NewSettingsStore failed: %v", err)
	}

	s := store.Get()
	if s.OutDir != "./out" {
		t.Fatalf("expected default OutDir, got %q", s.OutDir)
	}
	if !s.EnableFileBackups {
		t.Fatal("expected EnableFileBackups to default true")
	}
}

func TestSettingsStoreSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ui.json")

	store, err := NewSettingsStore(path)
	if err != nil {
		t.Fatalf("NewSettingsStore failed: %v", err)
	}

	cfg := UISettings{
		OutDir:            filepath.Join(dir, "out"),
		DatabasePath:      filepath.Join(dir, "db.sqlite"),
		EnableFileBackups: false,
		UseLocalAI:        true,
		LlamaTemp:         0.5,
		LlamaContext:      4096,
		LlamaModel:        "model.gguf",
		LlamaAPIKey:       "abc123",
	}

	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read saved settings: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("settings file was empty")
	}

	store2, err := NewSettingsStore(path)
	if err != nil {
		t.Fatalf("NewSettingsStore re-open failed: %v", err)
	}

	saved := store2.Get()
	if saved.OutDir != cfg.OutDir || saved.DatabasePath != cfg.DatabasePath {
		t.Fatalf("expected saved settings to round-trip: %+v", saved)
	}
	if saved.LlamaTemp != cfg.LlamaTemp || saved.LlamaContext != cfg.LlamaContext {
		t.Fatalf("expected llama settings to persist: %+v", saved)
	}
}
