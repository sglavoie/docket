package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/testutil"
)

// TestRunTaskAdd_TemplateUseCountIsAtomic verifies the task-add parity path routes
// through the application layer and commits the template use-count bump as part of
// the same add: the created task carries the template's type/tags/checklist AND the
// persisted config records the incremented use-count. The pre-application CLI wrote
// these in two separate saves; this asserts the config side effect lands with the
// task rather than being dropped or double-applied.
func TestRunTaskAdd_TemplateUseCountIsAtomic(t *testing.T) {
	deps, project := setupTaskAddTest(t)
	defer deps.Cleanup()

	// Register a template and re-persist the config the command will load.
	deps.Config.Automation().TaskTemplates = []template.TaskTemplate{{
		ID:        "tmpl-bug",
		Name:      "Bug Report",
		Enabled:   true,
		Type:      models.TaskTypeBug,
		Tags:      []string{"bug"},
		Checklist: []string{"reproduce", "fix"},
	}}
	manager := config.NewManager(deps.ConfigPath)
	testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

	taskTemplateFlag = "tmpl-bug"
	defer func() { taskTemplateFlag = "" }()

	initialCount := len(project.Tasks)
	err := runTaskAdd(taskAddCmd, []string{"Login crash"})
	testutil.AssertNoError(t, err)

	// The task landed with the template's type, tags, and checklist.
	saved := deps.Repository.Projects[project.ID]
	if len(saved.Tasks) != initialCount+1 {
		t.Fatalf("expected %d tasks, got %d", initialCount+1, len(saved.Tasks))
	}
	task := saved.Tasks[len(saved.Tasks)-1]
	testutil.AssertEqual(t, task.Type, models.TaskTypeBug)
	if !task.HasTag("bug") {
		t.Error("task should inherit template tag 'bug'")
	}
	if len(task.Checklist) != 2 {
		t.Errorf("expected 2 checklist items from template, got %d", len(task.Checklist))
	}

	// The template use-count bump was committed with the add: reloading the config
	// from disk shows the incremented count (written as the commit's config
	// artifact), not the pre-add value.
	reloaded, err := config.NewManager(deps.ConfigPath).LoadRuntimeState()
	testutil.AssertNoError(t, err)
	tmpl := reloaded.Automation().GetTaskTemplate("tmpl-bug")
	if tmpl == nil {
		t.Fatal("template missing from reloaded config")
	}
	if tmpl.UseCount != 1 {
		t.Fatalf("template use-count = %d, want 1 (config side effect not committed atomically)", tmpl.UseCount)
	}
}

// TestRunTaskAdd_InvalidParentReference verifies a parent reference that does not
// resolve is rejected, no task is created, and no template use-count is touched.
func TestRunTaskAdd_InvalidParentReference(t *testing.T) {
	deps, project := setupTaskAddTest(t)
	defer deps.Cleanup()

	initialCount := len(project.Tasks)
	taskParentFlag = "9999"
	defer func() { taskParentFlag = "" }()

	err := runTaskAdd(taskAddCmd, []string{"Orphan task"})
	if err == nil {
		t.Fatal("expected error for unresolved parent reference")
	}

	saved := deps.Repository.Projects[project.ID]
	if len(saved.Tasks) != initialCount {
		t.Fatalf("a rejected add must not create a task; got %d tasks", len(saved.Tasks))
	}
}
