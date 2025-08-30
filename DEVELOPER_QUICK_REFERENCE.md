# Developer Quick Reference

## Project Overview

Blackbox is a Windows-only audio capture and transcription tool with both CLI and Wails-based GUI interfaces. The system records system audio (WASAPI loopback) and/or microphone input, transcribes audio using whisper.cpp, and provides a foundation for future summarization features.

## Key Features

- **Real-time Audio Capture**: WASAPI loopback for system audio + optional microphone input
- **Live Spectrum Analyzer**: Beautiful real-time visualization of audio activity in the GUI
- **High-Quality Transcription**: whisper.cpp integration with multiple model support
- **Modern GUI**: Clean, responsive Wails-based interface with Tailwind CSS styling
- **CLI Tools**: Command-line utilities for automation and scripting

## Architecture

### High-Level Structure
```
blackbox/
├── main.go                 # Wails GUI entrypoint
├── cmd/                    # CLI applications
│   ├── rec/               # Audio recording CLI
│   ├── transcribe/        # Transcription CLI
│   ├── summarise/         # AI-powered summarization CLI
│   └── gui/               # Alternative GUI entry
├── internal/               # Core application logic
│   ├── audio/             # Audio capture (WASAPI loopback + mic)
│   ├── ui/                # GUI backend services
│   ├── wav/               # WAV file handling
│   └── execx/             # External process execution
├── frontend/               # Static web assets for GUI
│   ├── src/               # Source HTML for Tailwind scanning
│   ├── dist/              # Built assets (HTML, CSS, JS)
│   ├── tailwind.config.js # Tailwind CSS configuration
│   ├── package.json       # Frontend dependencies and scripts
│   └── wailsjs/           # Wails-generated bindings
├── models/                 # Whisper model files
├── whisper-bin/            # Whisper.cpp executables
├── configs/                # Configuration files
├── package.json            # Project build scripts
├── build.bat               # Windows batch build script
├── build.ps1               # PowerShell build script
└── out/                    # Output directory
```

### Technology Stack
- **Backend**: Go 1.24+
- **GUI Framework**: Wails v2 (Go + WebView2)
- **Audio**: malgo (WASAPI loopback + capture)
- **Transcription**: whisper.cpp
- **Frontend**: Vanilla HTML/CSS/JavaScript with Tailwind CSS
- **Styling**: Tailwind CSS v3.4.17 + PostCSS + Autoprefixer
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
- **Sample Rate**: 48 kHz
- **Channels**: Stereo (loopback) + Mono (microphone)
- **Quality**: Optimized for transcription while maintaining excellent audio clarity
- **Mixing**: Sample-wise averaging to prevent clipping

### 2. Real-Time Spectrum Analyzer (`frontend/src/index.html`)

#### RealSpectrumAnalyzer Class
- **Purpose**: Provides real-time audio visualization in the GUI
- **Key Features**:
  - **32 Responsive Bars**: Each bar represents a frequency band
  - **Ultra-Sensitive Response**: Bars move dramatically with even quiet sounds
  - **60fps Animation**: Smooth visualization using `requestAnimationFrame`
  - **Dual Audio Sources**: Visualizes both loopback and microphone audio
  - **Professional Styling**: Dynamic color intensity based on audio levels

#### Key Methods
- `handleAudioData(data)`: Processes incoming PCM data from Go backend
- `updateSpectrum()`: Performs frequency analysis and updates bar heights
- `animate()`: 60fps animation loop for smooth visualization
- `startRecording()` / `stopRecording()`: Controls analyzer state

#### Audio Data Processing
- **Data Reception**: Receives raw PCM S16LE bytes via Wails events
- **Format Handling**: Supports ArrayBuffer, Uint8Array, Array, and base64 string formats
- **Frequency Analysis**: Simple FFT-like analysis dividing audio into 32 frequency bands
- **Normalization**: Ultra-sensitive normalization (`avgEnergy / 1000`) for maximum reactivity
- **Visual Response**: Exponential scaling (`Math.pow(normalizedEnergy, 0.3)`) for dramatic movement

### 3. WAV Handling (`internal/wav/`)

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

### 4. External Process Execution (`internal/execx/`)

#### Whisper Integration (`execx.go`)
- **Purpose**: Wraps whisper.cpp CLI execution
- **Key Methods**:
  - `RunWhisper(bin, model, wav, outDir, lang, threads, extraArgs)`: Execute transcription
  - `BuildWhisperArgs(...)`: Construct CLI arguments

#### Features
- Automatic log file generation (`out/<base>.log`)
- Fallback handling for different whisper binary names
- Error handling and validation

### 5. GUI Backend (`internal/ui/`)

#### App Structure (`app.go`)
- **Purpose**: Main GUI backend service
- **Key Methods**:
  - `StartRecording(withMic bool)`: Begin audio capture
  - `StartRecordingAdvanced(withMic, dictation bool)`: Advanced recording modes
  - `StopRecording()`: End capture and finalize WAV
  - `Transcribe(wavPath)`: Run whisper on WAV file
  - `Summarise(txtPath)`: Process transcript with AI-powered summarization
  - `PickWavFromOutDir()`: File picker for WAV files
  - `PickTxtFromOutDir()`: File picker for TXT files

#### Audio Data Emission
- **Real-Time Streaming**: `emitAudioData()` sends raw PCM data to frontend every frame
- **Wails Events**: Uses `wruntime.EventsEmit("audioData", ...)` for communication
- **Data Format**: Sends source, raw bytes, and length information
- **Dual Sources**: Emits both loopback and microphone audio data

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

### 6. Frontend (`frontend/`)

#### Structure
- **Assets**: Embedded via Go embed in `frontend/assets.go`
- **UI**: Single-page tabbed interface with modern Tailwind CSS styling
- **Scripts**: Vanilla JavaScript with Wails bindings
- **Styling**: Tailwind CSS v3.4.17 for rapid UI development

#### Tabs
1. **Record**: Audio capture with mic/dictation options + real-time spectrum analyzer
2. **Transcribe**: WAV file selection and transcription
3. **Record & Transcribe & Summarise**: Combined workflow with live audio feedback
4. **Summarise**: TXT file selection and processing
5. **Settings**: Configuration management

#### Tailwind CSS Integration
- **Configuration**: `frontend/tailwind.config.js` - scans HTML/JS files for classes
- **Input CSS**: `frontend/src/input.css` - contains Tailwind directives
- **Build Process**: Automated CSS generation with npm scripts
- **Development**: Watch mode for automatic CSS rebuilding
- **Production**: CSS embedded in final executable

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

### Real-Time Audio Events

The backend emits real-time audio data to the frontend:

```go
// Emitted every frame during recording
wruntime.EventsEmit(a.uiCtx, "audioData", map[string]interface{}{
    "source": source,    // "loopback" or "microphone"
    "data":   data,      // Raw PCM S16LE data
    "length": len(data), // Data length in bytes
})
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

### 2. Real-Time Visualization
- **60fps Animation**: Use `requestAnimationFrame` for smooth updates
- **Audio Data Handling**: Process incoming PCM data in real-time
- **Frequency Analysis**: Divide audio into frequency bands for visual representation
- **Responsive Design**: Ensure bars move dramatically with audio input

### 3. File Operations
- **Paths**: Use `filepath.Join()` for cross-platform compatibility
- **Permissions**: Create directories with `0755` permissions
- **Cleanup**: Close WAV writers and handle errors

### 4. Wails Integration
- **Context**: Store UI context for dialog operations
- **Bindings**: Expose methods through struct embedding
- **Assets**: Use Go embed for frontend files
- **Events**: Use `wruntime.EventsEmit` for real-time communication

### 5. Error Handling
- **Validation**: Check file existence and permissions
- **Recovery**: Clean up resources on errors
- **User Feedback**: Return meaningful error messages

### 6. Tailwind CSS Development
- **Content Scanning**: Configure `tailwind.config.js` to scan source HTML files
- **Build Process**: Use npm scripts for CSS generation (`tailwind:build`, `tailwind:watch`)
- **Source Files**: Maintain HTML in `frontend/src/` for Tailwind scanning
- **Production**: Ensure CSS is built before Wails build process
- **Wails Integration**: Configure `wails.json` with `frontend:dev:watcher` for development

## Build and Deployment

### Development
```bash
# Build all CLI tools
go build ./...

# Build GUI with Tailwind CSS
npm run build:css && wails build -clean

# Or use automated build script
npm run build:gui

# Run GUI in development (includes Tailwind watcher)
wails dev
```

### Production
```bash
# Build production GUI with Tailwind CSS
npm run build:gui

# Or manually:
npm run build:css && wails build -clean

# Output: build/bin/blackbox-gui.exe
```

### Dependencies
- **Go Modules**: `go.mod` and `go.sum`
- **Wails**: `github.com/wailsapp/wails/v2`
- **Audio**: `github.com/gen2brain/malgo`
- **System**: `golang.org/x/sys`
- **Frontend**: Node.js and npm for Tailwind CSS build process
- **CSS Framework**: `tailwindcss@^3.4.0`, `postcss`, `autoprefixer`

## Build Scripts and Automation

### Package.json Scripts
The project includes several npm scripts for automated builds:

- **`npm run build:css`**: Builds Tailwind CSS for production
- **`npm run build:gui`**: Complete production build (CSS + Wails)
- **`npm run dev`**: Runs Wails development server

### Build Scripts
Cross-platform build automation:

- **Windows**: `build.bat` - Batch file for production builds
- **PowerShell**: `build.ps1` - PowerShell script for production builds
- **npm**: `npm run build:gui` - Cross-platform build command

### Tailwind CSS Workflow
```bash
# Development (automatic CSS rebuilding)
wails dev

# Production build
npm run build:gui

# Manual CSS build
cd frontend && npm run tailwind:build
```

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

### UI Development with Tailwind CSS
1. **HTML Structure**: Add new HTML elements in `frontend/src/index.html`
2. **Styling**: Use Tailwind utility classes for consistent design
3. **Responsiveness**: Leverage Tailwind's responsive utilities
4. **Accessibility**: Include proper focus states and ARIA attributes
5. **CSS Generation**: Ensure new classes are included in Tailwind build
6. **Testing**: Verify styling in both development and production builds

## Troubleshooting

### Common Issues
1. **Audio Not Recording**: Check device permissions and default devices
2. **Spectrum Analyzer Not Moving**: Ensure audio is playing and recording is active
3. **Whisper Errors**: Verify binary path and model existence
4. **GUI Not Responding**: Ensure WebView2 runtime is installed
5. **Tailwind CSS Not Working**: Verify CSS build process and file paths
6. **Styling Missing in Production**: Ensure CSS is built before Wails build

### Debug Steps
1. Check console output for error messages
2. Verify file paths and permissions
3. Test CLI tools independently
4. Check environment variable overrides
5. **Tailwind CSS Issues**:
   - Verify `frontend/src/index.html` exists and contains classes
   - Check `frontend/dist/output.css` file size and content
   - Run `npm run tailwind:build` manually
   - Verify `tailwind.config.js` content paths
   - Check that CSS is linked in HTML files

## Future Enhancements

### Planned Features
- Device selection for audio sources
- Advanced audio processing (noise reduction, normalization)
- Real-time transcription streaming
- Integration with actual LLM APIs
- Audio format conversion options
- Batch processing capabilities

### Current UI Features
- **Modern Dark Theme**: Professional appearance with proper contrast
- **Responsive Layout**: Clean spacing and typography using Tailwind utilities
- **Interactive Elements**: Hover effects, focus states, and smooth transitions
- **Accessibility**: Proper focus indicators and disabled states
- **Tabbed Interface**: Clean navigation between different functionality
- **Real-Time Spectrum Analyzer**: Live audio visualization with 60fps animation

### Extension Points
- Audio source plugins
- Transcription engine abstraction
- Output format handlers
- Workflow automation
- Cloud storage integration
- Advanced visualization options

This documentation should provide developers with comprehensive understanding of the Blackbox project structure, enabling effective code analysis, modification, and extension.
