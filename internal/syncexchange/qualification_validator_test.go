package syncexchange

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// syntheticQualification builds the evidence a complete, clean trial would
// record, so the validator's refusals can be proven one omission at a time.
func syntheticQualification(t *testing.T) *qualificationEvidence {
	t.Helper()
	fixture, err := os.ReadFile(filepath.Join("..", "compat", "testdata", "workspaces", "representative", ".docket", "workspace.json"))
	if err != nil {
		t.Fatal(err)
	}
	workspaceID := string(regexp.MustCompile(`"workspace_id":\s*"([^"]+)"`).FindSubmatch(fixture)[1])
	const run = "20261001T090000Z"
	start := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	app := map[string]string{"tt": "sha256:current", "tt.debug.dylib": "sha256:currentdylib"}
	trial := "/Users/me/.docket-qualification/" + run + "/"

	ev := &qualificationEvidence{Format: "tt-device-qualification-evidence", FormatVersion: 1, RunID: run,
		StartedAt: start.Format(time.RFC3339), FinishedAt: start.Add(9 * 24 * time.Hour).Format(time.RFC3339)}
	ev.Builds.SourceFingerprint, ev.Builds.BridgeFingerprint, ev.Builds.DocketSHA256 = "src", "bridge", "tt"
	ev.Builds.AppExecutables = app
	ev.Builds.Installs = []qualificationInstall{{At: start.Format(time.RFC3339), Reason: "start", AppExecutables: app,
		SourceFingerprint: "src", BridgeFingerprint: "bridge"}}
	ev.Signing.TeamIdentifier = "ABCDE12345"
	ev.Signing.Authorities = []string{"Apple Development: Me (XYZ)", "Apple Worldwide Developer Relations Certification Authority"}
	ev.Signing.Verified = true
	ev.Signing.ApplicationGroups = []string{qualificationAppGroup}
	ev.Signing.ApplicationIdentifier = "ABCDE12345.io.github.sglavoie.docket.mobile"
	ev.Signing.InstalledAt, ev.Signing.InstalledWith = ev.StartedAt, "devicectl"
	ev.Mac.Model, ev.Mac.OSVersion, ev.Mac.DeviceID = "Mac14,14", "27.0", "mac-device"
	ev.Mac.Home, ev.Mac.Original, ev.Mac.DataDirectory = trial+"home", trial+"original", trial+"working"
	ev.IPhone.UDID, ev.IPhone.Model, ev.IPhone.Reality = "00008110-0001", "iPhone14,2", "physical"
	ev.Exchange.Provider = "iCloud Drive"
	ev.Exchange.Folder = "/Users/me/Library/Mobile Documents/com~apple~CloudDocs/docket-qualification/" + run
	ev.Workspace.Fixture, ev.Workspace.WorkspaceID = "internal/compat/testdata/workspaces/representative", workspaceID
	ev.Scenarios = map[string]struct {
		Checkpoints  []string       `json:"checkpoints"`
		Observations map[string]any `json:"observations"`
		Remarks      string         `json:"remarks"`
	}{}
	ev.Procedures = map[string]struct {
		ValidatedAt       string         `json:"validated_at"`
		Document          string         `json:"document"`
		Section           string         `json:"section"`
		SectionSHA256     string         `json:"section_sha256"`
		FollowedAsWritten bool           `json:"followed_as_written"`
		Remarks           string         `json:"remarks"`
		Checks            map[string]any `json:"checks"`
	}{}

	marker := func(scenario, tag string) string {
		return "q18 " + run + " " + strings.ReplaceAll(scenario, "_", "-") + " " + tag
	}
	at := start.Add(time.Hour)
	add := func(scenario, label string, rule checkpointRule, extra ...string) *qualificationCheckpoint {
		at = at.Add(10 * time.Minute)
		digest := "sha256:" + label
		c := qualificationCheckpoint{Scenario: scenario, Label: label, At: at.Format(time.RFC3339), Markers: map[string]int{}}
		c.Mac = qualificationDevice{Digest: digest, DeviceID: "mac-device", WorkspaceID: workspaceID}
		phone := qualificationDevice{Digest: digest, DeviceID: "iphone-device", WorkspaceID: workspaceID,
			ExportedAt: at.Add(-time.Minute).Format(time.RFC3339), AppExecutables: app}
		if !rule.converged {
			phone.Digest = "sha256:other"
		}
		c.Mac.LocalTransactions = 5
		phone.Received = map[string]uint64{"mac-device": 5}
		if rule.isolated {
			phone.Received["mac-device"] = 4
		}
		if rule.conflicts == "both" {
			c.Mac.Conflicts, phone.Conflicts = 1, 1
		}
		if rule.unreachable {
			condition := "needsFolderAccess"
			phone.Transport.Condition = &condition
		}
		c.IPhone = &phone
		for _, tag := range rule.present {
			c.Markers[marker(scenario, tag)] = 1
		}
		for _, tag := range rule.absent {
			c.Markers[marker(scenario, tag)] = 0
		}
		for _, title := range extra {
			c.Markers[title] = 1
		}
		ev.Checkpoints = append(ev.Checkpoints, c)
		return &ev.Checkpoints[len(ev.Checkpoints)-1]
	}

	add("enrollment", "joined", checkpointRule{converged: true})
	observations := map[string]map[string]any{
		"simultaneous_edits": {"kept_title": marker("simultaneous_edits", "renamed-on-mac")},
		"termination":        {"mac_exchanges_killed": float64(2)},
		"delayed_delivery":   {"online_after_seconds": float64(900)},
		"revoked_access":     {"saved_locally_banner": true},
		"large_attachments":  {"mac_attachment_sha256": "sha256:big", "opened_on_peer": true},
		"upgrade": {"from_commit": strings.Repeat("a", 40),
			"from_app_executables": map[string]any{"tt": "sha256:old", "tt.debug.dylib": "sha256:olddylib"}},
	}
	for _, scenario := range slices.Sorted(maps.Keys(qualificationRules)) {
		var labels []string
		for _, r := range qualificationRules[scenario] {
			labels = append(labels, r.label)
			var extra []string
			if scenario == "simultaneous_edits" && r.label == "resolved" {
				extra = []string{marker(scenario, "renamed-on-mac")}
			}
			c := add(scenario, r.label, r.rule, extra...)
			if scenario == "large_attachments" {
				c.Attachments = append(c.Attachments,
					struct {
						Path   string `json:"path"`
						Size   int64  `json:"size"`
						SHA256 string `json:"sha256"`
					}{"projects/a/.attachments/x/mac.bin", qualificationLargeMacBytes, "sha256:big"},
					struct {
						Path   string `json:"path"`
						Size   int64  `json:"size"`
						SHA256 string `json:"sha256"`
					}{"projects/a/.attachments/y/video.mov", 30 << 20, "sha256:video"})
			}
			if scenario == "upgrade" && r.label == "before" {
				c.IPhone.AppExecutables = map[string]string{"tt": "sha256:old", "tt.debug.dylib": "sha256:olddylib"}
			}
		}
		entry := ev.Scenarios[scenario]
		entry.Checkpoints, entry.Observations = labels, observations[scenario]
		ev.Scenarios[scenario] = entry
	}
	for day := range qualificationMinDays {
		date := start.AddDate(0, 0, day+1).Format(time.DateOnly)
		ev.DailyUse = append(ev.DailyUse, struct {
			Date                  string `json:"date"`
			LostEdits             bool   `json:"lost_edits"`
			UnexplainedDivergence bool   `json:"unexplained_divergence"`
			Remarks               string `json:"remarks"`
		}{Date: date})
		at = start.AddDate(0, 0, day+1).Add(20 * time.Hour)
		add("daily_use", date, checkpointRule{converged: true}, "q18 "+run+" day "+date+" mac", "q18 "+run+" day "+date+" iphone")
	}
	for name, heading := range qualificationProcedures {
		p := ev.Procedures[name]
		p.ValidatedAt, p.Document, p.Section = ev.StartedAt, "docs/mobile/personal-rollout.md", heading
		p.SectionSHA256, p.FollowedAsWritten = rolloutSectionDigest(t, heading), true
		p.Checks = map[string]any{"done": true}
		ev.Procedures[name] = p
	}
	return ev
}

// TestQualificationValidatorRefusesIncompleteEvidence proves the session-18
// gate is behavioral: a complete trial passes, and each kind of missing,
// failed, stale, or simulated evidence is refused with a reason.
func TestQualificationValidatorRefusesIncompleteEvidence(t *testing.T) {
	validate := func(ev *qualificationEvidence) []string {
		// Round-trip through JSON so the cases see what a recorded file holds.
		data, err := json.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}
		var decoded qualificationEvidence
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		return validateQualification(t, &decoded, "src", "bridge", "tt")
	}
	if problems := validate(syntheticQualification(t)); len(problems) > 0 {
		t.Fatalf("a complete trial was refused:\n%s", strings.Join(problems, "\n"))
	}

	checkpoint := func(ev *qualificationEvidence, key string) *qualificationCheckpoint {
		for i := range ev.Checkpoints {
			if ev.Checkpoints[i].Scenario+"/"+ev.Checkpoints[i].Label == key {
				return &ev.Checkpoints[i]
			}
		}
		t.Fatalf("no checkpoint %s", key)
		return nil
	}
	cases := map[string]struct {
		mutate func(*qualificationEvidence)
		want   string
	}{
		"stale sources":     {func(ev *qualificationEvidence) { ev.Builds.SourceFingerprint = "old" }, "Swift sources"},
		"stale tt":          {func(ev *qualificationEvidence) { ev.Builds.DocketSHA256 = "old" }, "the trial ran tt"},
		"unsigned":          {func(ev *qualificationEvidence) { ev.Signing.Verified = false }, "not verified as signed"},
		"no app group":      {func(ev *qualificationEvidence) { ev.Signing.ApplicationGroups = nil }, "app groups"},
		"simulator":         {func(ev *qualificationEvidence) { ev.IPhone.Reality, ev.IPhone.Model = "simulated", "arm64" }, "not a physical iPhone"},
		"real workspace":    {func(ev *qualificationEvidence) { ev.Mac.DataDirectory = "/Users/me/tt-synced" }, "not inside the trial"},
		"missing scenario":  {func(ev *qualificationEvidence) { delete(ev.Scenarios, "revoked_access") }, "revoked_access was not recorded"},
		"missing procedure": {func(ev *qualificationEvidence) { delete(ev.Procedures, "recovery") }, "recovery was not validated"},
		"never joined":      {func(ev *qualificationEvidence) { ev.Checkpoints = ev.Checkpoints[1:] }, "never joined"},
		"diverged":          {func(ev *qualificationEvidence) { checkpoint(ev, "airplane_mode/online").IPhone.Digest = "sha256:x" }, "the devices differ"},
		"lost edit": {func(ev *qualificationEvidence) {
			checkpoint(ev, "termination/relaunched").Markers["q18 "+ev.RunID+" termination iphone-1"] = 0
		}, "copies of \"iphone-1\""},
		"duplicate capture": {func(ev *qualificationEvidence) {
			checkpoint(ev, "termination/relaunched").Markers["q18 "+ev.RunID+" termination mac-1"] = 2
		}, "copies of \"mac-1\""},
		"never offline": {func(ev *qualificationEvidence) {
			checkpoint(ev, "airplane_mode/offline").IPhone.Received["mac-device"] = 5
		}, "was not cut off"},
		"conflict not kept": {func(ev *qualificationEvidence) { checkpoint(ev, "simultaneous_edits/conflicted").IPhone.Conflicts = 0 }, "not preserved on both"},
		"open conflict":     {func(ev *qualificationEvidence) { checkpoint(ev, "large_attachments/delivered").Mac.Conflicts = 1 }, "open conflicts"},
		"access never revoked": {func(ev *qualificationEvidence) {
			reachable := "reachable"
			checkpoint(ev, "revoked_access/revoked").IPhone.Transport.Condition = &reachable
		}, "still reached"},
		"no delay": {func(ev *qualificationEvidence) {
			ev.Scenarios["delayed_delivery"].Observations["online_after_seconds"] = float64(30)
		}, "came back after"},
		"small attachment": {func(ev *qualificationEvidence) {
			checkpoint(ev, "large_attachments/delivered").Attachments[1].Size = 1 << 20
		}, "large attachments"},
		"no upgrade": {func(ev *qualificationEvidence) {
			ev.Scenarios["upgrade"].Observations["from_app_executables"] = map[string]any{"tt": "sha256:current", "tt.debug.dylib": "sha256:currentdylib"}
		}, "which is one of the trial's own installs"},
		"other build": {func(ev *qualificationEvidence) {
			checkpoint(ev, "upgrade/after").IPhone.AppExecutables = map[string]string{"tt": "sha256:x"}
		}, "the iPhone ran"},
		"failed checkpoint":  {func(ev *qualificationEvidence) { checkpoint(ev, "upgrade/after").Problems = []string{"timed out"} }, "recorded as failed"},
		"no iPhone export":   {func(ev *qualificationEvidence) { checkpoint(ev, "delayed_delivery/delivered").IPhone = nil }, "no iPhone diagnostics"},
		"short trial":        {func(ev *qualificationEvidence) { ev.DailyUse = ev.DailyUse[:5] }, "daily use covers 5 days"},
		"lost edit on a day": {func(ev *qualificationEvidence) { ev.DailyUse[2].LostEdits = true }, "an edit was lost"},
		"unexplained":        {func(ev *qualificationEvidence) { ev.DailyUse[3].UnexplainedDivergence = true }, "diverged unexplained"},
		"edited procedure": {func(ev *qualificationEvidence) {
			p := ev.Procedures["backup"]
			p.SectionSHA256 = "sha256:old"
			ev.Procedures["backup"] = p
		}, "changed since it was validated"},
		"not as written": {func(ev *qualificationEvidence) {
			p := ev.Procedures["upgrade"]
			p.FollowedAsWritten = false
			ev.Procedures["upgrade"] = p
		}, "not followed as written"},
		"failed check": {func(ev *qualificationEvidence) {
			p := ev.Procedures["recovery"]
			p.Checks = map[string]any{"rejoined": false}
			ev.Procedures["recovery"] = p
		}, "check rejoined failed"},
		"unresolved failure": {func(ev *qualificationEvidence) { ev.Failures = []any{"sync stalled"} }, "unresolved failures"},
	}
	for name, c := range cases {
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			ev := syntheticQualification(t)
			c.mutate(ev)
			problems := validate(ev)
			if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, c.want) }) {
				t.Fatalf("want a problem containing %q, got:\n%s", c.want, fmt.Sprint(strings.Join(problems, "\n")))
			}
		})
	}
}

// TestQualificationValidatorReadsScenarioAttempts holds a rerun scenario to
// the titles its recorded attempt implies: suffixed markers pass, and the
// first attempt's unsuffixed ones no longer count.
func TestQualificationValidatorReadsScenarioAttempts(t *testing.T) {
	validate := func(ev *qualificationEvidence) []string {
		data, err := json.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}
		var decoded qualificationEvidence
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		return validateQualification(t, &decoded, "src", "bridge", "tt")
	}
	rerun := func(suffix bool) *qualificationEvidence {
		ev := syntheticQualification(t)
		ev.Scenarios["delayed_delivery"].Observations["attempt"] = float64(2)
		for i := range ev.Checkpoints {
			c := &ev.Checkpoints[i]
			if c.Scenario != "delayed_delivery" || !suffix {
				continue
			}
			renamed := map[string]int{}
			for title, n := range c.Markers {
				renamed[title+" a2"] = n
			}
			c.Markers = renamed
		}
		return ev
	}
	if problems := validate(rerun(true)); len(problems) > 0 {
		t.Fatalf("a second attempt with its own markers was refused:\n%s", strings.Join(problems, "\n"))
	}
	problems := validate(rerun(false))
	if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, "delayed_delivery/delivered") }) {
		t.Fatalf("a second attempt counted the first attempt's markers: %v", problems)
	}
}

// TestQualificationValidatorHoldsCheckpointsToInstalls proves the build
// binding survives a rebuild without loosening: a same-source reinstall is
// honored from its moment on, an install built from other sources is refused,
// and a checkpoint must show the install current at its time.
func TestQualificationValidatorHoldsCheckpointsToInstalls(t *testing.T) {
	validate := func(ev *qualificationEvidence) []string {
		data, err := json.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}
		var decoded qualificationEvidence
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		return validateQualification(t, &decoded, "src", "bridge", "tt")
	}
	resigned := map[string]string{"tt": "sha256:resigned", "tt.debug.dylib": "sha256:resigneddylib"}
	reinstall := func(ev *qualificationEvidence) time.Time {
		// Reinstall just before the daily use, so every daily checkpoint
		// comes after it.
		at, _ := time.Parse(time.DateOnly, ev.DailyUse[0].Date)
		ev.Builds.Installs = append(ev.Builds.Installs, qualificationInstall{At: at.Format(time.RFC3339), Reason: "reinstall",
			AppExecutables: resigned, SourceFingerprint: "src", BridgeFingerprint: "bridge"})
		return at
	}
	has := func(problems []string, want string) bool {
		return slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, want) })
	}

	ev := syntheticQualification(t)
	at := reinstall(ev)
	for i := range ev.Checkpoints {
		if exported, _ := time.Parse(time.RFC3339, ev.Checkpoints[i].IPhone.ExportedAt); !exported.Before(at) {
			ev.Checkpoints[i].IPhone.AppExecutables = resigned
		}
	}
	if problems := validate(ev); len(problems) > 0 {
		t.Fatalf("a same-source reinstall was refused:\n%s", strings.Join(problems, "\n"))
	}

	ev = syntheticQualification(t)
	reinstall(ev)
	if problems := validate(ev); !has(problems, "the trial's install at that time") {
		t.Fatalf("checkpoints after a reinstall still passed on the earlier build: %v", problems)
	}

	ev = syntheticQualification(t)
	reinstall(ev)
	ev.Builds.Installs[1].SourceFingerprint = "other"
	if problems := validate(ev); !has(problems, "install 1 (reinstall) was built from sources other") {
		t.Fatalf("an install from other sources was accepted: %v", problems)
	}
}

// TestQualificationValidatorResumedUpgrade covers a resumed upgrade: the
// person's iPhone markers from one attempt with the Mac's from a later one,
// and an earlier app identified by its pre-qualification export when its
// signed digests could not be kept.
func TestQualificationValidatorResumedUpgrade(t *testing.T) {
	validate := func(ev *qualificationEvidence) []string {
		data, err := json.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}
		var decoded qualificationEvidence
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		return validateQualification(t, &decoded, "src", "bridge", "tt")
	}
	resumed := func() *qualificationEvidence {
		ev := syntheticQualification(t)
		obs := ev.Scenarios["upgrade"].Observations
		obs["attempt"], obs["mac_attempt"] = float64(2), float64(3)
		obs["from_app_executables"] = map[string]any{}
		for i := range ev.Checkpoints {
			c := &ev.Checkpoints[i]
			if c.Scenario != "upgrade" {
				continue
			}
			renamed := map[string]int{}
			for title, n := range c.Markers {
				suffix := " a2"
				if strings.Contains(title, " mac-") {
					suffix = " a3"
				}
				renamed[title+suffix] = n
			}
			c.Markers = renamed
			if c.Label == "before" {
				c.IPhone.AppExecutables, c.IPhone.Digest = nil, ""
			}
		}
		return ev
	}
	if problems := validate(resumed()); len(problems) > 0 {
		t.Fatalf("a resumed upgrade was refused:\n%s", strings.Join(problems, "\n"))
	}
	ev := resumed()
	for i := range ev.Checkpoints {
		if ev.Checkpoints[i].Scenario == "upgrade" && ev.Checkpoints[i].Label == "before" {
			ev.Checkpoints[i].IPhone.Digest = "sha256:qualification"
		}
	}
	if problems := validate(ev); !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, "does not show the earlier app") }) {
		t.Fatalf("a before export from the trial's own build passed as the earlier app: %v", problems)
	}
	ev = resumed()
	delete(ev.Scenarios["upgrade"].Observations, "mac_attempt")
	if problems := validate(ev); !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, "upgrade/before") }) {
		t.Fatalf("Mac markers from another attempt counted without the record naming it: %v", problems)
	}
}
