# Blackbox

A Windows-only meeting & dictation recorder, featuring local transcription, and automatic summarisation.

<img width="1002" height="708" alt="image" src="https://github.com/user-attachments/assets/2e155ac5-025f-4301-8185-b97f1b0ebc5a" />  


## Features

- **Real-time Audio Capture**: WASAPI loopback for system audio + optional microphone input
- **Live Spectrum Analyser**: Real-time visualisation of audio activity in the GUI
- **High-Quality Transcription**: whisper.cpp integration with multiple model support
- **Summarisation**: Use any OpenAI compatible API endpoint (instructions included for local [llama.cpp](https://github.com/ggml-org/llama.cpp/tree/master) usage)
- **Modern GUI**: Clean, responsive Wails-based interface with Tailwind CSS styling

- **Audio Mixing**: User-selectable combinations of system and microphone audio
- **Flexible Output**: Configurable output directories and file naming

The GUI exposes the following functions:
- **Record Tab**: Audio capture with real-time spectrum analyser visualisation
- **Transcribe Tab**: WAV file selection and transcription processing
- **Record & Transcribe & Summarise Tab**: Combined workflow with live audio feedback
- **Summarise Tab**: Transcript processing and AI-powered summarisation
- **Settings Tab**: Configuration management

## Quick Start

### GUI (Recommended)
```bash
# Build and run the GUI
wails build
cp .\build\bin\blackbox-gui.exe .\blackbox-gui.exe
.\blackbox-gui.exe
```



## Audio Format

- **Format**: PCM S16LE (16-bit signed little-endian)
- **Sample Rate**: 16 kHz
- **Channels**: Stereo (loopback) + Mono (microphone)
- **Quality**: Optimised for transcription while maintaining excellent audio clarity
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

**LLM config** (`./configs/local.json` & `./configs/remote.json`):
```json
{
  "base_url": "http://localhost:8080",
  "api_key_env": "llama.cpp API key",
  "model": "model_name"
}
```

## AI Summarisation

Blackbox supports both local and remote AI summarisation, giving you complete control over where you're comfortable sending data.

### Local AI (llama.cpp) RECOMMENDED

Use your own hardware for private, offline summarisation with llama.cpp models.

#### Setup
1. **Download llama.cpp binaries**: Extract to `./llamacpp-bin/` directory. 
  Ensure you grab the correct version for your platform, i.e, Vulkan/CUDA etc. All testing performed on Vulkan binaries.
2. **Download a model**: Place GGUF model files in `./models/` directory
3. **Configure in Settings**:
   - **Model File**: Browse and select your GGUF model (e.g., `gemma-3-12b-it-q4_0.gguf`)
   - **Temperature**: Controls creativity (0.1 = focused, 1.0 = creative)
   - **Context Window**: Model's memory size (32000 = ~24k tokens) (If you have problems with large summaries, try increasing this)
   - **Server API Key**: Authentication key for llama-server (optional, recommended)

#### Usage
1. **Configure local AI settings** in the Settings tab
2. **Enable local AI** by checking "Local AI summarisation" in:
   - **Summarise tab**: For individual transcript processing
   - **Record & Transcribe & Summarise tab**: For automatic summarisation after recording
3. **Start summarisation**: llama-server will automatically start, process your transcript, then shut down

#### Configuration Files
- **`./configs/local.json`**: Client authentication for local server
- **GUI Settings**: Server parameters (model, temperature, context window)

### Remote AI (OpenAI Compatible)

Use cloud-based AI services for summarisation with any OpenAI-compatible API.

#### Setup
1. **Configure remote endpoint** in `./configs/remote.json`:
```json
{
  "base_url": "https://api.openai.com/v1",
  "api_key": "your-api-key-here",
  "model": "gpt-5-mini"
}
```

2. **Supported services**:
   - OpenAI (GPT-4, GPT-3.5)
   - Anthropic Claude (via OpenAI-compatible proxy)
   - Local servers (Ollama, LM Studio, etc.)
   - Any OpenAI-compatible API

#### Usage
1. **Leave "Local AI summarisation" unchecked** in the relevant tabs
2. **Start summarisation**: Requests will be sent to your configured remote endpoint
3. **No local processes**: Everything runs in the cloud

### Switching Between Local and Remote

- **Per-operation control**: Each tab has its own "Local AI summarisation" checkbox
- **Independent settings**: Local and remote configurations are completely separate
- **No conflicts**: You can use local AI in one tab and remote AI in another
- **Automatic cleanup**: Local llama-server shuts down after each use

### Configuration Examples

#### Local AI Setup (`./configs/local.json`)
```json
{
  "base_url": "http://localhost:8080",
  "api_key": "1234",
  "model": "gemma-3-12b-it-q4_0.gguf"
}
```

#### Remote AI Setup (`./configs/remote.json`)
```json
{
  "base_url": "https://api.openai.com/v1",
  "api_key": "sk-proj-your-openai-key",
  "model": "gpt-5-mini"
}
```

#### GUI Settings (Local AI Parameters)
```json
{
  "out_dir": "./out",
  "use_local_ai": true,
  "llama_model": "./models/gemma-3-12b-it-q4_0.gguf",
  "llama_temp": 0.1,
  "llama_context": 32000,
  "llama_api_key": "1234"
}
```

## Build and Development

### Prerequisites
- Go 1.24+
- Node.js and npm (for Tailwind CSS building)
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- [whisper-cli](https://github.com/ggml-org/whisper.cpp) binaries extracted to .\whisper-bin
- [llama.cpp](https://github.com/ggml-org/llama.cpp/tree/master) binaries extracted to .\llamacpp-bin


### Build Commands
- **Development**: `wails dev` - Run with hot reload and automatic CSS building
- **Production**: `wails build` - Build final executable with automatic CSS building

## Usage Examples

### Recording with automatic summary
1. Open Blackbox
2. Click on "Record & Transcribe & Summarise
3. Select which mode you want to record in (desktop only (untick Use Microphone), desktop + microphone, or microphone only (dictation mode))
4. Begin your meeting/dictation
5. Once done, click "Stop Recording."
6. The application will automatically transcribe + summarise your recording.

### Advanced Recording Modes
- **Loopback Only**: System audio capture with spectrum visualisation
- **Loopback + Mic**: Mixed audio with dual-source spectrum analysis
- **Dictation Mode**: Microphone-only with mic-focused visualisation

## Troubleshooting

### Common Issues
1. **Audio Not Recording**: Check device permissions and default audio devices
2. **Spectrum Analyser Not Moving**: Ensure audio is playing and recording is active, ensure you are using the correct audio interface in Windows settings
3. **Whisper Errors**: Verify binary path and model existence, refer to [whisper-cli](https://github.com/ggml-org/whisper.cpp)
4. **GUI Not Responding**: Ensure WebView2 runtime is installed

## Future Enhancements

- Device selection for audio sources
- Advanced audio processing (noise reduction, normalisation)
- Take notes and use them in the summary automatically with timestamping
- Integration with other LLM APIs
- Different summarisation styles (casual, meeting, standup, dictation)

## Contributing

See [DEVELOPER.md](https://github.com/abra5umente/blackbox/blob/main/DEVELOPER.md)

## License

See [LICENSE](https://github.com/abra5umente/blackbox/blob/main/LICENSE)
