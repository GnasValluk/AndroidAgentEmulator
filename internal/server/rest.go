// Package server exposes an open REST API for external access.
// Auth: Bearer token from EMULATOR_API_TOKEN. Bind 0.0.0.0 to allow LAN/external.
package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/android-agent-emulator/emulator/internal/adb"
	"github.com/android-agent-emulator/emulator/internal/config"
	"github.com/android-agent-emulator/emulator/internal/instance"
	"github.com/android-agent-emulator/emulator/internal/integrity"
	"github.com/android-agent-emulator/emulator/internal/screen"
	"github.com/android-agent-emulator/emulator/internal/snapshot"
	"github.com/android-agent-emulator/emulator/internal/version"
)

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
		props, err := adbClient.Props()
		if err != nil {
			write(w, 502, map[string]string{"error": err.Error()})
			return
		}
		write(w, 200, props)
	}))
	mux.HandleFunc("/v1/adb/wait", withAuth(func(w http.ResponseWriter, r *http.Request) {
		if err := adbClient.WaitForDevice(5 * time.Minute); err != nil {
			write(w, 504, map[string]string{"error": err.Error()})
			return
		}
		write(w, 200, map[string]string{"status": "boot_completed"})
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
		out, err := adbClient.Install(tmp.Name())
		if err != nil {
			write(w, 502, map[string]string{"error": out + err.Error()})
			return
		}
		write(w, 200, map[string]string{"result": out})
	}))
	mux.HandleFunc("/v1/screen.png", withAuth(func(w http.ResponseWriter, r *http.Request) {
		png, err := screen.Shot(adbClient)
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
	return mux
}
