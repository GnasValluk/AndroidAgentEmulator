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
	// Best-effort: register 127.0.0.1:<port> so both QEMU user-net forward
	// and SDK emulator (visible as emulator-5554) answer on this address.
	_, _ = c.Connect()
	full := append([]string{"-s", c.Address}, args...)
	cmd := exec.Command(c.Bin, full...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// RunBytes is run() for binary output (screencap).
func (c Client) runBytes(args ...string) ([]byte, error) {
	_, _ = c.Connect()
	full := append([]string{"-s", c.Address}, args...)
	cmd := exec.Command(c.Bin, full...)
	out, err := cmd.Output()
	return out, err
}

// Connect ensures adb server knows the emulator address.
func (c Client) Connect() (string, error) {
	cmd := exec.Command(c.Bin, "connect", c.Address)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Devices lists attached adb serials.
func Devices() []string {
	out, err := exec.Command("adb", "devices").Output()
	if err != nil {
		return nil
	}
	var serials []string
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(strings.TrimSpace(line))
		if len(f) >= 2 && f[1] == "device" && f[0] != "List" {
			serials = append(serials, f[0])
		}
	}
	return serials
}

// EmuAvdName asks a running emulator for its AVD name.
func (c Client) EmuAvdName(serial string) (string, error) {
	cmd := exec.Command(c.Bin, "-s", serial, "emu", "avd", "name")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		t := strings.TrimSpace(line)
		if t != "" && t != "OK" {
			return t, nil
		}
	}
	return "", fmt.Errorf("no avd name in %q", string(out))
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

// Screencap returns PNG bytes from `adb exec-out screencap -p`.
func (c Client) Screencap() ([]byte, error) {
	return c.runBytes("exec-out", "screencap", "-p")
}

// Emu runs `adb emu <args>` (emulator console: `avd snapshot save|list|...`).
// The emu console only answers on the emulator serial (emulator-5554), not on
// a manual TCP alias, so a loopback 127.0.0.1:<odd-port> address is mapped to
// emulator-<port-1>. Plain QEMU images have no emu console and return an error.
func (c Client) Emu(args ...string) (string, error) {
	serial := c.Address
	if host, port, ok := splitHostPort(c.Address); ok && (host == "127.0.0.1" || host == "localhost") && port%2 == 1 {
		serial = fmt.Sprintf("emulator-%d", port-1)
	}
	full := append([]string{"-s", serial, "emu"}, args...)
	cmd := exec.Command(c.Bin, full...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func splitHostPort(addr string) (string, int, bool) {
	i := strings.LastIndex(addr, ":")
	if i < 0 {
		return "", 0, false
	}
	var port int
	if _, err := fmt.Sscanf(addr[i+1:], "%d", &port); err != nil {
		return "", 0, false
	}
	return addr[:i], port, true
}
