package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/storage/format"
)

const (
	legacyV4MigrationFixtureRoot       = "testdata/v4_migration"
	legacyV4ArchiveReintegrationGolden = "testdata/v4_migration/expected-archive-reintegration-report.json"
)

func convertLegacyV4Fixture(t *testing.T, rel string) format.V5TaskConversionResult {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(legacyV4MigrationFixtureRoot, rel))
	if err != nil {
		t.Fatalf("read legacy v4 task fixture %s: %v", rel, err)
	}
	location, err := time.LoadLocation("America/Mexico_City")
	if err != nil {
		t.Fatalf("load migration location: %v", err)
	}
	document, err := format.DecodeLegacyV4TaskDocument(string(content), location)
	if err != nil {
		t.Fatalf("decode legacy v4 task fixture %s: %v", rel, err)
	}
	converted, err := format.ConvertLegacyV4TaskDocument(document, location)
	if err != nil {
		t.Fatalf("convert legacy v4 task fixture %s: %v", rel, err)
	}
	return converted
}

func legacyV4FixtureReintegrationInput(t *testing.T) V5TaskReintegrationInput {
	t.Helper()
	alphaDir := filepath.Join(legacyV4MigrationFixtureRoot, "workspace/projects/alpha")
	betaDir := filepath.Join(legacyV4MigrationFixtureRoot, "workspace/projects/beta")
	return V5TaskReintegrationInput{
		Projects: []V5ProjectTaskReintegrationInput{
			{
				ProjectID: "project-alpha",
				Active:    convertLegacyV4Fixture(t, "workspace/projects/alpha/tasks.md"),
				Archives: []V5ArchiveTaskSource{
					{
						SourceID:   "alpha/completed.md",
						Path:       filepath.Join(alphaDir, archiveFileName),
						Conversion: convertLegacyV4Fixture(t, "workspace/projects/alpha/completed.md"),
					},
				},
				ArchiveIndexPath: filepath.Join(alphaDir, archiveIndexFileName),
			},
			{
				ProjectID: "project-beta",
				Active:    convertLegacyV4Fixture(t, "workspace/projects/beta/tasks.md"),
				Archives: []V5ArchiveTaskSource{
					{
						SourceID:   "beta/completed.md",
						Path:       filepath.Join(betaDir, archiveFileName),
						Conversion: convertLegacyV4Fixture(t, "workspace/projects/beta/completed.md"),
					},
				},
			},
		},
	}
}

func v5TasksByID(tasks []format.V5TaskRecord) map[string]format.V5TaskRecord {
	result := make(map[string]format.V5TaskRecord, len(tasks))
	for _, task := range tasks {
		result[task.ID] = task
	}
	return result
}

func TestPlanV5ArchiveReintegrationMixedFixtureGolden(t *testing.T) {
	input := legacyV4FixtureReintegrationInput(t)
	originalOrphanParent := input.Projects[0].Active.Tasks[9].ParentID

	plan, err := PlanV5ArchiveReintegration(input)
	if err != nil {
		t.Fatalf("plan archive reintegration: %v", err)
	}
	if len(plan.Projects) != 2 {
		t.Fatalf("projects = %d, want 2", len(plan.Projects))
	}
	if len(plan.Projects[0].Tasks) != 12 || len(plan.Projects[1].Tasks) != 3 {
		t.Fatalf("reintegrated task counts = alpha:%d beta:%d, want alpha:12 beta:3",
			len(plan.Projects[0].Tasks), len(plan.Projects[1].Tasks))
	}

	alpha := v5TasksByID(plan.Projects[0].Tasks)
	beta := v5TasksByID(plan.Projects[1].Tasks)
	if task := alpha["synthetic-duplicate-active"]; task.Title != "Active duplicate wins" || task.ShortID != 9 {
		t.Fatalf("active duplicate winner = %+v", task)
	}
	if task := alpha["synthetic-alpha-archived"]; task.ShortID != 12 || task.State != "done" {
		t.Fatalf("alpha archived task = %+v, want reintegrated terminal task #12", task)
	}
	if task := beta["synthetic-beta-archived"]; task.ShortID != 3 || task.State != "canceled" {
		t.Fatalf("beta archived task = %+v, want reintegrated terminal task #3", task)
	}
	orphan := alpha["synthetic-alpha-orphan"]
	if orphan.ParentID != "" {
		t.Fatalf("orphan parent = %q, want root", orphan.ParentID)
	}
	if !slices.Equal(orphan.DependsOn, []string{"synthetic-duplicate-active", "project-beta:synthetic-beta-root"}) {
		t.Fatalf("orphan dependencies = %v, want active winner and cross-project target", orphan.DependsOn)
	}
	if !slices.Equal(beta["synthetic-beta-root"].DependsOn, []string{"project-alpha:synthetic-alpha-review"}) {
		t.Fatalf("cross-project dependency changed: %v", beta["synthetic-beta-root"].DependsOn)
	}

	wantCleanup := []string{
		filepath.Join(legacyV4MigrationFixtureRoot, "workspace/projects/alpha", archiveIndexFileName),
		filepath.Join(legacyV4MigrationFixtureRoot, "workspace/projects/alpha", archiveFileName),
		filepath.Join(legacyV4MigrationFixtureRoot, "workspace/projects/beta", archiveFileName),
	}
	if len(plan.Cleanup) != len(wantCleanup) {
		t.Fatalf("cleanup artifacts = %d, want %d", len(plan.Cleanup), len(wantCleanup))
	}
	for index, artifact := range plan.Cleanup {
		if !artifact.Delete || artifact.Path != wantCleanup[index] {
			t.Errorf("cleanup[%d] = %+v, want delete %q", index, artifact, wantCleanup[index])
		}
	}

	gotReport, err := json.MarshalIndent(plan.Report, "", "  ")
	if err != nil {
		t.Fatalf("marshal reintegration report: %v", err)
	}
	gotReport = append(gotReport, '\n')
	wantReport, err := os.ReadFile(legacyV4ArchiveReintegrationGolden)
	if err != nil {
		t.Fatalf("read reintegration report golden: %v\ncandidate golden:\n%s", err, gotReport)
	}
	if string(gotReport) != string(wantReport) {
		t.Fatalf("reintegration report differs from golden\n--- got ---\n%s\n--- want ---\n%s", gotReport, wantReport)
	}

	// Planning is pure: output-only ShortIDs and repairs must not leak into the
	// already validated conversion graph.
	if input.Projects[0].Active.Tasks[0].ShortID != 0 ||
		input.Projects[0].Archives[0].Conversion.Tasks[1].ShortID != 0 ||
		input.Projects[0].Active.Tasks[9].ParentID != originalOrphanParent {
		t.Fatal("reintegration planner mutated its conversion input")
	}
}

func TestPlanV5ArchiveReintegrationIsIndependentOfArchiveInputOrder(t *testing.T) {
	active := format.V5TaskConversionResult{
		ProjectName: "Ordering",
		Tasks:       []format.V5TaskRecord{{ID: "active", State: "ready"}},
	}
	archives := []V5ArchiveTaskSource{
		{
			SourceID: "z/completed.2.md",
			Path:     "/workspace/projects/ordering/completed.2.md",
			Conversion: format.V5TaskConversionResult{
				ProjectName: "Ordering",
				Tasks:       []format.V5TaskRecord{{ID: "z-archive", State: "done"}},
			},
		},
		{
			SourceID: "a/completed.1.md",
			Path:     "/workspace/projects/ordering/completed.1.md",
			Conversion: format.V5TaskConversionResult{
				ProjectName: "Ordering",
				Tasks:       []format.V5TaskRecord{{ID: "a-archive", State: "canceled"}},
			},
		},
	}
	input := V5TaskReintegrationInput{Projects: []V5ProjectTaskReintegrationInput{{
		ProjectID: "ordering",
		Active:    active,
		Archives:  append([]V5ArchiveTaskSource(nil), archives...),
	}}}
	forward, err := PlanV5ArchiveReintegration(input)
	if err != nil {
		t.Fatalf("plan forward archives: %v", err)
	}

	slices.Reverse(input.Projects[0].Archives)
	reversed, err := PlanV5ArchiveReintegration(input)
	if err != nil {
		t.Fatalf("plan reversed archives: %v", err)
	}
	if !reflect.DeepEqual(forward, reversed) {
		t.Fatalf("archive input order changed plan\nforward: %+v\nreversed: %+v", forward, reversed)
	}

	gotIDs := []string{
		forward.Projects[0].Tasks[0].ID,
		forward.Projects[0].Tasks[1].ID,
		forward.Projects[0].Tasks[2].ID,
	}
	if !slices.Equal(gotIDs, []string{"active", "a-archive", "z-archive"}) {
		t.Fatalf("deterministic task order = %v", gotIDs)
	}
}

func TestPlanV5ArchiveReintegrationAppliesValidatedStableIDRemap(t *testing.T) {
	input := V5TaskReintegrationInput{
		StableIDRemap: map[string]string{"old-target": "new-target"},
		Projects: []V5ProjectTaskReintegrationInput{
			{
				ProjectID: "alpha",
				Active: format.V5TaskConversionResult{
					ProjectName: "Alpha",
					Tasks:       []format.V5TaskRecord{{ID: "old-target", State: "ready"}},
				},
				Archives: []V5ArchiveTaskSource{{
					SourceID: "alpha/completed.md",
					Path:     "/workspace/projects/alpha/completed.md",
					Conversion: format.V5TaskConversionResult{
						ProjectName: "Alpha",
						Tasks: []format.V5TaskRecord{{
							ID:        "archived-child",
							State:     "done",
							ParentID:  "old-target",
							DependsOn: []string{"old-target"},
						}},
					},
				}},
			},
			{
				ProjectID: "beta",
				Active: format.V5TaskConversionResult{
					ProjectName: "Beta",
					Tasks: []format.V5TaskRecord{{
						ID:        "cross-project-dependent",
						State:     "ready",
						DependsOn: []string{"old-target"},
					}},
				},
			},
		},
	}

	plan, err := PlanV5ArchiveReintegration(input)
	if err != nil {
		t.Fatalf("plan stable-ID remap: %v", err)
	}
	alpha := v5TasksByID(plan.Projects[0].Tasks)
	if _, exists := alpha["old-target"]; exists {
		t.Fatal("old stable ID remained in output")
	}
	if target := alpha["new-target"]; target.ID == "" {
		t.Fatal("remapped target is missing")
	}
	child := alpha["archived-child"]
	if child.ParentID != "new-target" || !slices.Equal(child.DependsOn, []string{"new-target"}) {
		t.Fatalf("remapped child references = parent:%q dependencies:%v", child.ParentID, child.DependsOn)
	}
	if got := plan.Projects[1].Tasks[0].DependsOn; !slices.Equal(got, []string{"alpha:new-target"}) {
		t.Fatalf("cross-project dependency remap = %v", got)
	}
	if input.Projects[0].Active.Tasks[0].ID != "old-target" ||
		input.Projects[0].Archives[0].Conversion.Tasks[0].ParentID != "old-target" {
		t.Fatal("stable-ID remapping mutated its input")
	}
	for _, field := range []string{"stable_id", "parent_id", "depends_on"} {
		if !v5RepairReportHasField(plan.Report, field) {
			t.Errorf("repair report is missing remapped %s entry: %+v", field, plan.Report.Entries)
		}
	}
}

func TestPlanV5ArchiveReintegrationCanonicalizesQualifiedDependencies(t *testing.T) {
	input := V5TaskReintegrationInput{
		StableIDRemap: map[string]string{"old-target": "new-target"},
		Projects: []V5ProjectTaskReintegrationInput{
			{
				ProjectID: "alpha",
				Active: format.V5TaskConversionResult{
					ProjectName: "Alpha",
					Tasks:       []format.V5TaskRecord{{ID: "old-target", State: "ready"}},
				},
			},
			{
				ProjectID: "beta",
				Active: format.V5TaskConversionResult{
					ProjectName: "Beta",
					Tasks: []format.V5TaskRecord{{
						ID:        "dependent",
						State:     "ready",
						DependsOn: []string{"legacy-alpha:old-target", "alpha:new-target"},
					}},
				},
			},
		},
	}

	plan, err := PlanV5ArchiveReintegration(input)
	if err != nil {
		t.Fatalf("PlanV5ArchiveReintegration: %v", err)
	}
	got := plan.Projects[1].Tasks[0].DependsOn
	if !slices.Equal(got, []string{"alpha:new-target"}) {
		t.Fatalf("canonical qualified dependencies = %v, want [alpha:new-target]", got)
	}
	if input.Projects[1].Active.Tasks[0].DependsOn[0] != "legacy-alpha:old-target" {
		t.Fatal("qualified dependency repair mutated its input")
	}
	if !v5RepairReportHasAction(plan.Report, "qualified", "depends_on") ||
		!v5RepairReportHasAction(plan.Report, "deduplicated", "depends_on") {
		t.Fatalf("repair report lacks qualification/deduplication entries: %+v", plan.Report.Entries)
	}
}

func TestPlanV5ArchiveReintegrationRejectsInvalidCandidates(t *testing.T) {
	t.Run("stable ID remap cycle", func(t *testing.T) {
		_, err := PlanV5ArchiveReintegration(V5TaskReintegrationInput{
			StableIDRemap: map[string]string{"a": "b", "b": "a"},
		})
		if err == nil || !strings.Contains(err.Error(), "cycle") {
			t.Fatalf("error = %v, want stable-ID remap cycle", err)
		}
	})

	t.Run("nonterminal archive task", func(t *testing.T) {
		_, err := PlanV5ArchiveReintegration(V5TaskReintegrationInput{
			Projects: []V5ProjectTaskReintegrationInput{{
				ProjectID: "project",
				Active:    format.V5TaskConversionResult{ProjectName: "Project"},
				Archives: []V5ArchiveTaskSource{{
					SourceID: "project/completed.md",
					Path:     "/workspace/projects/project/completed.md",
					Conversion: format.V5TaskConversionResult{
						ProjectName: "Project",
						Tasks:       []format.V5TaskRecord{{ID: "not-terminal", State: "ready"}},
					},
				}},
			}},
		})
		if err == nil || !strings.Contains(err.Error(), "nonterminal") {
			t.Fatalf("error = %v, want nonterminal archive rejection", err)
		}
	})
}

func v5RepairReportHasField(report V5TaskReintegrationReport, field string) bool {
	for _, entry := range report.Entries {
		if entry.Action == "remapped" && entry.Field == field {
			return true
		}
	}
	return false
}

func v5RepairReportHasAction(report V5TaskReintegrationReport, action, field string) bool {
	for _, entry := range report.Entries {
		if entry.Action == action && entry.Field == field {
			return true
		}
	}
	return false
}
