package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// baselineDocumentPath is the consolidation baseline whose slow-moving,
// load-bearing numbers this file gates. The fast-churning counts the document
// used to carry (test functions, test files, packages, wall time) are
// deliberately not asserted: they move on nearly every commit.
const baselineDocumentPath = "docs/baselines/baseline.md"

func readRepositoryFile(t *testing.T, relative string) string {
	t.Helper()

	path := filepath.Join(repositoryRootDir(t), relative)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// coverageFloor is one `.coverage-packages` row: a package import path and the
// percentage floor recorded beside it, kept as the file's own literal text so
// the document and the gate cannot disagree about formatting.
type coverageFloor struct {
	pkg     string
	percent string
}

func parseCoveragePackageFloors(t *testing.T, content string) []coverageFloor {
	t.Helper()

	var floors []coverageFloor
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf(".coverage-packages row %q is not a package/floor pair", line)
		}
		floors = append(floors, coverageFloor{pkg: fields[0], percent: fields[1]})
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan .coverage-packages: %v", err)
	}
	if len(floors) == 0 {
		t.Fatal(".coverage-packages declares no per-package floors")
	}
	return floors
}

// parseMutationCallsiteTotal reads the generated inventory's own header rather
// than recounting its rows. TestMutationCallsiteInventoryBaseline owns the
// derivation; this gate only proves the document quotes it.
func parseMutationCallsiteTotal(t *testing.T, content string) string {
	t.Helper()

	const marker = "# Total callsites:"
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if total, ok := strings.CutPrefix(line, marker); ok {
			return strings.TrimSpace(total)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan mutation callsite baseline: %v", err)
	}
	t.Fatalf("mutation callsite baseline has no %q header", marker)
	return ""
}

func TestBaselineDocumentMatchesMeasuredGates(t *testing.T) {
	document := strings.Join(strings.Fields(readRepositoryFile(t, baselineDocumentPath)), " ")

	scenarios, err := filepath.Glob(filepath.Join(repositoryRootDir(t), "docs", "tui-testing", "scenarios", "*.json"))
	if err != nil {
		t.Fatalf("glob TUI scenarios: %v", err)
	}
	if len(scenarios) == 0 {
		t.Fatal("no committed TUI scenarios found")
	}

	threshold := strings.TrimSpace(readRepositoryFile(t, ".coverage-threshold"))
	floors := parseCoveragePackageFloors(t, readRepositoryFile(t, ".coverage-packages"))
	callsites := parseMutationCallsiteTotal(t,
		readRepositoryFile(t, filepath.Join("internal", "storage", "testdata", "mutation_callsites_baseline.txt")))

	required := []string{
		fmt.Sprintf("| Committed TUI scenarios | %d |", len(scenarios)),
		fmt.Sprintf("| Repository-wide coverage floor | %s%% |", threshold),
		fmt.Sprintf("| Direct storage mutation callsites (cmd/, tui/) | %s |", callsites),
	}
	for _, floor := range floors {
		required = append(required, fmt.Sprintf("| `%s` | %s%% |", floor.pkg, floor.percent))
	}

	for _, needle := range required {
		if !strings.Contains(document, needle) {
			t.Errorf("%s missing measured gate %q", baselineDocumentPath, needle)
		}
	}
}

func TestBaselineDocumentDropsFastChurningCounts(t *testing.T) {
	document := strings.Join(strings.Fields(readRepositoryFile(t, baselineDocumentPath)), " ")
	for _, stale := range []string{
		"| Test functions",
		"| Test files",
		"| Packages",
		"seconds wall time",
		"TestLegacyWorkspaceFixtureParsesLegacyFields` and is pinned",
		"threshold check in `.github/workflows/ci.yml`",
	} {
		if strings.Contains(document, strings.Join(strings.Fields(stale), " ")) {
			t.Errorf("%s still records the churning or incorrect claim %q", baselineDocumentPath, stale)
		}
	}
}

func TestBaselineHistoricalRecordKeepsRemovalContract(t *testing.T) {
	document := strings.Join(strings.Fields(readRepositoryFile(t, "docs/baselines/outliner-v5.md")), " ")
	for _, needle := range []string{
		"## Permanent removal contract (still binding)",
		"mode picker, command palette, Finder, and recent destinations",
		"No universal Finder or workspace-wide search overlay returns",
		"Search autocomplete stays Outliner-only",
		"Project Hub aliases never include task titles, task descriptions, task tags, " +
			"task comments, or task content",
	} {
		if !strings.Contains(document, strings.Join(strings.Fields(needle), " ")) {
			t.Errorf("outliner-v5 historical record missing %q", needle)
		}
	}
}
