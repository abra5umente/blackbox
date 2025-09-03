# Developer Quick Reference

## Project Overview

Blackbox is a Windows-only audio capture and transcription tool with a Wails-based GUI interface. The system records system audio (WASAPI loopback) and/or microphone input, transcribes audio using whisper.cpp, and provides AI-powered summarization using both remote APIs (OpenAI) and local AI (llama.cpp).

## Key Features

- **Real-time Audio Capture**: WASAPI loopback for system audio + optional microphone input
- **Live Spectrum Analyzer**: Beautiful real-time visualization of audio activity in the GUI
- **High-Quality Transcription**: whisper.cpp integration with multiple model support
- **AI-Powered Summarization**: Both remote (OpenAI) and local (llama.cpp) AI summarization
- **Modern GUI**: Clean, responsive Wails-based interface with Tailwind CSS styling

- **Local AI Support**: Full llama.cpp integration for privacy-focused local processing

## Architecture

### High-Level Structure
```
blackbox/
├── main.go                 # Wails GUI entrypoint
├── internal/               # Core application logic
│   ├── audio/             # Audio capture (WASAPI loopback + mic)
│   ├── ui/                # GUI backend services
│   ├── wav/               # WAV file handling
│   └── execx/             # External process execution
├── frontend/               # Static web assets for GUI
│   ├── dist/              # Built assets (HTML, CSS, JS)
│   ├── wailsjs/           # Wails-generated bindings
│   ├── tailwind.config.js # Tailwind CSS configuration
│   └── package.json       # Frontend dependencies and scripts
├── models/                 # Whisper and Llama model files
├── whisper-bin/            # Whisper.cpp executables
├── llamacpp-bin/           # Llama.cpp executables for local AI
├── configs/                # Application configuration files
│   ├── llm.example.json   # Example LLM configuration
│   ├── local.json         # Local AI configuration
│   └── remote.json        # Remote AI configuration
├── config/                 # GUI settings (auto-created)
├── package.json            # Project build scripts
└── out/                    # Output directory
```

### Technology Stack
- **Backend**: Go 1.24.5
- **GUI Framework**: Wails v2.10.2 (Go + WebView2)
- **Audio**: malgo (WASAPI loopback + capture)
- **Transcription**: whisper.cpp
- **AI Summarization**: OpenAI API + llama.cpp (local)
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
- **Sample Rate**: 16 kHz (optimized for speech recognition)
- **Channels**: Mono (both loopback and microphone)
- **Quality**: Optimized for transcription while maintaining excellent audio clarity
- **Mixing**: Sample-wise averaging to prevent clipping

### 2. Real-Time Spectrum Analyzer (`frontend/dist/index.html`)

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
- **Purpose**: Wraps whisper.cpp execution
- **Key Methods**:
  - `RunWhisper(bin, model, wav, outDir, lang, threads, extraArgs)`: Execute transcription
  - `BuildWhisperArgs(...)`: Construct whisper arguments

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
  - `PickModelFile()`: File picker for Llama model files

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
  - `UseLocalAI`: Enable local AI summarization
  - `LlamaTemp`: Temperature for local AI (0.0-2.0)
  - `LlamaContext`: Context window size for local AI
  - `LlamaModel`: Path to Llama model file
  - `LlamaAPIKey`: API key for llama-server authentication

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
3. **Record & Transcribe & Summarise**: Combined workflow with live audio feedback + AI summarization
4. **Summarise**: TXT file selection and AI processing (remote or local)
5. **Settings**: Configuration management including local AI settings

#### Tailwind CSS Integration
- **Configuration**: `frontend/tailwind.config.js` - scans HTML/JS files for classes
- **Input CSS**: `frontend/dist/input.css` - contains Tailwind directives
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
PickModelFile() (string, error)                        // Returns selected model path

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
  "out_dir": "./out",
  "use_local_ai": false,
  "llama_temp": 0.1,
  "llama_context": 32000,
  "llama_model": "",
  "llama_api_key": ""
}
```

### Remote AI Config (`./configs/remote.json`)
```json
{
  "base_url": "https://api.openai.com/v1",
  "api_key": "your_openai_api_key_here",
  "model": "gpt-4o-mini"
}
```

### Local AI Config (`./configs/local.json`)
```json
{
  "base_url": "http://127.0.0.1:8080",
  "api_key": "1234",
  "model": "gemma-3-12b-it-q4_0.gguf"
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
- **Content Scanning**: Configure `tailwind.config.js` to scan HTML files in `frontend/dist/`
- **Build Process**: Use npm scripts for CSS generation (`tailwind:build`, `tailwind:watch`)
- **Source Files**: Maintain HTML in `frontend/dist/` for Tailwind scanning
- **Production**: Ensure CSS is built before Wails build process
- **Wails Integration**: Configure `wails.json` with `frontend:dev:watcher` for development

### 7. Local AI Integration
- **Server Management**: Automatic llama-server startup/shutdown for local AI
- **Model Selection**: File picker for GGUF model files
- **Configuration**: Temperature, context window, and API key settings
- **Fallback**: Graceful fallback to remote AI if local AI fails
- **Privacy**: Complete local processing without external API calls

## Build and Deployment

### Development
```bash
# Build the GUI application
wails build

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
- **Wails**: `github.com/wailsapp/wails/v2@v2.10.2`
- **Audio**: `github.com/gen2brain/malgo@v0.11.23`
- **System**: `golang.org/x/sys@v0.35.0`
- **Frontend**: Node.js and npm for Tailwind CSS build process
- **CSS Framework**: `tailwindcss@^3.4.17`, `postcss@^8.5.6`, `autoprefixer@^10.4.21`

## Build Scripts and Automation

### Package.json Scripts
The project includes several npm scripts for automated builds:

- **`npm run build:css`**: Builds Tailwind CSS for production
- **`npm run build:gui`**: Complete production build (CSS + Wails)
- **`npm run dev`**: Runs Wails development server

### Build Commands
Cross-platform build automation:

- **npm**: `npm run build:gui` - Cross-platform build command (recommended)
- **Manual**: `npm run build:css && wails build -clean` - Manual build process

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
1. **HTML Structure**: Add new HTML elements in `frontend/dist/index.html`
2. **Styling**: Use Tailwind utility classes for consistent design
3. **Responsiveness**: Leverage Tailwind's responsive utilities
4. **Accessibility**: Include proper focus states and ARIA attributes
5. **CSS Generation**: Ensure new classes are included in Tailwind build
6. **Testing**: Verify styling in both development and production builds

### Local AI Setup
1. **Model Download**: Download GGUF model files to `./models/` directory
2. **Configuration**: Set model path, temperature, and context window in settings
3. **API Key**: Configure authentication key for llama-server
4. **Testing**: Use local AI checkbox in Summarise or RT tabs
5. **Performance**: Adjust context window based on available RAM

## Troubleshooting

### Common Issues
1. **Audio Not Recording**: Check device permissions and default devices
2. **Spectrum Analyzer Not Moving**: Ensure audio is playing and recording is active
3. **Whisper Errors**: Verify binary path and model existence
4. **GUI Not Responding**: Ensure WebView2 runtime is installed
5. **Tailwind CSS Not Working**: Verify CSS build process and file paths
6. **Styling Missing in Production**: Ensure CSS is built before Wails build
7. **Local AI Not Working**: Check llama-server binary and model file paths
8. **Summarization Fails**: Verify API keys and network connectivity

### Debug Steps
1. Check console output for error messages
2. Verify file paths and permissions
3. Test GUI functionality
4. Check environment variable overrides
5. **Tailwind CSS Issues**:
   - Verify `frontend/dist/index.html` exists and contains classes
   - Check `frontend/dist/output.css` file size and content
   - Run `npm run tailwind:build` manually
   - Verify `tailwind.config.js` content paths
   - Check that CSS is linked in HTML files
6. **Local AI Issues**:
   - Verify `llamacpp-bin/llama-server.exe` exists
   - Check model file path in settings
   - Ensure sufficient RAM for model loading
   - Test llama-server manually: `./llamacpp-bin/llama-server.exe --help`

## Future Enhancements

### Planned Features
- Device selection for audio sources
- Advanced audio processing (noise reduction, normalization)
- Real-time transcription streaming
- Multiple AI provider support (Anthropic, Google, etc.)
- Audio format conversion options
- Batch processing capabilities
- Model management and automatic updates
- Advanced prompt customization

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
- AI provider plugins
- Custom prompt templates
- Export formats (PDF, DOCX, etc.)

This documentation should provide developers with comprehensive understanding of the Blackbox project structure, enabling effective code analysis, modification, and extension.
