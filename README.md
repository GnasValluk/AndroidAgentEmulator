# AndroidAgentEmulator

[![ci](https://github.com/GnasValluk/AndroidAgentEmulator/actions/workflows/ci.yml/badge.svg)](https://github.com/GnasValluk/AndroidAgentEmulator/actions/workflows/ci.yml)

Headless Android emulator on Windows (QEMU + WHPX, VirtIO-GPU). CLI + REST + ADB over TCP.

## Requirements

- Windows 10/11 with `Windows Hypervisor Platform` and `Virtual Machine Platform` enabled
- QEMU, Android platform-tools (`adb`), optional `scrcpy`

```powershell
powershell -ExecutionPolicy Bypass -File scripts/Install-DepsWindows.ps1
```

## Images

```powershell
powershell -File scripts/Fetch-ImageWindows.ps1 -Version 16
```

GSI builds rotate monthly; the script does not pin URLs. Place results in `./images` as `android-<version>.qcow2` and `android-<version>-kernel`.

## Usage

```powershell
go run ./cmd/emulator-cli --api auto doctor
go run ./cmd/emulator-cli --api auto version
go run ./cmd/emulator-cli --api auto integrity
go run ./cmd/emulator-cli --api auto qemu-dry-run
go run ./cmd/emulator-cli --api 16 start
go run ./cmd/emulator-cli --api 16 status
go run ./cmd/emulator-cli --api 16 wait
go run ./cmd/emulator-cli --api 16 adb-props
go run ./cmd/emulator-cli --api 16 adb-shell -- getprop ro.build.fingerprint
go run ./cmd/emulator-cli --api 16 install app.apk
go run ./cmd/emulator-cli --api 16 snapshot list
go run ./cmd/emulator-cli --api 16 snapshot create cold-boot
go run ./cmd/emulator-cli --api 16 screenshot screen.png
go run ./cmd/emulator-cli --api 16 view
go run ./cmd/emulator-cli --api 16 stop
```

`--api` accepts `auto`, an Android version (`16`), or an API level (`36`). Resolution is driven by `versions/matrix.json` plus a live SDK repository check; override with `--api` or `EMULATOR_ANDROID_API`. See `scripts/Update-Matrix.ps1`.

Snapshots are offline `qemu-img` operations; stop the VM first.

## Configuration

`configs/default.json`. Secrets via environment, never committed:

| Variable | Purpose |
|---|---|
| `EMULATOR_LISTEN_ADDR` | REST bind, e.g. `0.0.0.0:18037` for LAN access (default `127.0.0.1:18037`; port 5037 belongs to adb) |
| `EMULATOR_API_TOKEN` | Bearer token for REST |
| `EMULATOR_FINGERPRINT` | Certified device fingerprint for `MEETS_DEVICE_INTEGRITY` |
| `EMULATOR_IMAGE_DIR` | Image directory override |
| `EMULATOR_QEMU_BIN` | QEMU binary override |
| `EMULATOR_ANDROID_API` | Version override |
| `EMULATOR_MATRIX` | Version matrix path override |

## REST

```powershell
$env:EMULATOR_LISTEN_ADDR="0.0.0.0:18037"; $env:EMULATOR_API_TOKEN="<secret>"
go run ./cmd/emulator-cli --api auto serve
```

Auth header: `Authorization: Bearer <secret>`. Spec: `api/openapi.json`.

- `GET /v1/status`, `/v1/version`, `/v1/integrity`, `/v1/adb/props`, `/v1/adb/wait`, `/v1/screen.png`, `/v1/snapshots`
- `POST /v1/instances/start`, `/v1/instances/stop`, `/v1/apk/install` (raw APK bytes), `/v1/snapshots?name=`, `/v1/snapshots/restore?name=`, `/v1/snapshots/delete?name=`

## Play Integrity

Local `integrity` output is an offline readiness heuristic only. Targets: `MEETS_BASIC_INTEGRITY`, `MEETS_DEVICE_INTEGRITY` (requires user-supplied certified fingerprint), `MEETS_VIRTUAL_INTEGRITY`, clean `appAccessRiskVerdict`, `playProtectVerdict=NO_ISSUES`. `MEETS_STRONG_INTEGRITY` is reported as FAIL by design: it requires hardware-backed attestation on certified hardware and cannot pass on an emulator. Details: `docs/INTEGRITY.md`.

ALLOW/DENY decisions belong server-side on the decrypted verdict:

```powershell
python samples/verify-play-integrity/selftest.py
python samples/verify-play-integrity/verify.py verdict.json --package com.example.app --nonce <expected> --policy standard
```

Policies: `standard` (requires DEVICE), `emulator` (VIRTUAL or DEVICE), `strict` (STRONG). Details: `docs/SERVER_VERIFY.md`.

## Layout

```
cmd/emulator-cli      CLI entrypoint
internal/config       file + env config
internal/version      matrix-driven version resolver
internal/qemu         QEMU command builder (WHPX)
internal/instance     background process lifecycle (PID file)
internal/adb          adb client wrapper
internal/snapshot     qemu-img snapshot ops
internal/screen       screencap PNG + scrcpy hint
internal/integrity    offline readiness checks
internal/server       REST API
samples/verify-play-integrity  server-side verdict verifier
samples/test-apk               minimal APK built without Gradle (e2e install fixture)
scripts               Windows setup helpers
versions/matrix.json  version data (refreshed by CI)
```
