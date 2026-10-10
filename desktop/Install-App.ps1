# Per-user install of Emulator Manager. No admin required.
# Installs to %LOCALAPPDATA%\Programs\EmulatorManager, creates Start Menu +
# Desktop shortcuts and an Add/Remove Programs entry (HKCU).
# Usage:
#   powershell -ExecutionPolicy Bypass -File desktop/Install-App.ps1                    # build from source (needs dotnet SDK)
#   powershell -ExecutionPolicy Bypass -File desktop/Install-App.ps1 -CliZip <path>     # reuse a prebuilt publish dir
param([string]$CliZip = "")
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$dest = Join-Path $env:LOCALAPPDATA "Programs\EmulatorManager"
New-Item -ItemType Directory -Force -Path $dest | Out-Null

$dotnet = Join-Path "D:\data\tools\dotnet\dotnet.exe"
if (-not (Test-Path $dotnet)) { $dotnet = "dotnet" }
if ($CliZip -ne "" -and (Test-Path $CliZip)) {
  Copy-Item (Join-Path $CliZip "*") $dest -Recurse -Force
} else {
  & $dotnet publish (Join-Path $root "desktop\EmulatorManager\EmulatorManager.csproj") `
    -c Release -o (Join-Path $dest "app") --nologo -v quiet
}
# Backend CLI next to the app so ResolveCli finds it first.
$cliSrc = Join-Path $root "emulator-cli.exe"
if (-not (Test-Path (Join-Path $dest "app\emulator-cli.exe"))) {
  if (Test-Path $cliSrc) { Copy-Item $cliSrc (Join-Path $dest "app") -Force }
  else {
    Write-Host "Fetching emulator-cli from latest GitHub release…"
    $rel = Invoke-RestMethod "https://api.github.com/repos/GnasValluk/AndroidAgentEmulator/releases/latest"
    $asset = $rel.assets | Where-Object { $_.name -eq "emulator-cli.exe" } | Select-Object -First 1
    if (-not $asset) { throw "emulator-cli.exe not found in latest release" }
    Invoke-WebRequest $asset.browser_download_url -OutFile (Join-Path $dest "app\emulator-cli.exe")
  }
}
# Token shared with the backend serve process.
$tokFile = Join-Path $env:APPDATA "EmulatorManager\token.txt"
if (-not (Test-Path $tokFile)) {
  New-Item -ItemType Directory -Force -Path (Split-Path $tokFile) | Out-Null
  (-join ((48..57) + (65..90) + (97..122) | Get-Random -Count 32 | ForEach-Object { [char]$_ })) | Out-File $tokFile -NoNewline
}
$exe = Join-Path $dest "app\EmulatorManager.exe"
$wsh = New-Object -ComObject WScript.Shell
foreach ($dir in @((Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs"), ([Environment]::GetFolderPath("Desktop")))) {
  $lnk = $wsh.CreateShortcut((Join-Path $dir "Emulator Manager.lnk"))
  $lnk.TargetPath = $exe; $lnk.WorkingDirectory = (Join-Path $dest "app"); $lnk.Save()
}
$un = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\EmulatorManager"
New-Item -Path $un -Force | Out-Null
Set-ItemProperty $un -Name DisplayName -Value "Emulator Manager"
Set-ItemProperty $un -Name DisplayVersion -Value "0.1.0"
Set-ItemProperty $un -Name InstallLocation -Value $dest
Set-ItemProperty $un -Name UninstallString -Value "powershell -ExecutionPolicy Bypass -File `"$root\desktop\Uninstall-App.ps1`""
Write-Host "Installed to $dest"
