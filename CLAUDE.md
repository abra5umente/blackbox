# CLAUDE.md - Blackbox Project Documentation

## Project Overview

**Blackbox** is a Windows desktop application for real-time audio capture, automatic transcription, and AI-powered summarization. It captures system audio via WASAPI loopback and microphone input, transcribes using whisper.cpp, and generates summaries using either remote APIs (OpenAI) or local AI (llama.cpp).

**Key Metrics:**
- **Language**: Go 1.24.5 + JavaScript
- **Framework**: Wails v2.10.2 (Desktop GUI)
- **Platform**: Windows 11+ only
- **License**: MIT
- **Total LOC**: ~3,200+ lines
- **Architecture**: Monolithic desktop app with backend/frontend split

## Architecture Overview

### High-Level Architecture

```
┌─────────────────────────────────────┐
│      Frontend (HTML/JS/CSS)         │
│     - Tabbed UI interface           │
│     - Real-time spectrum analyser   │
│     - Audio playback                │
│     - Settings management           │
└──────────────┬──────────────────────┘
               │ Wails IPC Events
┌──────────────▼──────────────────────┐
│    Backend (Go) - Main Controller   │
│  internal/ui/app.go (976 LOC)       │
│     - Recording orchestration       │
│     - External process management   │
│     - AI API integration            │
│     - Settings persistence          │
└──────────────┬──────────────────────┘
               │
      ┌────────┼────────┬──────────┐
      │        │        │          │
  ┌───▼──┐ ┌──▼──┐ ┌───▼───┐ ┌──▼────┐
  │Audio │ │ WAV │ │External│ │Settings
  │      │ │     │ │Process │ │Store  │
  │-Loop │ │Writer│ │-whisper│-Config
  │-Mic  │ │RIFF │ │-llama │ │Storage
  └───┬──┘ └──┬──┘ └───┬───┘ └──┬────┘
      │       │        │       │
  ┌───▼───────▼────────▼───────▼──┐
  │    External Dependencies       │
  │ - malgo (WASAPI loopback)      │
  │ - whisper-cli (transcription)  │
  │ - llama-server (local AI)      │
  │ - OpenAI/compatible API        │
  └────────────────────────────────┘
```

### Core Workflows

#### Complete Recording → Transcription → Summarization
```
User clicks "Begin"
    ↓
App.StartRecording(withMic, dictation)
    ├─ Create WAV file
    ├─ Start audio recorders (loopback ± mic)
    ├─ Launch writer goroutine
    ├─ Emit audio data events (60fps)
    └─ Return WAV path
    ↓
App.emitAudioData() → Frontend
    ├─ Wails EventsEmit("audioData", {...})
    └─ Frontend RealSpectrumAnalyser updates
    ↓
User clicks "Stop Recording"
    ↓
App.StopRecording()
    ├─ Stop recorders
    ├─ Finalize WAV headers
    └─ Return final path
    ↓
Frontend calls App.Transcribe(wavPath)
    ├─ Validates WAV exists
    ├─ execx.RunWhisper() spawns whisper-cli
    ├─ Generates transcript.txt
    └─ Returns TXT path
    ↓
Frontend calls App.Summarise(txtPath)
    ├─ Reads prompt configuration
    ├─ Checks use_local_ai flag
    ├─ If local: starts llama-server, sends request
    ├─ If remote: sends to OpenAI/compatible API
    ├─ Saves summary to _summary.txt
    └─ Returns formatted output
    ↓
Frontend:
    ├─ Renders markdown summary
    ├─ Displays audio player
    └─ Shows system messages
```

## Directory Structure

```
/home/alex/projects/blackbox/
├── main.go                            # Wails entry point
├── go.mod / go.sum                    # Go dependency management
├── wails.json                         # Wails configuration
├── package.json                       # npm scripts (build automation)
│
├── internal/                          # Core backend packages
│   ├── audio/                         # Audio capture system
│   │   ├── loopback.go               # WASAPI loopback recorder (142 LOC)
│   │   └── mic.go                    # Microphone recorder (101 LOC)
│   ├── ui/                            # GUI backend services
│   │   ├── app.go                    # Main application controller (976 LOC)
│   │   └── settings.go               # Settings store (115 LOC)
│   ├── wav/                           # WAV file handling
│   │   └── writer.go                 # WAV file writer (150 LOC)
│   └── execx/                         # External process execution
│       └── execx.go                  # whisper.cpp wrapper (92 LOC)
│
├── frontend/                          # GUI frontend (Wails assets)
│   ├── dist/
│   │   ├── index.html                # Main UI (781 lines)
│   │   ├── input.css                 # Tailwind input directives
│   │   └── output.css                # Generated CSS (auto-built)
│   ├── wailsjs/                      # Auto-generated Wails bindings
│   │   ├── go/ui/App.js             # JavaScript bindings
│   │   └── runtime/runtime.js       # Wails runtime
│   ├── assets.go                     # Go embed file (embeds dist/)
│   ├── tailwind.config.js            # Tailwind CSS configuration
│   └── postcss.config.js             # PostCSS plugins config
│
├── config/                            # Prompt configuration directory
│   ├── README.md                     # Prompt creation guide
│   ├── meeting.json                  # Default: Meeting transcript prompt
│   ├── dictation.json                # Default: Dictation notes prompt
│   └── technical.json                # Default: Technical documentation prompt
│
├── configs/                           # LLM service configuration
│   └── llm.example.json              # Example LLM endpoint config
│
└── build/                             # Build output directory
    └── bin/
        └── blackbox-gui.exe          # Final executable
```

## Key Components

### 1. Audio Capture System (internal/audio/)

#### Loopback Recorder (`loopback.go`)
**Purpose**: Captures system audio (speaker output) via WASAPI loopback

**Key Methods**:
```go
NewRecorder(bufferCallbacks int) (*Recorder, error)
Start(sampleRate, channels uint32) error
Stop()
Data() <-chan []byte                    // Returns audio stream channel
RunUntil(ctx context.Context, sink func([]byte) error) error
```

**Technical Details**:
- Uses `malgo` (miniaudio wrapper) for WASAPI interface
- PCM S16LE format (16-bit signed little-endian)
- 16 kHz sample rate (optimized for speech recognition)
- Mono channels (channels=1)
- Buffered channel with drop-on-slow-consumer strategy
- Windows build tag: `//go:build windows`

**Location**: `internal/audio/loopback.go:1-142`

#### Microphone Recorder (`mic.go`)
**Purpose**: Captures microphone input via WASAPI capture

**Key Methods**: Same interface as Loopback Recorder

**Location**: `internal/audio/mic.go:1-101`

#### Audio Mixing
**Function**: `mixS16Mono(loop, mic []byte) []byte`

**Algorithm**:
```go
for each 16-bit sample pair:
    loopback_value = decode S16 from bytes
    mic_value = decode S16 from bytes
    mixed = (loopback_value + mic_value) / 2
    clip to [-32768, 32767]
    encode back to bytes
```

**Use Cases**:
- **Loopback Only**: No mixing (pure system audio)
- **Loopback + Mic**: Average mixing (prevents clipping)
- **Dictation**: Mic only (no loopback)

### 2. WAV File System (internal/wav/writer.go)

**Purpose**: Writes properly formatted RIFF WAV files with correct headers

**Key Methods**:
```go
NewWriter(path, sampleRate, channels, bitsPerSample) (*Writer, error)
Write(p []byte) (int, error)                    // Write PCM data
Flush() error                                   // Force buffered data to disk
Close() error                                   // Finalize RIFF headers
```

**Implementation Details**:
- 1 MiB internal buffer for efficient I/O
- Periodic flushing during recording (500ms ticker)
- Placeholder headers on creation, updated on close
- Only supports 16-bit PCM

**Location**: `internal/wav/writer.go:1-150`

### 3. Main Application Controller (internal/ui/app.go)

**Type**: App Struct (976 LOC)
```go
type App struct {
    // Settings
    settings *SettingsStore

    // Recording state
    mu          sync.Mutex
    recording   bool
    dictation   bool
    rec         *audio.Recorder       // Loopback recorder
    mic         *audio.MicRecorder    // Microphone recorder
    writer      *wav.Writer           // WAV file writer
    ctx         context.Context       // Recording context
    cancel      context.CancelFunc    // Stop recording signal

    // Llama server management
    llamaServer *exec.Cmd
    llamaMu     sync.Mutex

    // Prompt management
    selectedPrompt string
    promptCache    map[string]PromptConfig
    promptMu       sync.RWMutex

    // Wails context
    uiCtx context.Context
}
```

**Core Methods**:

| Method | Purpose | Returns | Location |
|--------|---------|---------|----------|
| `StartRecording(withMic)` | Begin audio capture | WAV path, error | app.go:220 |
| `StartRecordingAdvanced()` | Dictation or loopback mode | WAV path, error | app.go:230 |
| `StopRecording()` | Stop capture and finalize WAV | WAV path, error | app.go:370 |
| `Transcribe(wavPath)` | Run whisper on WAV file | TXT path, error | app.go:450 |
| `Summarise(txtPath)` | Generate summary via AI | Summary text, error | app.go:520 |
| `emitAudioData()` | Send real-time audio to frontend | N/A (event) | app.go:340 |

### 4. Settings Management (internal/ui/settings.go)

**Type**: SettingsStore (thread-safe JSON persistence)

```go
type UISettings struct {
    OutDir        string      // Output directory
    UseLocalAI    bool        // Enable local LLM
    LlamaTemp     float64     // Temperature (0.0-2.0)
    LlamaContext  int         // Context window size
    LlamaModel    string      // Path to GGUF model file
    LlamaAPIKey   string      // API key for llama-server
}
```

**Storage**: `./config/ui.json` (auto-created with defaults)

**Location**: `internal/ui/settings.go:1-115`

### 5. External Process Execution (internal/execx/execx.go)

**Purpose**: Manages execution of whisper.cpp for transcription

**Key Functions**:
```go
RunWhisper(whisperBin, modelPath, wavPath, outDir, lang, threads, extraArgs) (string, error)
BuildWhisperArgs(modelPath, wavPath, lang, threads, outBase, extraArgs) []string
```

**Whisper Integration**:
```bash
whisper-cli -m <modelPath>
            -f <wavPath>
            -otxt
            -l en
            -of <outputBase>
```

**Location**: `internal/execx/execx.go:1-92`

## Configuration

### Application Settings (config/ui.json)

```json
{
    "out_dir": "./out",
    "use_local_ai": false,
    "llama_temp": 0.1,
    "llama_context": 32000,
    "llama_model": "",
    "llama_api_key": ""
}
```

### AI Service Configuration (configs/)

**Remote API** (`remote.json`):
```json
{
    "base_url": "https://api.openai.com/v1",
    "api_key": "sk-proj-...",
    "model": "gpt-4o-mini"
}
```

**Local AI** (`local.json`):
```json
{
    "base_url": "http://127.0.0.1:8080",
    "api_key": "1234",
    "model": "gemma-3-12b-it-q4_0.gguf"
}
```

### Prompt Configuration (config/)

**Structure of each prompt file**:
```json
{
    "name": "prompt_name",
    "description": "What this prompt does",
    "prompt": "Detailed system prompt for AI model"
}
```

**Default Prompts**:
- `meeting.json` - Executive summary, themes, decisions, action items
- `dictation.json` - Summary, topics, important details, action items
- `technical.json` - Technical summary, decisions, implementation notes

## API Reference

### Wails IPC API - Backend Methods Exposed to Frontend

**Recording**:
```typescript
StartRecording(withMic: boolean): Promise<string>
StartRecordingAdvanced(withMic: boolean, dictation: boolean): Promise<string>
StopRecording(): Promise<string>
IsRecording(): boolean
```

**File Selection**:
```typescript
PickWavFromOutDir(): Promise<string>
PickTxtFromOutDir(): Promise<string>
PickModelFile(): Promise<string>
```

**Processing**:
```typescript
Transcribe(wavPath: string): Promise<string>
Summarise(txtPath: string): Promise<string>
```

**Settings Management**:
```typescript
GetSettings(): Promise<UISettings>
SaveSettings(jsonStr: string): Promise<UISettings>
```

**Prompt Management**:
```typescript
GetAvailablePrompts(): Promise<PromptConfig[]>
GetSelectedPrompt(): string
SetSelectedPrompt(promptName: string): Promise<void>
GetPromptConfig(promptName: string): Promise<PromptConfig>
SaveCustomPrompt(config: PromptConfig): Promise<void>
```

### Wails Runtime Events - Real-Time Communication

**Frontend Listener**:
```javascript
window.go.runtime.EventsOn("audioData", (data) => {
    // data = {
    //   source: "loopback" | "microphone"
    //   data: ArrayBuffer | Uint8Array | Array | base64 string
    //   length: number
    // }
});
```

**Backend Emitter**:
```go
wruntime.EventsEmit(a.uiCtx, "audioData", map[string]interface{}{
    "source": source,
    "data":   data,
    "length": len(data),
})
```

## External Integrations

### Speech Recognition (whisper.cpp)

**Binary Execution**:
```bash
./whisper-bin/whisper-cli.exe \
    -m <model_file> \
    -f <audio_file.wav> \
    -otxt \
    -l en \
    [-t <threads>] \
    [-of <output_base>]
```

**Supported Models**:
- ggml-tiny.en.bin (39 MB)
- ggml-base.en.bin (140 MB) - Default
- ggml-small.en.bin (466 MB)

### Remote AI APIs (OpenAI-compatible)

**Chat Completion API**:
```
POST <base_url>/chat/completions
Authorization: Bearer <api_key>
Content-Type: application/json

{
    "model": "<model_name>",
    "messages": [
        {"role": "system", "content": "<prompt>"},
        {"role": "user", "content": "<transcript>"}
    ],
    "max_completion_tokens": 2000
}
```

**Supported Services**:
- OpenAI (GPT-4, GPT-5)
- Anthropic Claude (via proxy)
- Ollama
- LM Studio
- Any OpenAI-compatible endpoint

**Timeout**: 360 seconds

### Local AI (llama.cpp)

**Server Management**:
```bash
./llamacpp-bin/llama-server.exe \
    --model <model_file> \
    --host 127.0.0.1 \
    --port 8080 \
    --ctx-size <context_window> \
    --temp <temperature> \
    --api-key <key>
```

**Health Check**:
```
GET http://127.0.0.1:8080/health
Expected: Status 200
Retry: Up to 30 times (30 seconds total)
```

**Lifecycle**:
1. Check if server already running
2. Start server if needed (waits for health check)
3. Send request
4. Shutdown server after completion

## Build & Deployment

### Development

```bash
wails dev
# Starts hot-reload server with Tailwind CSS watcher
```

### Production

```bash
npm run build:gui
# Equivalent to: npm run build:css && wails build -clean
```

**Output**: `build/bin/blackbox-gui.exe` (~15 MB)

### Build Pipeline

```
Step 1: CSS Generation
├─ Input: frontend/dist/input.css
├─ Process: tailwindcss -i input.css -o output.css
└─ Output: frontend/dist/output.css

Step 2: Frontend Assets Preparation
├─ Collect: frontend/dist/*
├─ Generate: frontend/assets.go (Go embed)
└─ Ready: Wails asset server

Step 3: Go Compilation
├─ Build: go build (or wails build)
├─ Link: Embed frontend/assets.go
└─ Output: build/bin/blackbox-gui.exe
```

### Prerequisites

**Development**:
- Go 1.24+
- Node.js & npm
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- C compiler (mingw on Windows)

**Runtime**:
- Windows 11+ (with WebView2 runtime)
- Optional: whisper-cli binary + model
- Optional: llama-server binary + model

## Dependencies

### Go Module Dependencies

```go
require (
    github.com/gen2brain/malgo v0.11.23      // Audio capture (WASAPI)
    github.com/wailsapp/wails/v2 v2.10.2     // Desktop GUI framework
)
```

### Frontend Dependencies

```json
{
    "devDependencies": {
        "tailwindcss": "^3.4.17",
        "postcss": "^8.5.6",
        "autoprefixer": "^10.4.21",
        "@tailwindcss/typography": "^0.5.15"
    }
}
```

**CDN Libraries**:
- marked v12.0.0 (Markdown rendering)

## Design Patterns

### Patterns Used

1. **Model-View-Controller (MVC)**:
   - Model: Settings, Prompts, Audio state
   - View: HTML/CSS/JS frontend
   - Controller: App struct methods

2. **Factory Pattern**:
   - `NewRecorder()`, `NewMicRecorder()`, `NewWriter()`, `NewApp()`

3. **Observer Pattern**:
   - Wails EventsEmit/EventsOn for real-time updates

4. **Strategy Pattern**:
   - Local vs. Remote AI summarization (runtime selection)

5. **Command Pattern**:
   - External process execution (whisper.cpp, llama-server)

### Thread Safety

**Synchronization Mechanisms**:

1. **Mutex Protection** (sync.Mutex):
   - `App.mu` - Recording state, recorder instances
   - `App.promptMu` - Prompt cache and selection
   - `App.llamaMu` - Llama server state

2. **Channel-Based Concurrency**:
   - Audio data channels (buffered, drop on slow consumer)
   - Error channels for device stop signals

3. **Context Cancellation**:
   - `App.ctx` for recording context
   - `App.cancel` to stop writer goroutine

## State Management

### Application State Machine

```
            ┌─────────────────────────────────────┐
            │       NOT RECORDING                 │
            │    (App.recording = false)          │
            └──────────────┬──────────────────────┘
                           │ User clicks Start
                           ▼
            ┌─────────────────────────────────────┐
            │       RECORDING ACTIVE              │
            │    (App.recording = true)           │
            │    - Writer goroutine running       │
            │    - Audio emitting in real-time    │
            │    - File being written             │
            └──────────────┬──────────────────────┘
                           │ User clicks Stop
                           ▼
            ┌─────────────────────────────────────┐
            │       RECORDING STOPPED             │
            │    - Finalize WAV headers           │
            │    - Close resources                │
            │    - Return to NOT RECORDING        │
            └─────────────────────────────────────┘
```

## Testing

**Current Status**: No automated tests found

**Recommended Testing Strategy**:

1. **Unit Tests**:
   - WAV header generation
   - Audio mixing algorithm
   - Settings persistence
   - Prompt loading logic

2. **Integration Tests**:
   - Recording workflow (mock WASAPI)
   - Whisper integration (mock binary)
   - LLM API calls (mock HTTP responses)

3. **End-to-End Tests**:
   - Complete workflow testing
   - GUI automation (Wails testing framework)

## Known Limitations

1. **Platform**: Windows 11+ only (no macOS/Linux)
2. **Audio**: Fixed 16 kHz, mono only
3. **Device Selection**: No UI for device selection (uses defaults)
4. **Testing**: No automated tests
5. **Scalability**: Single-file output only (no batch processing)

## Future Enhancements

- [ ] Device selection UI
- [ ] Advanced audio processing (noise reduction, normalization)
- [ ] Real-time transcription streaming
- [ ] Multiple AI provider support
- [ ] Audio format conversion
- [ ] Batch processing
- [ ] Model management UI
- [ ] Advanced prompt customization
- [ ] Take notes with timestamping
- [ ] Different summarization styles
- [ ] Export formats (PDF, DOCX)

## Related Documentation

- [README.md](README.md) - User-focused documentation
- [agents.md](agents.md) - AI agent development guide
- [config/README.md](config/README.md) - Prompt configuration guide

## Technical Highlights

✅ **Strengths**:
- Clean separation between backend (Go) and frontend (JS)
- Effective use of Wails for desktop development
- Real-time audio streaming with efficient visualization
- Flexible AI integration (local + remote)
- User-configurable summarization prompts
- Small binary footprint (15 MB)

🔧 **Architecture**:
- WASAPI audio capture for system + microphone audio
- Proper WAV file format implementation
- Real-time 60fps spectrum visualization
- Thread-safe concurrent audio processing
- Hot-reload development workflow

---

*This documentation is maintained for Claude Code and other AI agents working on the Blackbox project.*
