package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/grammar"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

func TestParseQuickEditPinPatchesAndPreviews(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "pin", input: "Ship release *pin", want: true},
		{name: "unpin", input: "Ship release *unpin", want: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			task := &models.Task{ID: "task", Title: "Ship release", Pinned: !testCase.want}
			project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{task}}

			result, err := grammar.ParseQuickEdit(testCase.input, task, project)
			if err != nil {
				t.Fatalf("parseQuickEdit: %v", err)
			}
			if !result.PinnedSet || result.Pinned != testCase.want {
				t.Fatalf("pinned = %v set=%v, want %v", result.Pinned, result.PinnedSet, testCase.want)
			}
			if result.Title != "Ship release" {
				t.Fatalf("title = %q, want the sigil stripped", result.Title)
			}
			if result.Preview.Pinned != testCase.want {
				t.Fatalf("preview.Pinned = %v, want %v", result.Preview.Pinned, testCase.want)
			}
			req := outliner.TaskLineEditRequest(taskref.TaskIdentity{ProjectID: project.ID, TaskID: task.ID}, result)
			if req.Pinned == nil || *req.Pinned != testCase.want {
				t.Fatalf("request Pinned = %v, want %v", req.Pinned, testCase.want)
			}
		})
	}
}

func TestParseQuickEditWithoutPinPreservesPinnedState(t *testing.T) {
	pinnedAt := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	task := &models.Task{ID: "task", Title: "Ship release", Pinned: true, PinnedAt: &pinnedAt}
	project := &models.Project{ID: "project", Name: "Project", Tasks: []*models.Task{task}}

	result, err := grammar.ParseQuickEdit("Ship it !1", task, project)
	if err != nil {
		t.Fatalf("parseQuickEdit: %v", err)
	}
	if result.PinnedSet {
		t.Fatalf("pinnedSet = true, want an omitted sigil to preserve the pinned state")
	}
	if !result.Preview.Pinned {
		t.Fatalf("preview.Pinned = false, want the task's own state carried through")
	}
	req := outliner.TaskLineEditRequest(taskref.TaskIdentity{ProjectID: project.ID, TaskID: task.ID}, result)
	if req.Pinned != nil {
		t.Fatalf("request Pinned = %v, want nil", req.Pinned)
	}
}

func TestQuickEditSnapshotDoesNotPrefillPin(t *testing.T) {
	pinnedAt := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	task := &models.Task{ID: "task", Title: "Ship release", Pinned: true, PinnedAt: &pinnedAt}

	snapshot := grammar.QuickEditSnapshot(task)
	if snapshot == nil {
		t.Fatal("quickEditSnapshot = nil")
	}
	if snapshot.PinnedSet || snapshot.Pinned {
		t.Fatalf("prefill pinned = %v set=%v, want the title-only prefill", snapshot.Pinned, snapshot.PinnedSet)
	}
}
