package syncexchange

import (
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

// qualificationValidator collects every reason a recorded trial falls short.
type qualificationValidator struct {
	t                 *testing.T
	ev                *qualificationEvidence
	problems          []string
	byKey             map[string]qualificationCheckpoint
	started, finished time.Time
}

func validateQualification(t *testing.T, ev *qualificationEvidence, wantSource, wantBridge, wantTT string) []string {
	v := &qualificationValidator{t: t, ev: ev, byKey: map[string]qualificationCheckpoint{}}
	v.header(wantSource, wantBridge, wantTT)
	v.installation()
	for _, c := range ev.Checkpoints {
		key := c.Scenario + "/" + c.Label
		_, dup := v.byKey[key]
		v.check(!dup, "checkpoint %s recorded twice", key)
		v.byKey[key] = c
	}
	// Enrollment: the iPhone joined the Mac's disposable workspace.
	if c, ok := v.byKey["enrollment/joined"]; ok {
		v.verify(c, checkpointRule{converged: true, conflicts: "none"})
	} else {
		v.fail("no enrollment checkpoint: the iPhone never joined")
	}
	v.scenarios()
	v.dailyUse()
	v.procedures()
	v.check(len(ev.Failures) == 0, "unresolved failures: %v", ev.Failures)
	return v.problems
}

func (v *qualificationValidator) check(ok bool, format string, args ...any) {
	if !ok {
		v.fail(fmt.Sprintf(format, args...))
	}
}

func (v *qualificationValidator) fail(problem string) { v.problems = append(v.problems, problem) }

func (v *qualificationValidator) parse(label, value string) time.Time {
	at, err := time.Parse(time.RFC3339, value)
	v.check(err == nil, "%s %q is not a time", label, value)
	return at
}

// marker is device_qualification.py's Trial.marker: a scenario rerun after an
// interrupted attempt records its attempt and uses titles suffixed "aN".
func (v *qualificationValidator) marker(scenario, tag string) string {
	title := "q18 " + v.ev.RunID + " " + strings.ReplaceAll(scenario, "_", "-") + " " + tag
	obs := v.ev.Scenarios[scenario].Observations
	attempt, _ := obs["attempt"].(float64)
	// A resumed run can reuse the person's iPhone markers from one attempt
	// while the Mac's come from a later one; the record names both.
	if mac, ok := obs["mac_attempt"].(float64); ok && strings.HasPrefix(tag, "mac") {
		attempt = mac
	}
	if attempt > 1 {
		title += fmt.Sprintf(" a%d", int(attempt))
	}
	return title
}

// installAt is the executables of the trial's latest install made at or
// before moment: what the iPhone must have been running then.
func (v *qualificationValidator) installAt(moment time.Time) map[string]string {
	var want map[string]string
	for _, install := range v.ev.Builds.Installs {
		at, err := time.Parse(time.RFC3339, install.At)
		if err == nil && !at.After(moment) {
			want = install.AppExecutables
		}
	}
	return want
}

// header holds the trial to the builds under test.
func (v *qualificationValidator) header(wantSource, wantBridge, wantTT string) {
	ev := v.ev
	v.check(ev.Format == "tt-device-qualification-evidence" && ev.FormatVersion == 1, "format %s v%d", ev.Format, ev.FormatVersion)
	v.check(regexp.MustCompile(`^\d{8}T\d{6}Z$`).MatchString(ev.RunID), "run id %q", ev.RunID)
	v.started, v.finished = v.parse("started_at", ev.StartedAt), v.parse("finished_at", ev.FinishedAt)
	v.check(ev.Builds.SourceFingerprint == wantSource, "the trial ran Swift sources %s, the tree is %s: run a new trial", ev.Builds.SourceFingerprint, wantSource)
	v.check(ev.Builds.BridgeFingerprint == wantBridge, "the trial ran bridge %s, the tree builds %s", ev.Builds.BridgeFingerprint, wantBridge)
	v.check(ev.Builds.TTSHA256 == wantTT, "the trial ran tt %s, the tree builds %s", ev.Builds.TTSHA256, wantTT)
	v.check(len(ev.Builds.AppExecutables) > 0, "no app build hashes")
	v.check(len(ev.Builds.Installs) > 0 && maps.Equal(ev.Builds.Installs[0].AppExecutables, ev.Builds.AppExecutables),
		"the trial's first install is not the build it started with")
	for i, install := range ev.Builds.Installs {
		v.check(install.SourceFingerprint == wantSource && install.BridgeFingerprint == wantBridge,
			"install %d (%s) was built from sources %s and bridge %s, not the tree's", i, install.Reason, install.SourceFingerprint, install.BridgeFingerprint)
		v.check(len(install.AppExecutables) > 0, "install %d records no executables", i)
	}
}

// installation holds the signed personal install, the devices, and the
// disposable workspace.
func (v *qualificationValidator) installation() {
	ev, s := v.ev, v.ev.Signing
	v.check(s.Verified && s.TeamIdentifier != "" && s.TeamIdentifier != "not set", "the app was not verified as signed by a team: %+v", s)
	v.check(slices.ContainsFunc(s.Authorities, func(a string) bool {
		return strings.HasPrefix(a, "Apple Development:") || strings.HasPrefix(a, "Apple Distribution:") || strings.HasPrefix(a, "iPhone Developer:")
	}), "signing authorities %v name no Apple signing certificate", s.Authorities)
	v.check(slices.Contains(s.ApplicationGroups, qualificationAppGroup), "the signed app holds app groups %v, not %s", s.ApplicationGroups, qualificationAppGroup)
	v.check(strings.HasPrefix(s.ApplicationIdentifier, s.TeamIdentifier+"."), "application identifier %q is not the team's", s.ApplicationIdentifier)
	v.check(s.InstalledWith == "devicectl" && s.InstalledAt != "", "installation %q at %q", s.InstalledWith, s.InstalledAt)
	v.check(ev.IPhone.Reality == "physical" && strings.HasPrefix(ev.IPhone.Model, "iPhone") && ev.IPhone.UDID != "",
		"iPhone %q %q %q is not a physical iPhone", ev.IPhone.UDID, ev.IPhone.Model, ev.IPhone.Reality)
	v.check(strings.HasPrefix(ev.Mac.Model, "Mac") && ev.Mac.OSVersion != "", "Mac %q %q", ev.Mac.Model, ev.Mac.OSVersion)
	v.check(ev.Exchange.Provider == "iCloud Drive" && strings.Contains(ev.Exchange.Folder, "/Library/Mobile Documents/com~apple~CloudDocs/") &&
		filepath.Base(ev.Exchange.Folder) == ev.RunID, "exchange %s at %s is not the run's iCloud Drive folder", ev.Exchange.Provider, ev.Exchange.Folder)

	fixture, err := os.ReadFile(filepath.Join("..", "compat", "testdata", "workspaces", "representative", ".tt", "workspace.json"))
	if err != nil {
		v.t.Fatal(err)
	}
	match := regexp.MustCompile(`"workspace_id":\s*"([^"]+)"`).FindSubmatch(fixture)
	if match == nil {
		v.t.Fatal("the fixture catalog names no workspace")
	}
	v.check(ev.Workspace.WorkspaceID == string(match[1]), "the trial workspace %s is not the fixture's %s", ev.Workspace.WorkspaceID, match[1])
	for label, dir := range map[string]string{"home": ev.Mac.Home, "original": ev.Mac.Original, "working copy": ev.Mac.DataDirectory} {
		v.check(strings.Contains(dir, string(filepath.Separator)+ev.RunID+string(filepath.Separator)), "the Mac %s %q is not inside the trial", label, dir)
	}
}

// verify holds one checkpoint to its rule, recomputed from what both
// devices reported.
func (v *qualificationValidator) verify(c qualificationCheckpoint, rule checkpointRule) {
	ev := v.ev
	key := c.Scenario + "/" + c.Label
	v.check(len(c.Problems) == 0, "%s was recorded as failed: %v", key, c.Problems)
	at := v.parse(key, c.At)
	v.check(!at.Before(v.started) && !at.After(v.finished), "%s at %s is outside the trial", key, c.At)
	p := c.IPhone
	if p == nil {
		v.fail(key + ": no iPhone diagnostics")
		return
	}
	v.check(p.WorkspaceID == ev.Workspace.WorkspaceID && c.Mac.WorkspaceID == ev.Workspace.WorkspaceID,
		"%s: workspaces Mac %s iPhone %s", key, c.Mac.WorkspaceID, p.WorkspaceID)
	v.check(c.Mac.DeviceID == ev.Mac.DeviceID && p.DeviceID != "" && p.DeviceID != c.Mac.DeviceID,
		"%s: devices Mac %s iPhone %s", key, c.Mac.DeviceID, p.DeviceID)
	exported := v.parse(key+" iPhone export", p.ExportedAt)
	v.check(!exported.After(at) && at.Sub(exported) < 2*time.Hour, "%s: the iPhone export %s does not belong to this checkpoint", key, p.ExportedAt)
	if !rule.anyBuild {
		want := v.installAt(exported)
		v.check(want != nil && maps.Equal(p.AppExecutables, want), "%s: the iPhone ran %v, the trial's install at that time was %v", key, p.AppExecutables, want)
	}
	for _, tag := range rule.present {
		n := c.Markers[v.marker(c.Scenario, tag)]
		v.check(n == 1, "%s: the Mac holds %d copies of %q, want 1", key, n, tag)
	}
	for _, tag := range rule.absent {
		n, recorded := c.Markers[v.marker(c.Scenario, tag)]
		v.check(recorded && n == 0, "%s: %q is on the Mac (%d) or was not counted", key, tag, n)
	}
	if rule.converged {
		v.check(strings.HasPrefix(c.Mac.Digest, "sha256:") && c.Mac.Digest == p.Digest, "%s: the devices differ: Mac %s iPhone %s", key, c.Mac.Digest, p.Digest)
		v.check(c.Mac.OutstandingUp == 0 && p.OutstandingUp == 0, "%s: unpublished work Mac %d iPhone %d", key, c.Mac.OutstandingUp, p.OutstandingUp)
		v.check(c.Mac.Held == 0, "%s: the Mac holds %d transactions for an update", key, c.Mac.Held)
	}
	switch rule.conflicts {
	case "none":
		v.check(c.Mac.Conflicts == 0 && p.Conflicts == 0, "%s: open conflicts Mac %d iPhone %d", key, c.Mac.Conflicts, p.Conflicts)
	case "both":
		v.check(c.Mac.Conflicts > 0 && p.Conflicts > 0, "%s: the conflict is not preserved on both: Mac %d iPhone %d", key, c.Mac.Conflicts, p.Conflicts)
	}
	if rule.isolated {
		// iOS writes the exchange files into the local iCloud Drive copy
		// even offline, so unpublished work proves nothing. Isolation is
		// what neither side could get: the Mac, having exchanged, lacks the
		// iPhone's markers (the absent rule), and the iPhone had not applied
		// the Mac's latest transaction.
		got := p.Received[c.Mac.DeviceID]
		v.check(c.Mac.LocalTransactions > 0 && got < c.Mac.LocalTransactions,
			"%s: the iPhone applied the Mac's transaction %d of %d, so it was not cut off", key, got, c.Mac.LocalTransactions)
	}
	if rule.unreachable {
		reachable := p.Transport.Condition != nil && *p.Transport.Condition == "reachable"
		v.check(!reachable || (p.Transport.LastError != nil && *p.Transport.LastError != ""), "%s: the iPhone still reached the exchange", key)
	}
}

// scenarios holds every scenario's checkpoints and what each one adds.
func (v *qualificationValidator) scenarios() {
	for _, scenario := range slices.Sorted(maps.Keys(qualificationRules)) {
		record, ok := v.ev.Scenarios[scenario]
		if !ok {
			v.fail("scenario " + scenario + " was not recorded")
			continue
		}
		var labels []string
		for _, r := range qualificationRules[scenario] {
			labels = append(labels, r.label)
			c, ok := v.byKey[scenario+"/"+r.label]
			if !ok {
				v.fail(fmt.Sprintf("scenario %s has no %s checkpoint", scenario, r.label))
				continue
			}
			v.verify(c, r.rule)
		}
		v.check(slices.Equal(record.Checkpoints, labels), "scenario %s recorded checkpoints %v, want %v", scenario, record.Checkpoints, labels)
		v.observations(scenario, record.Observations)
	}
}

func (v *qualificationValidator) observations(scenario string, obs map[string]any) {
	number := func(key string) float64 { n, _ := obs[key].(float64); return n }
	switch scenario {
	case "simultaneous_edits":
		c := v.byKey[scenario+"/resolved"]
		phone, mac := v.marker(scenario, "renamed-on-iphone"), v.marker(scenario, "renamed-on-mac")
		a, b := c.Markers[phone], c.Markers[mac]
		v.check(a+b == 1 && (obs["kept_title"] == phone || obs["kept_title"] == mac),
			"the resolution kept %d iPhone and %d Mac titles (%v), want exactly one", a, b, obs["kept_title"])
	case "termination":
		v.check(number("mac_exchanges_killed") >= 1, "no Mac exchange was killed mid-pass")
	case "delayed_delivery":
		v.check(number("online_after_seconds") >= qualificationDelaySeconds, "the iPhone came back after %.0f s, want at least %d", number("online_after_seconds"), qualificationDelaySeconds)
	case "revoked_access":
		v.check(obs["saved_locally_banner"] == true, "the iPhone did not say its work was saved locally")
	case "large_attachments":
		c := v.byKey[scenario+"/delivered"]
		sent, _ := obs["mac_attachment_sha256"].(string)
		var macCopy, phoneCopy bool
		for _, a := range c.Attachments {
			if a.SHA256 == sent && a.Size >= qualificationLargeMacBytes {
				macCopy = true
			} else if a.Size >= qualificationLargeMinBytes {
				phoneCopy = true
			}
		}
		v.check(macCopy && phoneCopy, "large attachments: the Mac's %v and an iPhone file of %d MB, recorded %+v", macCopy, qualificationLargeMinBytes>>20, c.Attachments)
		v.check(obs["opened_on_peer"] == true, "a large attachment did not open on the other device")
	case "upgrade":
		from, _ := obs["from_app_executables"].(map[string]any)
		old := map[string]string{}
		for name, digest := range from {
			old[name], _ = digest.(string)
		}
		before, haveBefore := v.byKey[scenario+"/before"]
		if len(old) == 0 {
			// The earlier build's signed digests were not kept and a rebuild
			// re-signs, so the earlier app is identified by its export, which
			// predates the qualification fields this trial's build reports.
			v.check(haveBefore && before.IPhone != nil && len(before.IPhone.AppExecutables) == 0 && before.IPhone.Digest == "",
				"no earlier build digests, and the before export does not show the earlier app either")
		} else {
			current := false
			for _, install := range v.ev.Builds.Installs {
				current = current || maps.Equal(old, install.AppExecutables)
			}
			v.check(!current, "the upgrade started from %v, which is one of the trial's own installs", old)
			if haveBefore && before.IPhone != nil && len(before.IPhone.AppExecutables) > 0 {
				v.check(maps.Equal(before.IPhone.AppExecutables, old), "before the upgrade the iPhone ran %v, not the earlier build", before.IPhone.AppExecutables)
			}
		}
		fromCommit, _ := obs["from_commit"].(string)
		v.check(regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(fromCommit), "upgrade from commit %q", fromCommit)
	}
}

// dailyUse holds the sustained trial: a week of days, each converged with
// both markers, nothing lost and nothing unexplained.
func (v *qualificationValidator) dailyUse() {
	var days []time.Time
	seen := map[string]bool{}
	for _, day := range v.ev.DailyUse {
		date, err := time.Parse(time.DateOnly, day.Date)
		v.check(err == nil && !seen[day.Date], "daily-use date %q is invalid or repeated", day.Date)
		seen[day.Date] = true
		days = append(days, date)
		v.check(!day.LostEdits, "on %s an edit was lost: %s", day.Date, day.Remarks)
		v.check(!day.UnexplainedDivergence, "on %s the devices diverged unexplained: %s", day.Date, day.Remarks)
		c, ok := v.byKey["daily_use/"+day.Date]
		if !ok {
			v.fail("no checkpoint for daily use on " + day.Date)
			continue
		}
		v.verify(c, checkpointRule{converged: true, conflicts: "none"})
		for _, side := range []string{"mac", "iphone"} {
			title := "q18 " + v.ev.RunID + " day " + day.Date + " " + side
			v.check(c.Markers[title] == 1, "daily use %s: the Mac holds %d copies of the %s marker", day.Date, c.Markers[title], side)
		}
	}
	slices.SortFunc(days, func(a, b time.Time) int { return a.Compare(b) })
	v.check(len(days) >= qualificationMinDays && days[len(days)-1].Sub(days[0]) >= (qualificationMinDays-1)*24*time.Hour,
		"daily use covers %d days, want at least %d spanning a week", len(days), qualificationMinDays)
}

// procedures holds every rollout procedure, validated against the
// instructions as they read now.
func (v *qualificationValidator) procedures() {
	for _, name := range slices.Sorted(maps.Keys(qualificationProcedures)) {
		heading := qualificationProcedures[name]
		p, ok := v.ev.Procedures[name]
		if !ok {
			v.fail("procedure " + name + " was not validated")
			continue
		}
		v.check(p.Document == "docs/mobile/personal-rollout.md" && p.Section == heading, "procedure %s validated %s %s", name, p.Document, p.Section)
		v.check(p.SectionSHA256 == rolloutSectionDigest(v.t, heading), "procedure %s: %q changed since it was validated", name, heading)
		v.check(p.FollowedAsWritten, "procedure %s was not followed as written: %s", name, p.Remarks)
		v.check(len(p.Checks) > 0, "procedure %s recorded no checks", name)
		for key, value := range p.Checks {
			if ok, isBool := value.(bool); isBool {
				v.check(ok, "procedure %s: check %s failed", name, key)
			}
		}
	}
}
