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
go build .\...

# Record system audio
.\cmd\rec\rec.exe --dur 30 --with-mic

# Transcribe WAV file
.\cmd\transcribe\transcribe.exe --wav .\out\audio.wav

# Summarize transcript
.\cmd\summarise\summarise.exe --txt .\out\audio.txt
```

## Audio Format

- **Format**: PCM S16LE (16-bit signed little-endian)
- **Sample Rate**: 16 kHz
- **Channels**: Stereo (loopback) + Mono (microphone)
- **Quality**: Optimized for transcription while maintaining excellent audio clarity
- **File Sizes**: ~1.6-2.0 MB per minute

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
  "model": "gpt-5-mini"
}
```

## Build and Development

### Prerequisites
- Go 1.24+
- Node.js and npm
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- [whisper-cli](https://github.com/ggml-org/whisper.cpp) built files extracted to ./whisper-cli

### Development
```bash
# Install dependencies
npm install

# Run development server (standard Wails command)
wails dev

# Build production GUI (standard Wails command)
wails build
```

### Build Commands
- **Standard Wails**: `wails build` - Direct Wails build command (recommended)
- **With CSS build**: `npm run build:css && wails build -clean` - Manual build with CSS
- **npm wrapper**: `npm run build:gui` - Custom npm script that wraps Wails commands
```

## Usage Examples

### Recording with automatic summary
1. Open Blackbox
2. Click on "Record & Transcribe & Summarise
3. Select which mode you want to record in (desktop only (untick Use Microphone), desktop + microphone, or microphone only (dictation mode))
4. Begin your meeting/dictation
5. Once done, clikc "Stop Recording."
6. The application will automatically transcribe + summarise your recording.
7. The recording is then deleted - all you are left with is the transcript and the summary.

### Advanced Recording Modes
- **Loopback Only**: System audio capture with spectrum visualization
- **Loopback + Mic**: Mixed audio with dual-source spectrum analysis
- **Dictation Mode**: Microphone-only with mic-focused visualization

## Troubleshooting

### Common Issues
1. **Audio Not Recording**: Check device permissions and default audio devices
2. **Spectrum Analyzer Not Moving**: Ensure audio is playing and recording is active, ensure you are using the correct audio interface in Windows settings
3. **Whisper Errors**: Verify binary path and model existence, refer to [whisper-cli](https://github.com/ggml-org/whisper.cpp)
4. **GUI Not Responding**: Ensure WebView2 runtime is installed

## Future Enhancements

- Device selection for audio sources
- Advanced audio processing (noise reduction, normalization)
- Take notes and use them in the summary automatically with timestamping
- Integration with other LLM APIs
- Different summarisation styles (casual, meeting, standup, dictation)

## Contributing

See DEVELOPER.md

## License

See LICENSE.md
