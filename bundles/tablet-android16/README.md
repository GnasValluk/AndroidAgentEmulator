# Tablet Android 16

Install with the common setup (recommended):

1. Run `installer/Output/AndroidLab-Setup-0.5.1.exe`
   (or build it: `"D:\data\Inno Setup 6\ISCC.exe" installer\AndroidLab.iss`)
2. Tick **Tablet Android 16**, choose the install folder and shortcuts
3. Double-click the **Tablet Android 16** icon

First launch downloads the image (~3 GB, one time), then boots in ~10 min;
later launches take seconds from snapshot.

## Files and apps

- Drop an **APK** onto the emulator window to install it.
- Other files: `powershell -File Send-File.ps1 C:\path\to\file` (lands in
  Download). CLI alternative: `emulator-cli install app.apk`.
