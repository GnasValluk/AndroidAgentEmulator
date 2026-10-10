package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// No hardcoded device secrets. Fingerprint / GMS keys must come from env or user config.
// This struct only holds tunable, non-sensitive defaults.

type DeviceProfile struct {
	Name         string `json:"name"`
	APIVersion   string `json:"api_version"` // e.g. "16", resolved dynamically, never pinned in code
	RAMMB        int    `json:"ram_mb"`
	CPUs         int    `json:"cpus"`
	GPUMode      string `json:"gpu_mode"` // virtio-gpu | swiftshader | host
	BootMode     string `json:"boot_mode"` // cold | snapshot
	SnapshotName string `json:"snapshot_name,omitempty"`
}

type ServerConfig struct {
	ListenAddr string `json:"listen_addr"` // default 127.0.0.1:18037 (5037 is adb), use 0.0.0.0:18037 for external
	AuthToken  string `json:"auth_token"`  // loaded from env EMULATOR_API_TOKEN, never committed
	AdbPort    int    `json:"adb_port"`
}

type IntegrityConfig struct {
	// User-supplied certified fingerprint. Empty = run in baseline mode (will NOT pass DEVICE).
	// Example format set via env EMULATOR_FINGERPRINT, never checked into git.
	ExpectedFingerprint string `json:"expected_fingerprint,omitempty"`
	VerifiedBootState   string `json:"verified_boot_state"` // must be "green" for DEVICE
	BootloaderLocked    bool   `json:"bootloader_locked"`
	Debuggable          bool   `json:"debuggable"` // must be false for integrity
}

type Config struct {
	Profile   DeviceProfile   `json:"profile"`
	Server    ServerConfig    `json:"server"`
	Integrity IntegrityConfig `json:"integrity"`
	ImageDir  string          `json:"image_dir"`
	QemuBin   string          `json:"qemu_bin"`
}

func Default() Config {
	return Config{
		Profile: DeviceProfile{
			Name:       "pixel-16-baseline",
			APIVersion: "auto", // resolved at runtime via version.Resolver
			RAMMB:      4096,
			CPUs:       4,
			GPUMode:    "virtio-gpu",
			BootMode:   "snapshot",
		},
		Server: ServerConfig{
			ListenAddr: "127.0.0.1:18037",
			AdbPort:    5555,
		},
		Integrity: IntegrityConfig{
			VerifiedBootState: "green",
			BootloaderLocked:  true,
			Debuggable:        false,
		},
		ImageDir: "./images",
		QemuBin:  "qemu-system-x86_64",
	}
}

// Load merges file + env. Env wins. No secrets in file.
func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return cfg, fmt.Errorf("read config: %w", err)
		}
		if err == nil {
			if err := json.Unmarshal(b, &cfg); err != nil {
				return cfg, fmt.Errorf("parse config: %w", err)
			}
		}
	}
	// Env overrides (external access + secrets, no hardcode)
	if v := os.Getenv("EMULATOR_LISTEN_ADDR"); v != "" {
		cfg.Server.ListenAddr = v
	}
	if v := os.Getenv("EMULATOR_API_TOKEN"); v != "" {
		cfg.Server.AuthToken = v
	}
	if v := os.Getenv("EMULATOR_FINGERPRINT"); v != "" {
		cfg.Integrity.ExpectedFingerprint = v
	}
	if v := os.Getenv("EMULATOR_IMAGE_DIR"); v != "" {
		cfg.ImageDir = v
	}
	if v := os.Getenv("EMULATOR_QEMU_BIN"); v != "" {
		cfg.QemuBin = v
	}
	if v := os.Getenv("EMULATOR_ANDROID_API"); v != "" {
		cfg.Profile.APIVersion = v
	}
	return cfg, nil
}

func (c Config) ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".android-agent-emulator")
}
