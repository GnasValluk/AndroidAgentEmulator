// Package screen provides remote viewing: PNG screenshot via adb + scrcpy helper.
package screen

import (
	"fmt"
	"os/exec"

	"github.com/android-agent-emulator/emulator/internal/adb"
)

// Shot captures `adb exec-out screencap -p` (PNG bytes).
func Shot(c adb.Client) ([]byte, error) {
	cmd := exec.Command(c.Bin, "-s", c.Address, "exec-out", "screencap", "-p")
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("screencap: %s", string(ee.Stderr))
		}
		return nil, err
	}
	return out, nil
}

// ScrcpyHint returns install/run hint for interactive viewing from outside.
func ScrcpyHint(c adb.Client) string {
	if _, err := exec.LookPath("scrcpy"); err != nil {
		return "scrcpy not found: winget install Genymobile.scrcpy, then: scrcpy -s " + c.Address
	}
	return "scrcpy -s " + c.Address
}
