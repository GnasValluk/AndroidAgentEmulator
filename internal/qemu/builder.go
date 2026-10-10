// Package qemu builds the Windows-optimized QEMU command line.
// Host is Windows + WHPX. No Linux-only assumptions.
package qemu

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/android-agent-emulator/emulator/internal/config"
	"github.com/android-agent-emulator/emulator/internal/version"
)

type Spec struct {
	Bin  string
	Args []string
	Dir  string
}

// Build returns a headless, lightweight QEMU spec.
// Image files are named by Android version (android-16.qcow2), API shown separately.
func Build(cfg config.Config, v version.Resolved) Spec {
	image := filepath.Join(cfg.ImageDir, fmt.Sprintf("android-%s.qcow2", v.Version))
	args := []string{
		"-m", fmt.Sprintf("%d", cfg.Profile.RAMMB),
		"-smp", fmt.Sprintf("%d", cfg.Profile.CPUs),
		"-accel", "whpx",
		"-machine", "q35,accel=whpx",
		"-cpu", "max",
		"-drive", fmt.Sprintf("file=%s,if=virtio,cache=writeback,discard=unmap", image),
		"-nic", "user,model=virtio-net-pci,hostfwd=tcp::" + fmt.Sprintf("%d-:5555", cfg.Server.AdbPort),
		"-nographic",
		"-serial", "stdio",
		"-device", "virtio-rng-pci",
		"-device", "virtio-balloon-pci",
	}
	switch cfg.Profile.GPUMode {
	case "virtio-gpu":
		args = append(args, "-device", "virtio-gpu-pci", "-display", "none")
	case "swiftshader", "swiftshader-indirect":
		args = append(args, "-device", "virtio-gpu-pci", "-display", "none")
	default:
		args = append(args, "-display", "none")
	}
	if cfg.Profile.BootMode == "snapshot" {
		args = append(args, "-loadvm", snapshotName(cfg))
	}
	// Integrity-relevant kernel cmdline is set inside the image (verified boot green,
	// ro.debuggable=0). We only pass minimal androidboot flags here.
	args = append(args,
		"-kernel", filepath.Join(cfg.ImageDir, fmt.Sprintf("android-%s-kernel", v.Version)),
		"-append", "androidboot.verifiedbootstate=green androidboot.veritymode=enforcing androidboot.debuggable=0",
	)
	return Spec{Bin: cfg.QemuBin, Args: args, Dir: cfg.ImageDir}
}

func snapshotName(cfg config.Config) string {
	if cfg.Profile.SnapshotName != "" {
		return cfg.Profile.SnapshotName
	}
	return "cold-boot"
}

// EnsureImageDir creates ./images if missing.
func EnsureImageDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}
