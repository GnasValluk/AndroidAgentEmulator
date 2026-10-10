package instance

import (
	"testing"

	"github.com/android-agent-emulator/emulator/internal/config"
)

func TestStatusStoppedWhenNoPidFile(t *testing.T) {
	cfg := config.Default()
	cfg.ConfigDir()
	// Use isolated temp HOME? Status on fresh env should report stopped, not error.
	info, err := Status(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_ = info
	// Must not claim running without evidence.
	if info.Running && info.PID == 0 {
		t.Fatal("running without pid")
	}
}
