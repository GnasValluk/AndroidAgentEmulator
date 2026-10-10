# Boot an AVD headless with tuned flags (GPU host, snapshot, no audio/anim).
# Usage: powershell -File scripts/Boot-Avd.ps1 [-Avd a16tab] [-Snapshot tab-first-boot] [-Gpu host]
param([string]$Avd = "a16tab", [string]$Snapshot = "", [string]$Gpu = "host")
$ErrorActionPreference = "Stop"
$env:ANDROID_SDK_ROOT = 'D:\data\android-sdk'
$env:ANDROID_AVD_HOME = 'D:\data\avd'
$args = @('-avd', $Avd, '-no-window', '-no-audio', '-no-boot-anim', '-gpu', $Gpu)
if ($Snapshot -ne "") { $args += @('-snapshot', $Snapshot, '-no-snapshot-save') }
& D:\data\android-sdk\emulator\emulator.exe @args
