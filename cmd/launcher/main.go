// Command launcher boots one device bundle straight into the emulator
// window: no manager, no dashboard. The exe name decides the config file:
// Tablet16.exe reads Tablet16.json next to it.
//
//	{"avd":"a16tab","sdk_dir":"sdk","avd_dir":"avd","gpu":"host","tune":"1680x1050"}
//
// Progress prints to the console; the window closes itself when done and the
// emulator window stays open. Closing the emulator window ends the session.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/android-agent-emulator/emulator/internal/adb"
	"github.com/android-agent-emulator/emulator/internal/avd"
)

type DeviceConfig struct {
	Avd      string `json:"avd"`
	SdkDir   string `json:"sdk_dir"`
	AvdDir   string `json:"avd_dir"`
	GPU      string `json:"gpu"`
	Tune     string `json:"tune"`
	Snapshot string `json:"snapshot"`
	Memory   string `json:"memory"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		fmt.Println("Press Enter to close...")
		fmt.Scanln()
		os.Exit(1)
	}
}

func run() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	base := strings.TrimSuffix(filepath.Base(exe), ".exe")
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(exe), base+".json"))
	if err != nil {
		return fmt.Errorf("read %s.json: %w (copy me as <Device>.exe + <Device>.json)", base, err)
	}
	var dc DeviceConfig
	if err := json.Unmarshal(raw, &dc); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	if dc.Avd == "" {
		return fmt.Errorf("config %s.json: avd name required", base)
	}
	def := func(s, d string) string {
		if s != "" {
			return s
		}
		return d
	}
	gpu := def(dc.GPU, "host")
	root := filepath.Dir(exe)
	mgr := avd.New(
		joinRoot(root, def(dc.SdkDir, "sdk")),
		joinRoot(root, def(dc.AvdDir, "avd")),
		filepath.Join(root, "state"),
	)

	want := dc.Snapshot
	if want == "" || want == "auto" {
		want = ""
		if stored, err := mgr.SnapshotsStored(dc.Avd); err == nil && len(stored) > 0 {
			want = stored[0]
			for _, s := range stored {
				if s == dc.Avd+"-fast" {
					want = s
					break
				}
			}
		}
	}
	fresh := want == ""
	if fresh {
		fmt.Printf("[1/4] %s: cold boot (first run, ~10 min)...\n", dc.Avd)
	} else {
		fmt.Printf("[1/4] %s: loading snapshot %s...\n", dc.Avd, want)
	}
	before := adb.Devices()
	startOpt := avd.StartOptions{GPU: gpu, Snapshot: want, NoSave: true, NoWindow: false}
	if dc.Memory != "" {
		startOpt.ExtraArgs = []string{"-memory", dc.Memory}
	}
	info, err := mgr.Start(dc.Avd, startOpt)
	if err != nil {
		return err
	}
	fmt.Println("      emulator pid", info["pid"])
	serial := waitNewEmulator(before, 90*time.Second)
	if serial == "" {
		return fmt.Errorf("no new emulator appeared")
	}
	fmt.Println("[2/4] device", serial, "found, waiting for boot...")
	port := adbPortOfSerial(serial)
	if err := adb.New(port).WaitForDevice(12 * time.Minute); err != nil {
		return err
	}
	fmt.Println("[3/4] booted, tuning...")
	client := adb.New(port)
	if dc.Tune != "" {
		if w, h, ok := strings.Cut(dc.Tune, "x"); ok {
			if out, err := client.Shell([]string{"wm", "size", w + "x" + h}); err != nil {
				fmt.Println("      wm size:", strings.TrimSpace(out), err)
			}
		}
	}
	for _, kv := range [][2]string{
		{"window_animation_scale", "0"}, {"transition_animation_scale", "0"}, {"animator_duration_scale", "0"},
	} {
		_, _ = client.Shell([]string{"settings", "put", "global", kv[0], kv[1]})
	}
	if fresh {
		fmt.Println("[4/4] saving fast snapshot (one time)...")
		name := dc.Avd + "-fast"
		out, err := client.Emu("avd", "snapshot", "save", name)
		fmt.Print(prefixLines(out))
		if err != nil {
			return err
		}
	}
	fmt.Println("Done. Use the emulator window; closing it ends the session.")
	return nil
}

func joinRoot(root, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}

func prefixLines(s string) string {
	var b strings.Builder
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			b.WriteString("      " + l + "\n")
		}
	}
	return b.String()
}

func adbPortOfSerial(serial string) int {
	var n int
	_, _ = fmt.Sscanf(strings.TrimPrefix(serial, "emulator-"), "%d", &n)
	if n == 0 {
		return 5555
	}
	return n + 1
}

func waitNewEmulator(before []string, timeout time.Duration) string {
	known := map[string]bool{}
	for _, s := range before {
		known[s] = true
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, s := range adb.Devices() {
			if strings.HasPrefix(s, "emulator-") && !known[s] {
				return s
			}
		}
		time.Sleep(3 * time.Second)
	}
	return ""
}
