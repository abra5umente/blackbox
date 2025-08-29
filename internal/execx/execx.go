package execx

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BuildWhisperArgs builds arguments for whisper.cpp CLI.
// It uses -m <model> -f <wav> -otxt and, if outBase provided, -of <outBase>.
func BuildWhisperArgs(modelPath, wavPath, lang string, threads int, outBase string, extraArgs string) []string {
	args := []string{"-m", modelPath, "-f", wavPath, "-otxt"}
	if lang != "" {
		args = append(args, "-l", lang)
	}
	if threads > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", threads))
	}
	if outBase != "" {
		args = append(args, "-of", outBase)
	}
	if strings.TrimSpace(extraArgs) != "" {
		// naive split; keep simple for now
		parts := strings.Fields(extraArgs)
		args = append(args, parts...)
	}
	return args
}

// RunWhisper runs the whisper binary and returns the transcript .txt path.
// Logs are written to outDir/<base>.log.
func RunWhisper(whisperBin, modelPath, wavPath, outDir, lang string, threads int, extraArgs string) (string, error) {
	if _, err := os.Stat(wavPath); err != nil {
		return "", fmt.Errorf("wav missing: %w", err)
	}
	if whisperBin == "" {
		return "", errors.New("whisper binary not specified")
	}
	if _, err := os.Stat(whisperBin); err != nil {
		return "", fmt.Errorf("whisper binary missing: %w", err)
	}
	if _, err := os.Stat(modelPath); err != nil {
		return "", fmt.Errorf("model missing: %w", err)
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}

	baseName := strings.TrimSuffix(filepath.Base(wavPath), filepath.Ext(wavPath))
	outBase := filepath.Join(outDir, baseName)
	txtPath := outBase + ".txt"
	logPath := outBase + ".log"

	args := BuildWhisperArgs(modelPath, wavPath, lang, threads, outBase, extraArgs)

	cmd := exec.Command(whisperBin, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()

	// Write combined logs
	_ = os.WriteFile(logPath, append(stdoutBuf.Bytes(), stderrBuf.Bytes()...), 0644)

	if err != nil {
		return "", fmt.Errorf("whisper failed: %w", err)
	}

	if _, err := os.Stat(txtPath); err == nil {
		return txtPath, nil
	}

	// Fallback: create txt from stdout if flag unsupported
	if stdoutBuf.Len() > 0 {
		if writeErr := os.WriteFile(txtPath, stdoutBuf.Bytes(), 0644); writeErr == nil {
			return txtPath, nil
		}
	}
	return "", fmt.Errorf("transcript not produced: expected %s", txtPath)
}
