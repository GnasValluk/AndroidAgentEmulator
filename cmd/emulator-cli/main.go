package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/android-agent-emulator/emulator/internal/adb"
	"github.com/android-agent-emulator/emulator/internal/config"
	"github.com/android-agent-emulator/emulator/internal/instance"
	"github.com/android-agent-emulator/emulator/internal/integrity"
	"github.com/android-agent-emulator/emulator/internal/qemu"
	"github.com/android-agent-emulator/emulator/internal/screen"
	"github.com/android-agent-emulator/emulator/internal/server"
	"github.com/android-agent-emulator/emulator/internal/snapshot"
	"github.com/android-agent-emulator/emulator/internal/version"
)

var cliVersion = "dev"

func main() {
	cfgPath := flag.String("config", "configs/default.json", "path to config JSON")
	apiFlag := flag.String("api", "auto", "android version|api: auto|latest|<version e.g.16>|<api e.g.36> (matrix-driven, never hardcoded)")
	adbPort := flag.Int("adb-port", 0, "adb TCP port override (e.g. 5557 for second emulator; default from config)")
	showVer := flag.Bool("version", false, "print CLI version and exit")
	flag.Parse()
	if *showVer {
		fmt.Println("emulator-cli", cliVersion)
		return
	}
	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	want := *apiFlag
	if want == "auto" && cfg.Profile.APIVersion != "" && cfg.Profile.APIVersion != "auto" {
		want = cfg.Profile.APIVersion
	}
	ctx := context.Background()
	v, err := version.New().Resolve(ctx, want)
	if err != nil {
		fmt.Fprintln(os.Stderr, "version resolve:", err)
		os.Exit(1)
	}
	if *adbPort != 0 {
		cfg.Server.AdbPort = *adbPort
	}
	adbClient := adb.New(cfg.Server.AdbPort)

	switch args[0] {
	case "version":
		fmt.Printf("Android %s (API %s, %s) branch=%s source=%s checked=%s\n", v.Version, v.APILevel, v.Codename, v.Branch, v.Source, v.CheckedAt.Format("2006-01-02"))
	case "doctor":
		runDoctor(cfg)
	case "integrity", "attest-check":
		for _, c := range integrity.Evaluate(cfg) {
			fmt.Printf("[%s] %s: %s\n", c.Verdict, c.Name, c.Detail)
		}
	case "qemu-dry-run":
		spec := qemu.Build(cfg, v)
		fmt.Println(spec.Bin, argList(spec.Args))
	case "start":
		info, err := instance.Start(cfg, v)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("started pid=%d android=%s api=%s log=%s\n", info.PID, info.Version, info.API, info.LogFile)
	case "start-fg":
		if err := qemu.EnsureImageDir(cfg.ImageDir); err != nil {
			fatal(err)
		}
		spec := qemu.Build(cfg, v)
		fmt.Println("booting Android", v.Version, "(API "+v.APILevel+")", "branch", v.Branch)
		cmd := exec.Command(spec.Bin, spec.Args...)
		cmd.Dir = spec.Dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			fatal(err)
		}
	case "stop":
		if err := instance.Stop(cfg); err != nil {
			fatal(err)
		}
		fmt.Println("stopped")
	case "status":
		info, _ := instance.Status(cfg)
		if !info.Running {
			fmt.Println("stopped")
			return
		}
		fmt.Printf("running pid=%d android=%s api=%s started=%s log=%s\n", info.PID, info.Version, info.API, info.StartedAt.Format(time.RFC3339), info.LogFile)
	case "wait":
		fmt.Println("waiting for boot (adb", adbClient.Address, ")...")
		if err := adbClient.WaitForDevice(10 * time.Minute); err != nil {
			fatal(err)
		}
		fmt.Println("boot_completed=1")
	case "adb-props":
		props, err := adbClient.Props()
		if err != nil {
			fatal(err)
		}
		for k, val := range props {
			fmt.Printf("%s=%s\n", k, val)
		}
	case "adb-shell":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: emulator-cli adb-shell -- <cmd...>")
			os.Exit(2)
		}
		// support both `adb-shell uptime` and `adb-shell -- uptime`
		shellArgs := args[1:]
		if len(shellArgs) > 0 && shellArgs[0] == "--" {
			shellArgs = shellArgs[1:]
		}
		out, err := adbClient.Shell(shellArgs)
		fmt.Print(out)
		if err != nil {
			os.Exit(1)
		}
	case "install":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: emulator-cli install <app.apk>")
			os.Exit(2)
		}
		out, err := adbClient.Install(args[1])
		fmt.Print(out)
		if err != nil {
			os.Exit(1)
		}
	case "snapshot":
		snapCmd(args[1:], cfg, v)
	case "emu-snapshot":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: emu-snapshot save <name>|list")
			os.Exit(2)
		}
		var out string
		var err error
		switch args[1] {
		case "save":
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "usage: emu-snapshot save <name>")
				os.Exit(2)
			}
			out, err = adbClient.Emu("avd", "snapshot", "save", args[2])
		case "list":
			out, err = adbClient.Emu("avd", "snapshot", "list")
		default:
			fmt.Fprintln(os.Stderr, "unknown emu-snapshot subcommand:", args[1])
			os.Exit(2)
		}
		fmt.Print(out)
		if err != nil {
			os.Exit(1)
		}
	case "screenshot":
		out := "screen.png"
		if len(args) >= 2 {
			out = args[1]
		}
		png, err := screen.Shot(adbClient)
		if err != nil {
			fatal(err)
		}
		if err := os.WriteFile(out, png, 0o644); err != nil {
			fatal(err)
		}
		fmt.Println("saved", out, len(png), "bytes")
	case "view":
		fmt.Println(screen.ScrcpyHint(adbClient))
	case "serve":
		mux := server.New(cfg, v)
		fmt.Println("REST listening on", cfg.Server.ListenAddr, "(set EMULATOR_LISTEN_ADDR=0.0.0.0:18037 for external)")
		if err := http.ListenAndServe(cfg.Server.ListenAddr, mux); err != nil {
			fatal(err)
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", args[0])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(`emulator-cli --api auto <command>
  version        resolve Android version dynamically (matrix + live SDK check)
  doctor         check Windows deps (QEMU/WHPX/adb)
  integrity      local Play Integrity readiness (BASIC/DEVICE/VIRTUAL + risk + protect)
  qemu-dry-run   print QEMU command without booting
  start          boot emulator in background (PID file)
  start-fg       boot in foreground (logs to console)
  stop | status  lifecycle
  wait           wait until sys.boot_completed=1 via adb
  adb-props      dump integrity-relevant getprops
  adb-shell -- <cmd>  run shell command
  install <apk>  adb install -r
  snapshot list|create|restore|delete <name> (qcow2 offline, VM stopped)
  emu-snapshot save <name>|list (live SDK emulator via adb emu)
  screenshot [out.png]  adb screencap to file
  view           scrcpy hint for remote viewing
  serve          expose open REST API for external access`)
}

func snapCmd(args []string, cfg config.Config, v version.Resolved) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: snapshot list|create|restore|delete [name]")
		os.Exit(2)
	}
	var out string
	var err error
	switch args[0] {
	case "list":
		out, err = snapshot.List(cfg, v)
	case "create":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: snapshot create <name>")
			os.Exit(2)
		}
		out, err = snapshot.Create(cfg, v, args[1])
	case "restore", "apply":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: snapshot restore <name>")
			os.Exit(2)
		}
		out, err = snapshot.Apply(cfg, v, args[1])
	case "delete", "del":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: snapshot delete <name>")
			os.Exit(2)
		}
		out, err = snapshot.Delete(cfg, v, args[1])
	default:
		fmt.Fprintln(os.Stderr, "unknown snapshot subcommand:", args[0])
		os.Exit(2)
	}
	fmt.Print(out)
	if err != nil {
		os.Exit(1)
	}
}

func argList(a []string) string {
	s := ""
	for _, x := range a {
		if len(x) > 0 && (x[0] == '-' || len(x) < 2) {
			s += x + " "
		} else if len(s) > 0 {
			s += `"` + x + `" `
		} else {
			s += x + " "
		}
	}
	return s
}

func runDoctor(cfg config.Config) {
	for _, bin := range []string{cfg.QemuBin, "qemu-img", "adb", "scrcpy"} {
		if p, err := exec.LookPath(bin); err != nil {
			fmt.Printf("[FAIL] %s not found (run scripts/Install-DepsWindows.ps1)\n", bin)
		} else {
			fmt.Printf("[PASS] %s -> %s\n", bin, p)
		}
	}
	fmt.Println("Windows hypervisor (WHPX): enable 'Windows Hypervisor Platform' + 'Virtual Machine Platform' in Optional Features.")
	fmt.Println("External access: EMULATOR_LISTEN_ADDR=0.0.0.0:18037 EMULATOR_API_TOKEN=<secret> emulator-cli serve")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
