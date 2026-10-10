# Build IntegrityTest APK without Gradle.
# Library versions are inputs (defaults verified 2026-10-10: integrity 1.6.0, tasks 18.4.1).
# AARs live outside the repo (D:\data\libs); only sources are committed.
# Usage: powershell -File samples/integrity-app/Build-Windows.ps1
param(
  [string]$SdkRoot = "D:\data\android-sdk",
  [string]$BuildTools = "36.1.0",
  [int]$Api = 36,
  [string]$LibDir = "D:\data\libs",
  [string]$IntegrityVer = "1.6.0",
  [string]$TasksVer = "18.4.1"
)
$ErrorActionPreference = "Stop"
$here = $PSScriptRoot
$bt = Join-Path $SdkRoot "build-tools\$BuildTools"
$androidJar = Join-Path $SdkRoot "platforms\android-$Api\android.jar"
$intJar = Join-Path $LibDir "integrity-$IntegrityVer.jar"
$taskJar = Join-Path $LibDir "tasks-$TasksVer.jar"
foreach ($aar in @("integrity-$IntegrityVer.aar", "tasks-$TasksVer.aar")) {
  $jar = (Join-Path $LibDir ([IO.Path]::ChangeExtension($aar, ".jar")))
  if (-not (Test-Path $jar)) {
    python -c "import zipfile,sys; z=zipfile.ZipFile(sys.argv[1]); open(sys.argv[2],'wb').write(z.read('classes.jar'))" (Join-Path $LibDir $aar) $jar
  }
}
$out = Join-Path $here "out"
if (Test-Path $out) { Remove-Item $out -Recurse -Force }
New-Item -ItemType Directory -Force -Path (Join-Path $out "classes") | Out-Null
& javac --release 11 -cp "$androidJar;$intJar;$taskJar" -d (Join-Path $out "classes") (Join-Path $here "java\com\example\integritytest\MainActivity.java")
$classes = Get-ChildItem (Join-Path $out "classes") -Recurse -Filter *.class | ForEach-Object { $_.FullName }
& (Join-Path $bt "d8.bat") --lib $androidJar --min-api 23 --output $out @($classes + @($intJar, $taskJar))
& (Join-Path $bt "aapt2.exe") link -o (Join-Path $out "app-unsigned.apk") -I $androidJar --manifest (Join-Path $here "AndroidManifest.xml") --min-sdk-version 23 --target-sdk-version $Api
python -c "import zipfile,sys; z=zipfile.ZipFile(sys.argv[1],'a',zipfile.ZIP_DEFLATED); z.write(sys.argv[2],'classes.dex'); z.close()" (Join-Path $out "app-unsigned.apk") (Join-Path $out "classes.dex")
& (Join-Path $bt "zipalign.exe") -f 4 (Join-Path $out "app-unsigned.apk") (Join-Path $out "app-aligned.apk")
$ks = Join-Path ($here | Split-Path -Parent) "test-apk\debug.keystore"
if (-not (Test-Path $ks)) {
  & keytool -genkeypair -keystore $ks -storepass android -keypass android -alias androiddebugkey -keyalg RSA -keysize 2048 -validity 10950 -dname "CN=Android Debug,O=Android,C=US"
}
& (Join-Path $bt "apksigner.bat") sign --ks $ks --ks-pass pass:android --key-pass pass:android --out (Join-Path $here "integrity-test.apk") (Join-Path $out "app-aligned.apk")
& (Join-Path $bt "apksigner.bat") verify --print-certs (Join-Path $here "integrity-test.apk") | Select-Object -First 2
Write-Host "Built $(Join-Path $here 'integrity-test.apk')"
