# AndroidAgentEmulator

Trình giả lập Android headless cho **Windows host**: mạnh/mượt (QEMU+WHPX+VirtIO-GPU), nhẹ (headless+snapshot), CLI mở cho truy cập bên ngoài (REST + ADB-over-TCP + token).

- **Không hardcode version:** `--api auto` resolve Android mới nhất động (hiện tại 16/Baklava, CI check hàng tuần). Override: `--api 16` hoặc `EMULATOR_ANDROID_API`.
- **Integrity cao (trung thực):** target `BASIC + DEVICE + VIRTUAL + AppAccessRisk sạch + Play Protect NO_ISSUES`. `STRONG` báo FAIL rõ — cần hardware thật, xem `docs/INTEGRITY.md`.

## Nhanh
```powershell
powershell -ExecutionPolicy Bypass -File scripts/Install-DepsWindows.ps1
powershell -File scripts/Fetch-ImageWindows.ps1 -Version 16  # hướng dẫn lấy GSI x86_64 GMS + kernel
go run ./cmd/emulator-cli --api auto doctor
go run ./cmd/emulator-cli --api auto version     # Android 16 (API 36 Baklava), matrix + live SDK check
go run ./cmd/emulator-cli --api auto integrity
go run ./cmd/emulator-cli --api auto qemu-dry-run
go run ./cmd/emulator-cli --api 16 start         # boot nền (PID file)
go run ./cmd/emulator-cli --api 16 status
go run ./cmd/emulator-cli --api 16 wait          # chờ sys.boot_completed=1
go run ./cmd/emulator-cli --api 16 adb-props     # audit fingerprint/verifiedboot/debuggable
go run ./cmd/emulator-cli --api 16 install app.apk
go run ./cmd/emulator-cli --api 16 snapshot list
go run ./cmd/emulator-cli --api 16 snapshot create cold-boot   # sau boot sạch đầu tiên, VM stopped
go run ./cmd/emulator-cli --api 16 screenshot screen.png
go run ./cmd/emulator-cli --api 16 view                        # hint scrcpy -s 127.0.0.1:5555
# Mở API cho máy ngoài:
$env:EMULATOR_LISTEN_ADDR="0.0.0.0:5037"; $env:EMULATOR_API_TOKEN="<secret>"
go run ./cmd/emulator-cli --api auto serve
# GET /v1/status|version|integrity|screen.png, POST /v1/instances/start|stop, GET /v1/adb/props|wait, POST /v1/apk/install, GET|POST /v1/snapshots
# curl -H "Authorization: Bearer <secret>" http://<host>:5037/v1/integrity
```
Secrets qua env (`EMULATOR_FINGERPRINT`, `EMULATOR_API_TOKEN`), không commit.
