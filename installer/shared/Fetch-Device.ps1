# Fetch SDK pieces + create an AVD without Java (direct zips + INI templates).
# Idempotent: present pieces are skipped.
# Usage: powershell -File Fetch-Device.ps1 -InstallDir D:\Apps\TabletLab `
#   -AvdName a16tab -ImagePackage 'system-images;android-36;google_apis_playstore;x86_64' `
#   -Target android-36 -Template awd-config.ini
param(
  [string]$InstallDir = "D:\Apps\TabletLab",
  [string]$AvdName = "a16tab",
  [string]$ImagePackage = "system-images;android-36;google_apis_playstore;x86_64",
  [string]$Target = "android-36",
  [string]$Template = ""
)
$ErrorActionPreference = "Stop"
$sdk = Join-Path $InstallDir "sdk"
$avdHome = Join-Path $InstallDir "avd"
New-Item -ItemType Directory -Force -Path $sdk, $avdHome | Out-Null

function Get-RemotePackage($RepoUrl, $Path) {
  $tmp = Join-Path $env:TEMP ("repo-" + [IO.Path]::GetRandomFileName() + ".xml")
  curl.exe -sL --retry 3 -o $tmp $RepoUrl
  $xml = Get-Content $tmp -Raw
  Remove-Item $tmp -Force
  $blocks = [regex]::Matches($xml, '<remotePackage path="' + [regex]::Escape($Path) + '".*?</remotePackage>', 'Singleline')
  if ($blocks.Count -eq 0) { throw "package $Path not found in $RepoUrl" }
  $best = $null; $bestRev = @(-1, -1, -1)
  foreach ($b in $blocks) {
    $rev = @(0, 0, 0)
    if ($b.Value -match '<major>(\d+)</major>\s*<minor>(\d+)</minor>\s*<micro>(\d+)</micro>') {
      $rev = @([int]$Matches[1], [int]$Matches[2], [int]$Matches[3])
    }
    $all = [regex]::Matches($b.Value, '<url>([^<]+\.zip)</url>') | ForEach-Object { $_.Groups[1].Value }
    $urls = @($all | Where-Object { $_ -match 'windows' -and $_ -notmatch 'arm64' })
    if ($urls.Count -eq 0) {
      $urls = @($all | Where-Object { $_ -match '[-_]win\.' -and $_ -notmatch 'darwin|arm64|linux' })
    }
    if ($urls.Count -eq 0 -and @($all).Count -eq 1) {
      $urls = @($all)  # device images ship one zip (no OS variants)
    }
    if ($urls.Count -eq 0) { continue }
    $newer = $false
    for ($i = 0; $i -lt 3; $i++) {
      if ($rev[$i] -gt $bestRev[$i]) { $newer = $true; break }
      if ($rev[$i] -lt $bestRev[$i]) { break }
    }
    if ($newer) { $bestRev = $rev; $best = $urls[0] }
  }
  if (-not $best) { throw "no windows zip for $Path" }
  return $best
}

function Expand-Dl($Url, $DestDir, $Base) {
  if ($Url -notmatch '^https?://') { $Url = $Base + $Url }
  $zip = Join-Path $env:TEMP ("fetch-" + [IO.Path]::GetFileName($Url))
  Write-Host "Downloading $([IO.Path]::GetFileName($Url))"
  curl.exe -L --retry 3 -o $zip $Url
  Expand-Archive $zip -DestinationPath $DestDir -Force
  Remove-Item $zip -Force
}

$repoBase = "https://dl.google.com/android/repository/"
# 1) emulator + platform-tools (skip when present)
if (-not (Test-Path (Join-Path $sdk "emulator\emulator.exe"))) {
  $u = Get-RemotePackage ($repoBase + "repository2-1.xml") "emulator"
  Expand-Dl $u $sdk $repoBase
}
if (-not (Test-Path (Join-Path $sdk "platform-tools\adb.exe"))) {
  $u = Get-RemotePackage ($repoBase + "repository2-1.xml") "platform-tools"
  Expand-Dl $u $sdk $repoBase
}
# 2) system image (skip when present)
$sysdir = $ImagePackage -replace ';', '\'
if (-not (Test-Path (Join-Path $sdk ($sysdir + "\system.img")))) {
  if ($ImagePackage -like "*playstore*") { $repo = "sys-img/google_apis_playstore/sys-img2-1.xml" }
  elseif ($ImagePackage -like "*google_apis*") { $repo = "sys-img/google_apis/sys-img2-1.xml" }
  else { $repo = "sys-img/android/sys-img2-1.xml" }
  $u = Get-RemotePackage ($repoBase + $repo) $ImagePackage
  $base = ($repoBase + ($repo -replace '/[^/]+$', '/'))
  if ($repo -like "*playstore*") { $base = "https://dl.google.com/android/repository/sys-img/google_apis_playstore/" }
  elseif ($repo -like "*google_apis*") { $base = "https://dl.google.com/android/repository/sys-img/google_apis/" }
  else { $base = "https://dl.google.com/android/repository/sys-img/android/" }
  Expand-Dl $u (Join-Path $sdk "system-images") $base
  # sys-img zips extract into a single folder; relocate it under the package path.
  $found = Get-ChildItem (Join-Path $sdk "system-images") -Recurse -Filter "system.img" -ErrorAction SilentlyContinue | Select-Object -First 1
  $target = Join-Path $sdk $sysdir
  if ($found -and ((Split-Path $found.FullName) -ne $target)) {
    New-Item -ItemType Directory -Force -Path $target | Out-Null
    Move-Item (Join-Path (Split-Path $found.FullName) "*") $target -Force
  }
}
# 3) AVD ini + config from template (always refresh text, keep userdata)
$dir = Join-Path $avdHome ($AvdName + ".avd")
New-Item -ItemType Directory -Force -Path $dir | Out-Null
@"
avd.ini.encoding=UTF-8
path=$dir
target=$Target
"@ | Out-File (Join-Path $avdHome ($AvdName + ".ini")) -Encoding ascii
if ($Template -ne "" -and (Test-Path $Template)) {
  $cfg = Get-Content $Template -Raw
  $cfg = $cfg -replace '__AVD_NAME__', $AvdName
  $cfg = $cfg -replace '__TARGET__', $Target
  $cfg = $cfg -replace '__SYSDIR__', ($sysdir + '\')
  $cfg | Out-File (Join-Path $dir "config.ini") -Encoding ascii
}
Write-Host "Ready: $AvdName"
