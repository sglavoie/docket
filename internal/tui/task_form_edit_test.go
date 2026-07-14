package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestTaskForm_EditMode_LoadsBugAttrs(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg := newTestConfig()
	if len(project.Tasks) == 0 {
		t.Skip("test project has no tasks")
	}
	task := project.Tasks[0]
	task.Type = models.TaskTypeBug
	task.BugAttrs = &models.BugAttrs{
		Severity:     models.BugSeverityMajor,
		Reproducible: models.BugReproducibleYes,
		Workaround:   "some workaround",
	}

	m := NewTaskFormModelEdit(project, task, testRuntimeAt(cfg.DataDir(), cfg))
	if m.draft.bugAttrs.Severity != models.BugSeverityMajor {
		t.Errorf("expected Severity=major, got %v", m.draft.bugAttrs.Severity)
	}
	if m.draft.bugAttrs.Reproducible != models.BugReproducibleYes {
		t.Errorf("expected Reproducible=yes, got %v", m.draft.bugAttrs.Reproducible)
	}
	if m.draft.bugAttrs.Workaround != "some workaround" {
		t.Errorf("expected Workaround='some workaround', got %q", m.draft.bugAttrs.Workaround)
	}
}

func TestTaskFormEdit_NonGTDSectionIgnoredInDraft(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg := newTestConfig()
	if len(project.Tasks) == 0 {
		t.Skip("test project has no tasks")
	}
	task := project.Tasks[0]
	task.SetSection("Sprint-1")
	if sec := models.SectionForState(task.State()); sec != "" {
		t.Fatalf("precondition: expected Section=\"\", got %q", sec)
	}

	m := NewTaskFormModelEdit(project, task, testRuntimeAt(cfg.DataDir(), cfg))
	if m.draft.section != "" {
		t.Errorf("expected draft.section empty, got %q", m.draft.section)
	}
}

func TestTaskFormEdit_GTDSection_PreservedInDraft(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg := newTestConfig()
	if len(project.Tasks) == 0 {
		t.Skip("test project has no tasks")
	}
	task := project.Tasks[0]
	task.SetSection("Waiting")

	m := NewTaskFormModelEdit(project, task, testRuntimeAt(cfg.DataDir(), cfg))
	if m.draft.section != "Waiting" {
		t.Errorf("expected draft.section=%q, got %q", "Waiting", m.draft.section)
	}
}

func TestTaskFormEdit_ActiveSection_PreservedInDraft(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg := newTestConfig()
	if len(project.Tasks) == 0 {
		t.Skip("test project has no tasks")
	}
	task := project.Tasks[0]
	task.SetSection("")

	m := NewTaskFormModelEdit(project, task, testRuntimeAt(cfg.DataDir(), cfg))
	if m.draft.section != "" {
		t.Errorf("expected draft.section=\"\" (Active), got %q", m.draft.section)
	}
}

func TestTaskFormEdit_NonGTDSectionIgnoredRoundTrip(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	project.EnsureSection("Sprint-1")
	cfg := newTestConfig()
	if len(project.Tasks) == 0 {
		t.Skip("test project has no tasks")
	}
	task := project.Tasks[0]
	task.SetSection("Sprint-1")

	m := NewTaskFormModelEdit(project, task, testRuntimeAt(cfg.DataDir(), cfg))

	tmpDir := t.TempDir()
	project.Path = tmpDir
	if err := os.WriteFile(filepath.Join(tmpDir, "tasks.md"), []byte("# Test Project\n"), 0o644); err != nil {
		t.Fatalf("failed to write tasks.md: %v", err)
	}

	cmd := m.handleSave()
	if m.err != nil {
		t.Fatalf("handleSave error: %v", m.err)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd from handleSave")
	}

	if sec := models.SectionForState(task.State()); sec != "" {
		t.Errorf("expected task.Section empty after save, got %q", sec)
	}
}

func TestBuildValidParentsExcludesSelfInEditMode(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test")
	cfg := newTestConfig()

	epic := testutil.NewTestTask("Epic", models.TaskTypeEpic)
	story := testutil.NewTestTask("Story", models.TaskTypeStory)
	story.ParentID = &epic.ID
	task := testutil.NewTestTask("Leaf Task", models.TaskTypeTask)
	task.ParentID = &story.ID

	project.Tasks = []*models.Task{epic, story, task}
	project.BuildTaskTree()

	m := NewTaskFormModelEdit(project, story, testRuntimeAt(cfg.DataDir(), cfg))
	parents := m.buildValidParents()

	for _, p := range parents {
		if p.ID == story.ID {
			t.Errorf("buildValidParents should exclude the task being edited (story)")
		}
		if p.ID == task.ID {
			t.Errorf("buildValidParents should exclude descendants of the task being edited (leaf task)")
		}
	}

	found := false
	for _, p := range parents {
		if p.ID == epic.ID {
			found = true
		}
	}
	if !found {
		t.Error("buildValidParents should include valid ancestors (epic)")
	}
}

func TestBuildValidParentsIncludesAllInCreateMode(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test")
	cfg := newTestConfig()

	epic := testutil.NewTestTask("Epic", models.TaskTypeEpic)
	project.Tasks = []*models.Task{epic}
	project.BuildTaskTree()

	m := NewTaskFormModelCreate(project, testRuntimeAt(cfg.DataDir(), cfg), nil)
	parents := m.buildValidParents()

	found := false
	for _, p := range parents {
		if p.ID == epic.ID {
			found = true
		}
	}
	if !found {
		t.Error("buildValidParents in create mode should include all valid parents")
	}
}
