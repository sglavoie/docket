package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The daily-use refinement is the third independently numbered plan over the
// same product, and its map is a third file for the reason the second one was a
// second file: eighteen, eight, and thirteen sessions overlap completely, so
// "session 4" is ambiguous the moment two of those numberings share a document,
// and renumbering to make room would silently re-address contracts that are
// already settled.
//
// These gates hold this map to the two claims both earlier gates make — every
// artifact it names exists, and every planned session has at least one — plus
// the claim the clarity gate added, that the recorded expectations later
// sessions replace are named by their owner. The one claim that is new here is
// that *both* earlier maps survive: with three plans in flight, the failure mode
// is no longer one map eroding another but the oldest one quietly going away.
//
// They reuse backtickedPathPattern, uxRefinementSessionHeading, and
// isRegressionArtifactPath from ux_refinement_regression_map_test.go rather than
// restating them; the three maps share a format on purpose. They are named with
// the TestBaseline prefix so scripts/check_docs.sh selects them.

const uxDailyUseRegressionMapPath = "docs/tui-testing/ux-daily-use-regression-map.md"

// uxDailyUseRecordedExpectationsPaths is where the expectations this plan
// replaces are asserted. The map names these tests, and the gate below reads
// the names out of the files rather than out of a list kept here, so a renamed or
// deleted contract fails at the map instead of becoming a second place to
// update. The review matrix is deliberately not scanned: it records evidence
// that every session re-exports, not an expectation any one session replaces.
var uxDailyUseRecordedExpectationsPaths = []string{
	"internal/tui/ux_daily_use_baseline_test.go",
	"internal/tui/ux_daily_use_baseline_capture_test.go",
	"internal/tui/ux_daily_use_baseline_guidance_test.go",
	"internal/tui/ux_daily_use_baseline_tags_test.go",
}

// uxDailyUsePlannedSessions is the span this map covers. Session 1 establishes
// the baseline, the review frames, and the map itself, so it owns no later
// regression home — the same division both earlier plans made.
const (
	uxDailyUseFirstPlannedSession = 2
	uxDailyUseLastPlannedSession  = 13
)

func TestBaselineUXDailyUseMapNamesArtifactsThatExist(t *testing.T) {
	root := repositoryRootDir(t)
	content := readRepositoryFile(t, uxDailyUseRegressionMapPath)

	var checked int
	seen := map[string]bool{}
	for _, match := range backtickedPathPattern.FindAllStringSubmatch(content, -1) {
		candidate := strings.TrimSpace(match[1])
		if !isRegressionArtifactPath(candidate) || seen[candidate] {
			continue
		}
		seen[candidate] = true
		checked++
		if _, err := os.Stat(filepath.Join(root, candidate)); err != nil {
			t.Errorf("%s names regression artifact %q: %v", uxDailyUseRegressionMapPath, candidate, err)
		}
	}
	if checked == 0 {
		t.Fatalf("%s names no regression artifacts; the gate is not reaching the map", uxDailyUseRegressionMapPath)
	}
}

func TestBaselineUXDailyUseMapGivesEverySessionARegressionHome(t *testing.T) {
	content := readRepositoryFile(t, uxDailyUseRegressionMapPath)

	headings := uxRefinementSessionHeading.FindAllStringSubmatchIndex(content, -1)
	if len(headings) == 0 {
		t.Fatalf("%s carries no per-session headings", uxDailyUseRegressionMapPath)
	}

	homes := map[int]int{}
	for i, heading := range headings {
		number, err := strconv.Atoi(content[heading[2]:heading[3]])
		if err != nil {
			t.Fatalf("session heading %q has no number: %v", content[heading[0]:heading[1]], err)
		}
		end := len(content)
		if i+1 < len(headings) {
			end = headings[i+1][0]
		}
		// The section's own artifacts, not the whole document's: a session that
		// only names prose contracts has recorded an intention, not a regression
		// home a failure can land in.
		for _, match := range backtickedPathPattern.FindAllStringSubmatch(content[heading[1]:end], -1) {
			if isRegressionArtifactPath(strings.TrimSpace(match[1])) {
				homes[number]++
			}
		}
	}

	var missing []string
	for session := uxDailyUseFirstPlannedSession; session <= uxDailyUseLastPlannedSession; session++ {
		if homes[session] == 0 {
			missing = append(missing, fmt.Sprintf("session %d", session))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%s names no regression home for %s", uxDailyUseRegressionMapPath, strings.Join(missing, ", "))
	}
}

// TestBaselineUXDailyUseMapPreservesTheEarlierMaps states the separation as a
// claim rather than as a convention. Three plans now number sessions over one
// product, and the only thing keeping them legible is that they are three files
// with three gates. Each earlier map's last session is the part a newer plan
// erodes first, since renumbering starts by dropping whatever the new plan does
// not reach.
func TestBaselineUXDailyUseMapPreservesTheEarlierMaps(t *testing.T) {
	root := repositoryRootDir(t)

	earlier := []struct {
		path string
		last int
	}{
		{uxRefinementRegressionMapPath, uxRefinementLastPlannedSession},
		{uxClarityRegressionMapPath, uxClarityLastPlannedSession},
	}
	for _, m := range earlier {
		if m.path == uxDailyUseRegressionMapPath {
			t.Fatalf("%s is also this plan's map, so one plan's numbering is overwriting the other's", m.path)
		}
		if _, err := os.Stat(filepath.Join(root, m.path)); err != nil {
			t.Fatalf("the earlier map %s is gone: %v", m.path, err)
		}
		content := readRepositoryFile(t, m.path)
		last := fmt.Sprintf("### Session %d — ", m.last)
		if !strings.Contains(content, last) {
			t.Fatalf("%s no longer carries %q, so its numbering was rewritten",
				m.path, strings.TrimSpace(last))
		}
	}

	daily := readRepositoryFile(t, uxDailyUseRegressionMapPath)
	for _, name := range []string{"ux-refinement-regression-map.md", "ux-clarity-regression-map.md"} {
		if !strings.Contains(daily, name) {
			t.Errorf("%s does not say it is separate from %s", uxDailyUseRegressionMapPath, name)
		}
	}
}

// TestBaselineUXDailyUseMapNamesTheExpectationsItReplaces is the claim specific
// to a plan that rewrites behavior other tests already assert. Each recorded
// expectation exists to be replaced by a named session; one nobody claimed would
// be rewritten by whoever hit it first, which is exactly how a deliberate change
// and a regression stop being distinguishable in review.
func TestBaselineUXDailyUseMapNamesTheExpectationsItReplaces(t *testing.T) {
	daily := readRepositoryFile(t, uxDailyUseRegressionMapPath)

	for _, path := range uxDailyUseRecordedExpectationsPaths {
		recorded := readRepositoryFile(t, path)
		var named int
		for _, line := range strings.Split(recorded, "\n") {
			const prefix = "func TestUXDailyUse"
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			name := line[len("func "):]
			if cut := strings.IndexByte(name, '('); cut >= 0 {
				name = name[:cut]
			}
			named++
			if !strings.Contains(daily, name) {
				t.Errorf("%s records %s but %s never names it, so no session owns replacing it",
					path, name, uxDailyUseRegressionMapPath)
			}
		}
		if named == 0 {
			t.Fatalf("%s declares no recorded expectations; the gate is not reaching them",
				path)
		}
	}
}
