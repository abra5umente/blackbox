package execx

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildWhisperArgs(t *testing.T) {
	args := BuildWhisperArgs("model.bin", "clip.wav", "en", 4, "out/base", "--foo bar")

	joined := strings.Join(args, " ")
	for _, expected := range []string{"-m model.bin", "-f clip.wav", "-otxt", "-l en", "-t 4", "-of out/base", "--foo", "bar"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected args to contain %q, got %q", expected, joined)
		}
	}
}

func TestRunWhisperSuccess(t *testing.T) {
	tmp := t.TempDir()
	wavPath := filepath.Join(tmp, "audio.wav")
	modelPath := filepath.Join(tmp, "model.bin")
	outDir := filepath.Join(tmp, "out")

	if err := os.WriteFile(wavPath, []byte("wav"), 0644); err != nil {
		t.Fatalf("write wav failed: %v", err)
	}
	if err := os.WriteFile(modelPath, []byte("model"), 0644); err != nil {
		t.Fatalf("write model failed: %v", err)
	}

	bin := buildStubWhisper(t, tmp)

	got, err := RunWhisper(bin, modelPath, wavPath, outDir, "en", 2, "")
	if err != nil {
		t.Fatalf("RunWhisper failed: %v", err)
	}

	want := filepath.Join(outDir, strings.TrimSuffix(filepath.Base(wavPath), filepath.Ext(wavPath))+
		".txt")
	if got != want {
		t.Fatalf("unexpected transcript path: got %q want %q", got, want)
	}

	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("read transcript failed: %v", err)
	}

	if string(data) != "stub transcript" {
		t.Fatalf("unexpected transcript contents: %q", data)
	}

	logPath := strings.TrimSuffix(got, ".txt") + ".log"
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("expected log file: %v", err)
	}
}

func TestRunWhisperErrors(t *testing.T) {
	tmp := t.TempDir()
	model := filepath.Join(tmp, "model.bin")
	os.WriteFile(model, []byte("model"), 0644)

	bin := buildStubWhisper(t, tmp)

	if _, err := RunWhisper(bin, model, filepath.Join(tmp, "missing.wav"), tmp, "", 0, ""); err == nil {
		t.Fatalf("expected error for missing wav")
	}

	wav := filepath.Join(tmp, "audio.wav")
	os.WriteFile(wav, []byte("wav"), 0644)

	if _, err := RunWhisper("", model, wav, tmp, "", 0, ""); err == nil {
		t.Fatalf("expected error for missing binary")
	}

	if _, err := RunWhisper(bin, filepath.Join(tmp, "missing.bin"), wav, tmp, "", 0, ""); err == nil {
		t.Fatalf("expected error for missing model")
	}
}

func buildStubWhisper(t *testing.T, dir string) string {
	t.Helper()

	source := `package main
import (
	"fmt"
	"os"
)
func main() {
	args := os.Args[1:]
	var outBase string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-of":
			i++
			if i < len(args) {
				outBase = args[i]
			}
		}
	}
	if outBase == "" {
		fmt.Fprintln(os.Stderr, "missing -of")
		os.Exit(1)
	}
	if err := os.WriteFile(outBase+".txt", []byte("stub transcript"), 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
`

	src := filepath.Join(dir, "stubwhisper.go")
	if err := os.WriteFile(src, []byte(source), 0644); err != nil {
		t.Fatalf("write stub whisper source failed: %v", err)
	}

	bin := filepath.Join(dir, "stubwhisper")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", bin, src)
	gocache := filepath.Join(dir, "gocache")
	_ = os.MkdirAll(gocache, 0755)
	cmd.Env = append(os.Environ(), "GOCACHE="+gocache)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build stub whisper: %v", err)
	}

	return bin
}
