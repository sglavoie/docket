package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestGlobalSearchNoteRowRespectsVisualWidth(t *testing.T) {
	project, err := models.NewProject("proj-1", "Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Slug = "漢字wide"

	note := &models.Note{Title: "漢字wide note title that should truncate cleanly"}

	m := &GlobalSearchModel{
		filtered: []filteredSearchResult{{
			result: models.SearchResult{Project: project, Note: note},
		}},
		listWidth: 24,
		cursor:    0,
	}

	rendered := m.renderTaskList(6)
	for line := range strings.SplitSeq(strings.TrimRight(rendered, "\n"), "\n") {
		if lipgloss.Width(ansi.Strip(line)) > m.listWidth {
			t.Fatalf("global search note row exceeds width %d: %q (width=%d)\n\n%s", m.listWidth, ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestTaskJumperViewRespectsVisualWidth(t *testing.T) {
	project, err := models.NewProject("proj-1", "task-jumper-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	task, err := models.NewTask("task-1", 17, models.TaskTypeTask, "修复任务跳转器在窄宽度下的宽字符截断问题")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.WorkflowState = models.StateInProgress
	project.Tasks = []*models.Task{task}

	m := NewTaskJumperModel(project)
	m.SetSize(32, 14)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 32 {
			t.Fatalf("task jumper line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestCrossProjectTaskJumperViewRespectsVisualWidth(t *testing.T) {
	project, err := models.NewProject("proj-1", "task-jumper-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Slug = "漢字wide-project-slug-that-truncates"
	task, err := models.NewTask("task-1", 17, models.TaskTypeTask, "修复跨项目任务跳转器在窄宽度下的宽字符截断问题")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.WorkflowState = models.StateInProgress
	project.Tasks = []*models.Task{task}

	m := NewCrossProjectTaskJumperModel([]*models.Project{project})
	m.SetSize(32, 14)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 32 {
			t.Fatalf("cross-project task jumper line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}

func TestNotesSearchViewRespectsVisualWidth(t *testing.T) {
	project, err := models.NewProject("proj-1", "notes-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	note := &models.Note{Title: "修复笔记搜索列表在窄宽度下的宽字符截断问题", Body: "body"}

	m := &NotesSearchModel{
		project:  project,
		filtered: []notesSearchResult{{note: note}},
	}
	m.SetSize(32, 14)
	rendered := m.View().Content

	for line := range strings.SplitSeq(rendered, "\n") {
		if lipgloss.Width(ansi.Strip(line)) > 32 {
			t.Fatalf("notes search line exceeds width 32: %q (width=%d)\n\n%s", ansi.Strip(line), lipgloss.Width(ansi.Strip(line)), ansi.Strip(rendered))
		}
	}
}
