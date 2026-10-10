// Package instance manages the QEMU background process on Windows.
// PID + logs stored under ~/.android-agent-emulator/ (no hardcode).
package instance

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/android-agent-emulator/emulator/internal/config"
	"github.com/android-agent-emulator/emulator/internal/qemu"
	"github.com/android-agent-emulator/emulator/internal/version"
)

type Info struct {
	Running   bool      `json:"running"`
	PID       int       `json:"pid,omitempty"`
	Version   string    `json:"android_version"`
	API       string    `json:"api_level"`
	StartedAt time.Time `json:"started_at,omitempty"`
	LogFile   string    `json:"log_file,omitempty"`
}

func dir(cfg config.Config) string {
	d := cfg.ConfigDir()
	_ = os.MkdirAll(d, 0o755)
	return d
}

func pidFile(cfg config.Config) string { return filepath.Join(dir(cfg), "qemu.pid") }
func logFile(cfg config.Config) string { return filepath.Join(dir(cfg), "qemu.log") }

// Start launches QEMU detached (background) and records PID.
func Start(cfg config.Config, v version.Resolved) (Info, error) {
	if st, _ := Status(cfg); st.Running {
		return st, fmt.Errorf("already running pid=%d", st.PID)
	}
	if err := qemu.EnsureImageDir(cfg.ImageDir); err != nil {
		return Info{}, err
	}
	spec := qemu.Build(cfg, v)
	log, err := os.OpenFile(logFile(cfg), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return Info{}, err
	}
	defer log.Close()
	cmd := exec.Command(spec.Bin, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Stdout = log
	cmd.Stderr = log
	// Detached: Start (not Run) so CLI returns immediately.
	if err := cmd.Start(); err != nil {
		return Info{}, fmt.Errorf("qemu start: %w (run doctor to check QEMU/WHPX)", err)
	}
	info := Info{Running: true, PID: cmd.Process.Pid, Version: v.Version, API: v.APILevel, StartedAt: time.Now().UTC(), LogFile: logFile(cfg)}
	// Release process handle but keep running: on Windows child survives parent.
	_ = cmd.Process.Release()
	_ = os.WriteFile(pidFile(cfg), []byte(fmt.Sprintf(`{"pid":%d,"version":%q,"api":%q,"started_at":%q}`, info.PID, v.Version, v.APILevel, info.StartedAt.Format(time.RFC3339))), 0o644)
	return info, nil
}

// Stop kills the recorded PID.
func Stop(cfg config.Config) error {
	b, err := os.ReadFile(pidFile(cfg))
	if err != nil {
		return fmt.Errorf("not running (no pid file)")
	}
	var rec struct {
		PID int `json:"pid"`
	}
	if err := json.Unmarshal(b, &rec); err != nil {
		// fallback plain int
		n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
		rec.PID = n
	}
	proc, err := os.FindProcess(rec.PID)
	if err != nil {
		_ = os.Remove(pidFile(cfg))
		return err
	}
	_ = proc.Kill()
	_ = os.Remove(pidFile(cfg))
	return nil
}

// Status checks if PID is alive.
func Status(cfg config.Config) (Info, error) {
	b, err := os.ReadFile(pidFile(cfg))
	if err != nil {
		return Info{Running: false}, nil
	}
	var rec struct {
		PID       int    `json:"pid"`
		Version   string `json:"version"`
		API       string `json:"api"`
		StartedAt string `json:"started_at"`
	}
	if err := json.Unmarshal(b, &rec); err != nil {
		return Info{Running: false}, nil
	}
	if !alive(rec.PID) {
		_ = os.Remove(pidFile(cfg))
		return Info{Running: false}, nil
	}
	t, _ := time.Parse(time.RFC3339, rec.StartedAt)
	return Info{Running: true, PID: rec.PID, Version: rec.Version, API: rec.API, StartedAt: t, LogFile: logFile(cfg)}, nil
}

func alive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows FindProcess always succeeds; send signal 0 equivalent via tasklist check.
	// Use `tasklist /FI "PID eq N"` — cheap and reliable.
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH").Output()
	if err != nil {
		return true // fail-open: assume alive, avoid false stop
	}
	_ = proc
	return strings.Contains(string(out), strconv.Itoa(pid))
}
