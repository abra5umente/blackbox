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
	promptCache    map[string]PromptConfig
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
		promptCache:    make(map[string]PromptConfig),
	}

	// Load default prompts
	if err := app.loadDefaultPrompts(); err != nil {
		return nil, fmt.Errorf("failed to load default prompts: %w", err)
	}

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

// GetAvailablePrompts returns a list of available prompt configurations
func (a *App) GetAvailablePrompts() ([]PromptConfig, error) {
	a.promptMu.RLock()
	defer a.promptMu.RUnlock()

	// Load custom prompts from config directory
	if err := a.loadCustomPrompts(); err != nil {
		// Log error but don't fail - custom prompts are optional
		fmt.Printf("Warning: failed to load custom prompts: %v\n", err)
	}

	var prompts []PromptConfig
	for _, prompt := range a.promptCache {
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

	// Check if prompt exists
	if _, exists := a.promptCache[promptName]; !exists {
		return fmt.Errorf("prompt '%s' not found", promptName)
	}

	a.selectedPrompt = promptName
	return nil
}

// GetPromptConfig returns the configuration for a specific prompt
func (a *App) GetPromptConfig(promptName string) (PromptConfig, error) {
	a.promptMu.RLock()
	defer a.promptMu.RUnlock()

	prompt, exists := a.promptCache[promptName]
	if !exists {
		return PromptConfig{}, fmt.Errorf("prompt '%s' not found", promptName)
	}

	return prompt, nil
}

// SaveCustomPrompt saves a custom prompt configuration
func (a *App) SaveCustomPrompt(config PromptConfig) error {
	if config.Name == "" {
		return errors.New("prompt name is required")
	}

	// Ensure config directory exists
	if err := os.MkdirAll("./config", 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Save to file
	filename := fmt.Sprintf("./config/%s.json", config.Name)
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal prompt config: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write prompt file: %w", err)
	}

	// Update cache
	a.promptMu.Lock()
	a.promptCache[config.Name] = config
	a.promptMu.Unlock()

	return nil
}

// loadDefaultPrompts loads the built-in prompt configurations
func (a *App) loadDefaultPrompts() error {
	defaultPrompts := []string{"meeting", "dictation"}

	for _, promptName := range defaultPrompts {
		filename := fmt.Sprintf("./config/%s.json", promptName)
		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", filename, err)
		}

		var config PromptConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse %s: %w", filename, err)
		}

		a.promptCache[promptName] = config
	}

	return nil
}

// loadCustomPrompts loads custom prompt files from the config directory
func (a *App) loadCustomPrompts() error {
	configDir := "./config"
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Skip default prompts
		if entry.Name() == "meeting.json" || entry.Name() == "dictation.json" {
			continue
		}

		filename := filepath.Join(configDir, entry.Name())
		data, err := os.ReadFile(filename)
		if err != nil {
			continue // Skip files that can't be read
		}

		var config PromptConfig
		if err := json.Unmarshal(data, &config); err != nil {
			continue // Skip files that can't be parsed
		}

		// Use filename without extension as key
		promptName := strings.TrimSuffix(entry.Name(), ".json")
		a.promptCache[promptName] = config
	}

	return nil
}

// --- Recording API ---

func (a *App) IsRecording() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.recording
}

// StartRecording starts loopback (and optional mic) capture and writes to a new WAV file under OutDir.
// Returns the path to the WAV file that will be written.
func (a *App) StartRecording(withMic bool) (string, error) {
	return a.StartRecordingAdvanced(withMic, false)
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

	// Get final file size
	fileInfo, err := os.Stat(wavPath)
	if err != nil {
		return wavPath, fmt.Errorf("stat wav file: %w", err)
	}

	// Calculate duration based on file size and audio format
	// PCM S16LE, 16kHz, mono: 2 bytes per sample, 16000 samples per second
	durationSeconds := float64(fileInfo.Size()) / (16000.0 * 2.0)

	// Update recording in database
	dbRecording, err := a.database.GetRecording(a.recordingID)
	if err != nil {
		return wavPath, fmt.Errorf("failed to get recording from database: %w", err)
	}

	dbRecording.FileSize = fileInfo.Size()
	dbRecording.DurationSeconds = &durationSeconds

	if err := a.database.UpdateRecording(dbRecording); err != nil {
		return wavPath, fmt.Errorf("failed to update recording in database: %w", err)
	}

	// Clear recording ID
	a.recordingID = 0

	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		return wavPath, runErr
	}
	return wavPath, nil
}

// Transcribe runs whisper.cpp on the selected WAV and returns the produced .txt path.
func (a *App) Transcribe(wavPath string) (string, error) {
	if strings.TrimSpace(wavPath) == "" {
		return "", errors.New("wav path required")
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

	startTime := time.Now()
	txtPath, err := execx.RunWhisper(whisperBin, modelPath, wavPath, outDir, "en", 0, "")
	if err != nil {
		return "", err
	}

	// Read transcript content
	transcriptContent, err := os.ReadFile(txtPath)
	if err != nil {
		return txtPath, fmt.Errorf("failed to read transcript file: %w", err)
	}

	// Find recording by filename
	filename := filepath.Base(wavPath)
	dbRecording, err := a.database.GetRecordingByFilename(filename)
	if err != nil {
		return txtPath, fmt.Errorf("failed to find recording in database: %w", err)
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
		return txtPath, fmt.Errorf("failed to save transcript to database: %w", err)
	}

	return txtPath, nil
}

// Summarise reads configs/llm.json and sends the transcript to OpenAI or local AI for summarisation.
func (a *App) Summarise(txtPathOrID string) (string, error) {
	if strings.TrimSpace(txtPathOrID) == "" {
		return "", errors.New("txt path or recording ID required")
	}

	uiCfg := a.settings.Get()

	var transcript string
	var err error

	// Check if it's a file path (for backwards compatibility)
	if _, err := os.Stat(txtPathOrID); err == nil {
		// It's a file path, read from file
		transcriptBytes, err := os.ReadFile(txtPathOrID)
		if err != nil {
			return "", fmt.Errorf("failed to read transcript: %w", err)
		}
		transcript = string(transcriptBytes)
	} else {
		// Try to parse as recording ID
		var recordingID int
		if _, parseErr := fmt.Sscanf(txtPathOrID, "%d", &recordingID); parseErr == nil {
			// It's a recording ID, get transcript from database
			recording, dbErr := a.database.GetRecording(recordingID)
			if dbErr != nil {
				return "", fmt.Errorf("failed to get recording from database: %v", dbErr)
			}

			// Get the transcript for this recording
			dbTranscript, transErr := a.database.GetTranscriptByRecordingID(recording.ID)
			if transErr != nil {
				return "", fmt.Errorf("failed to get transcript from database: %v", transErr)
			}
			transcript = dbTranscript.Content
		} else {
			return "", fmt.Errorf("invalid file path or recording ID: %s", txtPathOrID)
		}
	}

	// Get the selected prompt configuration
	promptConfig, err := a.GetPromptConfig(a.GetSelectedPrompt())
	if err != nil {
		return "", fmt.Errorf("failed to get prompt config: %w", err)
	}
	prompt := promptConfig.Prompt

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

	// Find transcript by filename (only if it's a file path)
	var dbRecording *db.Recording
	var transcriptErr error

	if _, err := os.Stat(txtPathOrID); err == nil {
		// It's a file path
		txtFilename := filepath.Base(txtPathOrID)
		wavFilename := strings.TrimSuffix(txtFilename, ".txt") + ".wav"
		dbRecording, transcriptErr = a.database.GetRecordingByFilename(wavFilename)
		if transcriptErr != nil {
			return "", fmt.Errorf("failed to find recording: %w", transcriptErr)
		}
	} else {
		// It's a recording ID, we already have the recording from earlier
		// dbRecording should already be set from the earlier database lookup
		if dbRecording == nil {
			return "", fmt.Errorf("recording not found for ID: %s", txtPathOrID)
		}
	}

	// Get transcript
	dbTranscript, err := a.database.GetTranscriptByRecordingID(dbRecording.ID)
	if err != nil {
		return "", fmt.Errorf("failed to find transcript in database: %w", err)
	}

	// Determine model used and endpoint
	modelUsed := "unknown"
	var apiEndpoint *string
	var localModelPath *string

	if uiCfg.UseLocalAI {
		modelUsed = "llama-local"
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
		SummaryType:    a.GetSelectedPrompt(),
		ModelUsed:      modelUsed,
		PromptUsed:     prompt,
		APIEndpoint:    apiEndpoint,
		LocalModelPath: localModelPath,
	}

	if err := a.database.CreateSummary(dbSummary); err != nil {
		return "", fmt.Errorf("failed to save summary to database: %w", err)
	}

	// Write summary to output file if file backups are enabled
	var outputPath string
	if uiCfg.EnableFileBackups {
		// Determine output path based on whether input was file or ID
		if _, err := os.Stat(txtPathOrID); err == nil {
			// It was a file path
			outputPath = strings.TrimSuffix(txtPathOrID, filepath.Ext(txtPathOrID)) + "_summary.txt"
		} else {
			// It was a recording ID, create output in default directory
			cfg := a.settings.Get()
			outputPath = filepath.Join(cfg.OutDir, fmt.Sprintf("%s_summary.txt", txtPathOrID))
		}

		if err := os.WriteFile(outputPath, []byte(summary), 0644); err != nil {
			return "", fmt.Errorf("failed to write summary: %w", err)
		}
	}

	return fmt.Sprintf("Summary saved to database (ID: %d)\n\n--- Summary ---\n%s", dbSummary.ID, summary), nil
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
func (a *App) StartRecordingAdvanced(withMic bool, dictation bool) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.recording {
		return "", errors.New("already recording")
	}

	cfg := a.settings.Get()
	if err := os.MkdirAll(cfg.OutDir, 0755); err != nil {
		return "", err
	}

	const sampleRate uint32 = 16000 // Reduced from 48000 - 16kHz is standard for speech recognition
	const channels uint32 = 1       // Reduced from 2 - mono is sufficient for speech and cuts file size in half
	const bits uint16 = 16

	startTime := time.Now()
	ts := startTime.Format("20060102_150405")
	wavPath := filepath.Join(cfg.OutDir, ts+".wav")
	writer, err := wav.NewWriter(wavPath, sampleRate, uint16(channels), bits)
	if err != nil {
		return "", fmt.Errorf("open wav: %w", err)
	}

	// Create recording entry in database
	recordingMode := "loopback"
	if dictation {
		recordingMode = "dictation"
	} else if withMic {
		recordingMode = "mixed"
	}

	dbRecording := &db.Recording{
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
	}

	if err := a.database.CreateRecording(dbRecording); err != nil {
		_ = writer.Close()
		return "", fmt.Errorf("failed to create recording in database: %w", err)
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
	a.recordingID = dbRecording.ID
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

// ImportData imports existing recordings, transcripts, and summaries from a directory
func (a *App) ImportData(importDir string, dryRun bool, autoDetectMode bool) (map[string]interface{}, error) {
	if a.database == nil {
		return nil, errors.New("database not initialized")
	}

	// Get current settings
	currentSettings := a.settings.Get()

	// Create a temporary config for the import
	config := map[string]interface{}{
		"database_path":    currentSettings.OutDir + "/data/blackbox.db",
		"import_dir":       importDir,
		"dry_run":          dryRun,
		"verbose":          true,
		"batch_size":       100,
		"auto_detect_mode": autoDetectMode,
		"default_mode":     "loopback",
	}

	// Save config to a temporary file
	tempConfigPath := filepath.Join(currentSettings.OutDir, "config", "temp_import.json")
	if err := os.MkdirAll(filepath.Dir(tempConfigPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %v", err)
	}

	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(tempConfigPath, configData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write temp config: %v", err)
	}

	// Find the import executable - check multiple possible locations
	var importExePath string

	// Method 1: Check same directory as GUI executable
	if exePath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), "import.exe")
		if _, err := os.Stat(candidate); err == nil {
			importExePath = candidate
		}
	}

	// Method 2: Check root directory relative to GUI location
	if importExePath == "" {
		if exePath, err := os.Executable(); err == nil {
			// Go up two levels from build/bin to reach project root
			rootDir := filepath.Dir(filepath.Dir(exePath))
			candidate := filepath.Join(rootDir, "import.exe")
			if _, err := os.Stat(candidate); err == nil {
				importExePath = candidate
			}
		}
	}

	// Method 3: Check current working directory
	if importExePath == "" {
		candidate := "./import.exe"
		if _, err := os.Stat(candidate); err == nil {
			importExePath = candidate
		}
	}

	if importExePath == "" {
		return nil, fmt.Errorf("import executable not found - please ensure import.exe is built and available")
	}

	// Run the import command
	cmd := exec.Command(importExePath, "run", "--config", tempConfigPath)
	cmd.Dir = "."

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	// Clean up temp config
	os.Remove(tempConfigPath)

	if err != nil {
		return nil, fmt.Errorf("import failed: %v\nStderr: %s", err, stderr.String())
	}

	// Parse the results
	result := map[string]interface{}{
		"success": true,
		"message": "Import completed successfully",
		"stdout":  stdout.String(),
		"stderr":  stderr.String(),
	}

	return result, nil
}

// GetImportProgress returns the current import progress (placeholder for now)
func (a *App) GetImportProgress() (map[string]interface{}, error) {
	// This would need to be implemented with proper progress tracking
	// For now, return a simple status
	return map[string]interface{}{
		"status":  "ready",
		"message": "Import system ready",
	}, nil
}

// ValidateImportDirectory validates that a directory contains importable files
func (a *App) ValidateImportDirectory(importDir string) (map[string]interface{}, error) {
	// Check if directory exists
	if _, err := os.Stat(importDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory does not exist: %s", importDir)
	}

	// Count files
	wavFiles := 0
	txtFiles := 0
	summaryFiles := 0

	err := filepath.Walk(importDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			filename := strings.ToLower(info.Name())
			if strings.HasSuffix(filename, ".wav") {
				wavFiles++
			} else if strings.HasSuffix(filename, ".txt") {
				if strings.HasSuffix(filename, "_summary.txt") {
					summaryFiles++
				} else {
					txtFiles++
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %v", err)
	}

	return map[string]interface{}{
		"valid":       true,
		"wav_files":   wavFiles,
		"transcripts": txtFiles,
		"summaries":   summaryFiles,
		"total_files": wavFiles + txtFiles + summaryFiles,
	}, nil
}
