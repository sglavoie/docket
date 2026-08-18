package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func taskContentFixtureApp(t *testing.T, fixture string) *AppModel {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(fixture, 120, 40)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	return driver.App()
}

func taskContentFixtureTask(t *testing.T, m *AppModel, title string) (*models.Task, *models.Project) {
	t.Helper()
	for _, project := range m.projects {
		for _, task := range project.Tasks {
			if task != nil && task.Title == title {
				return task, project
			}
		}
	}
	t.Fatalf("fixture has no task titled %q", title)
	return nil, nil
}

func taskContentFixtureProject(t *testing.T, m *AppModel, name string) *models.Project {
	t.Helper()
	for _, project := range m.projects {
		if project != nil && project.Name == name {
			return project
		}
	}
	t.Fatalf("fixture has no project named %q", name)
	return nil
}

func taskContentBuffer(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tt-edit.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write buffer: %v", err)
	}
	return path
}

// TestExternalEditorTitleAppliesCaptureSigils is the point of the feature: the
// editor's title line is the same grammar the inline row editor speaks, and its
// patch lands with the description as one edit.
func TestExternalEditorTitleAppliesCaptureSigils(t *testing.T) {
	m := taskContentFixtureApp(t, FixtureRich)
	task, project := taskContentFixtureTask(t, m, "Triage support inbox")
	path := taskContentBuffer(t, "buffer")

	handled, cmd := m.handleTaskContentEdited(TaskContentEditedMsg{
		Task:           task,
		Project:        project,
		Path:           path,
		NewTitle:       "Triage support inbox !1 #urgent $next",
		NewDescription: "Sort by age.",
	})
	if !handled || cmd == nil {
		t.Fatalf("handled=%t cmd=%v, want a handled commit", handled, cmd)
	}

	edited := m.projectByID(project.ID).GetTaskByID(task.ID)
	if edited.Title != "Triage support inbox" {
		t.Fatalf("title = %q, want the sigils consumed", edited.Title)
	}
	if edited.Priority != 1 || !edited.HasTag("urgent") {
		t.Fatalf("priority/tags = %d/%v", edited.Priority, edited.Tags)
	}
	if edited.Description != "Sort by age." {
		t.Fatalf("description = %q", edited.Description)
	}
	if !m.nextOrderIDs[TaskIdentity{ProjectID: project.ID, TaskID: task.ID}] {
		t.Error("$next did not add the task to the Next collection")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("committed edit kept the buffer at %s", path)
	}
}

// TestExternalEditorTitleMovesTheSubtreeAcrossProjects covers the structural half
// of the grammar, which only the inline editor and this surface accept.
func TestExternalEditorTitleMovesTheSubtreeAcrossProjects(t *testing.T) {
	m := taskContentFixtureApp(t, FixtureMultiProject)
	task, project := taskContentFixtureTask(t, m, "Alpha migration kickoff")
	destination := taskContentFixtureProject(t, m, "Client Beta")

	m.handleTaskContentEdited(TaskContentEditedMsg{
		Task:           task,
		Project:        project,
		NewTitle:       "Alpha migration kickoff %" + destination.Slug,
		NewDescription: "Moved by the editor.",
	})

	if m.projectByID(project.ID).GetTaskByID(task.ID) != nil {
		t.Error("the task stayed in its source project")
	}
	moved := m.projectByID(destination.ID).GetTaskByID(task.ID)
	if moved == nil {
		t.Fatalf("the task did not land in %s", destination.Slug)
	}
	if moved.Title != "Alpha migration kickoff" || moved.Description != "Moved by the editor." {
		t.Fatalf("moved task = %q / %q", moved.Title, moved.Description)
	}
}

// TestExternalEditorUnchangedTitleIsNeverParsed is the no-op safety rule: a title
// the user did not touch cannot have a literal sigil character eaten out of it.
func TestExternalEditorUnchangedTitleIsNeverParsed(t *testing.T) {
	m := taskContentFixtureApp(t, FixtureRich)
	task, project := taskContentFixtureTask(t, m, "Triage support inbox")
	renamed := "Fix bug #123"
	if _, err := m.appService.EditTaskLine(app.TaskLineEditRequest{
		Source: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:  renamed,
	}); err != nil {
		t.Fatalf("seed literal title: %v", err)
	}
	m.reloadProjectByID(project.ID)
	task = m.projectByID(project.ID).GetTaskByID(task.ID)

	m.handleTaskContentEdited(TaskContentEditedMsg{
		Task:           task,
		Project:        project,
		NewTitle:       renamed,
		NewDescription: "Only the body changed.",
	})

	edited := m.projectByID(project.ID).GetTaskByID(task.ID)
	if edited.Title != renamed {
		t.Fatalf("title = %q, want %q untouched", edited.Title, renamed)
	}
	if len(edited.Tags) != 0 {
		t.Fatalf("tags = %v, want the literal #123 left in the title", edited.Tags)
	}
	if edited.Description != "Only the body changed." {
		t.Fatalf("description = %q", edited.Description)
	}
}

// TestExternalEditorRefusedTitleReopensTheBuffer locks decision 2: a recognized
// but invalid value commits nothing and hands the same buffer back with the
// refusal and the description the user already typed.
func TestExternalEditorRefusedTitleReopensTheBuffer(t *testing.T) {
	m := taskContentFixtureApp(t, FixtureRich)
	task, project := taskContentFixtureTask(t, m, "Triage support inbox")
	path := taskContentBuffer(t, "buffer")

	_, cmd := m.handleTaskContentEdited(TaskContentEditedMsg{
		Task:           task,
		Project:        project,
		Path:           path,
		NewTitle:       "Triage support inbox $invalid",
		NewDescription: "Body survives.",
	})
	if cmd == nil {
		t.Fatal("a refused title produced no follow-up")
	}
	reopen, ok := cmd().(EditTaskContentMsg)
	if !ok {
		t.Fatalf("refusal produced %T, want EditTaskContentMsg", cmd())
	}
	if reopen.Path != path {
		t.Errorf("reopen path = %q, want the same buffer", reopen.Path)
	}
	if reopen.RejectedTitle != "Triage support inbox $invalid" || reopen.Notice == "" {
		t.Errorf("reopen = %q / %q, want the refused title and its reason", reopen.RejectedTitle, reopen.Notice)
	}
	if reopen.Description != "Body survives." {
		t.Errorf("reopen description = %q", reopen.Description)
	}
	if got := m.projectByID(project.ID).GetTaskByID(task.ID); got.Title != "Triage support inbox" || got.Description != "" {
		t.Errorf("a refused save committed %q / %q", got.Title, got.Description)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the reopened buffer was deleted: %v", err)
	}
}

// TestExternalEditorUnfixedRetryAbandons is the loop guard: saving a refused
// title unchanged reports the original refusal instead of reopening forever.
func TestExternalEditorUnfixedRetryAbandons(t *testing.T) {
	m := taskContentFixtureApp(t, FixtureRich)
	task, project := taskContentFixtureTask(t, m, "Triage support inbox")
	path := taskContentBuffer(t, "buffer")

	_, cmd := m.handleTaskContentEdited(TaskContentEditedMsg{
		Task:           task,
		Project:        project,
		Path:           path,
		RejectedTitle:  "Triage support inbox $invalid",
		Notice:         "unknown destination $invalid",
		NewTitle:       "Triage support inbox $invalid",
		NewDescription: "Body survives.",
	})
	if cmd == nil {
		t.Fatal("an abandoned retry produced no status")
	}
	errMsg, ok := cmd().(ErrorMsg)
	if !ok {
		t.Fatalf("abandoned retry produced %T, want ErrorMsg", cmd())
	}
	if !strings.Contains(errMsg.Err.Error(), "unknown destination $invalid") {
		t.Errorf("error = %q, want the original refusal", errMsg.Err)
	}
	if got := m.projectByID(project.ID).GetTaskByID(task.ID); got.Description != "" {
		t.Errorf("an abandoned retry committed the description %q", got.Description)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("an abandoned retry kept the buffer at %s", path)
	}
}

// TestExternalEditorInstructionStaysOneLine guards the buffer format: a second
// comment line would be saved as description, since parseTaskContent consumes
// only the first.
func TestExternalEditorInstructionStaysOneLine(t *testing.T) {
	line := taskContentInstruction("project not found:\n\"nope\"")
	if strings.Contains(line, "\n") {
		t.Fatalf("instruction spans lines: %q", line)
	}
	if !strings.HasPrefix(line, "<!--") || !strings.HasSuffix(line, "-->") {
		t.Fatalf("instruction is not one comment: %q", line)
	}
	if strings.Count(line, "-->") != 1 {
		t.Fatalf("instruction closes more than once: %q", line)
	}
	if strings.Count(taskContentInstruction("bad value -->x"), "-->") != 1 {
		t.Fatalf("a refusal quoting --> closed the comment early")
	}
}
