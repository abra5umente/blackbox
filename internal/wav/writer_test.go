package wav

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestNewWriterRejectsUnsupportedBitDepth(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.wav")
	if _, err := NewWriter(path, 16000, 1, 24); err == nil {
		t.Fatalf("expected error for non-16-bit PCM")
	}
}

func TestWriterWritesHeaderAndData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.wav")

	w, err := NewWriter(path, 16000, 1, 16)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	payload := []byte{0x01, 0x02, 0x03, 0x04}
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if string(data[:4]) != "RIFF" {
		t.Fatalf("expected RIFF header, got %q", data[:4])
	}

	chunkSize := binary.LittleEndian.Uint32(data[4:8])
	subchunk2Size := binary.LittleEndian.Uint32(data[40:44])

	if want := uint32(36 + len(payload)); chunkSize != want {
		t.Fatalf("chunkSize mismatch: got %d want %d", chunkSize, want)
	}
	if want := uint32(len(payload)); subchunk2Size != want {
		t.Fatalf("subchunk2Size mismatch: got %d want %d", subchunk2Size, want)
	}

	if got := data[len(data)-len(payload):]; !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: got %v want %v", got, payload)
	}
}
