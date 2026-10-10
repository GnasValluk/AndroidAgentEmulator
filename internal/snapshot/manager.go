// Package snapshot manages cold-boot snapshots via qemu-img (offline, safe).
// VM must be stopped before create/restore/delete. No hardcoded names.
package snapshot

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/android-agent-emulator/emulator/internal/config"
	"github.com/android-agent-emulator/emulator/internal/version"
)

func imagePath(cfg config.Config, v version.Resolved) string {
	return filepath.Join(cfg.ImageDir, fmt.Sprintf("android-%s.qcow2", v.Version))
}

func qemuImg() string {
	if p, err := exec.LookPath("qemu-img"); err == nil {
		return p
	}
	return "qemu-img"
}

// List returns snapshot table (raw output parsed loosely).
func List(cfg config.Config, v version.Resolved) (string, error) {
	cmd := exec.Command(qemuImg(), "snapshot", "-l", imagePath(cfg, v))
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Create tags current disk state. Run once after first clean boot + Play update
// to get fast `cold-boot` for every later start.
func Create(cfg config.Config, v version.Resolved, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("snapshot name required")
	}
	cmd := exec.Command(qemuImg(), "snapshot", "-c", name, imagePath(cfg, v))
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Apply restores a snapshot (offline).
func Apply(cfg config.Config, v version.Resolved, name string) (string, error) {
	cmd := exec.Command(qemuImg(), "snapshot", "-a", name, imagePath(cfg, v))
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Delete removes a snapshot.
func Delete(cfg config.Config, v version.Resolved, name string) (string, error) {
	cmd := exec.Command(qemuImg(), "snapshot", "-d", name, imagePath(cfg, v))
	out, err := cmd.CombinedOutput()
	return string(out), err
}
