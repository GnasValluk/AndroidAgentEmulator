// Package server exposes an open REST API for external access.
// Auth: Bearer token from EMULATOR_API_TOKEN. Bind 0.0.0.0 to allow LAN/external.
// It also serves the management dashboard (no auth for the page itself; the
// page sends the token stored in the browser).
package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/android-agent-emulator/emulator/internal/adb"
	"github.com/android-agent-emulator/emulator/internal/avd"
	"github.com/android-agent-emulator/emulator/internal/config"
	"github.com/android-agent-emulator/emulator/internal/instance"
	"github.com/android-agent-emulator/emulator/internal/integrity"
	"github.com/android-agent-emulator/emulator/internal/screen"
	"github.com/android-agent-emulator/emulator/internal/snapshot"
	"github.com/android-agent-emulator/emulator/internal/version"
)

//go:embed web
var webFS embed.FS

type State struct {
	mu      sync.Mutex
	Running bool   `json:"running"`
	PID     int    `json:"pid,omitempty"`
	Version string `json:"android_version"`
	API     string `json:"api_level"`
	Branch  string `json:"aosp_branch"`
}

func New(cfg config.Config, v version.Resolved) *http.ServeMux {
	st := &State{Version: v.Version, API: v.APILevel, Branch: v.Branch}
	adbClient := adb.New(cfg.Server.AdbPort)
	avdMgr := avd.New(cfg.SdkRootResolved(), cfg.AvdHomeResolved(), cfg.ConfigDir())
	mux := http.NewServeMux()
	withAuth := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if cfg.Server.AuthToken != "" {
				if r.Header.Get("Authorization") != "Bearer "+cfg.Server.AuthToken {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
			}
			next(w, r)
		}
	}
	write := func(w http.ResponseWriter, code int, x any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(x)
	}
	// adbFor honors ?port= so dashboards can target any running emulator.
	adbFor := func(r *http.Request) adb.Client {
		if p, err := strconv.Atoi(r.URL.Query().Get("port")); err == nil && p > 0 {
			return adb.New(p)
		}
		return adbClient
	}
	refresh := func() {
		if info, err := instance.Status(cfg); err == nil {
			st.Running = info.Running
			st.PID = info.PID
		}
	}
	mux.HandleFunc("/v1/status", withAuth(func(w http.ResponseWriter, r *http.Request) {
		refresh()
		write(w, 200, st)
	}))
	mux.HandleFunc("/v1/integrity", withAuth(func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, integrity.Evaluate(cfg))
	}))
	mux.HandleFunc("/v1/version", withAuth(func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, v)
	}))
	mux.HandleFunc("/v1/instances/start", withAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		info, err := instance.Start(cfg, v)
		if err != nil {
			write(w, 409, map[string]string{"error": err.Error()})
			return
		}
		write(w, 201, info)
	}))
	mux.HandleFunc("/v1/instances/stop", withAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		if err := instance.Stop(cfg); err != nil {
			write(w, 409, map[string]string{"error": err.Error()})
			return
		}
		write(w, 200, map[string]string{"status": "stopped"})
	}))
	mux.HandleFunc("/v1/adb/props", withAuth(func(w http.ResponseWriter, r *http.Request) {
		props, err := adbFor(r).Props()
		if err != nil {
			write(w, 502, map[string]string{"error": err.Error()})
			return
		}
		write(w, 200, props)
	}))
	mux.HandleFunc("/v1/adb/wait", withAuth(func(w http.ResponseWriter, r *http.Request) {
		if err := adbFor(r).WaitForDevice(5 * time.Minute); err != nil {
			write(w, 504, map[string]string{"error": err.Error()})
			return
		}
		write(w, 200, map[string]string{"status": "boot_completed"})
	}))
	// POST /v1/adb/input?port= JSON {action:tap,swipe,text,key, ...} for remote control.
	mux.HandleFunc("/v1/adb/input", withAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var in struct {
			Action string `json:"action"`
			X      int    `json:"x"`
			Y      int    `json:"y"`
			X1     int    `json:"x1"`
			Y1     int    `json:"y1"`
			X2     int    `json:"x2"`
			Y2     int    `json:"y2"`
			Ms     int    `json:"ms"`
			Text   string `json:"text"`
			Code   int    `json:"code"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&in); err != nil {
			write(w, 400, map[string]string{"error": err.Error()})
			return
		}
		out, err := adbFor(r).Input(in.Action, in.X, in.Y, in.X1, in.Y1, in.X2, in.Y2, in.Ms, in.Text, in.Code)
		if err != nil {
			write(w, 502, map[string]string{"error": out + err.Error()})
			return
		}
		write(w, 200, map[string]string{"result": out})
	}))
	// POST /v1/apk/install with raw APK bytes (external CI can push without adb locally).
	mux.HandleFunc("/v1/apk/install", withAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		tmp, err := os.CreateTemp("", "upload-*.apk")
		if err != nil {
			write(w, 500, map[string]string{"error": err.Error()})
			return
		}
		defer os.Remove(tmp.Name())
		if _, err := io.Copy(tmp, http.MaxBytesReader(w, r.Body, 500<<20)); err != nil {
			write(w, 400, map[string]string{"error": err.Error()})
			return
		}
		_ = tmp.Close()
		out, err := adbFor(r).Install(tmp.Name())
		if err != nil {
			write(w, 502, map[string]string{"error": out + err.Error()})
			return
		}
		write(w, 200, map[string]string{"result": out})
	}))
	mux.HandleFunc("/v1/screen.png", withAuth(func(w http.ResponseWriter, r *http.Request) {
		png, err := screen.Shot(adbFor(r))
		if err != nil {
			write(w, 502, map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	}))
	mux.HandleFunc("/v1/snapshots", withAuth(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			out, err := snapshot.List(cfg, v)
			if err != nil {
				write(w, 502, map[string]string{"error": out + err.Error()})
				return
			}
			write(w, 200, map[string]string{"snapshots": out})
		case "POST":
			name := r.URL.Query().Get("name")
			out, err := snapshot.Create(cfg, v, name)
			if err != nil {
				write(w, 400, map[string]string{"error": out + err.Error()})
				return
			}
			write(w, 201, map[string]string{"result": out})
		default:
			http.Error(w, "GET or POST only", http.StatusMethodNotAllowed)
		}
	}))
	mux.HandleFunc("/v1/snapshots/restore", withAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		name := r.URL.Query().Get("name")
		out, err := snapshot.Apply(cfg, v, name)
		if err != nil {
			write(w, 400, map[string]string{"error": out + err.Error()})
			return
		}
		write(w, 200, map[string]string{"result": out})
	}))
	mux.HandleFunc("/v1/snapshots/delete", withAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		name := r.URL.Query().Get("name")
		out, err := snapshot.Delete(cfg, v, name)
		if err != nil {
			write(w, 400, map[string]string{"error": out + err.Error()})
			return
		}
		write(w, 200, map[string]string{"result": out})
	}))
	mux.HandleFunc("/v1/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "api/openapi.json")
	})
	mux.HandleFunc("/v1/avds", withAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		list, err := avdMgr.List()
		if err != nil {
			write(w, 502, map[string]string{"error": err.Error()})
			return
		}
		if list == nil {
			list = []avd.Info{}
		}
		write(w, 200, list)
	}))
	// /v1/avds/{name}/start|stop|snapshots|config|log
	mux.HandleFunc("/v1/avds/", withAuth(func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/v1/avds/")
		name, action, _ := strings.Cut(rest, "/")
		if !validAvdName(name) {
			http.Error(w, "bad avd name", http.StatusBadRequest)
			return
		}
		switch action {
		case "start":
			if r.Method != "POST" {
				http.Error(w, "POST only", http.StatusMethodNotAllowed)
				return
			}
			var opt avd.StartOptions
			_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&opt)
			info, err := avdMgr.Start(name, opt)
			if err != nil {
				write(w, 409, map[string]string{"error": err.Error()})
				return
			}
			write(w, 201, info)
		case "stop":
			if r.Method != "POST" {
				http.Error(w, "POST only", http.StatusMethodNotAllowed)
				return
			}
			if err := avdMgr.Stop(name); err != nil {
				write(w, 409, map[string]string{"error": err.Error()})
				return
			}
			write(w, 200, map[string]string{"status": "stopped"})
		case "snapshots":
			if r.Method == "GET" {
				list, err := avdMgr.List()
				if err != nil {
					write(w, 502, map[string]string{"error": err.Error()})
					return
				}
				for _, a := range list {
					if a.Name == name {
						if a.Snapshots == nil {
							a.Snapshots = []string{}
						}
						write(w, 200, a.Snapshots)
						return
					}
				}
				write(w, 404, map[string]string{"error": "unknown avd"})
				return
			}
			if r.Method != "POST" {
				http.Error(w, "GET or POST only", http.StatusMethodNotAllowed)
				return
			}
			snap := r.URL.Query().Get("name")
			if snap == "" {
				write(w, 400, map[string]string{"error": "name required"})
				return
			}
			port, err := avdAdbPort(avdMgr, name)
			if err != nil {
				write(w, 409, map[string]string{"error": err.Error()})
				return
			}
			out, err := adb.New(port).Emu("avd", "snapshot", "save", snap)
			if err != nil {
				write(w, 502, map[string]string{"error": out + err.Error()})
				return
			}
			write(w, 201, map[string]string{"result": out})
		case "config":
			if r.Method == "GET" {
				list, err := avdMgr.List()
				if err != nil {
					write(w, 502, map[string]string{"error": err.Error()})
					return
				}
				for _, a := range list {
					if a.Name == name {
						write(w, 200, a)
						return
					}
				}
				write(w, 404, map[string]string{"error": "unknown avd"})
				return
			}
			if r.Method != "PUT" {
				http.Error(w, "GET or PUT only", http.StatusMethodNotAllowed)
				return
			}
			var p avd.ConfigPatch
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&p); err != nil {
				write(w, 400, map[string]string{"error": err.Error()})
				return
			}
			set, err := avdMgr.PatchConfig(name, p)
			if err != nil {
				write(w, 409, map[string]string{"error": err.Error()})
				return
			}
			write(w, 200, set)
		case "log":
			if r.Method != "GET" {
				http.Error(w, "GET only", http.StatusMethodNotAllowed)
				return
			}
			n := 80
			if t, err := strconv.Atoi(r.URL.Query().Get("tail")); err == nil && t > 0 && t <= 500 {
				n = t
			}
			out, err := avdMgr.LogTail(name, n)
			if err != nil {
				write(w, 404, map[string]string{"error": err.Error()})
				return
			}
			write(w, 200, map[string]string{"log": out})
		default:
			http.Error(w, "unknown action", http.StatusNotFound)
		}
	}))
	// Dashboard (page itself is public; API calls still need the token).
	webSub, _ := fs.Sub(webFS, "web")
	mux.HandleFunc("/app/", func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/app/", http.FileServer(http.FS(webSub))).ServeHTTP(w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		b, _ := fs.ReadFile(webSub, "index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})
	return mux
}

func validAvdName(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.') {
			return false
		}
	}
	return true
}

func avdAdbPort(m *avd.Manager, name string) (int, error) {
	list, err := m.List()
	if err != nil {
		return 0, err
	}
	for _, a := range list {
		if a.Name == name {
			if !a.Running {
				return 0, fmt.Errorf("avd %q is not running", name)
			}
			return a.AdbPort, nil
		}
	}
	return 0, fmt.Errorf("unknown avd %q", name)
}
