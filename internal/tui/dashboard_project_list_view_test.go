package tui

import (
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
)

// TestProjectStatusWidthBlockedMatchesIndicator ensures the precomputed
// status-indicator column width matches the actual rendered width when the
// project has blocked tasks. A mismatch causes column misalignment in the
// dashboard project list.
func TestProjectStatusWidthBlockedMatchesIndicator(t *testing.T) {
	icons.SetUseEmoji(true)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	p := &models.Project{
		ID:   "p1",
		Name: "Project",
		Tasks: []*models.Task{
			{ID: "t1", Title: "blocked task", Status: models.StatusBlocked},
		},
	}

	m := &DashboardModel{}
	got := m.projectStatusWidth(p, 3)
	rendered := m.projectRowStatusIndicator(p, 3)
	want := lipgloss.Width(rendered)

	if got != want {
		t.Fatalf("projectStatusWidth = %d, rendered width = %d (indicator %q)", got, want, rendered)
	}
}
