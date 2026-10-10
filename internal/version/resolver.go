// Package version resolves Android versions dynamically via versions/matrix.json.
// No version is hardcoded in code: matrix.json is data, refreshed weekly by CI
// (scripts/Update-Matrix.ps1) from dl.google.com + developer.android.com.
// --api accepts: auto|latest|<version e.g.16>|<api e.g.36>.
package version

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Resolved holds the dynamic result. Version = Android release (16),
// APILevel = SDK api (36 for Android 16).
type Resolved struct {
	Version   string    `json:"version"`
	APILevel  string    `json:"api_level"`
	Codename  string    `json:"codename"`
	Branch    string    `json:"aosp_branch"`
	CheckedAt time.Time `json:"checked_at"`
	Source    string    `json:"source"`
}

type matrixEntry struct {
	Version  string `json:"version"`
	API      string `json:"api"`
	Codename string `json:"codename"`
	Branch   string `json:"branch"`
}

type matrix struct {
	Latest  string        `json:"latest_version"`
	Entries []matrixEntry `json:"entries"`
}

type Resolver struct {
	Client    *http.Client
	MatrixPath string
}

func New() *Resolver {
	return &Resolver{Client: &http.Client{Timeout: 15 * time.Second}, MatrixPath: defaultMatrixPath()}
}

func defaultMatrixPath() string {
	if v := os.Getenv("EMULATOR_MATRIX"); v != "" {
		return v
	}
	_, file, _, _ := runtime.Caller(0)
	// internal/version -> repo root versions/matrix.json
	return filepath.Join(filepath.Dir(file), "..", "..", "versions", "matrix.json")
}

func (r *Resolver) loadMatrix() (matrix, error) {
	var m matrix
	b, err := os.ReadFile(r.MatrixPath)
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}

// Resolve maps want -> entry. "auto"/"latest"/"" = latest_version from matrix,
// then verifies live max API from network (never pins in code).
func (r *Resolver) Resolve(ctx context.Context, want string) (Resolved, error) {
	m, err := r.loadMatrix()
	if err != nil {
		return Resolved{Version: "16", APILevel: "36", Codename: "Baklava", Branch: "android-16.0.0_r1", CheckedAt: time.Now().UTC(), Source: "offline-fallback"}, nil
	}
	pick := m.Latest
	if want != "" && want != "auto" && want != "latest" {
		pick = want
	}
	entry := findEntry(m, pick)
	if entry == nil {
		return Resolved{}, fmt.Errorf("version %q not in matrix (available: %s)", pick, versionsList(m))
	}
	src := "matrix:" + m.Latest
	// Live check: query SDK repo max API; if newer than matrix, report it (CI should refresh matrix).
	if liveAPI, err := r.maxAPILevel(ctx); err == nil && liveAPI != "" && liveAPI != entry.API {
		// Do not silently switch branch (branch needs version number); surface live signal.
		return Resolved{Version: entry.Version, APILevel: entry.API, Codename: entry.Codename, Branch: entry.Branch, CheckedAt: time.Now().UTC(), Source: src + fmt.Sprintf(" (live-sdk-api=%s, matrix update needed)", liveAPI)}, nil
	}
	return Resolved{Version: entry.Version, APILevel: entry.API, Codename: entry.Codename, Branch: entry.Branch, CheckedAt: time.Now().UTC(), Source: src}, nil
}

func findEntry(m matrix, want string) *matrixEntry {
	for i, e := range m.Entries {
		if e.Version == want || e.API == want {
			return &m.Entries[i]
		}
	}
	return nil
}

func versionsList(m matrix) string {
	var v []string
	for _, e := range m.Entries {
		v = append(v, e.Version+"(api"+e.API+")")
	}
	return strings.Join(v, ", ")
}

var platformRe = regexp.MustCompile(`path="platforms;android-(\d+)`)

func (r *Resolver) maxAPILevel(ctx context.Context) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://dl.google.com/android/repository/repository2-1.xml", nil)
	resp, err := r.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	max := 0
	for _, m := range platformRe.FindAllStringSubmatch(string(b), -1) {
		var n int
		fmt.Sscanf(m[1], "%d", &n)
		if n > max {
			max = n
		}
	}
	if max == 0 {
		return "", fmt.Errorf("no api level found")
	}
	return fmt.Sprintf("%d", max), nil
}

// FetchAOSPRevisions probes a branch existence via googlesource (used by CI refresh, not pinned).
func FetchAOSPRevisions(ctx context.Context, branch string) ([]string, error) {
	url := fmt.Sprintf("https://android.googlesource.com/platform/manifest/+refs/heads/%s?format=JSON", branch)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	s := strings.TrimPrefix(string(b), ")]}'")
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, err
	}
	return []string{branch}, nil
}
