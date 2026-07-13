package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// legacyFixtureFile is the sanitized, hand-authored pre-migration workspace used
// as a stable input baseline for the WorkflowState migration sessions. It must
// never contain live task content; see testdata/legacy_workspace/README.md.
const legacyFixtureFile = "testdata/legacy_workspace/project-alpha.tasks.md"

// TestLegacyWorkspaceFixtureParsesLegacyFields pins the legacy schema signals
// (status, section, stage, active) that later sessions migrate to a single
// WorkflowState. If the legacy parser stops round-tripping these fields, the
// migration mapping loses its documented inputs and this baseline fails loudly.
func TestLegacyWorkspaceFixtureParsesLegacyFields(t *testing.T) {
	content, err := os.ReadFile(legacyFixtureFile)
	if err != nil {
		t.Fatalf("read legacy fixture: %v", err)
	}

	result, err := ParseTasksMarkdownWithSections(string(content))
	if err != nil {
		t.Fatalf("parse legacy fixture: %v", err)
	}
	tasks := result.Tasks
	if result.ProjectName != "Alpha Sandbox" {
		t.Fatalf("project name = %q, want %q", result.ProjectName, "Alpha Sandbox")
	}

	byID := make(map[string]*models.Task, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}

	// The retired legacy lifecycle fields (status/section/stage/active) are no
	// longer stored on the task: the parser derives a single canonical
	// WorkflowState and captures the raw legacy values in the migration audit.
	auditByID := make(map[string]MigrationAuditEntry, len(result.MigrationAudit))
	for _, entry := range result.MigrationAudit {
		auditByID[entry.TaskID] = entry
	}

	// Each expectation corresponds to one branch of the legacy -> WorkflowState
	// migration precedence documented in the fixture README: state is the derived
	// canonical WorkflowState, and status/section/stage/active are the raw legacy
	// signals captured in the migration audit.
	cases := []struct {
		id      string
		state   models.WorkflowState
		status  models.TaskStatus
		section string
		stage   string
		active  bool
	}{
		{"legacy-cancel-1", models.StateCanceled, models.StatusCanceled, "", "", false},
		{"legacy-done-1", models.StateDone, models.StatusDone, "", "", false},
		{"legacy-done-2", models.StateDone, models.StatusTodo, models.SectionCompleted, "", false},
		{"legacy-review-1", models.StateReview, models.StatusReview, "", "", false},
		{"legacy-inprogress-1", models.StateInProgress, models.StatusInProgress, "", "", false},
		{"legacy-blocked-1", models.StateWaiting, models.StatusBlocked, "", "", false},
		{"legacy-waiting-1", models.StateWaiting, models.StatusTodo, models.SectionWaiting, "", false},
		{"legacy-standby-1", models.StatePaused, models.StatusStandby, models.SectionStandby, "", false},
		{"legacy-maybe-1", models.StateSomeday, models.StatusMaybe, models.SectionSomeday, "", false},
		{"legacy-inbox-1", models.StateInbox, models.StatusTodo, models.SectionInbox, "", false},
		{"legacy-next-1", models.StateNext, models.StatusTodo, models.SectionNext, "", false},
		{"legacy-active-1", models.StateReady, models.StatusTodo, "", "", true},
		{"legacy-stage-1", models.StateReady, models.StatusTodo, "", "Brainstorm", false},
	}

	if len(cases) != len(tasks) {
		t.Fatalf("fixture has %d tasks but %d expectations; keep them in sync", len(tasks), len(cases))
	}

	for _, tc := range cases {
		task, ok := byID[tc.id]
		if !ok {
			t.Errorf("fixture missing task %q", tc.id)
			continue
		}
		if task.State() != tc.state {
			t.Errorf("%s: state = %q, want %q", tc.id, task.State(), tc.state)
		}

		entry, ok := auditByID[tc.id]
		if !ok {
			t.Errorf("%s: missing migration audit entry", tc.id)
			continue
		}
		if entry.Status != tc.status {
			t.Errorf("%s: audit status = %q, want %q", tc.id, entry.Status, tc.status)
		}
		if entry.Section != tc.section {
			t.Errorf("%s: audit section = %q, want %q", tc.id, entry.Section, tc.section)
		}
		if entry.Stage != tc.stage {
			t.Errorf("%s: audit stage = %q, want %q", tc.id, entry.Stage, tc.stage)
		}
		if entry.Active != tc.active {
			t.Errorf("%s: audit active = %v, want %v", tc.id, entry.Active, tc.active)
		}
	}
}

// TestLegacyWorkspaceFixtureIsSanitized guards the "never copy live task
// content" rule: the fixture path stays inside testdata and every task ID uses
// the synthetic "legacy-" prefix, so a future edit cannot quietly paste real
// workspace data into the repository.
func TestLegacyWorkspaceFixtureIsSanitized(t *testing.T) {
	if filepath.Base(filepath.Dir(legacyFixtureFile)) != "legacy_workspace" {
		t.Fatalf("legacy fixture must live under testdata/legacy_workspace, got %q", legacyFixtureFile)
	}

	content, err := os.ReadFile(legacyFixtureFile)
	if err != nil {
		t.Fatalf("read legacy fixture: %v", err)
	}
	_, tasks, err := ParseTasksMarkdown(string(content))
	if err != nil {
		t.Fatalf("parse legacy fixture: %v", err)
	}
	for _, task := range tasks {
		if len(task.ID) < len("legacy-") || task.ID[:len("legacy-")] != "legacy-" {
			t.Errorf("task ID %q is not a synthetic legacy- fixture id", task.ID)
		}
	}
}
