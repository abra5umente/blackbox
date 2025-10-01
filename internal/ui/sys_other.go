//go:build !windows

package ui

import "os/exec"

func setHideWindow(cmd *exec.Cmd) {
	// no-op on non-Windows platforms
}
