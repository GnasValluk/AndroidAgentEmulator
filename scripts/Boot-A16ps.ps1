# Boot a16ps with the tuned flags (GPU host, snapshot, headless).
# Usage: powershell -File scripts/Boot-A16ps.ps1 [-Snapshot ps-logged-in] [-Gpu host]
param([string]$Snapshot = "ps-logged-in", [string]$Gpu = "host")
$ErrorActionPreference = "Stop"
$env:ANDROID_SDK_ROOT = 'D:\data\android-sdk'
$env:ANDROID_AVD_HOME = 'D:\data\avd'
& D:\data\android-sdk\emulator\emulator.exe -avd a16ps -snapshot $Snapshot `
  -no-snapshot-save -no-window -no-audio -no-boot-anim -gpu $Gpu
