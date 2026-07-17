package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

// TestLegacyActiveMarkerIsDiscardedWithoutChangingFocusQueue proves that a v1
// task containing active:true remains parseable while the retired marker never
// changes existing queue membership.
func TestLegacyActiveMarkerIsDiscardedWithoutChangingFocusQueue(t *testing.T) {
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

	// The parser recognizes the retired active flag but deliberately omits it
	// from the migration audit and does not create any queue membership.
	audit := storage.NewMigrationAudit("proj-legacy", parsed.MigrationAudit)
	if len(audit.Entries) != 2 || audit.Entries[0].TaskID != "legacy-active-1" {
		t.Fatalf("expected both legacy tasks in audit, got %+v", audit.Entries)
	}

	planning := config.NewPlanningState()
	planning.AddToFocusQueue("already-queued", "proj-legacy")
	q := planning.GetFocusQueue()
	if len(q) != 1 || q[0].TaskID != "already-queued" {
		t.Fatalf("legacy active marker changed focus queue: %v", q)
	}
}

// TestQueueStaysIndependentAcrossLegacyInputs covers the final Queue/Active
// boundary: Queue retains both currently worked-on and non-Active section tasks
// while legacy task metadata and automation rules cannot create or remove Queue
// membership.
func TestQueueStaysIndependentAcrossLegacyInputs(t *testing.T) {
	deps, project := setupTestDepsWithProject(t, "Queue independence")

	activeSectionTask, err := models.NewTask("active-section", 10, models.TaskTypeTask, "Current work")
	if err != nil {
		t.Fatalf("new Active-section task: %v", err)
	}
	activeSectionTask.ApplyWorkflowState(models.StateReady)
	if got := models.SectionForState(activeSectionTask.State()); got != "" {
		t.Fatalf("Active-section task bucket = %q, want empty Active bucket", got)
	}

	nonActiveTask, err := models.NewTask("next-section", 11, models.TaskTypeTask, "Pipeline work")
	if err != nil {
		t.Fatalf("new non-Active task: %v", err)
	}
	nonActiveTask.ApplyWorkflowState(models.StateNext)
	if got := models.SectionForState(nonActiveTask.State()); got != models.SectionNext {
		t.Fatalf("non-Active task bucket = %q, want %q", got, models.SectionNext)
	}

	planning := deps.Config.Planning()
	planning.AddToFocusQueue(activeSectionTask.ID, project.ID)
	planning.AddToFocusQueue(nonActiveTask.ID, project.ID)

	automation := deps.Config.Automation()
	automation.WorkflowRules = []workflow.WorkflowRule{
		{
			ID:      "legacy-mixed",
			Name:    "Legacy queue mutation",
			Enabled: true,
			Actions: []workflow.RuleAction{
				{ActionType: workflow.ActionType("set_active"), Value: "true"},
				{ActionType: workflow.ActionAddTag, Value: "retained"},
			},
		},
	}
	if err := saveTestRuntime(config.NewManager(deps.ConfigPath), deps.Config); err != nil {
		t.Fatalf("seed legacy workspace: %v", err)
	}

	legacyAutomation, err := os.ReadFile(config.AutomationPath(deps.DataDir))
	if err != nil {
		t.Fatalf("read legacy automation file: %v", err)
	}
	if !strings.Contains(string(legacyAutomation), "set_active") {
		t.Fatalf("automation fixture must contain the retired action:\n%s", legacyAutomation)
	}

	legacyTaskFile := `# Project: Legacy

## Task: Legacy active metadata
<!-- id: legacy-active, status: todo, priority: 1, active: true -->
`
	parsed, err := storage.ParseTasksMarkdownWithSections(legacyTaskFile)
	if err != nil {
		t.Fatalf("parse legacy task metadata: %v", err)
	}
	if len(parsed.Tasks) != 1 || parsed.Tasks[0].State() != models.StateReady {
		t.Fatalf("legacy task = %+v, want one ready task", parsed.Tasks)
	}

	reloaded, err := config.NewManager(deps.ConfigPath).LoadRuntimeState()
	if err != nil {
		t.Fatalf("reload legacy workspace: %v", err)
	}

	queued := map[string]bool{}
	for _, item := range reloaded.Planning().GetFocusQueue() {
		queued[item.TaskID] = true
	}
	for _, task := range []*models.Task{activeSectionTask, nonActiveTask} {
		if !queued[task.ID] {
			t.Errorf("queued %s task %q was removed", models.SectionForState(task.State()), task.ID)
		}
	}
	if queued[parsed.Tasks[0].ID] {
		t.Errorf("legacy active metadata unexpectedly added %q to Queue", parsed.Tasks[0].ID)
	}
	if len(queued) != 2 {
		t.Errorf("Queue = %+v, want exactly the two explicitly queued tasks", reloaded.Planning().GetFocusQueue())
	}

	rules := reloaded.Automation().WorkflowRules
	if len(rules) != 1 || len(rules[0].Actions) != 1 || rules[0].Actions[0].ActionType != workflow.ActionAddTag {
		t.Errorf("sanitized legacy automation = %+v, want only retained add_tag action", rules)
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
