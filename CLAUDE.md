# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Blackbox is a Windows-only meeting and dictation recorder with local transcription and AI-powered summarization. It captures audio via WASAPI loopback (system audio) and/or microphone input, transcribes using whisper.cpp, and provides AI summaries via local llama.cpp or remote OpenAI-compatible APIs. Built with Wails (Go backend + WebView2 frontend), it includes a SQLite database for managing recordings, transcripts, and summaries.

## Build Commands

### Development
```bash
wails dev  # Runs dev server with hot reload and automatic Tailwind CSS watching
```

### Production
```bash
wails build  # Builds final executable with automatic Tailwind CSS compilation
# Output: build/bin/blackbox-gui.exe
```

### Tailwind CSS
- **Development**: Automatically watches via `wails dev` (configured in wails.json)
- **Manual build**: `npm run tailwind:build` (in frontend directory)
- **Input**: `frontend/dist/input.css`
- **Output**: `frontend/dist/output.css`

### Testing
```powershell
# PowerShell (Windows - recommended)
./run-tests.ps1
```
```bash
# Git Bash / WSL / macOS / Linux
./run-tests.sh
```
- Test scripts automatically set `GOCACHE` and `BLACKBOX_MIGRATIONS_DIR` for isolated test environments
- Non-Windows builds use stub files (`audio_stub.go`, `sys_other.go`) to satisfy imports
- Windows-specific functionality (audio capture, process window hiding) only works on Windows builds

## Architecture

### Backend (Go)
- **Entry point**: `main.go` - Wails app initialization
- **Core logic**: `internal/ui/app.go` - Main app struct with all GUI-exposed methods
- **Logging**: Structured logging with `log/slog` (JSON format) - see logger variable in app.go
- **Audio capture**: `internal/audio/` - WASAPI loopback and microphone recording via malgo
- **WAV handling**: `internal/wav/writer.go` - PCM S16LE WAV file writing
- **External processes**: `internal/execx/execx.go` - Wraps whisper.cpp execution
- **Database**: `internal/db/` - SQLite operations with automatic migrations

### Frontend (HTML/CSS/JS)
- **Location**: `frontend/dist/` - Static assets embedded at compile time
- **Framework**: Vanilla JavaScript with Wails runtime bindings
- **Styling**: Tailwind CSS v3.4.17 with typography plugin for markdown rendering
- **UI**: Three-tab interface (Auto, Tools, Settings)

### Database
- **Location**: `./data/blackbox.db` (configurable)
- **Migrations**: `migrations/*.sql` - Numbered files loaded automatically on startup
- **Schema**: recordings (metadata only, no audio) → transcripts → summaries with full metadata tracking
- **Features**: FTS5 full-text search, tagging system, processing metadata, error tracking for failed transcriptions
- **Storage**: Minimal size (~1KB per recording) - audio data NOT stored in database

### Configuration
- **GUI settings**: `./config/ui.json` - Output directory, database path, local AI settings
- **Remote AI**: `./configs/remote.json` - OpenAI-compatible API configuration
- **Local AI**: `./configs/local.json` - llama-server client authentication
- **Prompts**: `./config/*.json` - Summarization prompt templates (meeting, dictation, technical)

## Key Components

### Recording System
- **Loopback recorder** (`internal/audio/loopback.go`): Captures system audio via WASAPI loopback
- **Mic recorder** (`internal/audio/mic.go`): Captures microphone input
- **Audio format**: PCM S16LE, 16kHz, mono (optimized for speech recognition)
- **Mixing**: Sample-wise averaging when combining loopback + mic
- **Real-time events**: Backend emits raw PCM data to frontend every frame via `wruntime.EventsEmit`

### Transcription
- **Engine**: whisper.cpp (external binary)
- **Binary location**: `./whisper-bin/whisper-cli.exe`
- **Models**: `./models/` directory (default: `ggml-base.en.bin`)
- **Process**: Executes whisper binary, reads .txt output, stores in database
- **Logs**: Generated in temp directory with transcript output
- **API Methods**:
  - `TranscribeRecording(recordingID int)` - Type-safe, recommended
  - `TranscribeFile(wavPath string)` - For ad-hoc files
  - `Transcribe(wavPathOrID string)` - **Deprecated**, forwards to above methods

### AI Summarization
Two modes controlled by `use_local_ai` setting:

1. **Local AI** (llama.cpp):
   - Binary: `./llamacpp-bin/llama-server.exe`
   - Auto-starts server on-demand, shuts down after use
   - Configuration: Model path, temperature, context window, API key
   - Privacy-focused (no external API calls)
   - **Logging**: Server output logged to `./out/llama-server.log`
   - **VRAM monitoring**: Detects out-of-memory errors and suggests reducing context size
   - **Malformed JSON handling**: Automatically fixes LLM responses with literal newlines in JSON strings

2. **Remote AI** (OpenAI-compatible):
   - Configurable base URL, API key, model
   - Default: OpenAI GPT-4o-mini
   - Supports any OpenAI-compatible endpoint

### Database Operations
- **Automatic migrations**: Loads `.sql` files from `migrations/` directory on startup (configurable via `BLACKBOX_MIGRATIONS_DIR` env var)
- **CRUD operations**: Separate files per entity (`recordings.go`, `transcripts.go`, `summaries.go`)
- **Search**: FTS5 full-text search on transcript content
- **Relationships**: Foreign keys with cascade delete
- **Metadata tracking**: Processing times, model versions, error logs
- **Backup/Restore**: Manual file copy of `./data/blackbox.db` (plain SQLite file, no encryption)

### Frontend Features
- **Real-time spectrum analyzer**: 32-bar frequency visualization (60fps animation)
- **Markdown rendering**: Uses marked.js (CDN) for formatted transcript/summary display
- **Copy to clipboard**: One-click summary copying with visual feedback
- **Tab structure**:
  - **Auto**: Complete workflow (record → transcribe → summarize) with formatted output
  - **Tools**: Individual operations with manual control
  - **Settings**: Configuration management
  - **Database**: Browse recordings, transcripts, and summaries with search/filter

## Wails-Specific Patterns

### Backend Methods (exposed to frontend)
- Struct methods on `App` are automatically bound to frontend
- Return `(result, error)` pattern for all methods
- Use `wruntime.EventsEmit(ctx, event, data)` for real-time updates
- File dialogs: `wruntime.OpenFileDialog`, `wruntime.SaveFileDialog`

### Frontend Calls
- Import from `./wailsjs/go/ui/App.js`
- Example: `await StartRecording(true)` calls Go method
- Event listening: `runtime.EventsOn("audioData", handler)`

### Context Management
- `uiCtx` stored in App via `SetUIContext()` during `OnStartup`
- Required for all Wails runtime operations (dialogs, events)

## Important Implementation Details

### Audio Data Flow
1. malgo captures PCM frames → channel
2. Go goroutine reads frames → writes to WAV + emits to frontend
3. Frontend receives via `runtime.EventsOn("audioData")` → updates spectrum analyzer
4. On stop: finalize WAV, store metadata in database (audio_data remains NULL)

### Recording Modes
- **isToolsMode=true**: Saves to `OutDir/saved/` permanently (manual workflow)
- **isToolsMode=false**: Saves to `OutDir/` temporarily, metadata stored in database, WAV deleted after successful transcription or moved to `OutDir/retry/` on failure

### Local AI Server Management
- `startLlamaServer()`: Spawns llama-server.exe with hidden window
  - Logs stdout/stderr to `./out/llama-server.log`
  - Detects VRAM allocation failures and provides helpful error messages
  - On failure, shows last 500 chars of log plus full log path
- `waitForLlamaServer()`: Polls `/health` endpoint until ready (30s timeout)
- `stopLlamaServer()`: Graceful kill with 5s timeout, then force kill
  - Properly closes log file handle
- Server runs on `127.0.0.1:8080` during summarization only
- **JSON Response Handling**: Automatically fixes malformed JSON from LLMs that output literal newlines instead of `\n` in string values

### Audio Storage Philosophy
- **Audio data is NOT stored in database** - only metadata (filename, duration, sample rate, etc.)
- **Successful transcription**: WAV file is deleted permanently (blackbox philosophy: audio in → text out → audio gone)
  - File existence checked before deletion to avoid noisy warnings
  - Deletion failures tracked in `error_message` field for later investigation
- **Failed transcription**: WAV file moved to `OutDir/retry/` with error tracking in database for manual retry
- **Tools mode exception**: WAV files saved to `OutDir/saved/` permanently for manual management
- **Database size**: Stays minimal (~1KB per recording) since only text/metadata is stored

### Migration System
- Loads numbered SQL files (`001_*.sql`, `002_*.sql`, etc.) from `migrations/`
- Tracks applied migrations in `schema_migrations` table
- Splits statements on `;` (handles BEGIN/END blocks correctly)
- Applies in transaction (rollback on error)
- **Latest migration (007)**: Dropped `audio_data` column from recordings table (enforces audio-never-in-DB philosophy)

## Common Development Tasks

### Adding a New Recording Mode
1. Modify `internal/ui/app.go:startRecordingAdvancedInternal()` to handle new mode
2. Update frontend with new checkbox/option in Auto/Tools tab
3. Test audio mixing logic if combining sources

### Extending Database Schema
1. Create new migration file: `migrations/00X_description.sql`
2. Write SQL statements (CREATE TABLE, ALTER TABLE, etc.)
3. Update Go structs in `internal/db/db.go`
4. Add CRUD methods in appropriate `internal/db/*.go` file
5. Restart app (migrations apply automatically)

### Adding Custom Prompts
1. Create JSON file in `config/` directory with structure:
   ```json
   {
     "name": "custom_prompt",
     "description": "Brief description",
     "prompt": "System prompt text..."
   }
   ```
2. Add to database via `CreatePrompt()` method (or handled automatically at startup)
3. Prompt appears in frontend dropdown

### Modifying UI Styling
1. Edit HTML in `frontend/dist/index.html`
2. Use Tailwind utility classes (no custom CSS needed)
3. For development: `wails dev` (auto-rebuilds CSS on change)
4. For production: CSS automatically rebuilt during `wails build`

## External Dependencies

### Required Binaries (not in repo)
- **whisper.cpp**: Extract to `./whisper-bin/`
  - Required file: `whisper-cli.exe`
- **llama.cpp**: Extract to `./llamacpp-bin/`
  - Required file: `llama-server.exe`
  - Download Vulkan/CUDA version matching your system

### Models (not in repo)
- **Whisper models**: Place in `./models/` directory
  - Default: `ggml-base.en.bin`
  - Download from whisper.cpp releases
- **Llama models**: Place in `./models/` directory
  - Format: GGUF files (e.g., `gemma-3-12b-it-q4_0.gguf`)
  - Configure path in Settings tab

### Go Dependencies
- `github.com/wailsapp/wails/v2` - GUI framework
- `github.com/gen2brain/malgo` - Audio capture (malgo wraps miniaudio)
- `modernc.org/sqlite` - Pure Go SQLite driver

### Frontend Dependencies
- Tailwind CSS v3.4.17 + plugins (installed via `npm install` in `frontend/`)
- marked.js v12.0.0 (loaded via CDN, no install needed)

## File Structure Notes

### Output Directory (`./out/`)
- **Auto mode**: Temporary WAV files (deleted after successful transcription, moved to `./out/retry/` on failure)
- **Tools mode**: `./out/saved/` - Permanent storage for manual workflow
- **Retry directory**: `./out/retry/` - Failed transcriptions with error tracking in database
- **Log files**:
  - `llama-server.log` - llama-server stdout/stderr for debugging local AI issues
  - Transcription logs in temp directory
- Transcripts: `.txt` files (legacy file-based workflow)
- Summaries: `*_summary.txt` files (legacy file-based workflow)
- **Note**: Database is primary storage; audio is NOT stored in database

### Config Directory (`./config/`)
- `ui.json` - Main application settings (auto-created with defaults)
- Prompt JSON files - Summarization templates

### Configs Directory (`./configs/`)
- `local.json` - llama-server client config (API key, base URL)
- `remote.json` - Remote AI config (OpenAI, etc.)

## Platform Restrictions

- **Windows only**: Uses WASAPI for audio capture
- Requires **WebView2 runtime** (usually pre-installed on Windows 11)
- Build tags: `//go:build windows` in audio capture files

## Debugging Tips

### Audio Issues
- Check Windows default devices (Settings → Sound)
- Verify WASAPI loopback device is available
- Test spectrum analyzer movement (indicates audio flow)
- Check console for malgo errors

### Transcription Issues
- Verify `whisper-cli.exe` exists and is executable
- Check model file path in code (`./models/ggml-base.en.bin`)
- Look for `.log` files in temp directory for whisper output
- Ensure WAV file is valid (16kHz, mono, S16LE)

### Local AI Issues
- Verify `llama-server.exe` exists in `./llamacpp-bin/`
- Check model file path in Settings tab
- Test manual startup: `./llamacpp-bin/llama-server.exe --help`
- Check server logs in `./out/llama-server.log` for detailed error messages
- **VRAM allocation errors**: Reduce context size in Settings (try 4096 or 8192 instead of 16000+)
- **JSON parsing errors**: Check logs for "failed to parse structured summary payload" - the system now automatically fixes malformed JSON with literal newlines
- Ensure sufficient VRAM for model (close GPU-intensive apps if needed)
- For large models, consider reducing `llama_context` to 4096-8192 to fit in available VRAM

### Database Issues
- Check `./data/blackbox.db` exists and is writable
- Verify migrations were applied: check `schema_migrations` table
- Look for SQL errors in console output during startup
- Test with new database if corruption suspected

### UI Issues
- Check browser console (F12) for JavaScript errors
- Verify CSS was built: `frontend/dist/output.css` should exist and be >1KB
- Test with `wails dev` to see live reload behavior
- Check Wails bindings: `frontend/wailsjs/` should be populated

## Code Style Conventions

- **Go**: Standard Go formatting (`gofmt`)
- **Error handling**: Always return errors, use structured logging for warnings/info
- **Logging**: Use `logger.Info/Warn/Error()` with key-value pairs (not fmt.Printf)
- **File paths**: Use `filepath.Join()` for cross-platform compatibility
- **SQL**: Use parameterized queries (never string concatenation)
- **Frontend**: Vanilla JavaScript (no frameworks), Tailwind utility classes
- **Comments**: Document public methods and complex logic
- **API methods**: Return `(result, error)` pattern for all Wails-exposed methods

## Git Repository

- **Main branch**: `main`
- **Current branch**: `next`
- Recent commits focus on: Testing infrastructure, structured logging, database optimization, type-safe APIs, UI enhancements

## Recent Updates

### October 2025
- **Testing Infrastructure**: Cross-platform test scripts (`run-tests.ps1`, `run-tests.sh`) with environment variable support
- **Structured Logging**: Migrated to `log/slog` (JSON format) for better debugging and production monitoring
- **Type-Safe APIs**: New `TranscribeRecording(id)` and `TranscribeFile(path)` methods (deprecated old `Transcribe(string)`)
- **Database Optimization**: Migration 007 removed `audio_data` column (enforces audio-never-in-DB philosophy)
- **Error Tracking**: Enhanced error handling with proper null value management and LIKE pattern escaping
- **UI Improvements**: Added "Copy Summary" button in Auto tab with visual feedback
- **Documentation**: Comprehensive updates to CLAUDE.md, README.md, and agents.md with testing patterns and debugging tips