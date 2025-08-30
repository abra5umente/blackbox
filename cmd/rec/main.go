package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"blackbox/internal/audio"
	"blackbox/internal/wav"

	"golang.org/x/sys/windows"
)

func main() {
	// Flags
	outDirDefault := getenvDefault("LOOPBACK_NOTES_OUT", "./out")
	var (
		outDir     = flag.String("out-dir", outDirDefault, "Output directory")
		sampleRate = flag.Uint("sample-rate", 16000, "Sample rate (Hz) - 16kHz recommended for speech") // Changed from 48000
		bits       = flag.Uint("bits", 16, "Bits per sample (16)")
		channels   = flag.Uint("channels", 1, "Channels (1=mono recommended for speech)") // Changed from 2
		device     = flag.String("device", "", "Device id/name (ignored; default render loopback)")
		dur        = flag.Duration("dur", 0, "Record duration (e.g. 5s, 2m). 0=manual stop")
		stopKey    = flag.String("stop-key", "", "Hotkey chord to stop, e.g. 'ctrl+shift+9'")
		withMic    = flag.Bool("with-mic", false, "Also capture default microphone and mix with loopback")
	)
	flag.Parse()
	_ = device // Placeholder for future selection; we use default render loopback

	if *bits != 16 {
		fatalf(2, "only 16-bit PCM supported, got %d", *bits)
	}
	if err := os.MkdirAll(*outDir, 0755); err != nil {
		fatalf(2, "create out dir: %v", err)
	}

	ts := time.Now().Format("20060102_150405")
	wavPath := filepath.Join(*outDir, ts+".wav")

	writer, err := wav.NewWriter(wavPath, uint32(*sampleRate), uint16(*channels), uint16(*bits))
	if err != nil {
		fatalf(2, "open wav: %v", err)
	}
	defer writer.Close()

	rec, err := audio.NewRecorder(8)
	if err != nil {
		fatalf(1, "init recorder: %v", err)
	}
	if err := rec.Start(uint32(*sampleRate), uint32(*channels)); err != nil {
		fatalf(1, "start recorder: %v", err)
	}
	defer rec.Stop()

	var mic *audio.MicRecorder
	if *withMic {
		m, err := audio.NewMicRecorder(8)
		if err != nil {
			fatalf(1, "init mic: %v", err)
		}
		if err := m.Start(uint32(*sampleRate), uint32(*channels)); err != nil {
			fatalf(1, "start mic: %v", err)
		}
		mic = m
		defer mic.Stop()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Stop conditions: duration, Ctrl+C, hotkey
	if *dur > 0 {
		time.AfterFunc(*dur, cancel)
	}
	setupCtrlC(cancel)

	unregister := func() {}
	if strings.TrimSpace(*stopKey) != "" {
		if u, err := registerHotkey(*stopKey, cancel); err == nil {
			unregister = u
		} else {
			logf("hotkey '%s' not registered: %v", *stopKey, err)
		}
	}
	defer unregister()

	logf("recording to %s", wavPath)

	// Writer loop with periodic flush
	flushTicker := time.NewTicker(500 * time.Millisecond)
	defer flushTicker.Stop()

	runErrCh := make(chan error, 1)
	go func() {
		var micBuf []byte
		for {
			select {
			case <-ctx.Done():
				runErrCh <- nil
				return
			case b, ok := <-rec.Data():
				if !ok {
					runErrCh <- nil
					return
				}
				if len(b) > 0 {
					if mic != nil {
						// Try read mic buffer non-blocking
						select {
						case micBuf = <-mic.Data():
						default:
							micBuf = nil
						}
						mixed := mixS16Mono(b, micBuf)
						if _, err := writer.Write(mixed); err != nil {
							runErrCh <- err
							return
						}
					} else {
						if _, err := writer.Write(b); err != nil {
							runErrCh <- err
							return
						}
					}
				}
			case <-flushTicker.C:
				_ = writer.Flush()
			}
		}
	}()

	// Wait for completion
	select {
	case err := <-runErrCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			fatalf(1, "capture error: %v", err)
		}
	case <-ctx.Done():
	}

	// Finalize
	_ = writer.Flush()
	if err := writer.Close(); err != nil {
		fatalf(1, "finalize wav: %v", err)
	}

	fmt.Println(wavPath)
}

func getenvDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func logf(format string, a ...any) {
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(os.Stdout, "%s "+format+"\n", append([]any{ts}, a...)...)
}

func fatalf(code int, format string, a ...any) {
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(os.Stderr, "%s ERROR: "+format+"\n", append([]any{ts}, a...)...)
	os.Exit(code)
}

func setupCtrlC(cancel context.CancelFunc) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		<-ch
		cancel()
	}()
}

// --- Hotkey registration (Windows) ---

const (
	modAlt     = 0x0001
	modControl = 0x0002
	modShift   = 0x0004
	modWin     = 0x0008
	wmHotkey   = 0x0312
)

type msg struct {
	hwnd    uintptr
	message uint32
	wparam  uintptr
	lparam  uintptr
	time    uint32
	pt      point
}

type point struct{ x, y int32 }

func registerHotkey(spec string, onFire func()) (func(), error) {
	dll := windows.NewLazySystemDLL("user32.dll")
	procRegister := dll.NewProc("RegisterHotKey")
	procUnregister := dll.NewProc("UnregisterHotKey")
	procGetMsg := dll.NewProc("GetMessageW")

	mods, vk, err := parseHotkey(spec)
	if err != nil {
		return func() {}, err
	}
	// id 1
	r1, _, e1 := procRegister.Call(0, uintptr(1), uintptr(mods), uintptr(vk))
	if r1 == 0 {
		if e1 != nil {
			return func() {}, e1
		}
		return func() {}, errors.New("RegisterHotKey failed")
	}

	stop := make(chan struct{})
	go func() {
		var m msg
		for {
			r, _, _ := procGetMsg.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			if m.message == wmHotkey {
				onFire()
				return
			}
			select {
			case <-stop:
				return
			default:
			}
		}
	}()

	unregister := func() {
		close(stop)
		_, _, _ = procUnregister.Call(0, uintptr(1))
	}
	return unregister, nil
}

func parseHotkey(spec string) (mods uint32, vk uint32, err error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(spec)), "+")
	if len(parts) == 0 {
		return 0, 0, errors.New("empty hotkey")
	}
	key := parts[len(parts)-1]
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "ctrl", "control":
			mods |= modControl
		case "alt":
			mods |= modAlt
		case "shift":
			mods |= modShift
		case "win", "meta":
			mods |= modWin
		}
	}
	// Digits
	if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
		return mods, uint32(key[0]), nil // VK_0..VK_9 match ASCII
	}
	// Letters
	if len(key) == 1 && key[0] >= 'a' && key[0] <= 'z' {
		return mods, uint32(strings.ToUpper(key)[0]), nil
	}
	if strings.HasPrefix(key, "f") {
		n, perr := parseFKey(key)
		if perr == nil {
			return mods, n, nil
		}
	}
	return 0, 0, fmt.Errorf("unsupported key: %s", key)
}

func parseFKey(k string) (uint32, error) {
	switch strings.ToLower(k) {
	case "f1":
		return 0x70, nil
	case "f2":
		return 0x71, nil
	case "f3":
		return 0x72, nil
	case "f4":
		return 0x73, nil
	case "f5":
		return 0x74, nil
	case "f6":
		return 0x75, nil
	case "f7":
		return 0x76, nil
	case "f8":
		return 0x77, nil
	case "f9":
		return 0x78, nil
	case "f10":
		return 0x79, nil
	case "f11":
		return 0x7A, nil
	case "f12":
		return 0x7B, nil
	}
	return 0, fmt.Errorf("unsupported f-key: %s", k)
}

// mixS16Mono mixes two S16LE mono PCM buffers with simple averaging. If mic is nil/short, uses loop only.
func mixS16Mono(loop, mic []byte) []byte {
	if len(mic) == 0 {
		return loop
	}
	n := len(loop)
	if len(mic) < n {
		n = len(mic)
	}
	out := make([]byte, n)
	for i := 0; i < n; i += 2 {
		// little-endian int16
		lv := int16(int16(loop[i]) | int16(int16(loop[i+1])<<8))
		mv := int16(int16(mic[i]) | int16(int16(mic[i+1])<<8))
		// simple average to avoid clipping
		s := int32(lv) + int32(mv)
		s /= 2
		if s > 32767 {
			s = 32767
		} else if s < -32768 {
			s = -32768
		}
		out[i] = byte(uint16(int16(s)) & 0xFF)
		out[i+1] = byte((uint16(int16(s)) >> 8) & 0xFF)
	}
	return out
}
