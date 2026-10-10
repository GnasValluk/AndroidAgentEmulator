# Install host deps. Tools can live on any drive (e.g. D:\data\tools), C: is not required.
# - platform-tools + scrcpy: winget (no admin needed), then copied to $ToolsDir.
# - QEMU: winget installer needs admin (writes to C:\Program Files\qemu); if it is
#   already installed there, its folder is copied to $ToolsDir\qemu (portable).
# Usage: powershell -ExecutionPolicy Bypass -File scripts/Install-DepsWindows.ps1 [-ToolsDir D:\data\tools]
param([string]$ToolsDir = "D:\data\tools")
$ErrorActionPreference = "Stop"
New-Item -ItemType Directory -Force -Path $ToolsDir | Out-Null

winget install -e --id Google.PlatformTools --accept-source-agreements --accept-package-agreements --silent
winget install -e --id Genymobile.scrcpy --accept-source-agreements --accept-package-agreements --silent
# QEMU needs elevation; attempt, tolerate a UAC hang and fall back to the existing install.
$job = Start-Job { winget install -e --id SoftwareFreedomConservancy.QEMU --accept-source-agreements --accept-package-agreements --silent }
$job | Wait-Job -Timeout 180 | Out-Null
if ($job.State -eq 'Running') { Stop-Job $job; Write-Host "QEMU installer still running (likely UAC); continuing with files on disk." }
Remove-Job $job -Force -ErrorAction SilentlyContinue

$pkg = Join-Path $env:LOCALAPPDATA "Microsoft\WinGet\Packages"
robocopy "$pkg\Google.PlatformTools_Microsoft.Winget.Source_8wekyb3d8bbwe\platform-tools" "$ToolsDir\platform-tools" /E /NFL /NDL /NJH /NJS | Out-Null
$ss = Get-ChildItem "$pkg\Genymobile.scrcpy_Microsoft.Winget.Source_8wekyb3d8bbwe" -Directory | Select-Object -First 1
if ($ss) { robocopy $ss.FullName "$ToolsDir\scrcpy" /E /NFL /NDL /NJH /NJS | Out-Null }
if (Test-Path 'C:\Program Files\qemu') { robocopy 'C:\Program Files\qemu' "$ToolsDir\qemu" /E /NFL /NDL /NJH /NJS | Out-Null }

$want = @("$ToolsDir\qemu", "$ToolsDir\platform-tools", "$ToolsDir\scrcpy")
$cur = ([System.Environment]::GetEnvironmentVariable('Path', 'User') -split ';') | Where-Object { $_ -and $_ -ne 'C:\Program Files\qemu' }
[System.Environment]::SetEnvironmentVariable('Path', (($cur + $want | Select-Object -Unique) -join ';'), 'User')
Write-Host "Tools in $ToolsDir. Restart the shell, then: go run ./cmd/emulator-cli --api auto doctor"
