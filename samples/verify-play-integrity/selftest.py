"""Self-test for verify.py — runs without network or devices."""
import time

from verify import verify

NOW = int(time.time() * 1000)


def base():
    return {
        "requestDetails": {
            "requestPackageName": "com.example.app",
            "nonce": "n123",
            "timestampMillis": str(NOW - 60_000),
        },
        "accountDetails": {"appLicensingVerdict": "LICENSED"},
        "appIntegrity": {"appRecognitionVerdict": "PLAY_RECOGNIZED", "packageName": "com.example.app"},
        "deviceIntegrity": {"deviceRecognitionVerdict": ["MEETS_DEVICE_INTEGRITY", "MEETS_BASIC_INTEGRITY"]},
        "environmentDetails": {
            "appAccessRiskVerdict": {"appsDetected": ["KNOWN_INSTALLED"]},
            "playProtectVerdict": "NO_ISSUES",
        },
    }


def check(name, payload, expect_allow, **kw):
    ok, reasons = verify(payload, "com.example.app", expected_nonce="n123", **kw)
    status = "ALLOW" if ok else "DENY"
    want = "ALLOW" if expect_allow else "DENY"
    print(f"[{status}] {name}")
    assert ok == expect_allow, f"{name}: want {want}, got {status}: {reasons}"


def main():
    check("good device", base(), True)
    v = base()
    v["deviceIntegrity"]["deviceRecognitionVerdict"] = ["MEETS_VIRTUAL_INTEGRITY"]
    check("emulator virtual (emulator policy)", v, True, policy="emulator")
    check("emulator virtual vs standard policy", v, False, policy="standard")
    v = base()
    v["deviceIntegrity"] = {}
    check("rooted/empty verdict", v, False)
    v = base()
    v["deviceIntegrity"]["deviceRecognitionVerdict"] = ["MEETS_DEVICE_INTEGRITY", "MEETS_STRONG_INTEGRITY"]
    check("strong device, strict policy", v, True, policy="strict")
    check("device without strong, strict policy", base(), False, policy="strict")
    v = base()
    v["environmentDetails"]["appAccessRiskVerdict"] = {"appsDetected": ["KNOWN_INSTALLED", "UNKNOWN_CAPTURING"]}
    check("capturing app", v, False)
    v = base()
    v["environmentDetails"]["playProtectVerdict"] = "HIGH_RISK"
    check("play protect high risk", v, False)
    v = base()
    v["requestDetails"]["timestampMillis"] = str(NOW - 60 * 60 * 1000)
    check("stale token", v, False)
    v = base()
    v["accountDetails"]["appLicensingVerdict"] = "UNLICENSED"
    check("unlicensed sideload", v, False)
    print("selftest: all 10 cases passed")


if __name__ == "__main__":
    main()
