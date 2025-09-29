package ui

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"blackbox/internal/audio"
	"blackbox/internal/db"
	"blackbox/internal/execx"
	"blackbox/internal/wav"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// PromptConfig represents a summarisation prompt configuration
type PromptConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
}

// App exposes methods to the Wails frontend.
type App struct {
	settings *SettingsStore
	database *db.DB

	mu          sync.Mutex
	recording   bool
	dictation   bool
	recordingID int
	rec         *audio.Recorder
	mic         *audio.MicRecorder
	writer      *wav.Writer
	runErrCh    chan error
	ctx         context.Context
	cancel      context.CancelFunc
	flushTicker *time.Ticker
	wavPath     string

	// Llama server management
	llamaServer *exec.Cmd
	llamaMu     sync.Mutex

	// Prompt management
	selectedPrompt string
	promptMu       sync.RWMutex

	uiCtx context.Context
}

func NewApp(settingsPath string) (*App, error) {
	store, err := NewSettingsStore(settingsPath)
	if err != nil {
		return nil, err
	}
	s := store.Get()
	if err := os.MkdirAll(s.OutDir, 0755); err != nil {
		return nil, err
	}

	// Initialize database
	database, err := db.NewDB(s.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	app := &App{
		settings:       store,
		database:       database,
		selectedPrompt: "meeting", // Default to meeting prompt
	}

	// Clean up old temporary files on startup
	go func() {
		if err := app.CleanupTempFiles(); err != nil {
			fmt.Printf("Warning: failed to cleanup temporary files: %v\n", err)
		}
	}()

	return app, nil
}

// Close closes the database connection and cleans up resources
func (a *App) Close() error {
	if a.database != nil {
		return a.database.Close()
	}
	return nil
}

// --- Settings API ---

func (a *App) GetSettings() UISettings {
	return a.settings.Get()
}

func (a *App) SaveSettings(jsonStr string) (UISettings, error) {
	var cfg UISettings
	if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
		return UISettings{}, err
	}
	if cfg.OutDir == "" {
		cfg.OutDir = "./out"
	}
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = "./data/blackbox.db"
	}

	// Create output directory
	if err := os.MkdirAll(cfg.OutDir, 0755); err != nil {
		return UISettings{}, err
	}

	// Check if database path changed and handle database migration
	currentSettings := a.settings.Get()
	if currentSettings.DatabasePath != cfg.DatabasePath {
		// Close current database connection
		if a.database != nil {
			if err := a.database.Close(); err != nil {
				return UISettings{}, fmt.Errorf("failed to close current database: %w", err)
			}
		}

		// Create new database connection
		newDB, err := db.NewDB(cfg.DatabasePath)
		if err != nil {
			return UISettings{}, fmt.Errorf("failed to open new database: %w", err)
		}
		a.database = newDB
	}

	// Save settings
	if err := a.settings.Save(cfg); err != nil {
		return UISettings{}, err
	}
	return a.settings.Get(), nil
}

// --- Prompt Management API ---

// GetAvailablePrompts returns a list of available prompt configurations from the database
func (a *App) GetAvailablePrompts() ([]PromptConfig, error) {
	// Get active prompts from database
	dbPrompts, err := a.database.GetActivePrompts()
	if err != nil {
		return nil, fmt.Errorf("failed to get prompts from database: %w", err)
	}

	var prompts []PromptConfig
	for _, dbPrompt := range dbPrompts {
		description := ""
		if dbPrompt.Description != nil {
			description = *dbPrompt.Description
		}
		prompt := PromptConfig{
			Name:        dbPrompt.DisplayName, // Use DisplayName as the main name
			Description: description,
			Prompt:      dbPrompt.PromptText,
		}
		prompts = append(prompts, prompt)
	}

	return prompts, nil
}

// GetSelectedPrompt returns the currently selected prompt name
func (a *App) GetSelectedPrompt() string {
	a.promptMu.RLock()
	defer a.promptMu.RUnlock()
	return a.selectedPrompt
}

// SetSelectedPrompt sets the currently selected prompt
func (a *App) SetSelectedPrompt(promptName string) error {
	a.promptMu.Lock()
	defer a.promptMu.Unlock()

	// Check if prompt exists in database
	_, err := a.database.GetPromptByName(promptName)
	if err != nil {
		return fmt.Errorf("prompt '%s' not found: %w", promptName, err)
	}

	a.selectedPrompt = promptName
	return nil
}

// GetPromptConfig returns the configuration for a specific prompt from the database
func (a *App) GetPromptConfig(promptName string) (PromptConfig, error) {
	dbPrompt, err := a.database.GetPromptByName(promptName)
	if err != nil {
		return PromptConfig{}, fmt.Errorf("prompt '%s' not found: %w", promptName, err)
	}

	description := ""
	if dbPrompt.Description != nil {
		description = *dbPrompt.Description
	}
	prompt := PromptConfig{
		Name:        dbPrompt.DisplayName, // Use DisplayName as the main name
		Description: description,
		Prompt:      dbPrompt.PromptText,
	}

	return prompt, nil
}

// SaveCustomPrompt saves a custom prompt configuration to the database
func (a *App) SaveCustomPrompt(config PromptConfig) error {
	if config.Name == "" {
		return errors.New("prompt name is required")
	}

	// Check if prompt already exists
	exists, err := a.database.PromptExists(config.Name)
	if err != nil {
		return fmt.Errorf("failed to check prompt existence: %w", err)
	}

	description := config.Description
	dbPrompt := &db.Prompt{
		Name:        config.Name,
		DisplayName: config.Name, // Use Name as DisplayName for custom prompts
		Description: &description,
		PromptText:  config.Prompt,
		IsDefault:   false,
		IsActive:    true,
	}

	if exists {
		// Update existing prompt
		existingPrompt, err := a.database.GetPromptByName(config.Name)
		if err != nil {
			return fmt.Errorf("failed to get existing prompt: %w", err)
		}
		dbPrompt.ID = existingPrompt.ID
		dbPrompt.CreatedAt = existingPrompt.CreatedAt

		if err := a.database.UpdatePrompt(dbPrompt); err != nil {
			return fmt.Errorf("failed to update prompt in database: %w", err)
		}
	} else {
		// Create new prompt
		if err := a.database.CreatePrompt(dbPrompt); err != nil {
			return fmt.Errorf("failed to create prompt in database: %w", err)
		}
	}

	return nil
}

// --- Recording API ---

func (a *App) IsRecording() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.recording
}

// StartRecording starts loopback (and optional mic) capture and writes to a new WAV file under OutDir/saved.
// This is for the Tools tab - files are saved permanently for manual use.
// Returns the path to the WAV file that will be written.
func (a *App) StartRecording(withMic bool) (string, error) {
	return a.startRecordingAdvancedInternal(withMic, false, true)
}

// StartRecordingTools is specifically for the Tools tab - saves to OutDir/saved permanently.
func (a *App) StartRecordingTools(withMic bool, dictation bool) (string, error) {
	return a.startRecordingAdvancedInternal(withMic, dictation, true)
}

// StopRecording stops capture and finalises the WAV. Returns the WAV path.
func (a *App) StopRecording() (string, error) {
	a.mu.Lock()
	if !a.recording {
		a.mu.Unlock()
		return "", errors.New("not recording")
	}
	// Capture local references and clear state early to avoid reentry
	rec := a.rec
	mic := a.mic
	writer := a.writer
	flushTicker := a.flushTicker
	runErrCh := a.runErrCh
	cancel := a.cancel
	wavPath := a.wavPath
	a.dictation = false
	a.rec = nil
	a.mic = nil
	a.writer = nil
	a.flushTicker = nil
	a.runErrCh = nil
	a.cancel = nil
	a.ctx = nil
	a.recording = false
	a.wavPath = ""
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if flushTicker != nil {
		flushTicker.Stop()
	}
	if rec != nil {
		rec.Stop()
	}
	if mic != nil {
		mic.Stop()
	}

	var runErr error
	if runErrCh != nil {
		select {
		case runErr = <-runErrCh:
		case <-time.After(2 * time.Second):
			// timeout
		}
	}
	_ = writer.Flush()
	if err := writer.Close(); err != nil {
		return wavPath, fmt.Errorf("finalize wav: %w", err)
	}

	// Handle database recording update only for Auto mode
	var recordingID int
	if a.recordingID > 0 {
		recordingID = a.recordingID
		// Read the WAV file data
		audioData, err := os.ReadFile(wavPath)
		if err != nil {
			return wavPath, fmt.Errorf("read wav file: %w", err)
		}

		// Calculate duration based on file size and audio format
		// PCM S16LE, 16kHz, mono: 2 bytes per sample, 16000 samples per second
		durationSeconds := float64(len(audioData)) / (16000.0 * 2.0)

		// Update recording in database with audio data
		dbRecording, err := a.database.GetRecording(a.recordingID)
		if err != nil {
			return wavPath, fmt.Errorf("failed to get recording from database: %w", err)
		}

		dbRecording.FileSize = int64(len(audioData))
		dbRecording.DurationSeconds = &durationSeconds
		dbRecording.AudioData = audioData

		if err := a.database.UpdateRecording(dbRecording); err != nil {
			return wavPath, fmt.Errorf("failed to update recording in database: %w", err)
		}

		// Keep the WAV file for potential transcription - it will be cleaned up after successful transcription
	}

	// Clear recording ID
	a.recordingID = 0

	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		return wavPath, runErr
	}

	// Return the file path for Tools mode, or recording ID for Auto mode
	if recordingID > 0 {
		// Auto mode: return recording ID for transcription
		return fmt.Sprintf("%d", recordingID), nil
	} else {
		// Tools mode: return file path for manual use
		return wavPath, nil
	}
}

// Transcribe runs whisper.cpp on a recording and returns the transcript content.
// It accepts either a WAV file path (for backwards compatibility) or a recording ID.
func (a *App) Transcribe(wavPathOrID string) (string, error) {
	if strings.TrimSpace(wavPathOrID) == "" {
		return "", errors.New("wav path or recording ID required")
	}

	cfg := a.settings.Get()
	outDir := cfg.OutDir
	if outDir == "" {
		outDir = "./out"
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}

	whisperBin := getenvDefault("LOOPBACK_NOTES_WHISPER_BIN", "./whisper-bin/whisper-cli.exe")
	modelDir := getenvDefault("LOOPBACK_NOTES_MODELS", "./models")
	modelPath := filepath.Join(modelDir, "ggml-base.en.bin")

	var dbRecording *db.Recording
	var wavPath string
	var err error

	// Check if it's a numeric ID (recording ID from database)
	if _, err := strconv.Atoi(wavPathOrID); err == nil {
		// It's a recording ID
		recordingID, _ := strconv.Atoi(wavPathOrID)
		dbRecording, err = a.database.GetRecording(recordingID)
		if err != nil {
			return "", fmt.Errorf("failed to get recording from database: %w", err)
		}

		// Create temporary WAV file from database audio data
		wavPath = filepath.Join(outDir, dbRecording.Filename)
		if err := os.WriteFile(wavPath, dbRecording.AudioData, 0644); err != nil {
			return "", fmt.Errorf("failed to create temporary WAV file: %w", err)
		}
		defer os.Remove(wavPath) // Clean up temporary file
	} else {
		// It's a file path (backwards compatibility)
		wavPath = wavPathOrID
		filename := filepath.Base(wavPath)
		dbRecording, err = a.database.GetRecordingByFilename(filename)
		if err != nil {
			return "", fmt.Errorf("failed to find recording in database: %w", err)
		}
	}

	startTime := time.Now()

	// Create a temporary directory for whisper processing to avoid cluttering ./out
	tempDir := filepath.Join(outDir, "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	txtPath, err := execx.RunWhisper(whisperBin, modelPath, wavPath, tempDir, "en", 0, "")
	if err != nil {
		// Clean up temp directory on error
		os.RemoveAll(tempDir)
		return "", err
	}

	// Read transcript content
	transcriptContent, err := os.ReadFile(txtPath)
	if err != nil {
		os.RemoveAll(tempDir)
		return txtPath, fmt.Errorf("failed to read transcript file: %w", err)
	}

	// Read log content
	logPath := strings.TrimSuffix(txtPath, ".txt") + ".log"
	logContent := ""
	if logData, err := os.ReadFile(logPath); err == nil {
		logContent = string(logData)
	}

	// Calculate processing time
	processingTimeSeconds := time.Since(startTime).Seconds()

	// Create transcript in database
	dbTranscript := &db.Transcript{
		RecordingID:           dbRecording.ID,
		Content:               string(transcriptContent),
		ModelUsed:             "ggml-base.en",
		Language:              "en",
		ProcessingTimeSeconds: &processingTimeSeconds,
	}

	if err := a.database.CreateTranscript(dbTranscript); err != nil {
		os.RemoveAll(tempDir)
		return txtPath, fmt.Errorf("failed to save transcript to database: %w", err)
	}

	// Save processing metadata with log content
	endTime := time.Now()
	modelUsed := "ggml-base.en"
	parameters := `{"language":"en","threads":0,"extraArgs":""}`
	logFilePath := logPath
	if logContent != "" {
		logFilePath = logContent
	}

	processingMetadata := &db.ProcessingMetadata{
		RecordingID:     &dbRecording.ID,
		TranscriptID:    &dbTranscript.ID,
		ProcessType:     "transcription",
		Status:          "completed",
		ModelUsed:       &modelUsed,
		Parameters:      &parameters,
		StartTime:       startTime,
		EndTime:         &endTime,
		DurationSeconds: &processingTimeSeconds,
		LogFilePath:     &logFilePath,
	}

	if err := a.database.CreateProcessingMetadata(processingMetadata); err != nil {
		// Non-fatal error, just log it
		fmt.Printf("Warning: failed to save processing metadata: %v\n", err)
	}

	// Clean up temporary files
	os.RemoveAll(tempDir)

	// Clean up WAV file after successful transcription
	if err := os.Remove(wavPath); err != nil {
		fmt.Printf("Warning: failed to remove WAV file %s: %v\n", wavPath, err)
	}

	// Return the transcript content instead of file path
	return string(transcriptContent), nil
}

// Summarise reads configs/llm.json and sends the transcript to OpenAI or local AI for summarisation.
func (a *App) Summarise(txtPathOrID string) (string, error) {
	if strings.TrimSpace(txtPathOrID) == "" {
		return "", errors.New("txt path or recording ID required")
	}

	uiCfg := a.settings.Get()

	var (
		transcript         string
		dbRecording        *db.Recording
		dbTranscript       *db.Transcript
		sourceIsFile       bool
		sourceIsTranscript bool
	)

	// Special handling for transcript IDs passed via SummariseTranscript
	if strings.HasPrefix(txtPathOrID, "transcript:") {
		if a.database == nil {
			return "", errors.New("database not initialized")
		}

		transcriptIDStr := strings.TrimPrefix(txtPathOrID, "transcript:")
		transcriptID, err := strconv.Atoi(transcriptIDStr)
		if err != nil {
			return "", fmt.Errorf("invalid transcript ID: %w", err)
		}

		var dbErr error
		dbTranscript, dbErr = a.database.GetTranscript(transcriptID)
		if dbErr != nil {
			return "", fmt.Errorf("failed to get transcript from database: %v", dbErr)
		}
		transcript = dbTranscript.Content

		dbRecording, dbErr = a.database.GetRecording(dbTranscript.RecordingID)
		if dbErr != nil {
			return "", fmt.Errorf("failed to get recording from database: %v", dbErr)
		}

		sourceIsTranscript = true
	} else if _, err := os.Stat(txtPathOrID); err == nil {
		// It's a file path, read from file
		transcriptBytes, err := os.ReadFile(txtPathOrID)
		if err != nil {
			return "", fmt.Errorf("failed to read transcript: %w", err)
		}
		transcript = string(transcriptBytes)
		sourceIsFile = true
	} else {
		// Try to parse as recording ID
		var recordingID int
		if _, parseErr := fmt.Sscanf(txtPathOrID, "%d", &recordingID); parseErr == nil {
			if a.database == nil {
				return "", errors.New("database not initialized")
			}

			var dbErr error
			dbRecording, dbErr = a.database.GetRecording(recordingID)
			if dbErr != nil {
				return "", fmt.Errorf("failed to get recording from database: %v", dbErr)
			}

			dbTranscript, dbErr = a.database.GetTranscriptByRecordingID(dbRecording.ID)
			if dbErr != nil {
				return "", fmt.Errorf("failed to get transcript from database: %v", dbErr)
			}
			transcript = dbTranscript.Content
		} else {
			return "", fmt.Errorf("invalid file path or recording ID: %s", txtPathOrID)
		}
	}

	if strings.TrimSpace(transcript) == "" {
		return "", errors.New("transcript content is empty")
	}

	// Get the selected prompt configuration
	promptConfig, err := a.GetPromptConfig(a.GetSelectedPrompt())
	if err != nil {
		return "", fmt.Errorf("failed to get prompt config: %w", err)
	}
	prompt := promptConfig.Prompt

	// Get the prompt from database to get the ID and display name
	dbPrompt, err := a.database.GetPromptByName(a.GetSelectedPrompt())
	if err != nil {
		return "", fmt.Errorf("failed to get prompt from database: %w", err)
	}

	var summary string

	if uiCfg.UseLocalAI {
		// Use local AI (llama.cpp) - load from local.json
		summary, err = a.summariseWithLocalAI(transcript, prompt)
		if err != nil {
			return "", fmt.Errorf("local AI summarisation failed: %w", err)
		}
	} else {
		// Use remote AI - load from remote.json
		cfg, err := a.loadLLMConfig("./configs/remote.json")
		if err != nil {
			return "", err
		}

		if cfg.APIKey == "" {
			return "", fmt.Errorf("api_key is required in remote config")
		}

		// Prepare the chat request
		request := chatRequest{
			Model: cfg.Model,
			Messages: []chatMessage{
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
		summary, err = a.makeOpenAIRequest(cfg.BaseURL, cfg.APIKey, request)
		if err != nil {
			return "", fmt.Errorf("API request failed: %w", err)
		}
	}

	// If we have a file path, we need to find the recording by filename
	if dbRecording == nil {
		if a.database == nil {
			return "", errors.New("database not initialized")
		}
		// It's a file path, find the recording
		txtFilename := filepath.Base(txtPathOrID)
		wavFilename := strings.TrimSuffix(txtFilename, ".txt") + ".wav"
		var transcriptErr error
		dbRecording, transcriptErr = a.database.GetRecordingByFilename(wavFilename)
		if transcriptErr != nil {
			return "", fmt.Errorf("failed to find recording: %w", transcriptErr)
		}
	}

	// Ensure we have transcript metadata
	if dbTranscript == nil {
		var err error
		dbTranscript, err = a.database.GetTranscriptByRecordingID(dbRecording.ID)
		if err != nil {
			return "", fmt.Errorf("failed to find transcript in database: %w", err)
		}
	}

	// Determine model used and endpoint
	modelUsed := "unknown"
	var apiEndpoint *string
	var localModelPath *string

	if uiCfg.UseLocalAI {
		// Extract model name from the model file path
		modelName := filepath.Base(uiCfg.LlamaModel)
		// Remove .gguf extension if present
		modelName = strings.TrimSuffix(modelName, ".gguf")
		modelUsed = modelName
		localModelPath = &uiCfg.LlamaModel
	} else {
		cfg, _ := a.loadLLMConfig("./configs/remote.json")
		if cfg != nil {
			modelUsed = cfg.Model
			apiEndpoint = &cfg.BaseURL
		}
	}

	// Create summary in database
	dbSummary := &db.Summary{
		TranscriptID:   dbTranscript.ID,
		Content:        summary,
		SummaryType:    dbPrompt.DisplayName,
		ModelUsed:      modelUsed,
		PromptUsed:     prompt,
		PromptID:       &dbPrompt.ID,
		APIEndpoint:    apiEndpoint,
		LocalModelPath: localModelPath,
	}

	if err := a.database.CreateSummary(dbSummary); err != nil {
		return "", fmt.Errorf("failed to save summary to database: %w", err)
	}

	// Write summary to output file if file backups are enabled
	var outputPath string
	if uiCfg.EnableFileBackups {
		cfg := a.settings.Get()
		outDir := cfg.OutDir
		if strings.TrimSpace(outDir) == "" {
			outDir = "./out"
		}

		if sourceIsFile {
			outputPath = strings.TrimSuffix(txtPathOrID, filepath.Ext(txtPathOrID)) + "_summary.txt"
		} else if sourceIsTranscript && dbRecording != nil {
			baseName := strings.TrimSuffix(dbRecording.Filename, filepath.Ext(dbRecording.Filename))
			outputPath = filepath.Join(outDir, fmt.Sprintf("%s_summary.txt", baseName))
		} else {
			outputPath = filepath.Join(outDir, fmt.Sprintf("%s_summary.txt", txtPathOrID))
		}

		if err := os.WriteFile(outputPath, []byte(summary), 0644); err != nil {
			return "", fmt.Errorf("failed to write summary: %w", err)
		}
	}

	return fmt.Sprintf("Summary saved to database (ID: %d)\n\n--- Summary ---\n%s", dbSummary.ID, summary), nil
}

// SummariseTranscript summarises a transcript selected from the database by ID
func (a *App) SummariseTranscript(transcriptID int) (string, error) {
	if a.database == nil {
		return "", errors.New("database not initialized")
	}
	if transcriptID <= 0 {
		return "", fmt.Errorf("invalid transcript ID: %d", transcriptID)
	}

	return a.Summarise(fmt.Sprintf("transcript:%d", transcriptID))
}

// summariseWithLocalAI uses the local llama-server for summarisation
func (a *App) summariseWithLocalAI(transcript, prompt string) (string, error) {
	// Ensure llama-server is running
	if !a.isLlamaServerRunning() {
		if err := a.startLlamaServer(); err != nil {
			return "", fmt.Errorf("failed to start llama-server: %w", err)
		}
	}

	// Load API key from local.json for client authentication
	cfg, err := a.loadLLMConfig("./configs/local.json")
	if err != nil {
		return "", fmt.Errorf("failed to load local config: %w", err)
	}

	// Prepare the chat request for local AI
	request := chatRequest{
		Model: "local", // Model name doesn't matter for local AI
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: prompt,
			},
			{
				Role:    "user",
				Content: transcript,
			},
		},
		MaxTokens: 2000,
	}

	// Make the request to local llama-server using API key from local.json
	summary, err := a.makeOpenAIRequest("http://127.0.0.1:8080", cfg.APIKey, request)
	if err != nil {
		// Shutdown server on error
		a.stopLlamaServer()
		return "", fmt.Errorf("local AI request failed: %w", err)
	}

	// Shutdown llama-server after successful summarisation
	a.stopLlamaServer()

	return summary, nil
}

// Helper: load LLM config shared with CLI semantics
type llmConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

// Chat API types
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_completion_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (a *App) makeOpenAIRequest(baseURL, apiKey string, request chatRequest) (string, error) {
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
	client := &http.Client{Timeout: 360 * time.Second}
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
	var chatResp chatResponse
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

func (a *App) loadLLMConfig(path string) (*llmConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg llmConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	if cfg.BaseURL == "" || cfg.Model == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("missing required fields in config")
	}
	return &cfg, nil
}

func getenvDefault(k, def string) string {
	if v := os.Getenv(k); strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

// mixS16Mono mixes two S16LE mono buffers with simple averaging.
func mixS16Mono(loop, mic []byte) []byte {
	if len(mic) == 0 {
		return loop
	}
	n := len(loop)
	if len(mic) < n {
		n = len(mic)
	}
	out := make([]byte, n)
	for i := 0; i < n; i += 2 {
		lv := int16(int16(loop[i]) | int16(int16(loop[i+1])<<8))
		mv := int16(int16(mic[i]) | int16(int16(mic[i+1])<<8))
		s := int32(lv) + int32(mv)
		s /= 2
		if s > 32767 {
			s = 32767
		} else if s < -32768 {
			s = -32768
		}
		out[i] = byte(uint16(int16(s)) & 0xFF)
		out[i+1] = byte((uint16(int16(s)) >> 8) & 0xFF)
	}
	return out
}

// StartRecordingAdvanced allows selecting dictation mode (mic only) vs loopback+optional mic.
// If isToolsMode is true, saves to OutDir/saved for permanent storage (Tools tab).
// If isToolsMode is false, saves to OutDir for temporary processing (Auto tab).
func (a *App) StartRecordingAdvanced(withMic bool, dictation bool) (string, error) {
	fmt.Printf("DEBUG: StartRecordingAdvanced called with withMic=%v, dictation=%v, isToolsMode=false\n", withMic, dictation)
	return a.startRecordingAdvancedInternal(withMic, dictation, false)
}

// StartRecordingAdvancedInternal is the internal implementation that handles both Auto and Tools modes.
func (a *App) startRecordingAdvancedInternal(withMic bool, dictation bool, isToolsMode bool) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.recording {
		return "", errors.New("already recording")
	}

	cfg := a.settings.Get()

	// Determine output directory based on mode
	var outputDir string
	if isToolsMode {
		// Tools mode: save to OutDir/saved for permanent storage
		outputDir = filepath.Join(cfg.OutDir, "saved")
		fmt.Printf("DEBUG: Tools mode - saving to %s\n", outputDir)
	} else {
		// Auto mode: save to OutDir for temporary processing
		outputDir = cfg.OutDir
		fmt.Printf("DEBUG: Auto mode - saving to %s\n", outputDir)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}

	const sampleRate uint32 = 16000 // Reduced from 48000 - 16kHz is standard for speech recognition
	const channels uint32 = 1       // Reduced from 2 - mono is sufficient for speech and cuts file size in half
	const bits uint16 = 16

	startTime := time.Now()
	ts := startTime.Format("20060102_150405")

	// Create a file for recording
	wavPath := filepath.Join(outputDir, ts+".wav")
	writer, err := wav.NewWriter(wavPath, sampleRate, uint16(channels), bits)
	if err != nil {
		return "", fmt.Errorf("open wav: %w", err)
	}

	// Create recording entry in database (only for Auto mode)
	var dbRecording *db.Recording
	if !isToolsMode {
		recordingMode := "loopback"
		if dictation {
			recordingMode = "dictation"
		} else if withMic {
			recordingMode = "mixed"
		}

		dbRecording = &db.Recording{
			Filename:       ts + ".wav",
			FilePath:       wavPath,
			FileSize:       0, // Will be updated when recording stops
			SampleRate:     int(sampleRate),
			Channels:       int(channels),
			BitsPerSample:  int(bits),
			AudioFormat:    "PCM S16LE",
			RecordingMode:  recordingMode,
			WithMicrophone: withMic,
			RecordedAt:     &startTime, // Store when recording started
			AudioData:      nil,        // Will be populated when recording stops
		}

		if err := a.database.CreateRecording(dbRecording); err != nil {
			_ = writer.Close()
			return "", fmt.Errorf("failed to create recording in database: %w", err)
		}
	}

	var rec *audio.Recorder
	var mic *audio.MicRecorder

	if dictation {
		// Mic-only capture
		m, err := audio.NewMicRecorder(8)
		if err != nil {
			_ = writer.Close()
			return "", fmt.Errorf("init mic: %w", err)
		}
		if err := m.Start(sampleRate, channels); err != nil {
			_ = writer.Close()
			return "", fmt.Errorf("start mic: %w", err)
		}
		mic = m
	} else {
		// Loopback capture (optionally mix mic)
		r, err := audio.NewRecorder(8)
		if err != nil {
			_ = writer.Close()
			return "", fmt.Errorf("init recorder: %w", err)
		}
		if err := r.Start(sampleRate, channels); err != nil {
			_ = writer.Close()
			return "", fmt.Errorf("start recorder: %w", err)
		}
		rec = r
		if withMic {
			m, err := audio.NewMicRecorder(8)
			if err != nil {
				rec.Stop()
				_ = writer.Close()
				return "", fmt.Errorf("init mic: %w", err)
			}
			if err := m.Start(sampleRate, channels); err != nil {
				rec.Stop()
				_ = writer.Close()
				return "", fmt.Errorf("start mic: %w", err)
			}
			mic = m
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	flushTicker := time.NewTicker(500 * time.Millisecond)
	runErrCh := make(chan error, 1)

	// Writer loop
	go func() {
		var micBuf []byte
		for {
			select {
			case <-ctx.Done():
				runErrCh <- nil
				return
			default:
			}

			if dictation {
				// Mic only path
				select {
				case <-ctx.Done():
					runErrCh <- nil
					return
				case b, ok := <-mic.Data():
					if !ok {
						runErrCh <- nil
						return
					}
					if len(b) > 0 {
						if _, err := writer.Write(b); err != nil {
							runErrCh <- err
							return
						}
						a.emitAudioData(b, "microphone")
					}
				case <-flushTicker.C:
					_ = writer.Flush()
				}
				continue
			}

			// Loopback primary path
			select {
			case <-ctx.Done():
				runErrCh <- nil
				return
			case b, ok := <-rec.Data():
				if !ok {
					runErrCh <- nil
					return
				}
				if len(b) > 0 {
					if mic != nil {
						select {
						case micBuf = <-mic.Data():
						default:
							micBuf = nil
						}
						mixed := mixS16Mono(b, micBuf)
						if _, err := writer.Write(mixed); err != nil {
							runErrCh <- err
							return
						}
						a.emitAudioData(mixed, "loopback")
					} else {
						if _, err := writer.Write(b); err != nil {
							runErrCh <- err
							return
						}
						a.emitAudioData(b, "loopback")
					}
				}
			case <-flushTicker.C:
				_ = writer.Flush()
			}
		}
	}()

	a.recording = true
	if dbRecording != nil {
		a.recordingID = dbRecording.ID
	} else {
		a.recordingID = 0 // Tools mode doesn't use database
	}
	a.dictation = dictation
	a.rec = rec
	a.mic = mic
	a.writer = writer
	a.ctx = ctx
	a.cancel = cancel
	a.flushTicker = flushTicker
	a.runErrCh = runErrCh
	a.wavPath = wavPath
	return wavPath, nil
}

// SetUIContext stores the Wails runtime context for dialog APIs.
func (a *App) SetUIContext(ctx context.Context) { a.uiCtx = ctx }

// emitAudioData sends real-time audio data to the frontend for spectrum analysis
func (a *App) emitAudioData(data []byte, source string) {
	if a.uiCtx != nil {
		wruntime.EventsEmit(a.uiCtx, "audioData", map[string]interface{}{
			"source": source,    // "loopback" or "microphone"
			"data":   data,      // Raw PCM S16LE data
			"length": len(data), // Data length in bytes
		})
	}
}

// PickWavFromOutDir opens a file picker defaulting to OutDir filtered to .wav
func (a *App) PickWavFromOutDir() (string, error) {
	if a.uiCtx == nil {
		return "", errors.New("ui not ready")
	}
	cfg := a.settings.Get()
	path, err := wruntime.OpenFileDialog(a.uiCtx, wruntime.OpenDialogOptions{
		Title:            "Choose WAV",
		DefaultDirectory: cfg.OutDir,
		Filters:          []wruntime.FileFilter{{DisplayName: "WAV", Pattern: "*.wav"}},
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// ListRecordings returns a list of recordings for selection
func (a *App) ListRecordings(limit int) ([]*db.Recording, error) {
	if a.database == nil {
		return nil, errors.New("database not initialized")
	}

	return a.database.ListRecordings(limit, 0, nil, nil)
}

// GetRecordingByID returns a recording by its ID
func (a *App) GetRecordingByID(id int) (*db.Recording, error) {
	if a.database == nil {
		return nil, errors.New("database not initialized")
	}

	return a.database.GetRecording(id)
}

// CleanupTempFiles removes temporary WAV files from the output directory
// that are older than 1 hour and have corresponding database entries
func (a *App) CleanupTempFiles() error {
	cfg := a.settings.Get()
	outDir := cfg.OutDir
	if outDir == "" {
		outDir = "./out"
	}

	// Read directory contents
	files, err := os.ReadDir(outDir)
	if err != nil {
		return fmt.Errorf("failed to read output directory: %w", err)
	}

	cutoff := time.Now().Add(-1 * time.Hour) // Files older than 1 hour

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".wav") {
			continue
		}

		// Check if file is older than cutoff
		info, err := file.Info()
		if err != nil {
			continue
		}

		if info.ModTime().After(cutoff) {
			continue // File is too recent
		}

		// Check if this file exists in the database
		_, err = a.database.GetRecordingByFilename(file.Name())
		if err == nil {
			// File exists in database, safe to remove
			filePath := filepath.Join(outDir, file.Name())
			if err := os.Remove(filePath); err != nil {
				fmt.Printf("Warning: failed to remove old temporary file %s: %v\n", filePath, err)
			} else {
				fmt.Printf("Cleaned up old temporary file: %s\n", filePath)
			}
		}
	}

	return nil
}

// GetWavPathForRecording returns the WAV file path for a recording ID
// If the file doesn't exist, it creates a temporary one from database data
func (a *App) GetWavPathForRecording(recordingID int) (string, error) {
	if a.database == nil {
		return "", errors.New("database not initialized")
	}

	recording, err := a.database.GetRecording(recordingID)
	if err != nil {
		return "", fmt.Errorf("failed to get recording: %w", err)
	}

	cfg := a.settings.Get()
	outDir := cfg.OutDir
	if outDir == "" {
		outDir = "./out"
	}

	wavPath := filepath.Join(outDir, recording.Filename)

	// Check if file exists
	if _, err := os.Stat(wavPath); err == nil {
		// File exists, return the path
		return wavPath, nil
	}

	// File doesn't exist, create it from database data
	if recording.AudioData == nil {
		return "", fmt.Errorf("no audio data available for recording %d", recordingID)
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := os.WriteFile(wavPath, recording.AudioData, 0644); err != nil {
		return "", fmt.Errorf("failed to create WAV file: %w", err)
	}

	return wavPath, nil
}

// DeleteRecording deletes a recording and all associated transcripts and summaries
func (a *App) DeleteRecording(recordingID int) error {
	if a.database == nil {
		return errors.New("database not initialized")
	}

	// Get recording first to check if it exists
	recording, err := a.database.GetRecording(recordingID)
	if err != nil {
		return fmt.Errorf("failed to get recording: %w", err)
	}

	// Delete the recording (this will cascade delete transcripts and summaries)
	if err := a.database.DeleteRecording(recordingID); err != nil {
		return fmt.Errorf("failed to delete recording: %w", err)
	}

	// Also try to clean up any remaining WAV file
	cfg := a.settings.Get()
	outDir := cfg.OutDir
	if outDir == "" {
		outDir = "./out"
	}
	wavPath := filepath.Join(outDir, recording.Filename)
	if err := os.Remove(wavPath); err != nil {
		// Don't fail if file doesn't exist or can't be removed
		fmt.Printf("Warning: failed to remove WAV file %s: %v\n", wavPath, err)
	}

	return nil
}

// DeleteTranscript deletes a transcript and all associated summaries
func (a *App) DeleteTranscript(transcriptID int) error {
	if a.database == nil {
		return errors.New("database not initialized")
	}

	return a.database.DeleteTranscript(transcriptID)
}

// DeleteSummary deletes a summary
func (a *App) DeleteSummary(summaryID int) error {
	if a.database == nil {
		return errors.New("database not initialized")
	}

	return a.database.DeleteSummary(summaryID)
}

// GetRecordingsWithDetails returns recordings with their transcripts and summaries
func (a *App) GetRecordingsWithDetails(limit int, offset int) ([]*db.RecordingWithDetails, error) {
	if a.database == nil {
		return nil, errors.New("database not initialized")
	}

	return a.database.GetRecordingsWithDetails(limit, offset)
}

// GetTranscriptsByRecordingID returns all transcripts for a recording
func (a *App) GetTranscriptsByRecordingID(recordingID int) ([]*db.Transcript, error) {
	if a.database == nil {
		return nil, errors.New("database not initialized")
	}

	return a.database.GetTranscriptsByRecordingID(recordingID)
}

// GetSummariesByTranscriptID returns all summaries for a transcript
func (a *App) GetSummariesByTranscriptID(transcriptID int) ([]*db.Summary, error) {
	if a.database == nil {
		return nil, errors.New("database not initialized")
	}

	return a.database.GetSummariesByTranscriptID(transcriptID)
}

// PickTxtFromOutDir opens a file picker defaulting to OutDir filtered to .txt
func (a *App) PickTxtFromOutDir() (string, error) {
	if a.uiCtx == nil {
		return "", errors.New("ui not ready")
	}
	cfg := a.settings.Get()
	path, err := wruntime.OpenFileDialog(a.uiCtx, wruntime.OpenDialogOptions{
		Title:            "Choose Transcript (.txt)",
		DefaultDirectory: cfg.OutDir,
		Filters:          []wruntime.FileFilter{{DisplayName: "Text", Pattern: "*.txt"}},
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// PickModelFile opens a file picker for selecting llama model files
func (a *App) PickModelFile() (string, error) {
	if a.uiCtx == nil {
		return "", errors.New("ui not ready")
	}
	path, err := wruntime.OpenFileDialog(a.uiCtx, wruntime.OpenDialogOptions{
		Title:            "Choose Llama Model",
		DefaultDirectory: "./models",
		Filters: []wruntime.FileFilter{
			{DisplayName: "GGUF Models", Pattern: "*.gguf"},
			{DisplayName: "All Files", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// ListRecordingsWithTranscripts returns recordings that have transcripts for selection
func (a *App) ListRecordingsWithTranscripts() ([]*db.RecordingWithDetails, error) {
	if a.database == nil {
		return nil, errors.New("database not initialized")
	}

	// Get recordings that have transcripts
	recordings, err := a.database.ListRecordings(0, 0, nil, nil)
	if err != nil {
		return nil, err
	}

	var result []*db.RecordingWithDetails
	for _, rec := range recordings {
		// Get detailed info including transcript
		details, err := a.database.GetRecordingWithDetails(rec.ID)
		if err != nil {
			continue // Skip if we can't get details
		}
		// Only include recordings that have transcripts
		if details.TranscriptID != nil {
			result = append(result, details)
		}
	}

	return result, nil
}

// GetRecordingFilePath returns the file path for a recording
func (a *App) GetRecordingFilePath(recordingID int) (string, error) {
	if a.database == nil {
		return "", errors.New("database not initialized")
	}

	recording, err := a.database.GetRecording(recordingID)
	if err != nil {
		return "", err
	}

	return recording.FilePath, nil
}

// GetTranscriptContent returns the transcript content for a recording
func (a *App) GetTranscriptContent(recordingID int) (string, error) {
	if a.database == nil {
		return "", errors.New("database not initialized")
	}

	transcript, err := a.database.GetTranscriptByRecordingID(recordingID)
	if err != nil {
		return "", err
	}

	return transcript.Content, nil
}

// GetRecordingsWithTranscripts returns recordings that have transcripts available for summarisation
func (a *App) GetRecordingsWithTranscripts() ([]*db.RecordingWithTranscript, error) {
	if a.database == nil {
		return nil, errors.New("database not initialized")
	}

	return a.database.GetRecordingsWithTranscripts()
}

// PickDatabaseFile opens a file picker for selecting a database file
func (a *App) PickDatabaseFile() (string, error) {
	if a.uiCtx == nil {
		return "", errors.New("ui not ready")
	}
	cfg := a.settings.Get()
	defaultDir := "./data"
	if cfg.DatabasePath != "" {
		defaultDir = filepath.Dir(cfg.DatabasePath)
	}
	path, err := wruntime.SaveFileDialog(a.uiCtx, wruntime.SaveDialogOptions{
		Title:            "Choose Database",
		DefaultDirectory: defaultDir,
		DefaultFilename:  "blackbox.db",
		Filters: []wruntime.FileFilter{
			{DisplayName: "SQLite Database", Pattern: "*.db"},
			{DisplayName: "All Files", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// SelectDatabase switches to a new database file
func (a *App) SelectDatabase(dbPath string) error {
	if dbPath == "" {
		return errors.New("database path cannot be empty")
	}

	// Close current database connection
	if a.database != nil {
		if err := a.database.Close(); err != nil {
			return fmt.Errorf("failed to close current database: %w", err)
		}
	}

	// Create new database connection
	newDB, err := db.NewDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open new database: %w", err)
	}
	a.database = newDB

	// Update settings with new database path
	cfg := a.settings.Get()
	cfg.DatabasePath = dbPath
	if err := a.settings.Save(cfg); err != nil {
		// If settings update fails, we should still keep the new database
		// but log the error
		fmt.Printf("Warning: failed to update settings with new database path: %v\n", err)
	}

	return nil
}

// startLlamaServer starts the llama-server with the configured parameters
func (a *App) startLlamaServer() error {
	a.llamaMu.Lock()
	defer a.llamaMu.Unlock()

	// Stop existing server if running
	if a.llamaServer != nil {
		a.stopLlamaServer()
	}

	cfg := a.settings.Get()
	if cfg.LlamaModel == "" {
		return errors.New("no model selected")
	}

	// Check if model file exists
	if _, err := os.Stat(cfg.LlamaModel); err != nil {
		return fmt.Errorf("model file not found: %w", err)
	}

	// Build llama-server command
	llamaBin := "./llamacpp-bin/llama-server.exe"
	if _, err := os.Stat(llamaBin); err != nil {
		return fmt.Errorf("llama-server.exe not found in llamacpp-bin directory")
	}

	args := []string{
		"--model", cfg.LlamaModel,
		"--host", "127.0.0.1",
		"--port", "8080",
		"--ctx-size", fmt.Sprintf("%d", cfg.LlamaContext),
		"--temp", fmt.Sprintf("%.2f", cfg.LlamaTemp),
		"--api-key", cfg.LlamaAPIKey,
	}

	cmd := exec.Command(llamaBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Hide CMD window on Windows
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start llama-server: %w", err)
	}

	a.llamaServer = cmd

	// Wait for server to be ready
	return a.waitForLlamaServer()
}

// stopLlamaServer stops the running llama-server
func (a *App) stopLlamaServer() {
	a.llamaMu.Lock()
	defer a.llamaMu.Unlock()

	if a.llamaServer != nil {
		// Try graceful shutdown first
		if a.llamaServer.Process != nil {
			a.llamaServer.Process.Kill()
		}
		// Wait for process to exit (with timeout)
		done := make(chan error, 1)
		go func() {
			done <- a.llamaServer.Wait()
		}()

		select {
		case <-done:
			// Process exited
		case <-time.After(5 * time.Second):
			// Force kill if it doesn't exit gracefully
			if a.llamaServer.Process != nil {
				a.llamaServer.Process.Kill()
			}
		}

		a.llamaServer = nil
	}
}

// waitForLlamaServer waits for the llama-server to be responsive
func (a *App) waitForLlamaServer() error {
	client := &http.Client{Timeout: 5 * time.Second}

	for i := 0; i < 30; i++ { // Wait up to 30 seconds
		resp, err := client.Get("http://127.0.0.1:8080/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}

	return errors.New("llama-server failed to start or become responsive")
}

// isLlamaServerRunning checks if the llama-server is currently running
func (a *App) isLlamaServerRunning() bool {
	a.llamaMu.Lock()
	defer a.llamaMu.Unlock()

	if a.llamaServer == nil {
		return false
	}

	// Check if process is still running
	if a.llamaServer.ProcessState != nil && a.llamaServer.ProcessState.Exited() {
		a.llamaServer = nil
		return false
	}

	// Test if server is responsive
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:8080/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

// GetAudioDataURL returns a base64-encoded data URL for the given WAV file or recording ID
func (a *App) GetAudioDataURL(wavPathOrID string) (string, error) {
	var audioData []byte

	// Check if it's a file path (for backwards compatibility)
	if _, err := os.Stat(wavPathOrID); err == nil {
		// It's a file path, read from file
		audioData, err = os.ReadFile(wavPathOrID)
		if err != nil {
			return "", fmt.Errorf("failed to read audio file: %v", err)
		}
	} else {
		// Try to parse as recording ID
		var recordingID int
		if _, parseErr := fmt.Sscanf(wavPathOrID, "%d", &recordingID); parseErr == nil {
			// It's a recording ID, get from database
			recording, dbErr := a.database.GetRecording(recordingID)
			if dbErr != nil {
				return "", fmt.Errorf("failed to get recording from database: %v", dbErr)
			}
			if recording.AudioData == nil {
				return "", fmt.Errorf("recording has no audio data stored in database")
			}
			audioData = recording.AudioData
		} else {
			return "", fmt.Errorf("invalid file path or recording ID: %s", wavPathOrID)
		}
	}

	// Encode as base64
	base64Data := base64.StdEncoding.EncodeToString(audioData)

	// Return as data URL
	return "data:audio/wav;base64," + base64Data, nil
}

// UpdateRecording updates a recording in the database
func (a *App) UpdateRecording(recordingID int, updates map[string]interface{}) error {
	// Get the current recording
	recording, err := a.database.GetRecording(recordingID)
	if err != nil {
		return fmt.Errorf("failed to get recording: %w", err)
	}

	// Apply updates - only date/time for now
	if recordedAt, ok := updates["recorded_at"].(string); ok {
		if t, err := time.Parse("2006-01-02T15:04:05", recordedAt); err == nil {
			recording.RecordedAt = &t
		}
	}

	// Update in database
	return a.database.UpdateRecording(recording)
}

// UpdateSummary updates a summary in the database
func (a *App) UpdateSummary(summaryID int, updates map[string]interface{}) error {
	// Get the current summary
	summary, err := a.database.GetSummary(summaryID)
	if err != nil {
		return fmt.Errorf("failed to get summary: %w", err)
	}

	// Apply updates - only model for now
	if modelUsed, ok := updates["model_used"].(string); ok {
		summary.ModelUsed = modelUsed
	}

	// Update in database
	return a.database.UpdateSummary(summary)
}
