package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestTaskLinePriorityTitleRegressionStates(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	t.Run("active P1 and P2 titles keep priority style", func(t *testing.T) {
		for _, priority := range []int{1, 2} {
			task := newTaskTitleRegressionTask(t, fmt.Sprintf("active-p%d", priority), priority, fmt.Sprintf("Active P%d title", priority))
			task.Active = true
			line := TaskLine{
				Task:      task,
				Active:    task.Active,
				MaxTitle:  80,
				ViewWidth: 120,
			}.Render()

			want := GetTitlePriorityStyle(priority).Render(task.Title)
			if !strings.Contains(line, want) {
				t.Fatalf("expected active P%d title to use priority style in:\n%s", priority, ansi.Strip(line))
			}
			if activeTitle := ActiveStyle.Render(task.Title); activeTitle != want && strings.Contains(line, activeTitle) {
				t.Fatalf("active P%d title should not use ActiveStyle override in:\n%s", priority, ansi.Strip(line))
			}
			if !strings.Contains(ansi.Strip(line), "*") {
				t.Fatalf("expected active marker in:\n%s", ansi.Strip(line))
			}
		}
	})

	t.Run("selected rows hide priority title colors", func(t *testing.T) {
		task := newTaskTitleRegressionTask(t, "selected-p1", 1, "Selected priority title")
		line := TaskLine{
			Task:       task,
			IsSelected: true,
			MaxTitle:   80,
			ViewWidth:  120,
		}.Render()

		if !strings.Contains(ansi.Strip(line), task.Title) {
			t.Fatalf("selected row lost title text in:\n%s", ansi.Strip(line))
		}
		if strings.Contains(line, GetTitlePriorityStyle(task.Priority).Render(task.Title)) {
			t.Fatalf("selected row should not expose priority-styled title in:\n%s", ansi.Strip(line))
		}
	})

	t.Run("canceled and dimmed titles stay disabled", func(t *testing.T) {
		canceled := newTaskTitleRegressionTask(t, "canceled-p1", 1, "Canceled priority title")
		canceled.Status = models.StatusCanceled
		canceledLine := TaskLine{Task: canceled, MaxTitle: 80, ViewWidth: 120}.Render()
		if !strings.Contains(canceledLine, CanceledStyle.Render(canceled.Title)) {
			t.Fatalf("expected canceled title style in:\n%s", ansi.Strip(canceledLine))
		}
		if strings.Contains(canceledLine, GetTitlePriorityStyle(canceled.Priority).Render(canceled.Title)) {
			t.Fatalf("canceled title should not expose priority style in:\n%s", ansi.Strip(canceledLine))
		}

		dimmed := newTaskTitleRegressionTask(t, "dimmed-p1", 1, "Dimmed priority title")
		dimmed.Dimmed = true
		dimmedLine := TaskLine{Task: dimmed, MaxTitle: 80, ViewWidth: 120}.Render()
		if !strings.Contains(ansi.Strip(dimmedLine), dimmed.Title) {
			t.Fatalf("dimmed row lost title text in:\n%s", ansi.Strip(dimmedLine))
		}
		if dimmedLine == ansi.Strip(dimmedLine) {
			t.Fatalf("expected dimmed row to remain styled as disabled text:\n%s", ansi.Strip(dimmedLine))
		}
		if strings.Contains(dimmedLine, GetTitlePriorityStyle(dimmed.Priority).Render(dimmed.Title)) {
			t.Fatalf("dimmed title should not expose priority style in:\n%s", ansi.Strip(dimmedLine))
		}
	})

	t.Run("search highlights keep priority base style", func(t *testing.T) {
		task := newTaskTitleRegressionTask(t, "search-p1", 1, "Alpha base")
		line := TaskLine{
			Task:        task,
			MaxTitle:    80,
			ViewWidth:   120,
			SearchQuery: "A",
		}.Render()

		if !strings.Contains(line, HighlightStyle.Render("A")) {
			t.Fatalf("expected highlighted match in:\n%s", ansi.Strip(line))
		}
		if !strings.Contains(line, GetTitlePriorityStyle(task.Priority).Render("l")) {
			t.Fatalf("expected non-highlighted title characters to keep priority style in:\n%s", ansi.Strip(line))
		}
	})
}

func TestLinkedTaskReferenceRowsUsePriorityTitleStyle(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: ""})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{}) })

	task := newTaskTitleRegressionTask(t, "linked-p1", 1, "Linked priority title")
	project, err := models.NewProject("proj-1", "notes-proj", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Tasks = []*models.Task{task}
	note := &models.Note{
		Title:       "Reference note",
		Filename:    "reference.md",
		LinkedTasks: []string{task.ID},
	}
	want := TaskTitleStyleFor(task, TaskTitleStyleOptions{}).Render(task.Title)

	t.Run("sidebar dependency row", func(t *testing.T) {
		line := formatSidebarLinkedTask(sidebarLinkedTask{Task: task}, 80, false, false)
		if !strings.Contains(line, want) {
			t.Fatalf("expected sidebar linked task title to use priority style in:\n%s", ansi.Strip(line))
		}
	})

	t.Run("note detail linked task row", func(t *testing.T) {
		m := &NotesViewModel{
			project:       project,
			filterTags:    make(map[string]bool),
			selectedNotes: make(map[string]bool),
		}
		rendered := m.renderDetailsPane(note, 80, 20)
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected note detail linked task title to use priority style in:\n%s", ansi.Strip(rendered))
		}
	})

	t.Run("notes search preview linked task row", func(t *testing.T) {
		m := &NotesSearchModel{
			project:  project,
			filtered: []notesSearchResult{{note: note}},
		}
		m.SetSize(100, 20)
		rendered := m.renderPreview()
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected notes search preview linked task title to use priority style in:\n%s", ansi.Strip(rendered))
		}
	})
}

func TestMigratedTaskTitleRenderersAvoidActiveSuccessTitleOverrides(t *testing.T) {
	files := []string{
		"shared_render.go",
		"project_tree_view_node.go",
		"outliner_render_rows.go",
		"priority_view_render.go",
		"focus_board_render_helpers.go",
		"kanban_view.go",
		"kanban_detail_pane.go",
		"app_pinned_tasks_render.go",
		"task_quick_switch_render.go",
		"dashboard_panels_info_view.go",
		"metrics_dashboard_render.go",
		"workflow_dry_run_view.go",
		"roadmap_render.go",
		"task_jumper.go",
		"global_search_view_list.go",
		"global_search_view_preview.go",
		"palette_preview_primary.go",
		"project_preview.go",
		"parent_selector.go",
		"dependency_selector.go",
		"dependency_viewer.go",
		"linked_task_selector_view.go",
		"cross_project_linker_view.go",
		"global_task_picker.go",
		"quick_capture_autocomplete.go",
		"recent_activity_preview.go",
		"task_attachments_view.go",
		"task_comments_view.go",
		"task_decisions_view.go",
		"task_checklist_view.go",
		"fullscreen_task_helpers.go",
		"picker_items.go",
		"focus_board_render.go",
		"task_activity.go",
		"inbox_shared.go",
		"note_task_linker.go",
		"task_detail_view.go",
		"palette_view_items.go",
		"palette_preview_secondary.go",
		"sidebar_render_links.go",
		"notes_view_render.go",
		"notes_search_render.go",
	}

	forbidden := []struct {
		name string
		re   *regexp.Regexp
	}{
		{
			name: "ActiveStyle/SuccessStyle rendering title text directly",
			re:   regexp.MustCompile(`(?:ActiveStyle|SuccessStyle)(?:\.[A-Za-z0-9_]+\([^)\n]*\))*\.Render\([^)\n]*(?:\.Title|[tT]itle)\)`),
		},
		{
			name: "title style variable assigned ActiveStyle/SuccessStyle",
			re:   regexp.MustCompile(`\b(?:titleStyle|taskTitleStyle|renderedTitleStyle)\s*:?=\s*(?:ActiveStyle|SuccessStyle)\b`),
		},
		{
			name: "card/config title style set to ActiveStyle/SuccessStyle",
			re:   regexp.MustCompile(`\bTitleStyle:\s*(?:ActiveStyle|SuccessStyle)\b`),
		},
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)

	for _, file := range files {
		path := filepath.Join(dir, file)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", file, err)
		}
		for _, rule := range forbidden {
			if match := rule.re.Find(src); match != nil {
				t.Fatalf("%s in %s: %q", rule.name, file, match)
			}
		}
	}
}

func newTaskTitleRegressionTask(t *testing.T, id string, priority int, title string) *models.Task {
	t.Helper()

	task, err := models.NewTask(id, priority, models.TaskTypeTask, title)
	if err != nil {
		t.Fatalf("NewTask(%q): %v", id, err)
	}
	task.Priority = priority
	return task
}
