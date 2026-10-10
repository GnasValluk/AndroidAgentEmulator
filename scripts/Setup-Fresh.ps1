# Fresh end-to-end setup: tools + SDK + licenses + AVD. Idempotent, safe to re-run.
# Usage: powershell -ExecutionPolicy Bypass -File scripts/Setup-Fresh.ps1
param(
  [string]$ToolsDir = "D:\data\tools",
  [string]$SdkRoot = "D:\data\android-sdk",
  [string]$AvdHome = "D:\data\avd",
  [string]$Version = "16",
  [string]$Device = "medium_tablet",
  [string]$Image = "system-images;android-36;google_apis_playstore;x86_64",
  [string]$AvdName = "a16tab",
  [string]$CmdlineToolsBuild = "15859902"
)
$ErrorActionPreference = "Stop"
& (Join-Path $PSScriptRoot "Install-DepsWindows.ps1") -ToolsDir $ToolsDir
$ct = Join-Path $SdkRoot "cmdline-tools\latest"
if (-not (Test-Path (Join-Path $ct "bin\sdkmanager.bat"))) {
  New-Item -ItemType Directory -Force -Path (Join-Path $SdkRoot "cmdline-tools") | Out-Null
  curl.exe -L --retry 3 -o (Join-Path $SdkRoot "cmdline-tools.zip") `
    "https://dl.google.com/android/repository/commandlinetools-win-${CmdlineToolsBuild}_latest.zip"
  Expand-Archive (Join-Path $SdkRoot "cmdline-tools.zip") -DestinationPath (Join-Path $SdkRoot "tmpct") -Force
  New-Item -ItemType Directory -Force -Path $ct | Out-Null
  Copy-Item (Join-Path $SdkRoot "tmpct\cmdline-tools\*") $ct -Recurse -Force
  Remove-Item (Join-Path $SdkRoot "tmpct") -Recurse -Force
}
$sm = Join-Path $ct "bin\sdkmanager.bat"
1..20 | ForEach-Object { 'y' } | & $sm --sdk_root=$SdkRoot --licenses | Out-Null
& $sm --sdk_root=$SdkRoot 'platform-tools' 'emulator' $Image
$env:ANDROID_SDK_ROOT = $SdkRoot
$env:ANDROID_AVD_HOME = $AvdHome
New-Item -ItemType Directory -Force -Path $AvdHome | Out-Null
$avd = Join-Path $AvdHome "$AvdName.avd"
if (-not (Test-Path $avd)) {
  echo no | & (Join-Path $ct "bin\avdmanager.bat") create avd -n $AvdName -k $Image -d $Device --force
}
Write-Host "Done. Boot with: powershell -File scripts/Boot-Avd.ps1 -Avd $AvdName"
Write-Host "Then open scrcpy from $ToolsDir\scrcpy and the dashboard via: emulator-cli serve"
