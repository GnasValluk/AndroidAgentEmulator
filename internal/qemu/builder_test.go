package qemu

import (
	"strings"
	"testing"

	"github.com/android-agent-emulator/emulator/internal/config"
	"github.com/android-agent-emulator/emulator/internal/version"
	"time"
)

func TestBuildUsesWHPXAndGreenBoot(t *testing.T) {
	cfg := config.Default()
	v := version.Resolved{Version: "16", APILevel: "36", Branch: "android-16.0.0_r1", CheckedAt: time.Now()}
	spec := Build(cfg, v)
	joined := strings.Join(spec.Args, " ")
	for _, want := range []string{"whpx", "virtio-gpu", "verifiedbootstate=green", "debuggable=0"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %s", want, joined)
		}
	}
	if !strings.Contains(strings.Join([]string{spec.Bin}, " "), "qemu") {
		t.Fatal("qemu bin expected")
	}
}
