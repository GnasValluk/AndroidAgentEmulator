# Tablet Android 15

Standalone bundle. No manager app: double-click and use.

- AVD `a15tab` (medium_tablet, Play Store image, Android 15 / API 35)
- Boots windowed with GPU host, 1680x1050, animations off
- Fast snapshot `<avd>-fast` reused automatically

## Install (once)

```powershell
powershell -ExecutionPolicy Bypass -File Setup.ps1
```

Needs: Windows Hypervisor Platform enabled, ~4 GB free disk for the image
plus the shared Android SDK (reused across bundles).

## Run

Double-click `Launch.bat`, or:

```powershell
powershell -File Launch.ps1
```

First boot takes ~10 min (one time); later boots take seconds from snapshot.

## Files and apps

- Drop an **APK** onto the emulator window to install it.
- Other files: `powershell -File Send-File.ps1 C:\path\to\file` (lands in
  Download). CLI alternative: `emulator-cli install app.apk`.
