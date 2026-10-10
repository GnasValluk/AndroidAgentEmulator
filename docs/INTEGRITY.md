# Integrity cao trên Windows emulator — sự thật + checklist

Mục tiêu: pass ở mức độ nào đó 3 nhóm check mới nhất của Google (Play Integrity, Oct 2026):
1. `MEETS_BASIC_INTEGRITY` + `MEETS_DEVICE_INTEGRITY` + `MEETS_VIRTUAL_INTEGRITY`
2. `appAccessRiskVerdict` sạch (không CAPTURING/CONTROLLING/OVERLAYS)
3. `playProtectVerdict=NO_ISSUES` + `appLicensingVerdict=LICENSED` + `appRecognitionVerdict=PLAY_RECOGNIZED`

## Giới hạn trung thực
- `MEETS_STRONG_INTEGRITY` yêu cầu hardware-backed keymaster + patch <1 năm trên máy certified thật.
  **Emulator trên Windows không thể đạt.** Mọi cách "pass STRONG" bằng keybox leak đều bất hợp pháp và bị Google revoke.
  Dự án này báo `FAIL` rõ ràng cho STRONG, không fake.
- `DEVICE` chỉ đạt khi có fingerprint certified do người dùng tự cung cấp qua `EMULATOR_FINGERPRINT`
  (máy Pixel/CSA thật của bạn, không ship sẵn trong repo).

## Checklist để đạt BASIC + VIRTUAL + DEVICE (partial)
- [ ] Image GMS chính chủ, Play Store mới nhất (tránh `UNEVALUATED`)
- [ ] `androidboot.verifiedbootstate=green`, `veritymode=enforcing`, `ro.debuggable=0`, bootloader locked (QEMU cmdline đã set)
- [ ] `EMULATOR_FINGERPRINT` = fingerprint certified hợp lệ
- [ ] Đăng nhập Play, cài app từ Play để có `LICENSED` + `PLAY_RECOGNIZED`
- [ ] Template image sạch, không cài app quay màn hình/overlay
- [ ] Mở Play Protect, chạy scan đầu tiên → `NO_ISSUES`
- [ ] Server xác minh verdict Play Integrity (không tin client): kiểm tra `requestPackageName`, `requestHash/nonce`, freshness

Chạy kiểm tra offline: `go run ./cmd/emulator-cli --api auto integrity`
