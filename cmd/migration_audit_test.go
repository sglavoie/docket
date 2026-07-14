package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestActiveMarkerRoundTripsIntoFocusQueue proves the migration prerequisite: a
// v1 task parsed with active:true is surfaced by the loader-level migration
// audit and, at the config-aware boundary, added to the Focus Queue — so the
// active marker is not silently lost once the v2 writer stops persisting it.
func TestActiveMarkerRoundTripsIntoFocusQueue(t *testing.T) {
	content := `# Project: Legacy

## Task: Flagged active task
<!-- id: legacy-active-1, status: todo, priority: 1, active: true -->

Marked active under the deprecated focus flag.

## Task: Plain task
<!-- id: plain-1, status: todo, priority: 2 -->
`

	parsed, err := storage.ParseTasksMarkdownWithSections(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// The parser captures the retired active flag into the migration audit even
	// though the Task field is gone.
	audit := storage.NewMigrationAudit("proj-legacy", parsed.MigrationAudit)
	if ids := audit.ActiveTaskIDs(); len(ids) != 1 || ids[0] != "legacy-active-1" {
		t.Fatalf("expected audit to surface [legacy-active-1], got %v", ids)
	}

	planning := config.NewPlanningState()
	added := MigrateLegacyActiveToFocusQueue(planning, audit)
	if added != 1 {
		t.Fatalf("expected 1 task migrated, got %d", added)
	}

	q := planning.GetFocusQueue()
	if len(q) != 1 || q[0].TaskID != "legacy-active-1" || q[0].ProjectID != "proj-legacy" {
		t.Fatalf("expected legacy-active-1 in focus queue for proj-legacy, got %v", q)
	}

	// Re-running the migration is idempotent (already-present membership).
	if again := MigrateLegacyActiveToFocusQueue(planning, audit); again != 0 {
		t.Errorf("expected re-migration to add nothing, got %d", again)
	}
}

// TestLoadProjectWithMigrationAudit_FailsClosedOnLoadError verifies the
// config-aware load caller propagates a load error and produces no audit.
func TestLoadProjectWithMigrationAudit_FailsClosedOnLoadError(t *testing.T) {
	// A project whose path does not exist loads an empty task set (os.IsNotExist
	// is treated as an empty project), so the audit is empty, not an error.
	project := &models.Project{ID: "empty", Path: t.TempDir() + "/does-not-exist"}
	audit, err := LoadProjectWithMigrationAudit(project)
	if err != nil {
		t.Fatalf("unexpected error for missing tasks file: %v", err)
	}
	if audit == nil || len(audit.Entries) != 0 {
		t.Fatalf("expected empty audit, got %+v", audit)
	}
}
