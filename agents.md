# Blackbox - Agent Documentation

This document provides comprehensive information for AI agents and developers working with the Blackbox project. It covers the architecture, key components, API interfaces, and development patterns.

## Project Overview

Blackbox is a Windows-only audio capture and transcription tool with both CLI and Wails-based GUI interfaces. The system records system audio (WASAPI loopback) and/or microphone input, transcribes audio using whisper.cpp, and provides a foundation for future summarization features.

## Architecture

### High-Level Structure
```
blackbox/
├── main.go                 # Wails GUI entrypoint
├── cmd/                    # CLI applications
│   ├── rec/               # Audio recording CLI
│   ├── transcribe/        # Transcription CLI
│   ├── summarise/         # Summarization CLI (stub)
│   └── gui/               # Alternative GUI entry
├── internal/               # Core application logic
│   ├── audio/             # Audio capture (WASAPI loopback + mic)
│   ├── ui/                # GUI backend services
│   ├── wav/               # WAV file handling
│   └── execx/             # External process execution
├── frontend/               # Static web assets for GUI
├── models/                 # Whisper model files
├── whisper-bin/            # Whisper.cpp executables
├── configs/                # Configuration files
└── out/                    # Output directory
```

### Technology Stack
- **Backend**: Go 1.24+
- **GUI Framework**: Wails v2 (Go + WebView2)
- **Audio**: malgo (WASAPI loopback + capture)
- **Transcription**: whisper.cpp
- **Frontend**: Vanilla HTML/CSS/JavaScript
- **Platform**: Windows 11 only

## Core Components

### 1. Audio Capture System (`internal/audio/`)

#### Loopback Recorder (`loopback.go`)
- **Purpose**: Captures system audio via WASAPI loopback
- **Key Methods**:
  - `NewRecorder(bufferCallbacks int)`: Initialize with buffer capacity
  - `Start(sampleRate, channels uint32)`: Begin capture
  - `Data() <-chan []byte`: Stream of PCM S16LE frames
  - `Stop()`: Clean shutdown

#### Microphone Recorder (`mic.go`)
- **Purpose**: Captures default microphone input
- **Key Methods**:
  - `NewMicRecorder(bufferCallbacks int)`: Initialize mic capture
  - `Start(sampleRate, channels uint32)`: Begin mic capture
  - `Data() <-chan []byte`: Stream of PCM S16LE frames
  - `Stop()`: Clean shutdown

#### Audio Format
- **Format**: PCM S16LE (16-bit signed little-endian)
- **Sample Rate**: 48 kHz (configurable)
- **Channels**: 2 (stereo)
- **Mixing**: Sample-wise averaging to prevent clipping

### 2. WAV Handling (`internal/wav/`)

#### Writer (`writer.go`)
- **Purpose**: Writes PCM audio data to WAV files
- **Key Methods**:
  - `NewWriter(path, sampleRate, channels, bits)`: Create new WAV
  - `Write(data []byte)`: Write PCM frames
  - `Flush()`: Ensure data is written to disk
  - `Close()`: Finalize RIFF headers and close file

#### Features
- Automatic RIFF header management
- Periodic flushing during recording
- Proper cleanup on close

### 3. External Process Execution (`internal/execx/`)

#### Whisper Integration (`execx.go`)
- **Purpose**: Wraps whisper.cpp CLI execution
- **Key Methods**:
  - `RunWhisper(bin, model, wav, outDir, lang, threads, extraArgs)`: Execute transcription
  - `BuildWhisperArgs(...)`: Construct CLI arguments

#### Features
- Automatic log file generation (`out/<base>.log`)
- Fallback handling for different whisper binary names
- Error handling and validation

### 4. GUI Backend (`internal/ui/`)

#### App Structure (`app.go`)
- **Purpose**: Main GUI backend service
- **Key Methods**:
  - `StartRecording(withMic bool)`: Begin audio capture
  - `StartRecordingAdvanced(withMic, dictation bool)`: Advanced recording modes
  - `StopRecording()`: End capture and finalize WAV
  - `Transcribe(wavPath)`: Run whisper on WAV file
  - `Summarise(txtPath)`: Process transcript (stub)
  - `PickWavFromOutDir()`: File picker for WAV files
  - `PickTxtFromOutDir()`: File picker for TXT files

#### Settings Management (`settings.go`)
- **Purpose**: Persistent configuration storage
- **Storage**: `./config/ui.json`
- **Key Fields**:
  - `OutDir`: Output directory path
  - Extensible for future settings

#### Recording Modes
1. **Loopback Only**: System audio capture
2. **Loopback + Mic**: System audio mixed with microphone
3. **Dictation Mode**: Microphone only (useful when no system audio)

### 5. Frontend (`frontend/`)

#### Structure
- **Assets**: Embedded via Go embed in `frontend/assets.go`
- **UI**: Single-page tabbed interface
- **Scripts**: Vanilla JavaScript with Wails bindings

#### Tabs
1. **Record**: Audio capture with mic/dictation options
2. **Transcribe**: WAV file selection and transcription
3. **Record & Transcribe & Summarise**: Combined workflow
4. **Summarise**: TXT file selection and processing
5. **Settings**: Configuration management

## API Interfaces

### Wails Backend Bindings

The frontend communicates with the backend through these bound methods:

```go
// Recording
StartRecording(withMic bool) (string, error)           // Returns WAV path
StartRecordingAdvanced(withMic, dictation bool) (string, error)
StopRecording() (string, error)                        // Returns final WAV path

// File Operations
PickWavFromOutDir() (string, error)                    // Returns selected WAV path
PickTxtFromOutDir() (string, error)                    // Returns selected TXT path

// Processing
Transcribe(wavPath string) (string, error)             // Returns TXT path
Summarise(txtPath string) (string, error)              // Returns summary message

// Settings
GetSettings() UISettings                               // Returns current config
SaveSettings(jsonStr string) (UISettings, error)      // Saves and returns config
```

### CLI Interfaces

#### Recording (`cmd/rec/main.go`)
```bash
./cmd/rec/rec.exe [flags]
  --out-dir      "./out"        # Output directory
  --sample-rate  48000          # Sample rate (Hz)
  --bits         16             # Bits per sample
  --channels     2              # Channel count
  --dur          0              # Duration (0 = manual stop)
  --stop-key     ""             # Hotkey to stop
  --with-mic     false          # Include microphone
```

#### Transcription (`cmd/transcribe/main.go`)
```bash
./cmd/transcribe/transcribe.exe [flags]
  --wav          ""             # WAV file path (required)
  --model        "./models/ggml-base.en.bin"  # Model path
  --lang         "en"           # Language code
  --threads      0              # Thread count
  --out-dir      "./out"        # Output directory
  --extra-args   ""             # Additional whisper args
```

#### Summarization (`cmd/summarise/main.go`)
```bash
./cmd/summarise/summarise.exe [flags]
  --config       "./configs/llm.json"  # LLM config path
  --txt          ""             # TXT file path (required)
```

## Configuration

### Environment Variables
- `LOOPBACK_NOTES_OUT`: Output directory (default: `./out`)
- `LOOPBACK_NOTES_MODELS`: Models directory (default: `./models`)
- `LOOPBACK_NOTES_WHISPER_BIN`: Whisper binary path (default: `./whisper-bin/whisper-cli.exe`)

### Settings File (`./config/ui.json`)
```json
{
  "out_dir": "./out"
}
```

### LLM Config (`./configs/llm.json`)
```json
{
  "base_url": "https://api.openai.com/v1",
  "api_key_env": "OPENAI_API_KEY",
  "model": "gpt-4o-mini"
}
```

## Development Patterns

### 1. Audio Processing
- **Buffering**: Use buffered channels for audio data
- **Mixing**: Sample-wise averaging with clipping prevention
- **Cleanup**: Always call `Stop()` on recorders
- **Error Handling**: Check device initialization and start errors

### 2. File Operations
- **Paths**: Use `filepath.Join()` for cross-platform compatibility
- **Permissions**: Create directories with `0755` permissions
- **Cleanup**: Close WAV writers and handle errors

### 3. Wails Integration
- **Context**: Store UI context for dialog operations
- **Bindings**: Expose methods through struct embedding
- **Assets**: Use Go embed for frontend files

### 4. Error Handling
- **Validation**: Check file existence and permissions
- **Recovery**: Clean up resources on errors
- **User Feedback**: Return meaningful error messages

## Build and Deployment

### Development
```bash
# Build all CLI tools
go build ./...

# Build GUI
wails build -clean

# Run GUI in development
wails dev
```

### Production
```bash
# Build production GUI
wails build -clean

# Output: build/bin/blackbox-gui.exe
```

### Dependencies
- **Go Modules**: `go.mod` and `go.sum`
- **Wails**: `github.com/wailsapp/wails/v2`
- **Audio**: `github.com/gen2brain/malgo`
- **System**: `golang.org/x/sys`

## Common Tasks

### Adding New Audio Sources
1. Create new recorder in `internal/audio/`
2. Implement `Start()`, `Stop()`, `Data()` methods
3. Add to `App.StartRecordingAdvanced()` logic
4. Update frontend with new options

### Extending Settings
1. Add fields to `UISettings` struct
2. Update `SaveSettings()` validation
3. Add UI controls in frontend
4. Handle in backend logic

### Adding New Processing Steps
1. Create new method in `App` struct
2. Implement processing logic
3. Add frontend UI elements
4. Wire up in workflow tabs

## Troubleshooting

### Common Issues
1. **Audio Not Recording**: Check device permissions and default devices
2. **Whisper Errors**: Verify binary path and model existence
3. **GUI Not Responding**: Ensure WebView2 runtime is installed
4. **File Picker Issues**: Check UI context initialization

### Debug Steps
1. Check console output for error messages
2. Verify file paths and permissions
3. Test CLI tools independently
4. Check environment variable overrides

## Future Enhancements

### Planned Features
- Device selection for audio sources
- Advanced audio processing (noise reduction, normalization)
- Real-time transcription streaming
- Integration with actual LLM APIs
- Audio format conversion options
- Batch processing capabilities

### Extension Points
- Audio source plugins
- Transcription engine abstraction
- Output format handlers
- Workflow automation
- Cloud storage integration

This documentation should provide AI agents with comprehensive understanding of the Blackbox project structure, enabling effective code analysis, modification, and extension.
