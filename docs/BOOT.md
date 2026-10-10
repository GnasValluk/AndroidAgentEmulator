# Boot paths

## Status

- Vanilla QEMU 11.1.0 + WHPX boots (SeaBIOS probe passes, no accelerator errors).
- GSI `system.img` (Android 16, x86_64+GMS, build `BP2A.250605.031.A3`, SHA-256 verified,
  fingerprint `google/gsi_gms_x86_64/generic_x86_64:16/.../release-keys`) is on disk
  under `images/gsi/`. A GSI is system-only: no kernel/ramdisk ships with it, so it
  cannot boot under vanilla QEMU without a matching kernel + first-stage ramdisk.
  That work is not done here.
- First boot uses the official SDK emulator (QEMU fork with ranchu board + WHPX
  support) with the `system-images;android-36;google_apis;x86_64` image.

## SDK layout (D drive, no admin required except QEMU installer)

```
D:\data\android-sdk\
  cmdline-tools\latest\      (15859902)
  platform-tools\            (via sdkmanager)
  emulator\                 (37.x)
  system-images\android-36\google_apis\x86_64\
D:\data\tools\              (qemu, platform-tools, scrcpy on PATH)
```

Setup:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/Install-DepsWindows.ps1 -ToolsDir D:\data\tools
$env:ANDROID_SDK_ROOT='D:\data\android-sdk'
& D:\data\android-sdk\cmdline-tools\latest\bin\sdkmanager.bat --sdk_root=D:\data\android-sdk 'platform-tools' 'emulator' 'system-images;android-36;google_apis;x86_64'
```

## AVD + headless boot

```powershell
$env:ANDROID_SDK_ROOT='D:\data\android-sdk'
echo no | & D:\data\android-sdk\cmdline-tools\latest\bin\avdmanager.bat create avd -n a16 -k 'system-images;android-36;google_apis;x86_64' -d pixel --force
& D:\data\android-sdk\emulator\emulator.exe -avd a16 -no-window -no-audio -no-snapshot -gpu swiftshader_indirect
D:\data\tools\platform-tools\adb.exe wait-for-device
D:\data\tools\platform-tools\adb.exe shell getprop sys.boot_completed
```

ADB listens on `127.0.0.1:5555` (emulator-5554), which matches the default address
in `configs/default.json` and `internal/adb`, so `wait`, `adb-props`,
`screenshot`, and the REST `/v1/adb/*` endpoints work unchanged.

## Notes

- The emulator picks WHPX automatically when the Windows Hypervisor Platform API
  is available (verified working on this host via vanilla QEMU probe). If it
  falls back to AEHD, that driver install needs admin.
- `google_apis` images (not `google_apis_playstore`) have no Play Store; Play
  Integrity verdicts need Play services present. The GMS GSI remains the
  long-term image candidate once kernel/ramdisk boot is solved. A
  `google_apis_playstore` x86_64 image also exists for API 36
  (`system-images;android-36;google_apis_playstore;x86_64`).

## First-boot results (2026-10-10, AVD `a16`)
- `sys.boot_completed=1`, Android 16, SDK 36.
- Fingerprint `google/sdk_gphone64_x86_64/emu64xa:16/BE2A.250530.026.F3/13894323:userdebug/dev-keys`,
  `ro.debuggable=1`, empty `verifiedbootstate`. This is a userdebug image, so
  Play Integrity BASIC/DEVICE verdicts are out of reach on it; treat local
  `integrity` output as config heuristic, and confirm live state with
  `adb-props`.
- Host has 7.3 GB RAM; the 2 GB guest thrashed during first-boot dexopt
  (load ~20) and SystemUI showed a persistent ANR. Fixed by
  `pm bg-dexopt-job --cancel` + letting the system settle; home screen is
  clean afterwards. Do not raise guest RAM without checking host headroom.
- CLI `adb-props`, `screenshot`, `install`, `adb-shell` work against the
  running emulator after `adb connect 127.0.0.1:5555`; the adb client now
  connects automatically before each command.

## E2E results (2026-10-10)

- Minimal APK (`samples/test-apk`, built with javac + d8 + aapt2 + apksigner,
  build-tools 36.1.0, no Gradle): installed, launched, text renders on screen.
- REST on `127.0.0.1:18037` (port 5037 belongs to the adb server, so the
  default was moved): `/v1/status`, `/v1/version`, `/v1/integrity`,
  `/v1/adb/props`, `/v1/screen.png` (valid PNG), `POST /v1/apk/install`
  (`Success`), 401 without token. All passed.
- `system-images;android-36;google_apis_playstore;x86_64` downloaded to
  `D:\data\android-sdk` for the integrity track (ships Play Store).

## Second AVD: `a16ps` (playstore, 2026-10-10)

- Live props: `user/release-keys` (`...026.D1...`), `ro.debuggable=0`,
  boot completed, real Play Store (`Phonesky` 45.3.21). This is the best
  emulator base for integrity work: release-keys + non-debuggable + Play
  Store for licensing. DEVICE still needs a certified fingerprint.
- `google_apis` images only carry a `LicenseChecker` stub under the
  `com.android.vending` name (v1.8) — check `pm path` + version, not just
  package presence.
- Snapshots: `first-boot` (a16, 74M), `ps-first-boot` (a16ps, 68M) via
  `emu-snapshot save|list`. The `adb emu` console only answers on the
  `emulator-XXXX` serial, so the client maps loopback odd ports to it.
- Second emulator takes adb ports 5556/5557; use `--adb-port 5557` for all
  adb-backed commands. Do not run both emulators under host RAM pressure
  (7.3 GB host here); stop one with `adb -s emulator-XXXX emu kill`.
