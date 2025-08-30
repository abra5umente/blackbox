//go:build windows

package audio

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
)

// Recorder captures system (render) audio via WASAPI loopback.
// It emits raw PCM S16LE frames (interleaved) through a channel.
type Recorder struct {
	ctx       *malgo.AllocatedContext
	device    *malgo.Device
	onceClose sync.Once
	dataCh    chan []byte
	errCh     chan error
	wg        sync.WaitGroup
}

// NewRecorder initializes a WASAPI loopback recorder with given buffer capacity.
// bufferFrames controls internal queue size in number of callbacks; higher reduces risk of drop.
func NewRecorder(bufferCallbacks int) (*Recorder, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {
		// Optional: log miniaudio messages
		// fmt.Println("malgo:", message)
		_ = message
	})
	if err != nil {
		return nil, fmt.Errorf("init malgo context: %w", err)
	}

	r := &Recorder{
		ctx:    ctx,
		dataCh: make(chan []byte, bufferCallbacks),
		errCh:  make(chan error, 1),
	}
	return r, nil
}

// Start opens the default render device in loopback with the specified format.
// sampleRate must match device mix rate (16k recommended for speech). channels=1 (mono) recommended, format S16.
func (r *Recorder) Start(sampleRate uint32, channels uint32) error {
	if r.ctx == nil {
		return errors.New("context not initialized")
	}
	// Prefer loopback device type (miniaudio supports this on WASAPI)
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Loopback)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = uint32(channels)
	deviceConfig.SampleRate = sampleRate
	// Leave device IDs nil to use defaults (default render device for loopback)

	callbacks := malgo.DeviceCallbacks{
		Data: func(pOutputSample, pInputSample []byte, frameCount uint32) {
			// Copy buffer to avoid reuse by backend
			b := make([]byte, len(pInputSample))
			copy(b, pInputSample)
			select {
			case r.dataCh <- b:
			default:
				// Drop if slow consumer; better to drop than block audio thread
			}
		},
		Stop: func() {
			// Signal completion
			select {
			case r.errCh <- ioClosed:
			default:
			}
		},
	}

	dev, err := malgo.InitDevice(r.ctx.Context, deviceConfig, callbacks)
	if err != nil {
		r.ctx.Uninit()
		return fmt.Errorf("init device: %w", err)
	}
	r.device = dev
	if err := r.device.Start(); err != nil {
		r.device.Uninit()
		r.ctx.Uninit()
		return fmt.Errorf("start device: %w", err)
	}
	return nil
}

var ioClosed = errors.New("device stopped")

// Data returns the channel of PCM S16LE interleaved frames.
func (r *Recorder) Data() <-chan []byte { return r.dataCh }

// Errors emits terminal errors or device stop events.
func (r *Recorder) Errors() <-chan error { return r.errCh }

// Stop stops the device and closes channels after draining briefly.
func (r *Recorder) Stop() {
	r.onceClose.Do(func() {
		if r.device != nil {
			_ = r.device.Stop()
			r.device.Uninit()
			r.device = nil
		}
		if r.ctx != nil {
			r.ctx.Uninit()
			r.ctx = nil
		}
		close(r.dataCh)
	})
}

// RunUntil runs the recorder, forwarding samples into the provided sink function.
// It returns when context is done, an error occurs, or device stops.
func (r *Recorder) RunUntil(ctx context.Context, sink func([]byte) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-r.errCh:
			return err
		case b, ok := <-r.dataCh:
			if !ok {
				return nil
			}
			if len(b) == 0 {
				continue
			}
			if err := sink(b); err != nil {
				return err
			}
		}
	}
}

// Sleep is a helper that blocks for d while letting callbacks run.
func Sleep(d time.Duration) { time.Sleep(d) }
