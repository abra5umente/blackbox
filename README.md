# Blackbox

A Windows-only meeting & dictation recorder, featuring local transcription, and automatic summarisation.

<img width="1005" height="1207" alt="image" src="https://github.com/user-attachments/assets/6db8f51d-30df-4f22-9800-3ffd65a88aa3" />  

## Features

- **Real-time Audio Capture**: WASAPI loopback for system audio + optional microphone input
- **Live Spectrum Analyser**: Real-time visualisation of audio activity in the GUI
- **High-Quality Transcription**: whisper.cpp integration with multiple model support
- **AI-Powered Summarisation**: Use any OpenAI compatible API endpoint (instructions included for local [llama.cpp](https://github.com/ggml-org/llama.cpp/tree/master) usage)
- **Secure Audio Playback**: In-GUI audio players for listening to recorded WAV files
- **Formatted Output**: Markdown rendering for transcripts and summaries
- **Small Footprint**: Less than 15mb executable  
- **Audio Mixing**: User-selectable combinations of system and microphone audio
- **Flexible Output**: Configurable output directories and file naming

The GUI features a streamlined interface with three main tabs:

- **Auto Tab**: Complete workflow combining recording, transcription, and summarisation with live audio feedback and formatted output
- **Tools Tab**: Individual tools for recording, transcribing, and summarising with audio playback and formatted output
- **Settings Tab**: Configuration management including local AI settings

## Quick Start

### Build and run the GUI
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
   - **Tools tab Summarise section**: For individual transcript processing
   - **Auto tab**: For automatic summarisation after recording
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
   - OpenAI (GPT-5, GPT-4)
   - Anthropic Claude (via OpenAI-compatible proxy)
   - Local servers (Ollama, LM Studio, etc.)
   - Any OpenAI-compatible API

#### Usage
1. **Leave "Local AI summarisation" unchecked** in the Auto tab or Tools tab Summarise section
2. **Start summarisation**: Requests will be sent to your configured remote endpoint
3. **No local processes**: Everything runs in the cloud

### Switching Between Local and Remote

- **Per-operation control**: Each section has its own "Local AI summarisation" checkbox
- **Independent settings**: Local and remote configurations are completely separate
- **No conflicts**: You can use local AI in one section and remote AI in another
- **Automatic cleanup**: Local llama-server shuts down after each use

### Configuration Examples

#### Local AI Client Setup (`./configs/local.json`)
```json
{
  "base_url": "http://localhost:8080",
  "api_key": "1234",
  "model": "gemma-3-12b-it-q4_0.gguf"
}
```

#### Remote AI Client Setup (`./configs/remote.json`)
```json
{
  "base_url": "https://api.openai.com/v1",
  "api_key": "sk-proj-your-openai-key",
  "model": "gpt-5-mini"
}
```

#### App Settings (+ Local AI Server Parameters) (`./configs/ui.json)
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
2. Click on the **Auto** tab
3. Select which mode you want to record in (desktop only (untick Use Microphone), desktop + microphone, or microphone only (dictation mode))
4. Begin your meeting/dictation
5. Once done, click "Stop Recording"
6. The application will automatically transcribe + summarise your recording
7. Listen to your recording using the built-in audio player
8. View formatted output in the dedicated markdown section

### Advanced Recording Modes
- **Loopback Only**: System audio capture with spectrum visualisation
- **Loopback + Mic**: Mixed audio with dual-source spectrum analysis
- **Dictation Mode**: Microphone-only with mic-focused visualisation

### UI Features
- **Audio Playback**: Listen to recorded WAV files directly in the GUI using secure data URLs
- **Formatted Output**: Transcripts and summaries are rendered as beautiful markdown
- **System Messages**: Clear status updates separate from formatted content
- **Real-time Feedback**: Live spectrum analyser shows audio activity during recording

## Troubleshooting

### Common Issues
1. **Audio Not Recording**: Check device permissions and default audio devices
2. **Spectrum Analyser Not Moving**: Ensure audio is playing and recording is active, ensure you are using the correct audio interface in Windows settings
3. **Whisper Errors**: Verify binary path and model existence, refer to [whisper-cli](https://github.com/ggml-org/whisper.cpp)
4. **GUI Not Responding**: Ensure WebView2 runtime is installed
5. **Audio Playback Not Working**: Check browser console for errors, verify WAV file exists
6. **Markdown Not Rendering**: Ensure internet connection for marked.js CDN, check browser console

## Future Enhancements

- Device selection for audio sources
- Advanced audio processing (noise reduction, normalisation)
- Take notes and use them in the summary automatically with timestamping
- Integration with other LLM APIs
- Different summarisation styles (casual, meeting, standup, dictation)

## License

See [LICENSE](https://github.com/abra5umente/blackbox/blob/main/LICENSE)
