# Tablet Android 15 bundle: setup + launch + send files. No manager needed.
# 1) powershell -ExecutionPolicy Bypass -File Setup.ps1    (once: SDK image + AVD)
# 2) double-click Launch.bat                               (boots windowed, tuned, fast snapshot)
# Drag an APK onto the emulator window to install it.
# Send other files: powershell -File Send-File.ps1 C:\path\to\file.mp4
param()
$ErrorActionPreference = "Stop"
$here = $PSScriptRoot
$root = Split-Path -Parent (Split-Path -Parent $here)
& (Join-Path $root "scripts\Setup-Fresh.ps1") `
  -Version "15" -Device "medium_tablet" `
  -Image 'system-images;android-35;google_apis_playstore;x86_64' `
  -AvdName "a15tab"
