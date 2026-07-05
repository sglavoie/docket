package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// hasHeightOverflow reports whether normalizeScreen flags the rendered content
// as taller than the terminal at the given size.
func hasHeightOverflow(content string, width, height int) bool {
	_, issues := normalizeScreen(content, width, height)
	for _, iss := range issues {
		if iss.Kind == "height_overflow" {
			return true
		}
	}
	return false
}

// TestDateInputNoHeightOverflow guards against the fixed-size help box pushing
// the Set Due Date view past short terminals (regression for the 80x24 / 72x20
// height_overflow surfaced by tt tui-test).
func TestDateInputNoHeightOverflow(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	task := &models.Task{ID: "date-overflow", ShortID: 7, Title: "Triage inbox"}

	for _, h := range []int{20, 22, 24, 26, 27} {
		m := NewDateInputModel(task, project, "")
		m.SetSize(72, h)
		content := m.View().Content
		if hasHeightOverflow(content, 72, h) {
			t.Errorf("DateInput overflows terminal height at 72x%d:\n%s", h, content)
		}
	}
}

// TestSnoozeInputNoHeightOverflow is the SnoozeInput counterpart to
// TestDateInputNoHeightOverflow.
func TestSnoozeInputNoHeightOverflow(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Test Project")
	task := &models.Task{ID: "snooze-overflow", ShortID: 8, Title: "Triage inbox"}

	for _, h := range []int{20, 22, 24, 26, 27} {
		m := NewSnoozeInputModel(task, project)
		m.SetSize(72, h)
		content := m.View().Content
		if hasHeightOverflow(content, 72, h) {
			t.Errorf("SnoozeInput overflows terminal height at 72x%d:\n%s", h, content)
		}
	}
}

// TestRenderClampedHelpBox exercises the shared clamp helper directly: it keeps
// the full body when it fits, collapses the overflow tail into a muted
// "… N more", and omits the box when there is no room.
func TestRenderClampedHelpBox(t *testing.T) {
	body := []string{"row1", "row2", "row3", "row4", "row5"}
	chrome := BoxStyle.GetVerticalBorderSize() + BoxStyle.GetVerticalPadding()

	// Ample budget: every row present, no indicator.
	full := renderClampedHelpBox(body, len(body)+chrome+2)
	if strings.Contains(full, "more") {
		t.Errorf("expected no truncation with ample budget, got:\n%s", full)
	}
	for _, r := range body {
		if !strings.Contains(full, r) {
			t.Errorf("expected full box to contain %q, got:\n%s", r, full)
		}
	}

	// Tight budget: fits only two body rows, so one real row + "… N more".
	clamped := renderClampedHelpBox(body, chrome+2)
	if !strings.Contains(clamped, "… 4 more") {
		t.Errorf("expected '… 4 more' indicator, got:\n%s", clamped)
	}
	if strings.Contains(clamped, "row2") {
		t.Errorf("expected only the first row kept, got:\n%s", clamped)
	}

	// No room for even a minimal box: omit entirely.
	if got := renderClampedHelpBox(body, chrome); got != "" {
		t.Errorf("expected empty string when no room for box, got:\n%s", got)
	}
}
