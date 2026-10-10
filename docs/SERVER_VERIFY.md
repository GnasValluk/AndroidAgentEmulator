# Server-side verdict verification

Client (`integrity` command) chỉ là heuristic offline. Quyết định ALLOW/DENY thật
phải làm **trên server** với verdict đã giải mã từ Google.

## Flow
1. App gọi Play Integrity API → gửi token về server bạn.
2. Server giải mã token (dùng key của bạn) thành JSON verdict.
3. Chạy `samples/verify-play-integrity/verify.py`:

```bash
python samples/verify-play-integrity/verify.py verdict.json \
  --package com.example.app --nonce <expected-nonce> --policy standard
# exit 0 = ALLOW, exit 1 = DENY + reasons JSON
```

## Policy
- `standard` (mặc định): yêu cầu `MEETS_DEVICE_INTEGRITY`.
- `emulator`: chấp nhận `MEETS_VIRTUAL_INTEGRITY` hoặc `MEETS_DEVICE_INTEGRITY`
  (dùng cho farm giả lập đạt chuẩn).
- `strict`: yêu cầu `MEETS_STRONG_INTEGRITY` — chỉ hardware certified thật,
  emulator không bao giờ qua (script DENY rõ ràng, không fake).

Các check khác: `LICENSED` + `PLAY_RECOGNIZED`, freshness 10 phút,
`appAccessRisk` (chặn CAPTURING/CONTROLLING), `playProtect=NO_ISSUES`,
`min-sdk`, `deny-hyperactive` (LEVEL_4).

Tự kiểm tra không cần mạng: `python samples/verify-play-integrity/selftest.py`
