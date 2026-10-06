package syncexchange

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// evidencePath is the recorded real Mac/iPhone iCloud exchange
// (ios/scripts/record-icloud-exchange.sh).
var evidencePath = filepath.Join("..", "..", "docs", "mobile", "evidence", "session-8-icloud-exchange.json")

type partyEvidence struct {
	Role                string   `json:"role"`
	DeviceID            string   `json:"device_id"`
	WorkspaceID         string   `json:"workspace_id"`
	RunID               string   `json:"run_id"`
	Title               string   `json:"title"`
	Attachment          string   `json:"attachment"`
	Published           []string `json:"published"`
	Imported            []string `json:"imported"`
	SawPeerTitle        string   `json:"saw_peer_title"`
	SawPeerAttachment   string   `json:"saw_peer_attachment"`
	Passes              int      `json:"passes"`
	ConditionsObserved  []string `json:"conditions_observed"`
	WorkspaceDigest     string   `json:"workspace_digest"`
	LastSuccessfulExchg string   `json:"last_successful_exchange"`
}

type exchangeEvidence struct {
	Format        string `json:"format"`
	FormatVersion int    `json:"format_version"`
	RunID         string `json:"run_id"`
	RecordedAt    string `json:"recorded_at"`
	Builds        struct {
		SourceFingerprint string            `json:"source_fingerprint"`
		BridgeFingerprint string            `json:"bridge_fingerprint"`
		BridgehostSHA256  string            `json:"bridgehost_sha256"`
		HelperSHA256      string            `json:"helper_sha256"`
		AppExecutables    map[string]string `json:"app_executables"`
	} `json:"builds"`
	Mac struct {
		Model         string `json:"model"`
		OSVersion     string `json:"os_version"`
		DataDirectory string `json:"data_directory"`
	} `json:"mac"`
	IPhone struct {
		UDID            string            `json:"udid"`
		Model           string            `json:"model"`
		ModelIdentifier string            `json:"model_identifier"`
		OSVersion       string            `json:"os_version"`
		Reality         string            `json:"reality"`
		AppExecutables  map[string]string `json:"app_executables"`
		DataDirectory   string            `json:"data_directory"`
		Folder          string            `json:"folder"`
	} `json:"iphone"`
	Exchange struct {
		Provider  string `json:"provider"`
		Folder    string `json:"folder"`
		RunFolder string `json:"run_folder"`
	} `json:"exchange"`
	Workspace struct {
		WorkspaceID string `json:"workspace_id"`
		Fixture     string `json:"fixture"`
		SeedFiles   int    `json:"seed_files"`
	} `json:"workspace"`
	Originator partyEvidence `json:"originator"`
	Responder  partyEvidence `json:"responder"`
	XCResult   struct {
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

// TestRecordedICloudExchangeEvidence validates the recorded real exchange
// against the builds the verifier computed from the current tree: the Swift
// source fingerprint, the iPhone bridge fingerprint, and the reproducible
// bridgehost hash arrive through the environment, so evidence recorded from
// other sources, a simulator, another workspace, or a one-way exchange fails.
// Outside scripts/verify_mobile_session.sh 8 it has nothing to compare
// against and skips, which that verifier refuses.
func TestRecordedICloudExchangeEvidence(t *testing.T) {
	wantSource := os.Getenv("TT_ICLOUD_SOURCE_FINGERPRINT")
	wantBridge := os.Getenv("TT_IOS_BRIDGE_FINGERPRINT")
	wantBridgehost := os.Getenv("TT_BRIDGEHOST_SHA256")
	if wantSource == "" || wantBridge == "" || wantBridgehost == "" {
		t.Skip("set by scripts/verify_mobile_session.sh 8")
	}
	data, err := os.ReadFile(evidencePath)
	if err != nil {
		t.Fatalf("no recorded real iCloud exchange: %v (run ios/scripts/record-icloud-exchange.sh)", err)
	}
	var ev exchangeEvidence
	if err := json.Unmarshal(data, &ev); err != nil {
		t.Fatalf("evidence does not decode: %v", err)
	}
	check := func(ok bool, format string, args ...any) {
		t.Helper()
		if !ok {
			t.Errorf(format, args...)
		}
	}

	check(ev.Format == "tt-sync-icloud-exchange-evidence" && ev.FormatVersion == 1, "format %s v%d", ev.Format, ev.FormatVersion)
	check(regexp.MustCompile(`^\d{8}T\d{6}Z$`).MatchString(ev.RunID), "run id %q", ev.RunID)

	// Tied to the builds under test.
	check(ev.Builds.SourceFingerprint == wantSource, "evidence was recorded from Swift sources %s, the tree is %s: rerun ios/scripts/record-icloud-exchange.sh", ev.Builds.SourceFingerprint, wantSource)
	check(ev.Builds.BridgeFingerprint == wantBridge, "evidence was recorded with bridge %s, the tree builds %s", ev.Builds.BridgeFingerprint, wantBridge)
	check(ev.Builds.BridgehostSHA256 == wantBridgehost, "evidence was recorded with bridgehost %s, the tree builds %s", ev.Builds.BridgehostSHA256, wantBridgehost)
	check(ev.Builds.HelperSHA256 != "", "no helper hash")
	check(len(ev.Builds.AppExecutables) > 0, "no app build hashes")
	for name, digest := range ev.Builds.AppExecutables {
		check(ev.IPhone.AppExecutables[name] == digest, "the iPhone ran %s %s, the Mac built %s", name, ev.IPhone.AppExecutables[name], digest)
	}

	// A physical iPhone and a Mac, over iCloud Drive.
	check(ev.IPhone.Reality == "physical", "iPhone reality %q", ev.IPhone.Reality)
	check(strings.HasPrefix(ev.IPhone.Model, "iPhone") && strings.HasPrefix(ev.IPhone.ModelIdentifier, "iPhone"),
		"iPhone model %q / %q (a simulator reports arm64 or x86_64)", ev.IPhone.Model, ev.IPhone.ModelIdentifier)
	check(ev.IPhone.UDID != "" && slices.Contains(ev.XCResult.Devices, ev.IPhone.UDID), "the device suite ran on %v, not %s", ev.XCResult.Devices, ev.IPhone.UDID)
	check(strings.HasPrefix(ev.Mac.Model, "Mac") && ev.Mac.OSVersion != "", "Mac %q %q", ev.Mac.Model, ev.Mac.OSVersion)
	check(ev.Exchange.Provider == "iCloud Drive" && strings.Contains(ev.Exchange.Folder, "/Library/Mobile Documents/com~apple~CloudDocs/"),
		"exchange %s at %s is not iCloud Drive", ev.Exchange.Provider, ev.Exchange.Folder)
	check(strings.Contains(ev.IPhone.Folder, "Mobile Documents") || strings.Contains(ev.IPhone.Folder, "CloudDocs"),
		"the iPhone's folder %q is not in iCloud Drive", ev.IPhone.Folder)
	check(ev.Exchange.RunFolder == "runs/"+ev.RunID, "run folder %s", ev.Exchange.RunFolder)

	// Disposable: the fixture's identity, in run-scoped directories.
	fixture, err := os.ReadFile(filepath.Join("..", "compat", "testdata", "workspaces", "representative", ".tt", "workspace.json"))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`"workspace_id":\s*"([^"]+)"`).FindSubmatch(fixture)
	if match == nil {
		t.Fatal("the fixture catalog names no workspace")
	}
	check(ev.Workspace.WorkspaceID == string(match[1]) && ev.Workspace.SeedFiles > 0, "workspace %s (fixture %s), %d seed files", ev.Workspace.WorkspaceID, match[1], ev.Workspace.SeedFiles)
	check(strings.Contains(ev.Mac.DataDirectory, "tt-icloud-exchange."), "the Mac workspace %s is not the run's scratch copy", ev.Mac.DataDirectory)
	check(strings.Contains(ev.IPhone.DataDirectory, "contract-runs/"+ev.RunID+"/"), "the iPhone workspace %s is not the run's copy", ev.IPhone.DataDirectory)

	// Both directions, verified and converged.
	o, r := ev.Originator, ev.Responder
	for _, p := range []partyEvidence{o, r} {
		check(p.RunID == ev.RunID && p.WorkspaceID == ev.Workspace.WorkspaceID, "%s ran %s in %s", p.Role, p.RunID, p.WorkspaceID)
		check(len(p.Published) >= 2, "%s published %v: the task and its attachment are two transactions", p.Role, p.Published)
		check(p.Passes > 0 && slices.Contains(p.ConditionsObserved, "reachable"), "%s passes %d conditions %v", p.Role, p.Passes, p.ConditionsObserved)
		check(p.LastSuccessfulExchg != "", "%s never completed an exchange", p.Role)
		check(strings.HasPrefix(p.WorkspaceDigest, "sha256:"), "%s digest %q", p.Role, p.WorkspaceDigest)
	}
	check(o.Role == "originator" && r.Role == "responder" && o.DeviceID != r.DeviceID, "parties %s/%s %s/%s", o.Role, o.DeviceID, r.Role, r.DeviceID)
	check(strings.HasPrefix(o.DeviceID, "mac-") && strings.HasPrefix(r.DeviceID, "iphone-"), "device ids %s %s", o.DeviceID, r.DeviceID)
	sorted := func(s []string) []string { s = slices.Clone(s); slices.Sort(s); return s }
	check(slices.Equal(sorted(r.Imported), sorted(o.Published)), "the iPhone imported %v, the Mac published %v", r.Imported, o.Published)
	check(slices.Equal(sorted(o.Imported), sorted(r.Published)), "the Mac imported %v, the iPhone published %v", o.Imported, r.Published)
	check(r.SawPeerTitle == o.Title && r.SawPeerAttachment == o.Attachment, "the iPhone saw %q %q", r.SawPeerTitle, r.SawPeerAttachment)
	check(o.SawPeerTitle == r.Title && o.SawPeerAttachment == r.Attachment, "the Mac saw %q %q", o.SawPeerTitle, o.SawPeerAttachment)
	check(o.WorkspaceDigest == r.WorkspaceDigest, "the devices did not converge: %s vs %s", o.WorkspaceDigest, r.WorkspaceDigest)

	// The on-device suite itself.
	check(ev.XCResult.Result == "Passed" && ev.XCResult.Passed >= 1 && ev.XCResult.Failed == 0 && ev.XCResult.Skipped == 0,
		"device suite %s passed %d failed %d skipped %d", ev.XCResult.Result, ev.XCResult.Passed, ev.XCResult.Failed, ev.XCResult.Skipped)
	ran := false
	for _, test := range ev.XCResult.Tests {
		if strings.HasPrefix(test.Name, "testRespondsToTheMacOverICloudDrive") {
			ran = test.Result == "Passed"
		}
	}
	check(ran, "testRespondsToTheMacOverICloudDrive did not pass on the iPhone: %+v", ev.XCResult.Tests)
}
