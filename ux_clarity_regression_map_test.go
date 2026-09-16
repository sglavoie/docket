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

// The UX clarity refinement is a second, independently numbered plan over the
// same product. Its map is a separate file for that reason: merging it into the
// eighteen-session map would make every "session N" in either document
// ambiguous, and rewriting the older numbering to make room would silently
// re-address contracts that are already settled.
//
// These gates hold the clarity map to the same two claims the older gate holds
// the older map to — every artifact it names exists, and every planned session
// has at least one — and add the claim that is specific to this plan: the
// recorded expectations later sessions are supposed to replace are named here,
// so a session that quietly relaxes one instead of rewriting it is visible.
//
// They reuse backtickedPathPattern, uxRefinementSessionHeading, and
// isRegressionArtifactPath from ux_refinement_regression_map_test.go rather than
// restating them; the two maps share a format on purpose. They are named with
// the TestBaseline prefix so scripts/check_docs.sh selects them.

const uxClarityRegressionMapPath = "docs/tui-testing/ux-clarity-regression-map.md"

// uxClarityRecordedExpectationsPath is where the expectations this plan replaces
// are asserted. The map names the tests in it, and the gate below checks the
// naming against the file rather than against a list kept here, so a renamed
// test is a failure at the map instead of a second place to update.
const uxClarityRecordedExpectationsPath = "internal/tui/ux_clarity_baseline_test.go"

// uxClarityPlannedSessions is the span this map covers. Session 1 establishes the
// baseline and the review evidence, so it owns no later regression home of its
// own — the same division the earlier plan made.
const (
	uxClarityFirstPlannedSession = 2
	uxClarityLastPlannedSession  = 8
)

func TestBaselineUXClarityMapNamesArtifactsThatExist(t *testing.T) {
	root := repositoryRootDir(t)
	content := readRepositoryFile(t, uxClarityRegressionMapPath)

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
			t.Errorf("%s names regression artifact %q: %v", uxClarityRegressionMapPath, candidate, err)
		}
	}
	if checked == 0 {
		t.Fatalf("%s names no regression artifacts; the gate is not reaching the map", uxClarityRegressionMapPath)
	}
}

func TestBaselineUXClarityMapGivesEverySessionARegressionHome(t *testing.T) {
	content := readRepositoryFile(t, uxClarityRegressionMapPath)

	headings := uxRefinementSessionHeading.FindAllStringSubmatchIndex(content, -1)
	if len(headings) == 0 {
		t.Fatalf("%s carries no per-session headings", uxClarityRegressionMapPath)
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
		// only names contracts has recorded an intention, not a regression home.
		for _, match := range backtickedPathPattern.FindAllStringSubmatch(content[heading[1]:end], -1) {
			if isRegressionArtifactPath(strings.TrimSpace(match[1])) {
				homes[number]++
			}
		}
	}

	var missing []string
	for session := uxClarityFirstPlannedSession; session <= uxClarityLastPlannedSession; session++ {
		if homes[session] == 0 {
			missing = append(missing, fmt.Sprintf("session %d", session))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%s names no regression home for %s", uxClarityRegressionMapPath, strings.Join(missing, ", "))
	}
}

// TestBaselineUXClarityMapPreservesTheEarlierMap states the separation as a
// claim rather than as a convention. The earlier plan's numbering runs to
// eighteen and this one to eight, so the overlap is total; the only thing
// keeping them legible is that they are two files, each with its own gate.
func TestBaselineUXClarityMapPreservesTheEarlierMap(t *testing.T) {
	root := repositoryRootDir(t)
	if _, err := os.Stat(filepath.Join(root, uxRefinementRegressionMapPath)); err != nil {
		t.Fatalf("the earlier map is gone: %v", err)
	}
	if uxClarityRegressionMapPath == uxRefinementRegressionMapPath {
		t.Fatal("the two plans point at one file, so one plan's numbering is overwriting the other's")
	}

	earlier := readRepositoryFile(t, uxRefinementRegressionMapPath)
	// The earlier map's own last session is the part a second plan is most likely
	// to erode, since this plan's sessions stop at eight and renumbering would
	// start by dropping the tail.
	last := fmt.Sprintf("### Session %d — ", uxRefinementLastPlannedSession)
	if !strings.Contains(earlier, last) {
		t.Fatalf("%s no longer carries %q, so its numbering was rewritten",
			uxRefinementRegressionMapPath, strings.TrimSpace(last))
	}

	clarity := readRepositoryFile(t, uxClarityRegressionMapPath)
	if !strings.Contains(clarity, "ux-refinement-regression-map.md") {
		t.Errorf("%s does not say which map it is separate from", uxClarityRegressionMapPath)
	}
}

// TestBaselineUXClarityMapNamesTheExpectationsItReplaces is the claim specific to
// this plan. Each combined-state contract exists to be rewritten by a named
// session, and a contract nobody claimed would be rewritten by whoever hit it
// first — which is how a deliberate change and a regression become
// indistinguishable. Reading the test names out of the file rather than out of a
// list here keeps the map, not this gate, as the thing that has to stay current.
func TestBaselineUXClarityMapNamesTheExpectationsItReplaces(t *testing.T) {
	recorded := readRepositoryFile(t, uxClarityRecordedExpectationsPath)
	clarity := readRepositoryFile(t, uxClarityRegressionMapPath)

	var named int
	for _, line := range strings.Split(recorded, "\n") {
		const prefix = "func TestUXClarity"
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		name := line[len("func "):]
		if cut := strings.IndexByte(name, '('); cut >= 0 {
			name = name[:cut]
		}
		named++
		if !strings.Contains(clarity, name) {
			t.Errorf("%s records %s but %s never names it, so no session owns replacing it",
				uxClarityRecordedExpectationsPath, name, uxClarityRegressionMapPath)
		}
	}
	if named == 0 {
		t.Fatalf("%s declares no recorded expectations; the gate is not reaching them",
			uxClarityRecordedExpectationsPath)
	}
}
