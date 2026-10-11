package avd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRAM(t *testing.T) {
	for in, want := range map[string]int{"2048": 2048, "1907": 1907, "2G": 2048, "1536M": 1536, "": 0} {
		if got := parseRAM(in); got != want {
			t.Fatalf("parseRAM(%q)=%d want %d", in, got, want)
		}
	}
}

func TestTargetFromSysdir(t *testing.T) {
	got := targetFromSysdir(`system-images\android-36\google_apis_playstore\x86_64`)
	if got != "android-36 google_apis_playstore x86_64" {
		t.Fatalf("got %q", got)
	}
}

func TestListReadsAVD(t *testing.T) {
	home := t.TempDir()
	avd := filepath.Join(home, "tab1.avd")
	_ = os.MkdirAll(avd, 0o755)
	_ = os.WriteFile(filepath.Join(home, "tab1.ini"), []byte("avd.ini.encoding=UTF-8\npath="+avd+"\n"), 0o644)
	_ = os.WriteFile(filepath.Join(avd, "config.ini"), []byte("hw.cpu.ncore=4\nhw.ramSize=2G\nhw.lcd.width=1680\nimage.sysdir.1=system-images\\android-36\\google_apis\\x86_64\n"), 0o644)
	m := New("", home, t.TempDir())
	list, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "tab1" || list[0].RAMMB != 2048 || list[0].CPUs != 4 {
		t.Fatalf("bad list: %+v", list)
	}
	patch := ConfigPatch{}
	ram, cpus := 3072, 6
	patch.RAMMB, patch.CPUs = &ram, &cpus
	if _, err := m.PatchConfig("tab1", patch); err != nil {
		t.Fatal(err)
	}
	list, _ = m.List()
	if list[0].RAMMB != 3072 || list[0].CPUs != 6 {
		t.Fatalf("patch failed: %+v", list[0])
	}
}

func TestParseSnapshotList(t *testing.T) {
	sample := "List of snapshots present on all disks:\nID        TAG                 VM SIZE\n--        tab-first-boot          69M 2026-10-10 21:18:45   00:05:34\n--        default_boot            70M 2026-10-10 20:00:00   00:01:00\n"
	got := parseSnapshotList(sample)
	if len(got) != 2 || got[0] != "tab-first-boot" || got[1] != "default_boot" {
		t.Fatalf("got %v", got)
	}
}

func TestDecodeLogFile(t *testing.T) { // UTF-16LE with BOM
	raw := []byte{0xFF, 0xFE, 'h', 0, 'i', 0, '\n', 0}
	if got := string(decodeLogFile(raw)); got != "hi\n" {
		t.Fatalf("bom decode got %q", got)
	}
	// UTF-16LE without BOM, mostly ASCII
	raw = []byte{'O', 0, 'K', 0, '\r', 0, '\n', 0}
	pad := make([]byte, 200)
	copy(pad, raw)
	if got := string(decodeLogFile(pad)); got[:4] != "OK\r\n" {
		t.Fatalf("nobom decode got %q", got)
	}
	// Plain UTF-8 passes through
	if got := string(decodeLogFile([]byte("plain\n"))); got != "plain\n" {
		t.Fatalf("passthrough got %q", got)
	}
}
