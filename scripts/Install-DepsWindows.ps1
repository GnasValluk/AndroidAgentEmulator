#Requires -RunAsAdministrator
# Windows host deps: QEMU (WHPX-capable) + Android platform-tools (adb)
# Usage: powershell -ExecutionPolicy Bypass -File scripts/Install-DepsWindows.ps1
$ErrorActionPreference = "Stop"
winget install -e --id SoftwareFreedomConservancy.QEMU --accept-source-agreements --accept-package-agreements
winget install -e --id Google.PlatformTools --accept-source-agreements --accept-package-agreements
Write-Host "Enable WHPX:"
Write-Host "  Enable-WindowsOptionalFeature -Online -FeatureName HypervisorPlatform -All -NoRestart"
Write-Host "  Enable-WindowsOptionalFeature -Online -FeatureName VirtualMachinePlatform -All -NoRestart"
Enable-WindowsOptionalFeature -Online -FeatureName HypervisorPlatform -All -NoRestart
Enable-WindowsOptionalFeature -Online -FeatureName VirtualMachinePlatform -All -NoRestart
Write-Host "Done. Reboot, then: go run ./cmd/emulator-cli --api auto doctor"
