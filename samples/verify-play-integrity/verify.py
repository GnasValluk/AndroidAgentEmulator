"""Server-side Play Integrity verdict verification (stdlib only).

Usage:
    python verify.py verdict.json --package com.example.app --nonce <expected>
    python verify.py verdict.json --package com.example.app --request-hash <expected> --policy emulator

The input is the DECRYPTED JSON payload from Google's Play Integrity API
(see docs/INTEGRITY.md and developer.android.com/google/play/integrity/verdicts,
Oct 2026). Decrypt the integrity token on your server first; this script only
evaluates the verdict, it never trusts the client.

Policies (--policy):
  strict   : require MEETS_STRONG_INTEGRITY (real certified hardware only)
  standard : require MEETS_DEVICE_INTEGRITY (default)
  emulator : require MEETS_VIRTUAL_INTEGRITY or MEETS_DEVICE_INTEGRITY

Exit code 0 = ALLOW, 1 = DENY (reasons printed as JSON).
"""
import argparse
import json
import sys
import time


def _verdict_list(payload):
    try:
        return payload["deviceIntegrity"].get("deviceRecognitionVerdict", [])
    except (KeyError, AttributeError):
        return []


def verify(payload, expected_package, expected_nonce=None, expected_hash=None,
           allowed_window_ms=10 * 60 * 1000, now_ms=None,
           policy="standard", require_licensed=True,
           deny_capturing=True, deny_controlling=True, deny_overlays=False,
           allowed_protect=("NO_ISSUES",), min_sdk=None, deny_hyperactive=False):
    """Return (ok: bool, reasons: list[str]). Pure function, easy to test."""
    reasons = []
    ok = True

    def deny(msg):
        nonlocal ok
        ok = False
        reasons.append("DENY: " + msg)

    def note(msg):
        reasons.append("OK: " + msg)

    req = payload.get("requestDetails", {})
    if req.get("requestPackageName") != expected_package:
        deny(f"package mismatch: {req.get('requestPackageName')!r} != {expected_package!r}")
    else:
        note("package match")
    if expected_nonce is not None:
        if req.get("nonce") != expected_nonce:
            deny("nonce mismatch (replay or wrong request)")
        else:
            note("nonce match")
    if expected_hash is not None:
        if req.get("requestHash") != expected_hash:
            deny("requestHash mismatch")
        else:
            note("requestHash match")
    ts = req.get("timestampMillis")
    now = now_ms if now_ms is not None else int(time.time() * 1000)
    try:
        age = now - int(ts)
        if age < 0 or age > allowed_window_ms:
            deny(f"stale token: age_ms={age} window_ms={allowed_window_ms}")
        else:
            note(f"fresh token age_ms={age}")
    except (TypeError, ValueError):
        deny("missing/invalid timestampMillis")

    if require_licensed:
        lic = payload.get("accountDetails", {}).get("appLicensingVerdict")
        if lic != "LICENSED":
            deny(f"licensing verdict {lic!r} != LICENSED")
        else:
            note("licensed")

    app = payload.get("appIntegrity", {})
    if app.get("appRecognitionVerdict") != "PLAY_RECOGNIZED":
        deny(f"app verdict {app.get('appRecognitionVerdict')!r} != PLAY_RECOGNIZED")
    else:
        note("app recognized")
        if expected_package and app.get("packageName") not in (None, expected_package):
            deny(f"app packageName {app.get('packageName')!r} mismatch")

    labels = set(_verdict_list(payload))
    if policy == "strict":
        if "MEETS_STRONG_INTEGRITY" not in labels:
            deny(f"STRONG required, got {sorted(labels)} (emulators can never pass STRONG)")
        else:
            note("strong integrity")
    elif policy == "emulator":
        if not ({"MEETS_VIRTUAL_INTEGRITY", "MEETS_DEVICE_INTEGRITY"} & labels):
            deny(f"VIRTUAL or DEVICE required, got {sorted(labels)}")
        else:
            note(f"emulator policy labels={sorted(labels)}")
    else:  # standard
        if "MEETS_DEVICE_INTEGRITY" not in labels:
            deny(f"DEVICE required, got {sorted(labels)} (rooted/emulator without cert?)")
        else:
            note("device integrity")

    env = payload.get("environmentDetails", {})
    risk = env.get("appAccessRiskVerdict", {}).get("appsDetected", [])
    if deny_capturing and any("CAPTURING" in r for r in risk):
        deny(f"screen-capturing apps running: {risk}")
    if deny_controlling and any("CONTROLLING" in r for r in risk):
        deny(f"controlling apps running: {risk}")
    if deny_overlays and any("OVERLAYS" in r for r in risk):
        deny(f"overlay apps running: {risk}")
    if not risk:
        note("access risk clean/unevaluated-empty")
    else:
        note(f"access risk tolerated: {risk}")

    prot = env.get("playProtectVerdict")
    if prot is not None:
        if prot not in allowed_protect:
            deny(f"play protect {prot!r} not in {list(allowed_protect)}")
        else:
            note(f"play protect {prot}")

    if min_sdk is not None:
        sdk = payload.get("deviceIntegrity", {}).get("deviceAttributes", {}).get("sdkVersion")
        if sdk is None:
            deny("sdkVersion missing, cannot enforce min_sdk")
        elif int(sdk) < min_sdk:
            deny(f"sdk {sdk} < min {min_sdk}")
        else:
            note(f"sdk {sdk} >= {min_sdk}")

    lvl = payload.get("deviceIntegrity", {}).get("recentDeviceActivity", {}).get("deviceActivityLevel")
    if lvl == "LEVEL_4" and deny_hyperactive:
        deny("hyperactive device (LEVEL_4), possible attack")
    elif lvl:
        note(f"activity {lvl}")

    return ok, reasons


def main(argv=None):
    ap = argparse.ArgumentParser(description="Verify a decrypted Play Integrity verdict.")
    ap.add_argument("verdict_json")
    ap.add_argument("--package", required=True)
    ap.add_argument("--nonce", default=None)
    ap.add_argument("--request-hash", default=None)
    ap.add_argument("--policy", default="standard", choices=["strict", "standard", "emulator"])
    ap.add_argument("--window-ms", type=int, default=10 * 60 * 1000)
    ap.add_argument("--no-require-licensed", action="store_true")
    ap.add_argument("--allow-capturing", action="store_true")
    ap.add_argument("--allow-controlling", action="store_true")
    ap.add_argument("--deny-overlays", action="store_true")
    ap.add_argument("--min-sdk", type=int, default=None)
    ap.add_argument("--deny-hyperactive", action="store_true")
    args = ap.parse_args(argv)
    with open(args.verdict_json, encoding="utf-8") as f:
        payload = json.load(f)
    ok, reasons = verify(
        payload, args.package, expected_nonce=args.nonce,
        expected_hash=args.request_hash, allowed_window_ms=args.window_ms,
        policy=args.policy, require_licensed=not args.no_require_licensed,
        deny_capturing=not args.allow_capturing,
        deny_controlling=not args.allow_controlling,
        deny_overlays=args.deny_overlays, min_sdk=args.min_sdk,
        deny_hyperactive=args.deny_hyperactive,
    )
    print(json.dumps({"allow": ok, "reasons": reasons}, indent=2))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
