//go:build windows

package audio

import (
	"context"
	"errors"
	"fmt"

	"github.com/gen2brain/malgo"
)

// MicRecorder captures default microphone audio (WASAPI capture).
// It emits raw PCM S16LE frames (interleaved) through a channel.
type MicRecorder struct {
	ctx    *malgo.AllocatedContext
	device *malgo.Device
	dataCh chan []byte
}

func NewMicRecorder(bufferCallbacks int) (*MicRecorder, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {
		_ = message
	})
	if err != nil {
		return nil, fmt.Errorf("init malgo context (mic): %w", err)
	}
	return &MicRecorder{
		ctx:    ctx,
		dataCh: make(chan []byte, bufferCallbacks),
	}, nil
}

func (r *MicRecorder) Start(sampleRate uint32, channels uint32) error {
	if r.ctx == nil {
		return errors.New("context not initialized")
	}
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = channels
	deviceConfig.SampleRate = sampleRate

	callbacks := malgo.DeviceCallbacks{
		Data: func(pOutputSample, pInputSample []byte, frameCount uint32) {
			b := make([]byte, len(pInputSample))
			copy(b, pInputSample)
			select {
			case r.dataCh <- b:
			default:
			}
		},
	}

	dev, err := malgo.InitDevice(r.ctx.Context, deviceConfig, callbacks)
	if err != nil {
		r.ctx.Uninit()
		return fmt.Errorf("init mic device: %w", err)
	}
	r.device = dev
	if err := r.device.Start(); err != nil {
		r.device.Uninit()
		r.ctx.Uninit()
		return fmt.Errorf("start mic device: %w", err)
	}
	return nil
}

func (r *MicRecorder) Data() <-chan []byte { return r.dataCh }

func (r *MicRecorder) Stop() {
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
}

func (r *MicRecorder) RunUntil(ctx context.Context, sink func([]byte) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
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
