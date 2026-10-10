// Package avd manages SDK emulator AVDs: inventory, lifecycle, snapshots,
// config patching, logs. Paths resolve from ANDROID_SDK_ROOT / ANDROID_AVD_HOME
// (or Config fields); nothing is hardcoded.
package avd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/android-agent-emulator/emulator/internal/adb"
)

// Info describes one AVD.
type Info struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Target    string `json:"target"`
	RAMMB     int    `json:"ram_mb"`
	CPUs      int    `json:"cpus"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Density   int    `json:"density"`
	Running   bool   `json:"running"`
	Serial    string `json:"serial,omitempty"`
	AdbPort   int    `json:"adb_port,omitempty"`
	Snapshots []string `json:"snapshots,omitempty"`
}

// StartOptions for booting an AVD.
type StartOptions struct {
	GPU         string `json:"gpu"`          // host | swiftshader_indirect | auto
	Snapshot    string `json:"snapshot"`     // load named snapshot ("" = default behavior)
	NoSave      bool   `json:"no_save"`      // -no-snapshot-save
	NoWindow    bool   `json:"no_window"`
	ExtraArgs   []string `json:"extra_args,omitempty"`
}

// ConfigPatch edits .avd/config.ini (VM must be stopped).
type ConfigPatch struct {
	RAMMB   *int `json:"ram_mb,omitempty"`
	CPUs    *int `json:"cpus,omitempty"`
	Width   *int `json:"width,omitempty"`
	Height  *int `json:"height,omitempty"`
	Density *int `json:"density,omitempty"`
}

// Manager owns AVD state. Track started PIDs for stop fallback.
type Manager struct {
	SdkRoot string
	AvdHome string
	StateDir string // pid files

	mu    sync.Mutex
	procs map[string]*os.Process
}

func New(sdkRoot, avdHome, stateDir string) *Manager {
	_ = os.MkdirAll(stateDir, 0o755)
	return &Manager{SdkRoot: sdkRoot, AvdHome: avdHome, StateDir: stateDir, procs: map[string]*os.Process{}}
}

func (m *Manager) emulatorBin() string {
	return filepath.Join(m.SdkRoot, "emulator", "emulator.exe")
}

// List scans AvdHome/*.ini and merges live running state.
func (m *Manager) List() ([]Info, error) {
	entries, err := os.ReadDir(m.AvdHome)
	if err != nil {
		return nil, err
	}
	running := m.runningMap()
	var out []Info
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".ini") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".ini")
		avdPath := iniPath(m.AvdHome, name)
		info := Info{Name: name, Path: avdPath}
		if cfg, err := readINI(filepath.Join(avdPath, "config.ini")); err == nil {
			info.RAMMB = parseRAM(cfg["hw.ramSize"])
			info.CPUs = atoi(cfg["hw.cpu.ncore"])
			info.Width = atoi(cfg["hw.lcd.width"])
			info.Height = atoi(cfg["hw.lcd.height"])
			info.Density = atoi(cfg["hw.lcd.density"])
			info.Target = targetFromSysdir(cfg["image.sysdir.1"])
		}
		if r, ok := running[name]; ok {
			info.Running = true
			info.Serial = r.serial
			info.AdbPort = r.adbPort
			info.Snapshots = snapshotsOf(info.AdbPort)
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

type liveEnt struct {
	serial  string
	adbPort int
}

// runningMap discovers running emulators via adb, keyed by AVD name.
func (m *Manager) runningMap() map[string]liveEnt {
	out := map[string]liveEnt{}
	for _, s := range adb.Devices() {
		if !strings.HasPrefix(s, "emulator-") {
			continue
		}
		name, err := adb.New(adbPortOf(s)).EmuAvdName(s)
		if err != nil || name == "" {
			continue
		}
		out[name] = liveEnt{serial: s, adbPort: adbPortOf(s)}
	}
	return out
}

func adbPortOf(serial string) int {
	// emulator-5554 -> adb 5555
	n, err := strconv.Atoi(strings.TrimPrefix(serial, "emulator-"))
	if err != nil {
		return 5555
	}
	return n + 1
}

func snapshotsOf(adbPort int) []string {
	out, err := adb.New(adbPort).Emu("avd", "snapshot", "list")
	if err != nil {
		return nil
	}
	var names []string
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(strings.TrimSpace(line))
		// Rows look like: --  <tag>  <size>  <date>  <clock>
		if len(f) >= 2 && f[0] == "--" {
			names = append(names, f[1])
		}
	}
	return names
}

// Start boots an AVD detached, logging to <avd>/boot-ui.log.
func (m *Manager) Start(name string, opt StartOptions) (map[string]any, error) {
	if _, err := os.Stat(iniPath(m.AvdHome, name)); err != nil {
		return nil, fmt.Errorf("unknown avd %q", name)
	}
	gpu := opt.GPU
	if gpu == "" {
		gpu = "host"
	}
	args := []string{"-avd", name}
	if opt.Snapshot != "" {
		args = append(args, "-snapshot", opt.Snapshot)
	}
	if opt.NoSave {
		args = append(args, "-no-snapshot-save")
	}
	if opt.NoWindow {
		args = append(args, "-no-window")
	}
	args = append(args, "-no-audio", "-gpu", gpu)
	args = append(args, opt.ExtraArgs...)
	logPath := filepath.Join(iniPath(m.AvdHome, name), "boot-ui.log")
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	defer log.Close()
	cmd := exec.Command(m.emulatorBin(), args...)
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("emulator start: %w", err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	m.mu.Lock()
	m.procs[name] = nil // placeholder; real handle released
	m.mu.Unlock()
	_ = os.WriteFile(filepath.Join(m.StateDir, "avd-"+name+".pid"), []byte(strconv.Itoa(pid)), 0o644)
	return map[string]any{"pid": pid, "log": logPath}, nil
}

// Stop kills an AVD via emu console, falling back to the recorded PID.
func (m *Manager) Stop(name string) error {
	for avd, r := range m.runningMap() {
		if avd == name {
			c := adb.New(r.adbPort)
			if _, err := c.Emu("kill"); err == nil {
				_ = os.Remove(filepath.Join(m.StateDir, "avd-"+name+".pid"))
				return nil
			}
		}
	}
	return m.killPID(name)
}

func (m *Manager) killPID(name string) error {
	b, err := os.ReadFile(filepath.Join(m.StateDir, "avd-"+name+".pid"))
	if err != nil {
		return fmt.Errorf("avd %q not running (no pid record)", name)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	proc, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(filepath.Join(m.StateDir, "avd-"+name+".pid"))
		return err
	}
	_ = proc.Kill()
	_ = os.Remove(filepath.Join(m.StateDir, "avd-"+name+".pid"))
	return nil
}

// PatchConfig edits hw.* keys. The AVD must be stopped.
func (m *Manager) PatchConfig(name string, p ConfigPatch) (map[string]string, error) {
	for avd := range m.runningMap() {
		if avd == name {
			return nil, fmt.Errorf("stop avd %q before editing config", name)
		}
	}
	path := filepath.Join(iniPath(m.AvdHome, name), "config.ini")
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	set := map[string]string{}
	if p.RAMMB != nil {
		set["hw.ramSize"] = strconv.Itoa(*p.RAMMB)
	}
	if p.CPUs != nil {
		set["hw.cpu.ncore"] = strconv.Itoa(*p.CPUs)
	}
	if p.Width != nil {
		set["hw.lcd.width"] = strconv.Itoa(*p.Width)
	}
	if p.Height != nil {
		set["hw.lcd.height"] = strconv.Itoa(*p.Height)
	}
	if p.Density != nil {
		set["hw.lcd.density"] = strconv.Itoa(*p.Density)
	}
	out := applyKV(lines, set)
	if err := os.WriteFile(path, []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		return nil, err
	}
	return set, nil
}

// LogTail returns the last n lines of the UI boot log, falling back to the
// newest boot*.log when the AVD was started outside the dashboard.
func (m *Manager) LogTail(name string, n int) (string, error) {
	path := filepath.Join(iniPath(m.AvdHome, name), "boot-ui.log")
	if _, err := os.Stat(path); err != nil {
		if latest, err := newestBootLog(iniPath(m.AvdHome, name)); err == nil {
			path = latest
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n"), nil
}

// --- helpers (pure, unit-tested) ---

func newestBootLog(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	best := ""
	var bestTime int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "boot") || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		if fi, err := e.Info(); err == nil {
			if t := fi.ModTime().UnixNano(); t > bestTime {
				bestTime, best = t, filepath.Join(dir, e.Name())
			}
		}
	}
	if best == "" {
		return "", fmt.Errorf("no boot log in %s", dir)
	}
	return best, nil
}

func iniPath(home, name string) string {
	return filepath.Join(home, name+".avd")
}

func readINI(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if ok {
			m[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return m, nil
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

func applyKV(lines []string, set map[string]string) []string {
	seen := map[string]bool{}
	for i, line := range lines {
		t := strings.TrimSuffix(strings.TrimSpace(line), "\r")
		if t == "" || strings.HasPrefix(t, "#") || !strings.Contains(t, "=") {
			continue
		}
		k := strings.TrimSpace(t[:strings.Index(t, "=")])
		if v, ok := set[k]; ok {
			lines[i] = k + "=" + v
			seen[k] = true
		}
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		if !seen[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		lines = append(lines, k+"="+set[k])
	}
	return lines
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// parseRAM accepts "2048", "1907", "2G", "1536M".
func parseRAM(s string) int {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "G") || strings.HasSuffix(s, "g") {
		return atoi(strings.TrimSuffix(strings.TrimSuffix(s, "G"), "g")) * 1024
	}
	s = strings.TrimSuffix(strings.TrimSuffix(s, "M"), "m")
	return atoi(s)
}

// targetFromSysdir turns system-images\android-36\google_apis_playstore\x86_64
// into "android-36 google_apis_playstore x86_64".
func targetFromSysdir(p string) string {
	p = strings.ReplaceAll(p, "/", `\`)
	parts := strings.Split(p, `\`)
	if len(parts) >= 4 && parts[0] == "system-images" {
		return strings.Join(parts[1:], " ")
	}
	return p
}
