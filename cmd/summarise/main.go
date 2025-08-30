package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LLMConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_completion_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
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

	if cfg.APIKey == "" {
		fatal("api_key is required in config")
	}

	// Read the transcript file
	transcript, err := os.ReadFile(*txtPath)
	if err != nil {
		fatal(fmt.Sprintf("failed to read transcript: %v", err))
	}

	// Create the summarization prompt
	prompt := `You are an expert summarization specialist. Your task is to create a clear, concise summary of the provided transcript. Focus on:

1. Key points and main ideas
2. Important details and context
3. Any action items or decisions mentioned
4. Overall tone and sentiment

Please provide a well-structured summary that captures the essence of the conversation while maintaining clarity and readability.`

	// Prepare the chat request
	request := ChatRequest{
		Model: cfg.Model,
		Messages: []ChatMessage{
			{
				Role:    "system",
				Content: prompt,
			},
			{
				Role:    "user",
				Content: string(transcript),
			},
		},
		MaxTokens: 2000,
	}

	// Make the API request
	summary, err := makeOpenAIRequest(cfg.BaseURL, cfg.APIKey, request)
	if err != nil {
		fatal(fmt.Sprintf("API request failed: %v", err))
	}

	// Write summary to output file
	outputPath := strings.TrimSuffix(*txtPath, filepath.Ext(*txtPath)) + "_summary.txt"
	if err := os.WriteFile(outputPath, []byte(summary), 0644); err != nil {
		fatal(fmt.Sprintf("failed to write summary: %v", err))
	}

	fmt.Printf("Summary written to: %s\n", outputPath)
	fmt.Printf("\n--- Summary ---\n%s\n", summary)
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
	if cfg.BaseURL == "" || cfg.Model == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("missing required fields in config")
	}
	return &cfg, nil
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(2)
}

func makeOpenAIRequest(baseURL, apiKey string, request ChatRequest) (string, error) {
	// Prepare the request body
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := baseURL + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// Make the request
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API errors
	if chatResp.Error != nil {
		return "", fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	// Extract summary from response
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in API response")
	}

	return chatResp.Choices[0].Message.Content, nil
}
