# Refresh versions/matrix.json from live Google sources (no hardcoded versions in code).
# Queries SDK repository max API + probes developer.android.com/about/versions/<N>.
# Usage: powershell -File scripts/Update-Matrix.ps1
$ErrorActionPreference = "Stop"
$xml = (Invoke-WebRequest "https://dl.google.com/android/repository/repository2-1.xml" -TimeoutSec 30).Content
$apis = [regex]::Matches($xml, 'path="platforms;android-(\d+)') | ForEach-Object { [int]$_.Groups[1].Value } | Sort-Object -Unique
$maxApi = ($apis | Measure-Object -Maximum).Maximum
Write-Host "Live max SDK API: $maxApi"
foreach ($ver in 14..20) {
  try {
    $r = Invoke-WebRequest "https://developer.android.com/about/versions/$ver" -Method Head -TimeoutSec 15
    if ($r.StatusCode -eq 200) { Write-Host "Android version page exists: $ver" }
  } catch { }
}
Write-Host "Now edit versions/matrix.json: set latest_version + add {version,api,codename,branch} if maxApi is newer."
