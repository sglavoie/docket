package outliner

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskform"
)

func TestTaskComposerCarriesPinIntoEveryCaptureRequest(t *testing.T) {
	project := &models.Project{ID: "project", Slug: "project", Name: "Project"}
	composer := taskform.NewTaskComposerModel(taskform.TaskComposerPreset{
		Kind:        taskform.ComposerProject,
		Project:     project,
		AllProjects: []*models.Project{project},
	}, nil)
	composer.Input.SetValue("First *pin\nSecond\nThird *pin")

	reqs, ok := composer.ParseCompactLines()
	if !ok || len(reqs) != 3 {
		t.Fatalf("parseCompactLines = %+v, %v", reqs, ok)
	}
	got := []bool{reqs[0].Pinned, reqs[1].Pinned, reqs[2].Pinned}
	if !slices.Equal(got, []bool{true, false, true}) {
		t.Fatalf("Pinned per line = %v, want [true false true]", got)
	}
	if reqs[0].Title != "First" || reqs[2].Title != "Third" {
		t.Fatalf("titles kept the sigil: %q / %q", reqs[0].Title, reqs[2].Title)
	}
}

// One capture batch stamps every pinned line with the same clock tick, so the
// pinned section's order comes from the compareOrderedModeTasks fallthrough —
// the project's task order — rather than from the timestamps.
func TestPinnedProjectionOrdersOneCaptureTickByTaskOrder(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	tick := now.Add(-time.Hour)

	first := primaryRowsTask("first", models.StateInbox)
	first.Pinned, first.PinnedAt = true, &tick
	second := primaryRowsTask("second", models.StateInbox)
	second.Pinned, second.PinnedAt = true, &tick

	order := func(tasks ...*models.Task) []string {
		project := primaryRowsProject("project", tasks...)
		projection := buildPinnedModeProjection(OutlinerModeInput{Projects: []*models.Project{project}, Now: now})
		var ids []string
		for _, row := range projection.rows {
			ids = append(ids, row.Task.ID)
		}
		return ids
	}
	if got := order(first, second); !slices.Equal(got, []string{"first", "second"}) {
		t.Fatalf("tied PinnedAt order = %v, want project task order [first second]", got)
	}
	if got := order(second, first); !slices.Equal(got, []string{"second", "first"}) {
		t.Fatalf("tied PinnedAt order = %v, want project task order [second first]", got)
	}
}
