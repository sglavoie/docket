package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestHistoryCountsSummary(t *testing.T) {
	cases := []struct {
		name                                string
		commentsN, decisionsN, attachmentsN int
		want                                string
	}{
		{"all zero", 0, 0, 0, ""},
		{"singular comment", 1, 0, 0, "1 comment"},
		{"plural comments", 3, 0, 0, "3 comments"},
		{"singular decision only", 0, 1, 0, "1 decision"},
		{"singular attachment only", 0, 0, 1, "1 attachment"},
		{"mixed singular/plural", 3, 1, 2, "3 comments · 1 decision · 2 attachments"},
		{"skip zero entries", 0, 2, 0, "2 decisions"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := historyCountsSummary(c.commentsN, c.decisionsN, c.attachmentsN)
			if got != c.want {
				t.Errorf("historyCountsSummary(%d,%d,%d) = %q, want %q",
					c.commentsN, c.decisionsN, c.attachmentsN, got, c.want)
			}
		})
	}
}

func TestRenderHistoryClusterDivider(t *testing.T) {
	collapsed := renderHistoryClusterDivider(3, 1, 2, false, 100)
	if !strings.Contains(collapsed, "History") {
		t.Errorf("collapsed divider missing label: %q", collapsed)
	}
	if !strings.Contains(collapsed, "3 comments · 1 decision · 2 attachments") {
		t.Errorf("collapsed divider missing summary: %q", collapsed)
	}
	if !strings.Contains(collapsed, "press H to expand") {
		t.Errorf("collapsed divider missing expand hint: %q", collapsed)
	}

	expanded := renderHistoryClusterDivider(3, 1, 2, true, 100)
	if !strings.Contains(expanded, "press H to collapse") {
		t.Errorf("expanded divider missing collapse hint: %q", expanded)
	}
}

func TestTaskDetailHistoryToggleDefault(t *testing.T) {
	m := &TaskDetailModel{}
	if m.historyExpanded {
		t.Errorf("historyExpanded should default to false")
	}
}

// TestTaskDetailUpdate_IgnoresMessagesForOtherTasks verifies that
// TaskUpdatedMsg / TaskPropertyChangedMsg targeting a different task in the
// same project do not silently swap the currently-shown task.
func TestTaskDetailUpdate_IgnoresMessagesForOtherTasks(t *testing.T) {
	project, err := models.NewProject("proj-1", "Detail", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	shown, err := models.NewGenericTask("task-shown", 1, "Shown")
	if err != nil {
		t.Fatalf("NewTask shown: %v", err)
	}
	other, err := models.NewGenericTask("task-other", 2, "Other")
	if err != nil {
		t.Fatalf("NewTask other: %v", err)
	}
	project.Tasks = []*models.Task{shown, other}

	m := &TaskDetailModel{task: shown, project: project}

	if _, _ = m.Update(TaskUpdatedMsg{Task: other}); m.task != shown {
		t.Fatalf("TaskUpdatedMsg for other task swapped m.task: got %q, want %q", m.task.ID, shown.ID)
	}
	if _, _ = m.Update(TaskPropertyChangedMsg{Task: other}); m.task != shown {
		t.Fatalf("TaskPropertyChangedMsg for other task swapped m.task: got %q, want %q", m.task.ID, shown.ID)
	}

	// Sanity: a message for the shown task should still refresh the pointer
	// (here it remains the same since project storage is unchanged).
	if _, _ = m.Update(TaskUpdatedMsg{Task: shown}); m.task != shown {
		t.Fatalf("TaskUpdatedMsg for shown task should keep m.task pointing at shown, got %q", m.task.ID)
	}
}
