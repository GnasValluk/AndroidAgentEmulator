package integrity

import (
	"testing"

	"github.com/android-agent-emulator/emulator/internal/config"
)

func TestStrongAlwaysFails(t *testing.T) {
	cfg := config.Default()
	found := false
	for _, c := range Evaluate(cfg) {
		if c.Name == "MEETS_STRONG_INTEGRITY" {
			found = true
			if c.Verdict != Fail {
				t.Fatalf("STRONG must fail on emulator, got %s", c.Verdict)
			}
		}
	}
	if !found {
		t.Fatal("missing STRONG check")
	}
}

func TestDeviceSkipsWithoutFingerprint(t *testing.T) {
	cfg := config.Default()
	for _, c := range Evaluate(cfg) {
		if c.Name == "MEETS_DEVICE_INTEGRITY" && c.Verdict != Skip {
			t.Fatalf("expected SKIP without fingerprint, got %s", c.Verdict)
		}
	}
}
