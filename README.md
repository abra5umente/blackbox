# Blackbox

Windows-only audio capture and transcription tool with both CLI and a Wails-based GUI. It records system audio (WASAPI loopback) and/or microphone to WAV, transcribes with whisper.cpp, and includes a summariser stub. Features a modern, responsive UI built with Tailwind CSS.

## Layout

- `main.go` — Wails GUI entrypoint; embeds `frontend/dist` and binds `internal/ui.App`
- `cmd/gui/` — alternative GUI entry (not required to build)
- `frontend/` — static frontend assets with Tailwind CSS styling
- `internal/ui/` — GUI backend: settings, recording/transcribe/summarise APIs
- `cmd/rec/` — record desktop audio to WAV (CLI)
- `cmd/transcribe/` — run whisper.cpp on a WAV and produce `.txt` (CLI)
- `cmd/summarise/` — summariser stub (CLI)
- `internal/audio/` — WASAPI loopback + microphone capture via `malgo`
- `internal/wav/` — WAV writer that fixes headers on `Close`
- `internal/execx/` — wrapper to run whisper binary and capture logs
- `wails.json` — Wails build/runtime configuration
- `models/` — whisper ggml models (e.g., `ggml-base.en.bin`)
- `whisper-bin/` — whisper.cpp Windows binaries (e.g., `whisper-cli.exe`)
- `out/` — default output directory for WAV/TXT
- `configs/` — sample config for summariser

## Frontend & Styling

The GUI frontend is built with modern web technologies and styled using Tailwind CSS:

- **Tailwind CSS v3.4.17**: Utility-first CSS framework for rapid UI development
- **Modern Dark Theme**: Professional dark interface with blue accents
- **Responsive Design**: Clean layout with proper spacing and typography
- **Interactive Elements**: Hover effects, focus states, and smooth transitions

### Tailwind CSS Setup

The project includes a complete Tailwind CSS build pipeline:

- **Configuration**: `frontend/tailwind.config.js` - scans all HTML/JS files for classes
- **Input CSS**: `frontend/src/input.css` - contains Tailwind directives
- **Build Scripts**: 
  - `npm run tailwind:build` - one-time CSS build
  - `npm run tailwind:watch` - watch mode for development
- **Wails Integration**: `wails.json` configured with `frontend:dev:watcher` for automatic CSS rebuilding during development
- **Output**: `frontend/dist/output.css` - compiled CSS with only used utility classes

### Development Workflow

```bash
# From project root
wails dev                    # Runs Wails dev server + Tailwind watcher

# From frontend directory
npm run tailwind:build      # Build CSS once
npm run tailwind:watch      # Watch for changes
```

### Package.json Scripts

The `frontend/package.json` includes these Tailwind-related scripts:

- **`tailwind:build`**: Builds CSS once with all detected utility classes
- **`tailwind:watch`**: Watches for changes and rebuilds CSS automatically
- **Dependencies**: `tailwindcss@^3.4.0`, `postcss`, `autoprefixer`

The project root `package.json` includes these build scripts:

- **`build:css`**: Builds Tailwind CSS for production
- **`build:gui`**: Builds Tailwind CSS and then builds the Wails GUI
- **`dev`**: Runs Wails development server

## Requirements

- Windows 11
- Go 1.24+
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`) for GUI builds
- Microsoft WebView2 Runtime (usually present on Windows 11). If missing, install from the official site.
- `whisper.cpp` binaries placed in `./whisper-bin/` (e.g., `whisper-cli.exe` or `main.exe`)
- whisper model in `./models/` (e.g., `ggml-base.en.bin`)
- Node.js and npm (for Tailwind CSS build process)

## Binaries

- GUI: `build/bin/blackbox-gui.exe` (built via Wails)
- CLI: `cmd/rec/rec.exe`, `cmd/transcribe/transcribe.exe`, `cmd/summarise/summarise.exe`

## GUI Quickstart

```powershell
# Build the GUI
wails build -clean

# Run it
./build/bin/blackbox-gui.exe
```

## GUI Usage

- Record tab
  - Start/Stop recording to `OutDir`.
  - Use Microphone: mixes default mic with system audio.
  - Dictation mode (mic only): records only the microphone (use this if there’s no system/meeting audio).
- Transcribe tab
  - Choose WAV: opens a file picker rooted at `OutDir` filtered to `*.wav`.
  - Transcribe: runs whisper.cpp and writes `out/<base>.txt` and `out/<base>.log`.
- Summarise tab (stub)
  - Choose TXT: opens a file picker rooted at `OutDir` filtered to `*.txt`.
  - Summarise: reads `configs/llm.json` and prints the intended Chat Completions request.
- Record & Transcribe & Summarise tab
  - Begin: starts recording (honours Use Microphone / Dictation mode).
  - Stop Recording: finalises WAV, transcribes it, then runs the summariser stub on the produced `.txt`.
- Settings tab
  - Output Directory: set and save the output folder. Persisted at `./config/ui.json`. All flows honour this.

### UI Features

- **Modern Dark Theme**: Professional appearance with proper contrast
- **Responsive Layout**: Clean spacing and typography using Tailwind utilities
- **Interactive Elements**: Hover effects, focus states, and smooth transitions
- **Accessibility**: Proper focus indicators and disabled states

Notes

- The GUI resolves `whisper` binary and model using the current working directory (where you launch the `.exe`).
  - Binary default: `./whisper-bin/whisper-cli.exe`, fallback `./whisper-bin/main.exe`.
  - Model default: `./models/ggml-base.en.bin`.
  - You can override via environment variables below.

## Quickstart

```powershell
# 1) Record ~3 seconds of system audio (CLI)
go build ./...
./cmd/rec/rec.exe --dur 3s

# The WAV path is printed, e.g. .\out\20250829_101530.wav

# 2) Transcribe the WAV
./cmd/transcribe/transcribe.exe --wav .\out\20250829_101530.wav --model .\models\ggml-base.en.bin

# The transcript path is printed, e.g. .\out\20250829_101530.txt
```

## Development

### Building the GUI

```bash
# Build production GUI with Tailwind CSS
npm run build:gui

# Or manually:
npm run build:css && wails build -clean

# Development mode with Tailwind CSS watching
wails dev
```

### Tailwind CSS Development

```bash
# From frontend directory
cd frontend

# Install dependencies (first time only)
npm install

# Build CSS once
npm run tailwind:build

# Watch for changes during development
npm run tailwind:watch
```

The `wails dev` command automatically runs the Tailwind watcher, so CSS rebuilds happen automatically during development.

### Production Build Process

For production builds, the process ensures Tailwind CSS is properly included:

1. **CSS Build**: `npm run build:css` generates `frontend/dist/output.css` with all used utility classes
2. **Wails Build**: `wails build -clean` embeds the frontend assets (including CSS) into the executable
3. **Result**: The final `blackbox-gui.exe` includes all Tailwind CSS styling

**Note**: Always run `npm run build:css` before `wails build` to ensure the latest CSS is included, or use `npm run build:gui` which does both steps automatically.

## Environment overrides

- `LOOPBACK_NOTES_MODELS` — directory containing models (defaults to `./models`)
- `LOOPBACK_NOTES_WHISPER_BIN` — path to whisper executable (defaults to `./whisper-bin/whisper-cli.exe`, falls back to `./whisper-bin/main.exe`)
- `LOOPBACK_NOTES_OUT` — output directory (defaults to `./out`)

## rec usage

```powershell
./cmd/rec/rec.exe --dur 5m                      # record for 5 minutes
./cmd/rec/rec.exe --stop-key ctrl+shift+9       # stop on hotkey

# Flags
  --out-dir      "./out"
  --sample-rate  48000
  --bits         16
  --channels     2
  --device       ""       # optional device name/id (default render device is used)
  --dur          0        # duration (0 = manual stop)
  --stop-key     ""       # e.g. "ctrl+shift+9"
```

## transcribe usage

```powershell
./cmd/transcribe/transcribe.exe --wav .\out\20250829_101530.wav \
  --model .\models\ggml-base.en.bin \
  --lang en \
  --threads 4 \
  --out-dir .\out \
  --extra-args "-pp -su"
```

## summarise (stub)

```powershell
./cmd/summarise/summarise.exe --txt .\out\20250829_101530.txt
```

Reads `./configs/llm.json` with fields:

```json
{
  "base_url": "https://api.openai.com/v1",
  "api_key_env": "OPENAI_API_KEY",
  "model": "gpt-4o-mini"
}
```

No network request is sent in this pass; it only validates config and prints what it would call.

## Notes

- WAV writer fixes RIFF sizes on `Close` and writes PCM S16LE frames.
- Logs from whisper are saved next to the transcript: `out/<base>.log`.
- Non-zero exit codes on capture/exec/missing binary or model.

## Audio Format

- PCM S16LE, 48 kHz, stereo.
- Loopback uses the default render device; microphone uses the default capture device.
