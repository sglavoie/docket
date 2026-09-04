package tui

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

func TestMigratedTaskTitleRenderersAvoidActiveSuccessTitleOverrides(t *testing.T) {
	files := []string{
		// shared_render.go and fullscreen_task_helpers.go moved into the chrome
		// package in Session 2.8, quick_capture_autocomplete.go into grammar in
		// Session 2.11, picker_items.go into query in Session 2.13, and
		// parent_selector.go into listsurface in Session 2.19; the rule follows
		// the file, not the package.
		"chrome/shared_render.go",
		"outliner_render_rows.go",
		"app_pinned_tasks_render.go",
		"listsurface/parent_selector.go",
		"grammar/quick_capture_autocomplete.go",
		"task_attachments_view.go",
		"task_comments_view.go",
		"chrome/fullscreen_task_helpers.go",
		"query/picker_items.go",
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
