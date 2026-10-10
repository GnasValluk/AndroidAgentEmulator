# Per-user uninstall. No admin required.
$ErrorActionPreference = "SilentlyContinue"
Get-Process EmulatorManager -ErrorAction SilentlyContinue | Stop-Process -Force
Get-Process emulator-cli -ErrorAction SilentlyContinue | Stop-Process -Force
Remove-Item (Join-Path $env:LOCALAPPDATA "Programs\EmulatorManager") -Recurse -Force
Remove-Item (Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\Emulator Manager.lnk") -Force
Remove-Item (Join-Path ([Environment]::GetFolderPath("Desktop")) "Emulator Manager.lnk") -Force
Remove-Item "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\EmulatorManager" -Recurse -Force
Write-Host "Uninstalled."
