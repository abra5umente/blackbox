package wav

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Writer writes a PCM WAV file with a correct RIFF header.
// Call Close to fix header sizes.
type Writer struct {
	file          *os.File
	buf           *bufio.Writer
	sampleRate    uint32
	channels      uint16
	bitsPerSample uint16
	dataSize      uint32
	closed        bool
}

// NewWriter creates a new WAV writer and writes the header with placeholder sizes.
// Only PCM S16LE frames are supported (bitsPerSample must be 16).
func NewWriter(path string, sampleRate uint32, channels, bitsPerSample uint16) (*Writer, error) {
	if bitsPerSample != 16 {
		return nil, fmt.Errorf("only 16-bit PCM supported, got %d", bitsPerSample)
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	w := &Writer{
		file:          f,
		buf:           bufio.NewWriterSize(f, 1<<20), // 1 MiB buffer
		sampleRate:    sampleRate,
		channels:      channels,
		bitsPerSample: bitsPerSample,
	}
	if err := w.writeHeader(); err != nil {
		f.Close()
		return nil, err
	}
	return w, nil
}

func (w *Writer) writeHeader() error {
	// RIFF chunk descriptor
	if _, err := w.buf.WriteString("RIFF"); err != nil {
		return err
	}
	// ChunkSize placeholder (36 + Subchunk2Size)
	if err := binary.Write(w.buf, binary.LittleEndian, uint32(0)); err != nil {
		return err
	}
	if _, err := w.buf.WriteString("WAVE"); err != nil {
		return err
	}

	// fmt subchunk
	if _, err := w.buf.WriteString("fmt "); err != nil {
		return err
	}
	if err := binary.Write(w.buf, binary.LittleEndian, uint32(16)); err != nil { // Subchunk1Size for PCM
		return err
	}
	if err := binary.Write(w.buf, binary.LittleEndian, uint16(1)); err != nil { // AudioFormat PCM
		return err
	}
	if err := binary.Write(w.buf, binary.LittleEndian, w.channels); err != nil {
		return err
	}
	if err := binary.Write(w.buf, binary.LittleEndian, w.sampleRate); err != nil {
		return err
	}
	byteRate := w.sampleRate * uint32(w.channels) * uint32(w.bitsPerSample) / 8
	if err := binary.Write(w.buf, binary.LittleEndian, byteRate); err != nil {
		return err
	}
	blockAlign := w.channels * w.bitsPerSample / 8
	if err := binary.Write(w.buf, binary.LittleEndian, blockAlign); err != nil {
		return err
	}
	if err := binary.Write(w.buf, binary.LittleEndian, w.bitsPerSample); err != nil {
		return err
	}

	// data subchunk
	if _, err := w.buf.WriteString("data"); err != nil {
		return err
	}
	// Subchunk2Size placeholder
	if err := binary.Write(w.buf, binary.LittleEndian, uint32(0)); err != nil {
		return err
	}
	return w.buf.Flush()
}

// Write writes raw PCM bytes (S16LE) to the WAV file.
func (w *Writer) Write(p []byte) (int, error) {
	if w.closed {
		return 0, io.ErrClosedPipe
	}
	n, err := w.buf.Write(p)
	w.dataSize += uint32(n)
	if err != nil {
		return n, err
	}
	return n, nil
}

// Flush forces buffered data to disk.
func (w *Writer) Flush() error {
	if w.closed {
		return nil
	}
	return w.buf.Flush()
}

// Close updates the RIFF header sizes and closes the file.
func (w *Writer) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	if err := w.buf.Flush(); err != nil {
		w.file.Close()
		return err
	}

	// Update ChunkSize and Subchunk2Size
	if _, err := w.file.Seek(4, io.SeekStart); err != nil {
		w.file.Close()
		return err
	}
	if err := binary.Write(w.file, binary.LittleEndian, uint32(36)+w.dataSize); err != nil {
		w.file.Close()
		return err
	}
	if _, err := w.file.Seek(40, io.SeekStart); err != nil {
		w.file.Close()
		return err
	}
	if err := binary.Write(w.file, binary.LittleEndian, w.dataSize); err != nil {
		w.file.Close()
		return err
	}
	return w.file.Close()
}
