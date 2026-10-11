# Boot the Tablet Android 16 directly into its window (real-time display).
# Reuses the <avd>-fast snapshot when present, tunes resolution, saves it
# again with --save-snap on first run.
param()
$ErrorActionPreference = "Stop"
$cli = Get-Command emulator-cli -ErrorAction SilentlyContinue
if (-not $cli) {
  $local = Join-Path (Split-Path -Parent (Split-Path -Parent $PSScriptRoot)) "emulator-cli.exe"
  if (Test-Path $local) { $cli = $local } else { $cli = "emulator-cli" }
} else { $cli = $cli.Source }
& $cli boot-avd --avd a16tab --gpu host --tune 1680x1050 --save-snap
