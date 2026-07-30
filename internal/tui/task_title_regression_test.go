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

	t.Run("queued P1 and P2 titles keep priority style", func(t *testing.T) {
		for _, priority := range []int{1, 2} {
			task := newTaskTitleRegressionTask(t, fmt.Sprintf("queued-p%d", priority), priority, fmt.Sprintf("Queued P%d title", priority))
			line := TaskLine{
				Task:      task,
				MaxTitle:  80,
				ViewWidth: 120,
			}.Render()

			want := GetTitlePriorityStyle(priority).Render(task.Title)
			if !strings.Contains(line, want) {
				t.Fatalf("expected queued P%d title to use priority style in:\n%s", priority, ansi.Strip(line))
			}
			if activeTitle := ActiveStyle.Render(task.Title); activeTitle != want && strings.Contains(line, activeTitle) {
				t.Fatalf("queued P%d title should not use ActiveStyle override in:\n%s", priority, ansi.Strip(line))
			}
			if strings.Contains(ansi.Strip(line), "*") {
				t.Fatalf("queued task should not render an active marker in:\n%s", ansi.Strip(line))
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

	t.Run("canceled titles stay disabled", func(t *testing.T) {
		canceled := newTaskTitleRegressionTask(t, "canceled-p1", 1, "Canceled priority title")
		canceled.WorkflowState = models.StateCanceled
		canceledLine := TaskLine{Task: canceled, MaxTitle: 80, ViewWidth: 120}.Render()
		if !strings.Contains(canceledLine, CanceledStyle.Render(canceled.Title)) {
			t.Fatalf("expected canceled title style in:\n%s", ansi.Strip(canceledLine))
		}
		if strings.Contains(canceledLine, GetTitlePriorityStyle(canceled.Priority).Render(canceled.Title)) {
			t.Fatalf("canceled title should not expose priority style in:\n%s", ansi.Strip(canceledLine))
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

func TestMigratedTaskTitleRenderersAvoidActiveSuccessTitleOverrides(t *testing.T) {
	files := []string{
		"shared_render.go",
		"outliner_render_rows.go",
		"app_pinned_tasks_render.go",
		"parent_selector.go",
		"dependency_selector.go",
		"dependency_viewer.go",
		"cross_project_linker_view.go",
		"quick_capture_autocomplete.go",
		"task_attachments_view.go",
		"task_comments_view.go",
		"task_decisions_view.go",
		"task_checklist_view.go",
		"fullscreen_task_helpers.go",
		"picker_items.go",
		"task_detail_view.go",
		"notes_view_render.go",
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

	task, err := models.NewGenericTask(id, priority, title)
	if err != nil {
		t.Fatalf("NewTask(%q): %v", id, err)
	}
	task.Priority = priority
	return task
}
