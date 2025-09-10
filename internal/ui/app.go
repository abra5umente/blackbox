package ui

import (
	"bytes"
	"context"
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
	"blackbox/internal/execx"
	"blackbox/internal/wav"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App exposes methods to the Wails frontend.
type App struct {
	settings *SettingsStore

	mu          sync.Mutex
	recording   bool
	dictation   bool
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
	return &App{settings: store}, nil
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
	if err := os.MkdirAll(cfg.OutDir, 0755); err != nil {
		return UISettings{}, err
	}
	if err := a.settings.Save(cfg); err != nil {
		return UISettings{}, err
	}
	return a.settings.Get(), nil
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

	txtPath, err := execx.RunWhisper(whisperBin, modelPath, wavPath, outDir, "en", 0, "")
	if err != nil {
		return "", err
	}
	return txtPath, nil
}

// Summarise reads configs/llm.json and sends the transcript to OpenAI or local AI for summarisation.
func (a *App) Summarise(txtPath string) (string, error) {
	if strings.TrimSpace(txtPath) == "" {
		return "", errors.New("txt path required")
	}
	if _, err := os.Stat(txtPath); err != nil {
		return "", err
	}

	uiCfg := a.settings.Get()

	// Read the transcript file
	transcript, err := os.ReadFile(txtPath)
	if err != nil {
		return "", fmt.Errorf("failed to read transcript: %w", err)
	}

	// Create the summarisation prompt
	prompt := `You are a specialised transcript summariser. Your ONLY purpose is to read meeting transcripts
and produce comprehensive, well-structured summaries. You are verbose, detailed, and explanatory,
but still clear and readable. Never invent facts, names, or dates—if information is missing,
write "Unknown." Always use Australian spelling.

Instructions:
- Write in Markdown with clear headings and subheadings.
- Begin with an **Executive Summary**: 5–8 bullets that describe the meeting's purpose,
  key themes, overall tone, and main outcomes in slightly more detail than a brief recap.
- Create **Dynamic Thematic Sections**:
  • Identify 3–6 dominant themes in the transcript.  
  • For each theme, create a heading (≤6 words) that reflects the content.  
  • Under each heading, write 3–6 bullets that capture facts, reasoning, and context
    (not just short fragments). Each bullet should be 1–3 sentences.  
- Provide **Decisions & Rationale**:
  • List all decisions made, who agreed (if stated), and when they take effect.  
  • Include short explanations of why the decision was made, if mentioned.  
- Provide **Action Items**:
  • Use a table: Owner | Action | Due (if stated) | Priority (H/M/L).  
  • Add 1–2 sentence descriptions for context beneath each action item if needed.  
- Provide **Risks / Blockers**:
  • For each, include Risk, Impact, Mitigation (if given), and Confidence (High/Med/Low).  
  • Expand with a sentence of explanation for clarity.  
- Provide **Open Questions**:
  • List unresolved issues or uncertainties. Include context if available.  
- Provide **Per-Speaker Highlights** (optional):
  • If distinct speakers are clear, summarise each speaker's key contributions.  
  • Use "Speaker A / Speaker B" if no names are provided.  
- Provide **Notable Quotes**:
  • Select 2–4 direct quotes that highlight tone, attitude, or memorable phrasing.  
- End with **Next Steps / Follow-ups**:
  • 3–5 bullets describing agreed future work or items to revisit.  

Style:
- Be descriptive and explanatory. Expand on reasoning where visible in the transcript.
- Each bullet can be 2–3 sentences if needed; clarity and completeness matter more than brevity.
- Avoid fluff, but don't oversimplify—capture nuance and context.
- Never output anything except the summary.`

	var summary string

	if uiCfg.UseLocalAI {
		// Use local AI (llama.cpp) - load from local.json
		summary, err = a.summariseWithLocalAI(string(transcript), prompt)
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

	// Write summary to output file
	outputPath := strings.TrimSuffix(txtPath, filepath.Ext(txtPath)) + "_summary.txt"
	if err := os.WriteFile(outputPath, []byte(summary), 0644); err != nil {
		return "", fmt.Errorf("failed to write summary: %w", err)
	}

	return fmt.Sprintf("Summary written to: %s\n\n--- Summary ---\n%s", outputPath, summary), nil
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

	ts := time.Now().Format("20060102_150405")
	wavPath := filepath.Join(cfg.OutDir, ts+".wav")
	writer, err := wav.NewWriter(wavPath, sampleRate, uint16(channels), bits)
	if err != nil {
		return "", fmt.Errorf("open wav: %w", err)
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
