package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The UX refinement regression map claims that every behavior the plan changes
// already has a place where a regression would show. A map naming a file that
// moved, or a session nobody assigned a home to, is worse than no map: it reads
// as coverage. These gates hold it to both claims. They are named with the
// TestBaseline prefix so scripts/check_docs.sh selects them.

const uxRefinementRegressionMapPath = "docs/tui-testing/ux-refinement-regression-map.md"

// uxRefinementPlannedSessions is the span of the plan this map covers. Session 1
// establishes the baseline, so it owns no later regression home of its own.
const (
	uxRefinementFirstPlannedSession = 2
	uxRefinementLastPlannedSession  = 18
)

// backtickedPathPattern matches the inline-code spans the map uses for artifact
// paths. A path is recognized by its extension rather than by position, so a
// prose mention of `CLAUDE.md` or a symbol like `taskRowSegments` is not
// mistaken for one.
var backtickedPathPattern = regexp.MustCompile("`([^`\n]+)`")

// uxRefinementSessionHeading matches "### Session N — Title".
var uxRefinementSessionHeading = regexp.MustCompile(`(?m)^### Session (\d+) — (.+)$`)

func isRegressionArtifactPath(candidate string) bool {
	switch {
	case strings.HasSuffix(candidate, "_test.go"):
		return true
	case strings.HasPrefix(candidate, "docs/") && strings.HasSuffix(candidate, ".json"):
		return true
	default:
		return false
	}
}

func TestBaselineUXRefinementMapNamesArtifactsThatExist(t *testing.T) {
	root := repositoryRootDir(t)
	content := readRepositoryFile(t, uxRefinementRegressionMapPath)

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
			t.Errorf("%s names regression artifact %q: %v", uxRefinementRegressionMapPath, candidate, err)
		}
	}
	if checked == 0 {
		t.Fatalf("%s names no regression artifacts; the gate is not reaching the map", uxRefinementRegressionMapPath)
	}
}

func TestBaselineUXRefinementMapGivesEverySessionARegressionHome(t *testing.T) {
	content := readRepositoryFile(t, uxRefinementRegressionMapPath)

	headings := uxRefinementSessionHeading.FindAllStringSubmatchIndex(content, -1)
	if len(headings) == 0 {
		t.Fatalf("%s carries no per-session headings", uxRefinementRegressionMapPath)
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
	for session := uxRefinementFirstPlannedSession; session <= uxRefinementLastPlannedSession; session++ {
		if homes[session] == 0 {
			missing = append(missing, fmt.Sprintf("session %d", session))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%s names no regression home for %s", uxRefinementRegressionMapPath, strings.Join(missing, ", "))
	}
}
