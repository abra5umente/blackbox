package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type LLMConfig struct {
	BaseURL   string `json:"base_url"`
	APIKeyEnv string `json:"api_key_env"`
	Model     string `json:"model"`
}

func main() {
	cfgPath := flag.String("config", "./configs/llm.json", "Path to llm config json")
	txtPath := flag.String("txt", "", "Transcript file path")
	flag.Parse()

	if *txtPath == "" {
		fatal("--txt is required")
	}
	if _, err := os.Stat(*txtPath); err != nil {
		fatal(fmt.Sprintf("transcript missing: %v", err))
	}

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		fatal(fmt.Sprintf("config error: %v", err))
	}
	apiKey := os.Getenv(cfg.APIKeyEnv)
	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "warning: %s not set; would use it for auth\n", cfg.APIKeyEnv)
	}

	absTxt, _ := filepath.Abs(*txtPath)
	fmt.Printf("Would POST to %s/chat/completions with model=%s using key env %s for file %s\n",
		cfg.BaseURL, cfg.Model, cfg.APIKeyEnv, absTxt)
}

func loadConfig(path string) (*LLMConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg LLMConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	if cfg.BaseURL == "" || cfg.Model == "" || cfg.APIKeyEnv == "" {
		return nil, fmt.Errorf("missing required fields in config")
	}
	return &cfg, nil
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(2)
}
