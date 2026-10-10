# Boot paths

## Status

- Vanilla QEMU 11.1.0 + WHPX boots (SeaBIOS probe passes, no accelerator errors).
- GSI `system.img` (Android 16, x86_64+GMS, build `BP2A.250605.031.A3`, SHA-256 verified,
  fingerprint `google/gsi_gms_x86_64/generic_x86_64:16/.../release-keys`) is on disk
  under `images/gsi/`. A GSI is system-only: no kernel/ramdisk ships with it, so it
  cannot boot under vanilla QEMU without a matching kernel + first-stage ramdisk.
  That work is not done here.
- First boot uses the official SDK emulator (QEMU fork with ranchu board + WHPX
  support) with the `system-images;android-36;google_apis;x86_64` image.

## SDK layout (D drive, no admin required except QEMU installer)

```
D:\data\android-sdk\
  cmdline-tools\latest\      (15859902)
  platform-tools\            (via sdkmanager)
  emulator\                 (37.x)
  system-images\android-36\google_apis\x86_64\
D:\data\tools\              (qemu, platform-tools, scrcpy on PATH)
```

Setup:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/Install-DepsWindows.ps1 -ToolsDir D:\data\tools
$env:ANDROID_SDK_ROOT='D:\data\android-sdk'
& D:\data\android-sdk\cmdline-tools\latest\bin\sdkmanager.bat --sdk_root=D:\data\android-sdk 'platform-tools' 'emulator' 'system-images;android-36;google_apis;x86_64'
```

## AVD + headless boot

```powershell
$env:ANDROID_SDK_ROOT='D:\data\android-sdk'
echo no | & D:\data\android-sdk\cmdline-tools\latest\bin\avdmanager.bat create avd -n a16 -k 'system-images;android-36;google_apis;x86_64' -d pixel --force
& D:\data\android-sdk\emulator\emulator.exe -avd a16 -no-window -no-audio -no-snapshot -gpu swiftshader_indirect
D:\data\tools\platform-tools\adb.exe wait-for-device
D:\data\tools\platform-tools\adb.exe shell getprop sys.boot_completed
```

ADB listens on `127.0.0.1:5555` (emulator-5554), which matches the default address
in `configs/default.json` and `internal/adb`, so `wait`, `adb-props`,
`screenshot`, and the REST `/v1/adb/*` endpoints work unchanged.

## Notes

- The emulator picks WHPX automatically when the Windows Hypervisor Platform API
  is available (verified working on this host via vanilla QEMU probe). If it
  falls back to AEHD, that driver install needs admin.
- `google_apis` images (not `google_apis_playstore`) have no Play Store; Play
  Integrity verdicts need Play services present. The GMS GSI remains the
  long-term image candidate once kernel/ramdisk boot is solved.
- `MEETS_STRONG_INTEGRITY` cannot pass on any emulator; see `docs/INTEGRITY.md`.
