## Blackbox

Tiny Windows-only CLI pipeline to record system audio (WASAPI loopback), transcribe it with whisper.cpp, and scaffold a future summariser.

### Layout
- `cmd/rec/` — record desktop audio to WAV
- `cmd/transcribe/` — run whisper.cpp on a WAV and produce `.txt`
- `cmd/summarise/` — stub that reads config and shows intended OpenAI-compatible request
- `internal/audio/` — WASAPI loopback using `malgo`
- `internal/wav/` — simple WAV writer that fixes headers on `Close`
- `internal/execx/` — wrapper to run whisper binary and capture logs
- `models/` — whisper ggml models (e.g., `ggml-base.en.bin`)
- `whisper-bin/` — whisper.cpp Windows binaries (e.g., `whisper-cli.exe`)
- `out/` — default output directory for WAV/TXT
- `configs/` — sample config for summariser

### Requirements
- Windows 11
- Go 1.22+
- `whisper.cpp` binaries placed in `./whisper-bin/` (e.g., `whisper-cli.exe`)
- whisper model in `./models/` (e.g., `ggml-base.en.bin`)

### Quickstart
```powershell
# 1) Record ~3 seconds of system audio
go build ./...
./cmd/rec/rec.exe --dur 3s

# The WAV path is printed, e.g. .\out\20250829_101530.wav

# 2) Transcribe the WAV
./cmd/transcribe/transcribe.exe --wav .\out\20250829_101530.wav --model .\models\ggml-base.en.bin

# The transcript path is printed, e.g. .\out\20250829_101530.txt
```

### Environment overrides
- `LOOPBACK_NOTES_MODELS` — directory containing models (defaults to `./models`)
- `LOOPBACK_NOTES_WHISPER_BIN` — path to whisper executable (defaults to `./whisper-bin/whisper-cli.exe`, falls back to `./whisper-bin/main.exe`)
- `LOOPBACK_NOTES_OUT` — output directory (defaults to `./out`)

### rec usage
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

### transcribe usage
```powershell
./cmd/transcribe/transcribe.exe --wav .\out\20250829_101530.wav \
  --model .\models\ggml-base.en.bin \
  --lang en \
  --threads 4 \
  --out-dir .\out \
  --extra-args "-pp -su"
```

### summarise (stub)
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

### Notes
- WAV writer fixes RIFF sizes on `Close` and writes PCM S16LE frames.
- Logs from whisper are saved next to the transcript: `out/<base>.log`.
- Non-zero exit codes on capture/exec/missing binary or model.


