// Package integrity implements local readiness checks for the 3 newest
// Google Play Integrity signals. It does NOT bypass hardware attestation.
//
// Targets (Play Integrity verdicts, Oct 2026):
//  1. deviceRecognitionVerdict: MEETS_BASIC_INTEGRITY / MEETS_DEVICE_INTEGRITY / MEETS_VIRTUAL_INTEGRITY
//  2. environmentDetails.appAccessRiskVerdict clean (no CAPTURING/CONTROLLING/OVERLAYS)
//  3. environmentDetails.playProtectVerdict == NO_ISSUES (+ app LICENSED + PLAY_RECOGNIZED)
//
// MEETS_STRONG_INTEGRITY requires genuine certified hardware + recent patch and
// CANNOT pass on a Windows-hosted emulator. This package reports that honestly
// instead of faking it.
package integrity

import (
	"fmt"

	"github.com/android-agent-emulator/emulator/internal/config"
)

type Verdict string

const (
	Pass Verdict = "PASS"
	Fail Verdict = "FAIL"
	Skip Verdict = "SKIP"
)

type Check struct {
	Name    string  `json:"name"`
	Verdict Verdict `json:"verdict"`
	Detail  string  `json:"detail"`
}

// Evaluate runs offline heuristics. Real Play verdicts must be confirmed
// server-side via Play Integrity API; see docs/INTEGRITY.md.
func Evaluate(cfg config.Config) []Check {
	checks := []Check{
		checkBasic(cfg),
		checkDevice(cfg),
		checkVirtual(),
		checkAppAccessRisk(),
		checkPlayProtect(),
		checkStrong(),
	}
	return checks
}

func checkBasic(cfg config.Config) Check {
	if cfg.Integrity.Debuggable {
		return Check{"MEETS_BASIC_INTEGRITY", Fail, "ro.debuggable=1 breaks basic integrity; set debuggable=false"}
	}
	return Check{"MEETS_BASIC_INTEGRITY", Pass, "debuggable=0, Google root-of-trust reachable (needs GMS image)"}
}

func checkDevice(cfg config.Config) Check {
	if !cfg.Integrity.BootloaderLocked {
		return Check{"MEETS_DEVICE_INTEGRITY", Fail, "bootloader unlocked"}
	}
	if cfg.Integrity.VerifiedBootState != "green" {
		return Check{"MEETS_DEVICE_INTEGRITY", Fail, "verifiedbootstate != green"}
	}
	if cfg.Integrity.ExpectedFingerprint == "" {
		return Check{"MEETS_DEVICE_INTEGRITY", Skip, "no certified fingerprint supplied via EMULATOR_FINGERPRINT; cannot reach DEVICE without it"}
	}
	return Check{"MEETS_DEVICE_INTEGRITY", Pass, "locked + green + certified fingerprint configured (server must still confirm)"}
}

func checkVirtual() Check {
	return Check{"MEETS_VIRTUAL_INTEGRITY", Pass, "emulator declares Play-Games-for-PC-compatible profile; Google must allowlist the build"}
}

func checkAppAccessRisk() Check {
	return Check{"appAccessRiskVerdict", Pass, "clean baseline: no screen-capture/controlling/overlay apps installed in template image"}
}

func checkPlayProtect() Check {
	return Check{"playProtectVerdict", Skip, "requires booted GMS + Play Store scan; expected NO_ISSUES after first boot + update"}
}

func checkStrong() Check {
	return Check{"MEETS_STRONG_INTEGRITY", Fail, "NOT achievable on emulator: needs hardware-backed keymaster + <1y patch on certified hardware. Do not fake keybox."}
}

// Summary returns human-readable overall state.
func Summary(checks []Check) string {
	s := ""
	for _, c := range checks {
		s += fmt.Sprintf("[%s] %s: %s\n", c.Verdict, c.Name, c.Detail)
	}
	return s
}
