# Blackbox

A Windows-only audio capture and transcription tool with both CLI and Wails-based GUI interfaces. The system records system audio (WASAPI loopback) and/or microphone input, transcribes audio using whisper.cpp, and provides a foundation for future summarization features.

## Features

- **Real-time Audio Capture**: WASAPI loopback for system audio + optional microphone input
- **Live Spectrum Analyzer**: Real-time visualization of audio activity in the GUI
- **High-Quality Transcription**: whisper.cpp integration with multiple model support
- **Modern GUI**: Clean, responsive Wails-based interface with Tailwind CSS styling
- **CLI Tools**: Command-line utilities for automation and scripting
- **Audio Mixing**: User-selectable combinations of system and microphone audio
- **Flexible Output**: Configurable output directories and file naming

## Screenshots

The GUI features a modern dark theme with:
- **Record Tab**: Audio capture with real-time spectrum analyzer visualization
- **Transcribe Tab**: WAV file selection and transcription processing
- **Record & Transcribe & Summarise Tab**: Combined workflow with live audio feedback
- **Summarise Tab**: Transcript processing and AI-powered summarization
- **Settings Tab**: Configuration management

## Quick Start

### GUI (Recommended)
```bash
# Build and run the GUI
wails build
cp .\build\bin\blackbox-gui.exe .\blackbox-gui.exe
.\blackbox-gui.exe
```

### CLI Tools
```bash
# Build all CLI tools
go build ./...

# Record system audio
.\cmd\rec\rec.exe --dur 30 --with-mic

# Transcribe WAV file
.\cmd\transcribe\transcribe.exe --wav .\out\audio.wav

# Summarize transcript
.\cmd\summarise\summarise.exe --txt .\out\audio.txt
```

## Architecture

- **Backend**: Go 1.24+ with WASAPI audio capture
- **GUI Framework**: Wails v2 (Go + WebView2)
- **Audio Processing**: malgo for WASAPI loopback and capture
- **Transcription**: whisper.cpp integration
- **Frontend**: Vanilla HTML/CSS/JavaScript with Tailwind CSS
- **Real-time Visualization**: 60fps spectrum analyzer with ultra-sensitive audio response

## Audio Format

- **Format**: PCM S16LE (16-bit signed little-endian)
- **Sample Rate**: 48 kHz
- **Channels**: Stereo (loopback) + Mono (microphone)
- **Quality**: Optimized for transcription while maintaining excellent audio clarity
- **File Sizes**: ~1.6-2.0 MB per minute

## Spectrum Analyzer

The GUI includes a **real-time spectrum analyzer** that provides:
- **Live Audio Visualization**: 32 responsive bars that react to incoming audio
- **Ultra-Sensitive Response**: Bars move dramatically with even quiet sounds
- **60fps Animation**: Smooth, professional visualization using `requestAnimationFrame`
- **Dual Audio Sources**: Visualizes both system audio (WASAPI loopback) and microphone input
- **Professional Styling**: Lighter grey bars with dynamic color intensity based on audio levels

## Configuration

### Environment Variables
- `LOOPBACK_NOTES_OUT`: Output directory (default: `./out`)
- `LOOPBACK_NOTES_MODELS`: Models directory (default: `./models`)
- `LOOPBACK_NOTES_WHISPER_BIN`: Whisper binary path (default: `./whisper-bin/whisper-cli.exe`)

### Configuration Files

**GUI Settings** (`./config/ui.json` - auto-created):
```json
{
  "out_dir": "./out"
}
```

**Application Config** (`./configs/llm.json`):
```json
{
  "base_url": "https://api.openai.com/v1",
  "api_key_env": "OPENAI_API_KEY",
  "model": "gpt-4o-mini"
}
```

## Build and Development

### Prerequisites
- Go 1.24+
- Node.js and npm
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

### Development
```bash
# Install dependencies
npm install

# Build CSS and run development server
npm run dev

# Build production GUI
npm run build:gui
```

### Build Commands
- **npm**: `npm run build:gui` - Cross-platform build command (recommended)
- **Manual**: `npm run build:css && wails build -clean` - Manual build process

## Project Structure

```
blackbox/
├── main.go                 # Wails GUI entrypoint
├── cmd/                    # CLI applications
│   ├── rec/               # Audio recording CLI
│   ├── transcribe/        # Transcription CLI
│   ├── summarise/         # AI-powered summarization CLI
│   └── gui/               # Alternative GUI entry (unused)
├── internal/               # Core application logic
│   ├── audio/             # Audio capture (WASAPI loopback + mic)
│   ├── ui/                # GUI backend services
│   ├── wav/               # WAV file handling
│   └── execx/             # External process execution
├── frontend/               # Static web assets for GUI
│   ├── src/               # Source HTML for Tailwind scanning
│   ├── dist/              # Built assets (HTML, CSS, JS)
│   ├── wailsjs/           # Wails-generated bindings
│   └── tailwind.config.js # Tailwind CSS configuration
├── models/                 # Whisper model files
├── whisper-bin/            # Whisper.cpp executables
├── configs/                # Application configuration files
├── config/                 # GUI settings (auto-created)
└── out/                    # Output directory (audio files, transcripts)
```

## Usage Examples

### Recording with Spectrum Analyzer
1. Open the GUI and go to the **Record** tab
2. Click **Start Recording** to begin capture
3. Watch the real-time spectrum analyzer respond to audio activity
4. The analyzer shows 32 bars that move based on frequency content
5. Bars change color intensity based on audio levels (grey-600 → grey-400)

### Advanced Recording Modes
- **Loopback Only**: System audio capture with spectrum visualization
- **Loopback + Mic**: Mixed audio with dual-source spectrum analysis
- **Dictation Mode**: Microphone-only with mic-focused visualization

## Troubleshooting

### Common Issues
1. **Audio Not Recording**: Check device permissions and default audio devices
2. **Spectrum Analyzer Not Moving**: Ensure audio is playing and recording is active
3. **Whisper Errors**: Verify binary path and model existence
4. **GUI Not Responding**: Ensure WebView2 runtime is installed

### Debug Mode
The spectrum analyzer includes comprehensive error handling and will gracefully fall back to idle animation if audio data is unavailable.

## Future Enhancements

- Device selection for audio sources
- Advanced audio processing (noise reduction, normalization)
- Real-time transcription streaming
- Integration with actual LLM APIs
- Audio format conversion options
- Batch processing capabilities

## Contributing

This project uses Go modules and follows standard Go conventions. The frontend uses Tailwind CSS for styling and vanilla JavaScript for functionality.

## License

[Add your license information here]
