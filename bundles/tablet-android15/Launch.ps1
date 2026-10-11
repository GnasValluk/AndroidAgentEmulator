# Boot the Tablet Android 15 directly into its window (real-time display).
param()
$ErrorActionPreference = "Stop"
$cli = Get-Command emulator-cli -ErrorAction SilentlyContinue
if (-not $cli) {
  $local = Join-Path (Split-Path -Parent (Split-Path -Parent $PSScriptRoot)) "emulator-cli.exe"
  if (Test-Path $local) { $cli = $local } else { $cli = "emulator-cli" }
} else { $cli = $cli.Source }
& $cli boot-avd --avd a15tab --gpu host --tune 1680x1050 --save-snap
