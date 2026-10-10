# Fetch Android x86_64 GMS GSI for Windows QEMU.
# URLs + checksums come from versions/matrix.json (data, refreshed by CI),
# never hardcoded in this script. GSI license: accept by downloading.
# Usage: powershell -File scripts/Fetch-ImageWindows.ps1 -Version 16 [-OutDir ./images]
param([string]$Version = "16", [string]$OutDir = "./images")
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$matrix = Get-Content (Join-Path $root "versions/matrix.json") -Raw | ConvertFrom-Json
$entry = $matrix.entries | Where-Object { $_.version -eq $Version } | Select-Object -First 1
if (-not $entry -or -not $entry.gsi_x86_64_gms_url) {
  throw "No GSI URL in matrix for version $Version (see developer.android.com/about/versions/$Version/gsi-release-notes)"
}
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$zip = Join-Path $OutDir "gsi_gms_x86_64.zip"
Write-Host "Downloading $($entry.gsi_x86_64_gms_url)"
curl.exe -L --retry 3 -o $zip $entry.gsi_x86_64_gms_url
$hash = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
if ($hash -ne $entry.gsi_x86_64_gms_sha256.ToLower()) {
  throw "SHA256 mismatch: got $hash want $($entry.gsi_x86_64_gms_sha256)"
}
Write-Host "SHA256 OK, extracting system.img"
Expand-Archive -Path $zip -DestinationPath (Join-Path $OutDir "gsi") -Force
Get-ChildItem (Join-Path $OutDir "gsi") -Recurse | Format-Table Name, Length
Write-Host "Next: qemu-img convert -O qcow2 <system.img> $OutDir/android-$Version.qcow2"
Write-Host "Note: GSI is system-only; it needs a matching kernel+ramdisk to boot (see docs/BOOT.md)."
