package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// Summarise is a stub: reads configs/llm.json and returns the message it would POST.
func (a *App) Summarise(txtPath string) (string, error) {
	if strings.TrimSpace(txtPath) == "" {
		return "", errors.New("txt path required")
	}
	if _, err := os.Stat(txtPath); err != nil {
		return "", err
	}
	cfg, err := a.loadLLMConfig("./configs/llm.json")
	if err != nil {
		return "", err
	}
	apiKey := os.Getenv(cfg.APIKeyEnv)
	_ = apiKey // just to mirror CLI behaviour; no call is made
	absTxt, _ := filepath.Abs(txtPath)
	msg := fmt.Sprintf("Would POST to %s/chat/completions with model=%s using key env %s for file %s", cfg.BaseURL, cfg.Model, cfg.APIKeyEnv, absTxt)
	return msg, nil
}

// Helper: load LLM config shared with CLI semantics
type llmConfig struct {
	BaseURL   string `json:"base_url"`
	APIKeyEnv string `json:"api_key_env"`
	Model     string `json:"model"`
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
	if cfg.BaseURL == "" || cfg.Model == "" || cfg.APIKeyEnv == "" {
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

// mixS16Stereo mixes two S16LE buffers (same channels) with simple averaging.
func mixS16Stereo(loop, mic []byte) []byte {
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

	const sampleRate uint32 = 48000
	const channels uint32 = 2
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
						mixed := mixS16Stereo(b, micBuf)
						if _, err := writer.Write(mixed); err != nil {
							runErrCh <- err
							return
						}
					} else {
						if _, err := writer.Write(b); err != nil {
							runErrCh <- err
							return
						}
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
