package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func taskWithChecklist(done, total int) *models.Task {
	items := make([]models.ChecklistItem, total)
	for i := range items {
		items[i] = models.ChecklistItem{Text: "step", Done: i < done}
	}
	return &models.Task{Title: "t", Checklist: items}
}

func TestRenderChecklistIndicatorCompact(t *testing.T) {
	if got := renderChecklistIndicatorCompact(&models.Task{}); got != "" {
		t.Errorf("empty checklist should produce empty string, got %q", got)
	}
	got := ansi.Strip(renderChecklistIndicatorCompact(taskWithChecklist(3, 5)))
	if got != "3/5" {
		t.Errorf("compact form should be %q, got %q", "3/5", got)
	}
}

func TestRenderChecklistBar(t *testing.T) {
	if got := renderChecklistBar(&models.Task{}, 80); got != "" {
		t.Errorf("empty checklist should produce empty string, got %q", got)
	}

	// Wide enough -> bar form
	wide := ansi.Strip(renderChecklistBar(taskWithChecklist(3, 5), 80))
	if !strings.HasPrefix(wide, "[") || !strings.Contains(wide, "3/5") {
		t.Errorf("wide form should contain bar and fraction, got %q", wide)
	}

	// Narrow -> falls back to plain indicator (ends in "X/Y")
	narrow := ansi.Strip(renderChecklistBar(taskWithChecklist(3, 5), 8))
	if !strings.HasSuffix(narrow, "3/5") {
		t.Errorf("narrow form should fall back to plain indicator, got %q", narrow)
	}
	if strings.HasPrefix(narrow, "[") {
		t.Errorf("narrow form should not include bar brackets, got %q", narrow)
	}

	// 10/10 still renders bar at wide width
	full := ansi.Strip(renderChecklistBar(taskWithChecklist(10, 10), 80))
	if !strings.Contains(full, "10/10") {
		t.Errorf("full progress should display 10/10, got %q", full)
	}
}
