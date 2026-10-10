// Package adb wraps platform-tools adb for external access.
// All commands target 127.0.0.1:<adbPort> (QEMU hostfwd). No hardcoded serials.
package adb

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Client struct {
	Bin     string
	Address string // e.g. 127.0.0.1:5555
}

func New(adbPort int) Client {
	return Client{Bin: "adb", Address: fmt.Sprintf("127.0.0.1:%d", adbPort)}
}

func (c Client) run(args ...string) (string, error) {
	full := append([]string{"-s", c.Address}, args...)
	cmd := exec.Command(c.Bin, full...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Connect ensures adb server knows the emulator address.
func (c Client) Connect() (string, error) {
	cmd := exec.Command(c.Bin, "connect", c.Address)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// WaitForDevice polls `adb shell getprop sys.boot_completed` until 1 or timeout.
func (c Client) WaitForDevice(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	if _, err := c.Connect(); err != nil {
		// continue polling anyway
	}
	for time.Now().Before(deadline) {
		out, err := c.run("shell", "getprop", "sys.boot_completed")
		if err == nil && strings.TrimSpace(out) == "1" {
			return nil
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("timeout waiting for %s boot", c.Address)
}

// Props returns key integrity-relevant props for DEVICE readiness audit.
func (c Client) Props() (map[string]string, error) {
	keys := []string{
		"ro.build.fingerprint", "ro.boot.verifiedbootstate", "ro.boot.vbmeta.device_state",
		"ro.debuggable", "ro.build.version.release", "ro.build.version.sdk",
		"sys.boot_completed",
	}
	m := map[string]string{}
	for _, k := range keys {
		out, err := c.run("shell", "getprop", k)
		if err != nil {
			m[k] = "ERR:" + err.Error()
			continue
		}
		m[k] = strings.TrimSpace(out)
	}
	return m, nil
}

// Install APK from host path.
func (c Client) Install(apkPath string) (string, error) {
	return c.run("install", "-r", apkPath)
}

// Shell runs arbitrary shell args (used by CLI `adb-shell -- <cmd>`).
func (c Client) Shell(args []string) (string, error) {
	full := append([]string{"shell"}, args...)
	return c.run(full...)
}
