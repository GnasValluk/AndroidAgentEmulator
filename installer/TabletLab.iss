; TabletLab common installer. One Setup, selectable devices, per-device icons.
; Build: "D:\data\Inno Setup 6\ISCC.exe" installer\TabletLab.iss
; Requires staged files: Tablet16.exe, Tablet15.exe, emulator-cli.exe (see release job / local build).
#define MyAppName "TabletLab"
#define MyAppVersion "0.4.0"

[Setup]
AppId={{7A3B1E5F-9C2D-4A8B-BE11-TABLETLAB0001}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
DefaultDirName={autopf}\TabletLab
PrivilegesRequired=lowest
OutputDir=Output
OutputBaseFilename=TabletLab-Setup-{#MyAppVersion}
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
DisableWelcomePage=no
ShowLanguageDialog=no

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Types]
Name: "full"; Description: "All devices"
Name: "custom"; Description: "Custom"; Flags: iscustom

[Components]
Name: "tablet16"; Description: "Tablet Android 16 (Play Store)"; Types: full custom; ExtraDiskSpaceRequired: 6442450944
Name: "tablet15"; Description: "Tablet Android 15 (Play Store)"; Types: full custom; ExtraDiskSpaceRequired: 6442450944

[Tasks]
Name: "desktopicon"; Description: "Desktop shortcuts"; GroupDescription: "Icons:"

[Files]
; backend CLI (open, scriptable)
Source: "staging\emulator-cli.exe"; DestDir: "{app}"; Flags: ignoreversion
; shared fetch + template
Source: "shared\Fetch-Device.ps1"; DestDir: "{app}\shared"; Flags: ignoreversion
Source: "templates\avd-config.ini"; DestDir: "{app}\templates"; Flags: ignoreversion
; per-device launcher + config
Source: "staging\Tablet16.exe"; DestDir: "{app}"; Components: tablet16; Flags: ignoreversion
Source: "devices\Tablet16.json"; DestDir: "{app}"; Components: tablet16; Flags: ignoreversion
Source: "staging\Tablet15.exe"; DestDir: "{app}"; Components: tablet15; Flags: ignoreversion
Source: "devices\Tablet15.json"; DestDir: "{app}"; Components: tablet15; Flags: ignoreversion

[Icons]
Name: "{group}\Tablet Android 16"; Filename: "{app}\Tablet16.exe"; WorkingDir: "{app}"; Components: tablet16
Name: "{autodesktop}\Tablet Android 16"; Filename: "{app}\Tablet16.exe"; WorkingDir: "{app}"; Components: tablet16; Tasks: desktopicon
Name: "{group}\Tablet Android 15"; Filename: "{app}\Tablet15.exe"; WorkingDir: "{app}"; Components: tablet15
Name: "{autodesktop}\Tablet Android 15"; Filename: "{app}\Tablet15.exe"; WorkingDir: "{app}"; Components: tablet15; Tasks: desktopicon

[Run]
Filename: "powershell"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\shared\Fetch-Device.ps1"" -InstallDir ""{app}"" -AvdName a16tab -ImagePackage ""system-images;android-36;google_apis_playstore;x86_64"" -Target android-36 -Template ""{app}\templates\avd-config.ini"""; StatusMsg: "Downloading Tablet Android 16 (~3 GB, one time)…"; Components: tablet16; Flags: waituntilterminated
Filename: "powershell"; Parameters: "-ExecutionPolicy Bypass -File ""{app}\shared\Fetch-Device.ps1"" -InstallDir ""{app}"" -AvdName a15tab -ImagePackage ""system-images;android-35;google_apis_playstore;x86_64"" -Target android-35 -Template ""{app}\templates\avd-config.ini"""; StatusMsg: "Downloading Tablet Android 15 (~3 GB, one time)…"; Components: tablet15; Flags: waituntilterminated
