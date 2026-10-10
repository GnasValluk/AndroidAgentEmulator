# Fetch Android GSI + kernel for Windows QEMU (dynamic, no hardcoded build IDs in code).
# GSI source of truth: https://developer.android.com/about/versions/16/gsi-release-notes
# Usage: powershell -File scripts/Fetch-ImageWindows.ps1 -Version 16
param([string]$Version = "16", [string]$OutDir = "./images")
$ErrorActionPreference = "Stop"
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
Write-Host @"
Android $Version on Windows QEMU needs:
 1) GSI from gsi-release-notes page (download x86_64 + GMS variant for integrity),
    extract system.img -> rename to $OutDir/android-$Version.qcow2 (qemu-img convert -O qcow2).
 2) Kernel: use AOSP android-$Version.0.0_r1 prebuilt (or Cuttlefish kernel) -> $OutDir/android-$Version-kernel.
 3) qemu-img.exe create -f qcow2 -F raw -b system.img $OutDir/android-$Version.qcow2

This script intentionally does NOT pin a build URL (builds rotate monthly).
Open the GSI release-notes page, pick the newest x86_64 GMS build, then e.g.:
  Invoke-WebRequest <gsi_zip_url> -OutFile $OutDir/gsi.zip
"@
Get-ChildItem $OutDir | Format-Table Name, Length
