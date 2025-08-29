package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"blackbox/internal/execx"
)

func main() {
	outDirDefault := getenvDefault("LOOPBACK_NOTES_OUT", "./out")
	whisperDefault := getenvDefault("LOOPBACK_NOTES_WHISPER_BIN", "./whisper-bin/whisper-cli.exe")
	modelDefault := filepath.Join(getenvDefault("LOOPBACK_NOTES_MODELS", "./models"), "ggml-base.en.bin")

	var (
		wavPath    = flag.String("wav", "", "Path to WAV file")
		whisperBin = flag.String("whisper-bin", whisperDefault, "Path to whisper binary (whisper-cli.exe or main.exe)")
		modelPath  = flag.String("model", modelDefault, "Path to model (e.g., ./models/ggml-base.en.bin)")
		lang       = flag.String("lang", "en", "Language code (optional)")
		threads    = flag.Int("threads", 0, "Threads (optional)")
		outDir     = flag.String("out-dir", outDirDefault, "Output directory for transcript/log")
		extra      = flag.String("extra-args", "", "Additional args to pass to whisper")
	)
	flag.Parse()

	if *wavPath == "" {
		fatal(2, "--wav is required")
	}
	if _, err := os.Stat(*wavPath); err != nil {
		fatal(3, "wav not found: %v", err)
	}

	bin := *whisperBin
	if _, err := os.Stat(bin); err != nil {
		// fallback to main.exe in same dir
		if filepath.Base(bin) == "whisper-cli.exe" {
			alt := filepath.Join(filepath.Dir(bin), "main.exe")
			if _, e2 := os.Stat(alt); e2 == nil {
				bin = alt
			}
		}
	}

	if _, err := os.Stat(bin); err != nil {
		fatal(4, "whisper binary missing: %v", err)
	}
	if _, err := os.Stat(*modelPath); err != nil {
		fatal(5, "model missing: %v", err)
	}
	if err := os.MkdirAll(*outDir, 0755); err != nil {
		fatal(6, "create out dir: %v", err)
	}

	txtPath, err := execx.RunWhisper(bin, *modelPath, *wavPath, *outDir, *lang, *threads, *extra)
	if err != nil {
		fatal(7, "%v", err)
	}
	fmt.Println(txtPath)
}

func getenvDefault(k, def string) string {
	if v := os.Getenv(k); strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

func fatal(code int, format string, a ...any) {
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(os.Stderr, "%s ERROR: "+format+"\n", append([]any{ts}, a...)...)
	os.Exit(code)
}
