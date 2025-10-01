//go:build !windows

package audio

import "errors"

// ErrNotSupported indicates audio capture is unavailable on this platform.
var ErrNotSupported = errors.New("audio capture not supported on this platform")

// Recorder is a stub implementation used in non-Windows builds to satisfy imports.
type Recorder struct{}

func NewRecorder(bufferCallbacks int) (*Recorder, error) {
	return nil, ErrNotSupported
}

func (r *Recorder) Start(sampleRate uint32, channels uint32) error {
	return ErrNotSupported
}

func (r *Recorder) Data() <-chan []byte {
	return nil
}

func (r *Recorder) Stop() {}

// MicRecorder is a stub implementation used in non-Windows builds to satisfy imports.
type MicRecorder struct{}

func NewMicRecorder(bufferCallbacks int) (*MicRecorder, error) {
	return nil, ErrNotSupported
}

func (r *MicRecorder) Start(sampleRate uint32, channels uint32) error {
	return ErrNotSupported
}

func (r *MicRecorder) Data() <-chan []byte {
	return nil
}

func (r *MicRecorder) Stop() {}
