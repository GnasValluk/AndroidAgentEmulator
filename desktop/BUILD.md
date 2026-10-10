# Desktop app (Emulator Manager)

WPF (.NET 8) frontend to `emulator-cli serve`. Embedded device display
(polled screenshots + adb input injection), tray icon, per-user installer.

## Build (portable SDK, no admin)

```powershell
$env:DOTNET_CLI_TELEMETRY_OPTOUT=1
D:\data\tools\dotnet\dotnet.exe publish desktop\EmulatorManager\EmulatorManager.csproj `
  -c Release -o desktop\EmulatorManager\publish --nologo -v quiet
```

Note (2026-10-11): SDK 8.0.425 zip SHA-256 (`CD6EB1DF…`) does not match
`release-metadata` (`F0B6F1…`) via two independent downloads. The binaries
carry a valid Microsoft Authenticode signature, so the SDK is genuine and
the metadata hash is treated as stale. Re-verify before upgrading versions.

## Headless self-test (no window needed)

```powershell
.\desktop\EmulatorManager\publish\EmulatorManager.exe --self-test
```

Exercises the REST client against a running `serve` (AVD list, version,
screenshot PNG check). Exit 0 = OK.

## Install / uninstall (per-user, no admin)

```powershell
powershell -ExecutionPolicy Bypass -File desktop\Install-App.ps1
powershell -ExecutionPolicy Bypass -File desktop\Uninstall-App.ps1
```
