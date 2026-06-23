package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestOutlinerEmptyMessagePerspectiveHintUsesPerspectiveKeys(t *testing.T) {
	m := &OutlinerModel{project: &models.Project{}}

	got := m.outlinerEmptyMessage(Perspective{Name: "Next"})

	assertContains(t, got, "Try P to pick a perspective or h/l to cycle.", "perspective hint")
	assertNotContains(t, got, "shift+", "pinned-tag tab keys")
}

func TestOutlinerEmptyMessagePerspectiveZoomHintUsesPerspectiveKeys(t *testing.T) {
	project := &models.Project{
		Tasks: []*models.Task{
			{ID: "parent", Title: "Parent task"},
		},
	}
	m := &OutlinerModel{
		project:   project,
		zoomStack: []ZoomEntry{{TaskID: "parent"}},
	}

	got := m.outlinerEmptyMessage(Perspective{Name: "Next"})

	assertContains(t, got, `within "Parent task"`, "zoom context")
	assertContains(t, got, "Try P to pick a perspective or h/l to cycle.", "perspective hint")
	assertNotContains(t, got, "shift+", "pinned-tag tab keys")
}

func TestOutlinerEmptyMessageAllPerspectiveDoesNotShowPerspectiveHint(t *testing.T) {
	m := &OutlinerModel{project: &models.Project{}}

	got := m.outlinerEmptyMessage(Perspective{Name: "All"})

	if strings.Contains(got, "perspective") {
		t.Fatalf("All perspective empty message should not include perspective hint: %q", got)
	}
}
