# Push a file from the PC into the running emulator (/sdcard/Download).
# Usage: powershell -File Send-File.ps1 [-Serial emulator-5554] C:\path\to\file
param([string]$Serial = "", [Parameter(Mandatory = $true, Position = 0)][string]$File)
$ErrorActionPreference = "Stop"
$adb = $null
foreach ($cand in @("D:\data\tools\platform-tools\adb.exe",
    (Join-Path $env:ANDROID_SDK_ROOT "platform-tools\adb.exe"))) {
  if ($cand -and (Test-Path $cand)) { $adb = $cand; break }
}
if (-not $adb) { $adb = (Get-Command adb -ErrorAction SilentlyContinue).Source }
if (-not $adb) { throw "adb not found (run Setup.ps1 first)" }
if ($Serial -eq "") {
  $dev = (& $adb devices 2>$null | Select-String '^emulator-\d+\s+device' | Select-Object -First 1)
  if (-not $dev) { throw "no running emulator found" }
  $Serial = ($dev.Line -split '\s+')[0]
}
& $adb -s $Serial push $File /sdcard/Download/
Write-Host "sent to /sdcard/Download on $Serial"
