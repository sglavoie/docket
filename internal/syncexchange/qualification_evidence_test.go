package syncexchange

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// qualificationPath is the recorded real Mac/iPhone qualification trial
// (ios/scripts/record-device-qualification.sh).
var qualificationPath = filepath.Join("..", "..", "docs", "mobile", "evidence", "session-18-qualification.json")

var rolloutPath = filepath.Join("..", "..", "docs", "mobile", "personal-rollout.md")

const (
	qualificationLargeMacBytes = 64 << 20
	qualificationLargeMinBytes = 25 << 20
	qualificationDelaySeconds  = 600
	qualificationMinDays       = 7
	qualificationAppGroup      = "group.io.github.sglavoie.tt"
)

type qualificationDevice struct {
	Digest        string `json:"digest"`
	DeviceID      string `json:"device_id"`
	WorkspaceID   string `json:"workspace_id"`
	OutstandingUp int    `json:"outstanding_up"`
	// LocalTransactions is how many transactions this device authored, and
	// Received the highest sequence it applied from each other device.
	LocalTransactions uint64            `json:"local_transactions"`
	Received          map[string]uint64 `json:"received"`
	Conflicts         int               `json:"conflicts"`
	ExportedAt        string            `json:"exported_at"`
	AppExecutables    map[string]string `json:"app_executables"`
	EngineError       *string           `json:"engine_error"`
	OutstandingDown   int               `json:"outstanding_down"`
	Held              int               `json:"held"`
	Location          string            `json:"location"`
	Transport         struct {
		HasFolder          bool    `json:"has_folder"`
		Condition          *string `json:"condition"`
		LastError          *string `json:"last_error"`
		OutstandingUploads int     `json:"outstanding_uploads"`
	} `json:"transport"`
}

type qualificationCheckpoint struct {
	Scenario    string               `json:"scenario"`
	Label       string               `json:"label"`
	At          string               `json:"at"`
	Mac         qualificationDevice  `json:"mac"`
	IPhone      *qualificationDevice `json:"iphone"`
	Markers     map[string]int       `json:"markers"`
	Attachments []struct {
		Path   string `json:"path"`
		Size   int64  `json:"size"`
		SHA256 string `json:"sha256"`
	} `json:"attachments"`
	Problems []string `json:"problems"`
}

type qualificationEvidence struct {
	Format        string `json:"format"`
	FormatVersion int    `json:"format_version"`
	RunID         string `json:"run_id"`
	StartedAt     string `json:"started_at"`
	FinishedAt    string `json:"finished_at"`
	Builds        struct {
		SourceFingerprint string            `json:"source_fingerprint"`
		BridgeFingerprint string            `json:"bridge_fingerprint"`
		TTSHA256          string            `json:"tt_sha256"`
		AppExecutables    map[string]string `json:"app_executables"`
		// Installs are every install of the trial's own app, oldest first.
		// A rebuild re-signs the app, so its executable digests change; each
		// install is bound by the fingerprints it was built from.
		Installs []qualificationInstall `json:"installs"`
	} `json:"builds"`
	Signing struct {
		TeamIdentifier        string   `json:"team_identifier"`
		Authorities           []string `json:"authorities"`
		Verified              bool     `json:"verified"`
		ApplicationGroups     []string `json:"application_groups"`
		ApplicationIdentifier string   `json:"application_identifier"`
		InstalledAt           string   `json:"installed_at"`
		InstalledWith         string   `json:"installed_with"`
	} `json:"signing"`
	Mac struct {
		Model         string `json:"model"`
		OSVersion     string `json:"os_version"`
		Home          string `json:"home"`
		Original      string `json:"original"`
		DataDirectory string `json:"data_directory"`
		DeviceID      string `json:"device_id"`
	} `json:"mac"`
	IPhone struct {
		UDID      string `json:"udid"`
		Name      string `json:"name"`
		Model     string `json:"model"`
		OSVersion string `json:"os_version"`
		Reality   string `json:"reality"`
	} `json:"iphone"`
	Exchange struct {
		Provider string `json:"provider"`
		Folder   string `json:"folder"`
	} `json:"exchange"`
	Workspace struct {
		Fixture     string `json:"fixture"`
		WorkspaceID string `json:"workspace_id"`
	} `json:"workspace"`
	Checkpoints []qualificationCheckpoint `json:"checkpoints"`
	Scenarios   map[string]struct {
		Checkpoints  []string       `json:"checkpoints"`
		Observations map[string]any `json:"observations"`
		Remarks      string         `json:"remarks"`
	} `json:"scenarios"`
	DailyUse []struct {
		Date                  string `json:"date"`
		LostEdits             bool   `json:"lost_edits"`
		UnexplainedDivergence bool   `json:"unexplained_divergence"`
		Remarks               string `json:"remarks"`
	} `json:"daily_use"`
	Procedures map[string]struct {
		ValidatedAt       string         `json:"validated_at"`
		Document          string         `json:"document"`
		Section           string         `json:"section"`
		SectionSHA256     string         `json:"section_sha256"`
		FollowedAsWritten bool           `json:"followed_as_written"`
		Remarks           string         `json:"remarks"`
		Checks            map[string]any `json:"checks"`
	} `json:"procedures"`
	Failures []any `json:"failures"`
}

type qualificationInstall struct {
	At                string            `json:"at"`
	Reason            string            `json:"reason"`
	AppExecutables    map[string]string `json:"app_executables"`
	SourceFingerprint string            `json:"source_fingerprint"`
	BridgeFingerprint string            `json:"bridge_fingerprint"`
}

// checkpointRule is what one recorded checkpoint must show.
type checkpointRule struct {
	converged   bool     // equal digests and nothing unpublished on either side
	present     []string // marker tags the Mac holds exactly once
	absent      []string // marker tags the Mac must not hold yet
	conflicts   string   // "none" or "both"
	isolated    bool     // the iPhone had not applied the Mac's latest transaction
	unreachable bool     // the iPhone could not reach the exchange
	anyBuild    bool     // the iPhone ran an earlier build (upgrade's before)
}

// qualificationRules mirrors ios/scripts/device_qualification.py: the
// scenarios, their checkpoints in order, and what each must show.
var qualificationRules = map[string][]struct {
	label string
	rule  checkpointRule
}{
	"airplane_mode": {
		{"offline", checkpointRule{present: []string{"mac-1"}, absent: []string{"iphone-1", "iphone-2"}, isolated: true}},
		{"online", checkpointRule{converged: true, present: []string{"iphone-1", "iphone-2", "mac-1"}, conflicts: "none"}},
	},
	"simultaneous_edits": {
		{"base", checkpointRule{converged: true, present: []string{"shared"}, conflicts: "none"}},
		{"conflicted", checkpointRule{present: []string{"iphone-only", "mac-only"}, conflicts: "both"}},
		{"resolved", checkpointRule{converged: true, present: []string{"iphone-only", "mac-only"}, absent: []string{"shared"}, conflicts: "none"}},
	},
	"termination": {
		{"relaunched", checkpointRule{converged: true, present: []string{"iphone-1", "iphone-2", "mac-1"}, conflicts: "none"}},
	},
	"delayed_delivery": {
		{"delivered", checkpointRule{converged: true, present: []string{"mac-1", "iphone-1"}, conflicts: "none"}},
	},
	"revoked_access": {
		{"revoked", checkpointRule{present: []string{"mac-1"}, absent: []string{"iphone-1"}, isolated: true, unreachable: true}},
		{"restored", checkpointRule{converged: true, present: []string{"iphone-1", "mac-1"}, conflicts: "none"}},
	},
	"large_attachments": {
		{"delivered", checkpointRule{converged: true, present: []string{"mac", "iphone"}, conflicts: "none"}},
	},
	"upgrade": {
		{"before", checkpointRule{present: []string{"mac-old"}, absent: []string{"iphone-pending"}, anyBuild: true}},
		{"after", checkpointRule{converged: true, present: []string{"iphone-old", "mac-old", "iphone-pending", "mac-new"}, conflicts: "none"}},
	},
}

var qualificationProcedures = map[string]string{
	"enrollment": "## Enrollment", "backup": "## Backup", "recovery": "## Recovery",
	"troubleshooting": "## Troubleshooting", "upgrade": "## Upgrade",
}

// rolloutSectionDigest is device_qualification.py's section_digest: the
// heading line and everything after it up to the next level-two heading.
func rolloutSectionDigest(t *testing.T, heading string) string {
	t.Helper()
	data, err := os.ReadFile(rolloutPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	start := -1
	if strings.HasPrefix(text, heading+"\n") {
		start = 0
	} else if i := strings.Index(text, "\n"+heading+"\n"); i >= 0 {
		start = i + 1
	}
	if start < 0 {
		t.Fatalf("%s has no section %q", rolloutPath, heading)
	}
	body := text[start+len(heading)+1:]
	if end := strings.Index(body, "\n## "); end >= 0 {
		body = body[:end+1]
	} else if strings.HasPrefix(body, "## ") {
		body = ""
	}
	sum := sha256.Sum256([]byte(heading + "\n" + body))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// TestRecordedDeviceQualificationEvidence validates the recorded real-device
// trial against the builds the verifier computed from the current tree and
// recomputes every verdict from the raw observations: each scenario's
// checkpoints, the sustained daily use, the validated procedures, and the
// signed installation. Missing or failed evidence fails. Outside
// scripts/verify_mobile_session.sh 18 it has nothing to compare against and
// skips, which that verifier refuses.
func TestRecordedDeviceQualificationEvidence(t *testing.T) {
	wantSource := os.Getenv("TT_QUALIFICATION_SOURCE_FINGERPRINT")
	wantBridge := os.Getenv("TT_IOS_BRIDGE_FINGERPRINT")
	wantTT := os.Getenv("TT_QUALIFICATION_TT_SHA256")
	if wantSource == "" || wantBridge == "" || wantTT == "" {
		t.Skip("set by scripts/verify_mobile_session.sh 18")
	}
	data, err := os.ReadFile(qualificationPath)
	if err != nil {
		t.Fatalf("no recorded device qualification: %v (run ios/scripts/record-device-qualification.sh; docs/mobile/personal-rollout.md)", err)
	}
	var ev qualificationEvidence
	if err := json.Unmarshal(data, &ev); err != nil {
		t.Fatalf("evidence does not decode: %v", err)
	}
	for _, problem := range validateQualification(t, &ev, wantSource, wantBridge, wantTT) {
		t.Error(problem)
	}
}
