package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// reminderEvidencePath is the recorded real iPhone reminder delivery run
// (ios/scripts/record-reminder-evidence.sh).
var reminderEvidencePath = filepath.Join("..", "..", "docs", "mobile", "evidence", "session-15-reminders.json")

// requiredReminderDeviceTests are the delivery and action scenarios the
// physical iPhone must have passed.
var requiredReminderDeviceTests = []string{
	"testAcknowledgingACheckInFromItsNotification()",
	"testDeferringFromADeferralNotification()",
	"testTappingANotificationOpensItsTask()",
}

type reminderEvidence struct {
	Format        string `json:"format"`
	FormatVersion int    `json:"format_version"`
	RunID         string `json:"run_id"`
	RecordedAt    string `json:"recorded_at"`
	Builds        struct {
		SourceFingerprint string `json:"source_fingerprint"`
		BridgeFingerprint string `json:"bridge_fingerprint"`
		AppExecutables    string `json:"app_executables"`
	} `json:"builds"`
	Mac struct {
		Model     string `json:"model"`
		OSVersion string `json:"os_version"`
	} `json:"mac"`
	IPhone struct {
		UDID            string `json:"udid"`
		Model           string `json:"model"`
		ModelIdentifier string `json:"model_identifier"`
		OSVersion       string `json:"os_version"`
		Reality         string `json:"reality"`
	} `json:"iphone"`
	XCResult struct {
		Result  string   `json:"result"`
		Passed  int      `json:"passed"`
		Failed  int      `json:"failed"`
		Skipped int      `json:"skipped"`
		Devices []string `json:"devices"`
		Tests   []struct {
			Name   string `json:"name"`
			Result string `json:"result"`
		} `json:"tests"`
	} `json:"xcresult"`
}

// TestRecordedReminderDeviceEvidence validates the recorded iPhone run
// against the builds the verifier computed from the current tree: the Swift
// source fingerprint and the bridge fingerprint arrive through the
// environment, so evidence recorded from other sources, on a simulator, or
// with a delivery or action scenario missing, skipped, or failed fails here.
// Outside scripts/verify_mobile_session.sh 15 it has nothing to compare
// against and skips, which that verifier refuses.
func TestRecordedReminderDeviceEvidence(t *testing.T) {
	wantSource := os.Getenv("TT_REMINDER_SOURCE_FINGERPRINT")
	wantBridge := os.Getenv("TT_IOS_BRIDGE_FINGERPRINT")
	if wantSource == "" || wantBridge == "" {
		t.Skip("set by scripts/verify_mobile_session.sh 15")
	}
	data, err := os.ReadFile(reminderEvidencePath)
	if err != nil {
		t.Fatalf("no recorded iPhone reminder run: %v (run ios/scripts/record-reminder-evidence.sh)", err)
	}
	var ev reminderEvidence
	if decodeErr := json.Unmarshal(data, &ev); decodeErr != nil {
		t.Fatalf("evidence does not decode: %v", decodeErr)
	}
	check := func(ok bool, format string, args ...any) {
		t.Helper()
		if !ok {
			t.Errorf(format, args...)
		}
	}

	check(ev.Format == "tt-mobile-reminder-evidence" && ev.FormatVersion == 1, "format %s v%d", ev.Format, ev.FormatVersion)
	check(regexp.MustCompile(`^\d{8}T\d{6}Z$`).MatchString(ev.RunID), "run id %q", ev.RunID)
	recorded, err := time.Parse(time.RFC3339, ev.RecordedAt)
	check(err == nil && !recorded.IsZero(), "recorded_at %q", ev.RecordedAt)

	// Tied to the builds under test: the sources, the engine, and the
	// executables the running app reported back to the UI test.
	check(ev.Builds.SourceFingerprint == wantSource, "evidence was recorded from Swift sources %s, the tree is %s: rerun ios/scripts/record-reminder-evidence.sh", ev.Builds.SourceFingerprint, wantSource)
	check(ev.Builds.BridgeFingerprint == wantBridge, "evidence was recorded with bridge %s, the tree builds %s: rerun ios/scripts/record-reminder-evidence.sh", ev.Builds.BridgeFingerprint, wantBridge)
	check(regexp.MustCompile(`^tt=sha256:[0-9a-f]{64}(;tt\.debug\.dylib=sha256:[0-9a-f]{64})?$`).MatchString(ev.Builds.AppExecutables),
		"app executables %q", ev.Builds.AppExecutables)

	// A physical iPhone ran the suite, never a simulator.
	check(ev.IPhone.Reality == "physical", "iPhone reality %q", ev.IPhone.Reality)
	check(strings.HasPrefix(ev.IPhone.ModelIdentifier, "iPhone"), "iPhone model %q (a simulator reports arm64 or x86_64)", ev.IPhone.ModelIdentifier)
	check(ev.IPhone.UDID != "" && slices.Contains(ev.XCResult.Devices, ev.IPhone.UDID), "the suite ran on %v, not %s", ev.XCResult.Devices, ev.IPhone.UDID)
	check(ev.IPhone.OSVersion != "", "no iPhone OS version")
	check(strings.HasPrefix(ev.Mac.Model, "Mac") && ev.Mac.OSVersion != "", "Mac %q %q", ev.Mac.Model, ev.Mac.OSVersion)

	// Every delivery and action scenario passed, none skipped.
	check(ev.XCResult.Result == "Passed" && ev.XCResult.Failed == 0 && ev.XCResult.Skipped == 0,
		"suite %s: %d failed, %d skipped", ev.XCResult.Result, ev.XCResult.Failed, ev.XCResult.Skipped)
	check(ev.XCResult.Passed >= len(requiredReminderDeviceTests), "only %d tests passed", ev.XCResult.Passed)
	for _, name := range requiredReminderDeviceTests {
		passed := slices.ContainsFunc(ev.XCResult.Tests, func(test struct {
			Name   string `json:"name"`
			Result string `json:"result"`
		}) bool {
			return test.Name == name && test.Result == "Passed"
		})
		check(passed, "the iPhone run does not record %s as passed", name)
	}
}
