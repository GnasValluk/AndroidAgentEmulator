package snapshot

import (
	"testing"

	"github.com/android-agent-emulator/emulator/internal/config"
	"github.com/android-agent-emulator/emulator/internal/version"
)

func cfgForTest() config.Config { return config.Default() }
func verForTest() version.Resolved {
	return version.Resolved{Version: "16", APILevel: "36", Branch: "android-16.0.0_r1"}
}

func TestEmptyNameRejected(t *testing.T) {
	// Create with empty name must fail before touching qemu-img.
	if _, err := Create(cfgForTest(), verForTest(), ""); err == nil {
		t.Fatal("expected error for empty name")
	}
}
