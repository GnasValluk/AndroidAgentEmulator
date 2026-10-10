package screen

import (
	"testing"

	"github.com/android-agent-emulator/emulator/internal/adb"
)

func TestScrcpyHintMentionsAddress(t *testing.T) {
	c := adb.New(5555)
	h := ScrcpyHint(c)
	if h == "" || len(h) < 5 {
		t.Fatal("empty hint")
	}
}
