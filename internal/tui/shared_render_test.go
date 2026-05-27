package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestRenderDescriptionIndicator(t *testing.T) {
	t.Run("empty description returns empty string", func(t *testing.T) {
		task := &models.Task{Description: ""}
		if got := renderDescriptionIndicator(task); got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("non-empty description contains icon", func(t *testing.T) {
		task := &models.Task{Description: "Some details"}
		got := renderDescriptionIndicator(task)
		icon := IconDescription()
		if !strings.Contains(got, icon) {
			t.Errorf("expected output to contain icon %q, got %q", icon, got)
		}
	})
}

func TestRenderCrossProjectTaskLineShowDescription(t *testing.T) {
	proj := &models.Project{ID: "p1", Name: "Proj"}
	icon := IconDescription()

	t.Run("ShowDescription off omits icon even when description set", func(t *testing.T) {
		task := &models.Task{ID: "t1", Title: "Title", Description: "details"}
		got := RenderCrossProjectTaskLine(task, proj, nil, false, 120, 40, CrossProjectTaskLineOpts{
			ShowDescription: false,
		})
		if strings.Contains(got, icon) {
			t.Errorf("expected no description icon when ShowDescription=false, got %q", got)
		}
	})

	t.Run("ShowDescription on with description renders icon", func(t *testing.T) {
		task := &models.Task{ID: "t1", Title: "Title", Description: "details"}
		got := RenderCrossProjectTaskLine(task, proj, nil, false, 120, 40, CrossProjectTaskLineOpts{
			ShowDescription: true,
		})
		if !strings.Contains(got, icon) {
			t.Errorf("expected description icon when ShowDescription=true and task has description, got %q", got)
		}
	})

	t.Run("ShowDescription on without description omits icon", func(t *testing.T) {
		task := &models.Task{ID: "t1", Title: "Title", Description: ""}
		got := RenderCrossProjectTaskLine(task, proj, nil, false, 120, 40, CrossProjectTaskLineOpts{
			ShowDescription: true,
		})
		if strings.Contains(got, icon) {
			t.Errorf("expected no description icon when task has empty description, got %q", got)
		}
	})
}

// TestTaskLinePrioritySegmentDemotionPreservesAlignment guards against a
// regression where applyHeroDemotion stripped the alignment padding that
// PadBadge inserted for a task whose own priority was hidden by
// HideLowPriority. The padding (whitespace) was trimmed by
// demotedTaskLineSegment, collapsing the column to width 0 and breaking
// vertical alignment with sibling rows that did show a priority badge.
//
// Scenario: layout reserves PriorityWidth=1 (some sibling row has Priority<=2).
// The current task has HideLowPriority=true, Priority=5 (no badge) and is
// overdue (HeroOverdue triggers demotion of the priority segment). We assert
// the rendered prioritySegment occupies the same visual width as a non-demoted
// row where the priority badge is empty for the same layout — i.e. the
// padding survives demotion.
func TestTaskLinePrioritySegmentDemotionPreservesAlignment(t *testing.T) {
	layout := &ColumnLayout{HasPriority: true, PriorityWidth: 1}

	// Baseline: low-priority hidden, but no hero (Priority=3 is not P1, not
	// overdue, not in_review). prioritySegment should be PadBadge("",1)+" " = "  ".
	baselineTask := &models.Task{
		Title:    "samename",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 3,
	}
	baseline := TaskLine{
		Task:            baselineTask,
		MaxTitle:        20,
		HideLowPriority: true,
		Layout:          layout,
	}.Render()

	// Demoted case: same hidden priority, but task is overdue → HeroOverdue
	// triggers demotion of prioritySegment via demotedTaskLineSegment.
	yesterday := time.Now().Add(-48 * time.Hour)
	demotedTask := &models.Task{
		Title:    "samename",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 3,
		DueDate:  &yesterday,
	}
	demoted := TaskLine{
		Task:            demotedTask,
		MaxTitle:        20,
		HideLowPriority: true,
		Layout:          layout,
	}.Render()

	baselineW := lipgloss.Width(ansi.Strip(baseline))
	demotedW := lipgloss.Width(ansi.Strip(demoted))

	if baselineW != demotedW {
		t.Fatalf("hero demotion shrank prioritySegment alignment: baseline=%d demoted=%d\nbaseline=%q\ndemoted=%q",
			baselineW, demotedW, ansi.Strip(baseline), ansi.Strip(demoted))
	}
}
