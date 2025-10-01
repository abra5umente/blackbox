//go:build !windows

package execx

import "os/exec"

func setHideWindow(cmd *exec.Cmd) {
	// no-op on non-Windows platforms
}
