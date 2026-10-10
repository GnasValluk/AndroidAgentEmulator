# Build minimal test APK without Gradle (javac + d8 + aapt2 + zipalign + apksigner).
# Usage: powershell -File samples/test-apk/Build-Windows.ps1 [-SdkRoot D:\data\android-sdk]
param([string]$SdkRoot = "D:\data\android-sdk", [string]$BuildTools = "36.1.0", [int]$Api = 36)
$ErrorActionPreference = "Stop"
$here = $PSScriptRoot
$bt = Join-Path $SdkRoot "build-tools\$BuildTools"
$androidJar = Join-Path $SdkRoot "platforms\android-$Api\android.jar"
$out = Join-Path $here "out"
New-Item -ItemType Directory -Force -Path (Join-Path $out "classes") | Out-Null
& javac --release 11 -cp $androidJar -d (Join-Path $out "classes") (Join-Path $here "java\com\example\ping\MainActivity.java")
& (Join-Path $bt "d8.bat") --lib $androidJar --min-api 29 --output $out (Join-Path $out "classes\com\example\ping\MainActivity.class")
& (Join-Path $bt "aapt2.exe") link -o (Join-Path $out "app-unsigned.apk") -I $androidJar --manifest (Join-Path $here "AndroidManifest.xml") --min-sdk-version 29 --target-sdk-version $Api
python -c "import zipfile,sys; z=zipfile.ZipFile(sys.argv[1],'a',zipfile.ZIP_DEFLATED); z.write(sys.argv[2],'classes.dex'); z.close()" (Join-Path $out "app-unsigned.apk") (Join-Path $out "classes.dex")
& (Join-Path $bt "zipalign.exe") -f 4 (Join-Path $out "app-unsigned.apk") (Join-Path $out "app-aligned.apk")
$ks = Join-Path $here "debug.keystore"
if (-not (Test-Path $ks)) {
  & keytool -genkeypair -keystore $ks -storepass android -keypass android -alias androiddebugkey -keyalg RSA -keysize 2048 -validity 10950 -dname "CN=Android Debug,O=Android,C=US"
}
& (Join-Path $bt "apksigner.bat") sign --ks $ks --ks-pass pass:android --key-pass pass:android --out (Join-Path $here "ping.apk") (Join-Path $out "app-aligned.apk")
& (Join-Path $bt "apksigner.bat") verify --print-certs (Join-Path $here "ping.apk") | Select-Object -First 3
Write-Host "Built $(Join-Path $here 'ping.apk')"
