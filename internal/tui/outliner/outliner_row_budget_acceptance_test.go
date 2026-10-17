package outliner

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

// This file is the simplified row budget checked against the shapes the budget
// is most likely to break on rather than against the ideal row: deep nesting, a
// long project name, double-width text, and a visual selection. Each case
// measures the assembled row rather than a segment, because the claim under test
// is arithmetic — the row never overruns the columns it was given, and the
// essentials are still inside them. The editor laying its caret over that same
// remaining width is internal/tui/inline_row_editor_test.go's subject.

// budgetAcceptanceTask carries every signal the simplified row still draws:
// state, priority, a pin outside the Pinned lane, a deadline, an arrived
// check-in, and a reference census.
func budgetAcceptanceTask(title string) *models.Task {
	now := time.Now()
	due := util.StartOfDay(now.AddDate(0, 0, 3))
	checkIn := util.StartOfDay(now.AddDate(0, 0, -1))
	return &models.Task{
		ID:             "budget",
		ShortID:        41,
		Title:          title,
		WorkflowState:  models.StateWaiting,
		StateChangedAt: now.AddDate(0, 0, -11),
		UpdatedAt:      now,
		Priority:       1,
		Pinned:         true,
		DueDate:        &due,
		NextCheckIn:    &checkIn,
		Dependencies:   []models.TaskReference{{ProjectID: "budget-project", TaskID: "absent"}},
		Context:        "platform",
		Tags:           []string{"backend"},
		Checklist:      []models.ChecklistItem{{Text: "Chase", Done: true}, {Text: "Escalate"}},
	}
}

// TestNestedRowUnderALongProjectNameKeepsItsEssentials is the narrow-layout
// acceptance. Five levels of tree prefix under a project whose name fills a
// header leaves the least title width the product ever offers, and the row still
// has to say what state the work is in, that it is priority 1, that it is
// pinned, and when it is due. Supporting review and reference badges yield
// before the title; their full values remain in Peek.
func TestNestedRowUnderALongProjectNameKeepsItsEssentials(t *testing.T) {
	task := budgetAcceptanceTask("Confirm the vendor security questionnaire")
	project := renderFixtureProject(
		t,
		"budget-project",
		"platform-delivery-and-migration-workstream",
		"Platform Delivery and Migration Workstream",
		[]*models.Task{task},
	)
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := OutlinerRow{Task: task, ProjectID: project.ID, Prefix: "│   │   │   │   └── "}

	for _, width := range []int{60, 80, 100} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			plain := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, width, 0, nil))
			if got := lipgloss.Width(plain); got > width {
				t.Fatalf("%d-column row assembled %d columns: %q", width, got, plain)
			}
			essentials := map[string]string{
				"state badge":    ansi.Strip(theme.GetStateBadge(models.StateWaiting)),
				"priority badge": ansi.Strip(chrome.RenderPriorityBadge(1)),
				"pin marker":     "*",
				"deadline":       ansi.Strip(query.FormatDueDate(task)),
			}
			for name, want := range essentials {
				if !strings.Contains(plain, want) {
					t.Fatalf("%d-column nested row lost its %s (%q): %q", width, name, want, plain)
				}
			}
			// A recognizable phrase survives even in the deepest, narrowest row.
			if !strings.Contains(plain, "Confirm the vendor") {
				t.Fatalf("%d-column nested row spent nothing on the title: %q", width, plain)
			}
			// And the long project name never reaches the task row at all, so it
			// costs the title nothing at any width.
			if strings.Contains(plain, "platform-delivery") {
				t.Fatalf("%d-column task row repeated its project header: %q", width, plain)
			}
		})
	}
}

// TestNestedRowSupportingDetailStaysReadableInPeek is the other half of that
// trade. Everything the nested row stopped drawing is one keypress away on the
// same row, at full value rather than in a fixed-width badge.
func TestNestedRowSupportingDetailStaysReadableInPeek(t *testing.T) {
	task := budgetAcceptanceTask("Confirm the vendor security questionnaire")
	project := renderFixtureProject(t, "budget-project", "platform", "Platform", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := OutlinerRow{Task: task, ProjectID: project.ID, Prefix: "│   │   │   │   └── "}

	peek := ansi.Strip(strings.Join(peekDocumentBody(outliner, row, 60), "\n"))
	for _, want := range []string{
		"Confirm the vendor security questionnaire",
		"Waiting 11d",
		"Check-in:",
		"Checklist: 1/2 done",
		"Context: [platform]",
		"Tags: #backend",
	} {
		if !strings.Contains(peek, want) {
			t.Fatalf("Peek lost %q the row stopped drawing:\n%s", want, peek)
		}
	}
}

// TestDoubleWidthTitleFillsTheReclaimedBudgetByCells keeps the reclaimed columns
// honest about what a column is. The row budget is measured in display cells,
// not runes, so a CJK title spends twice the budget per character and must still
// fit the columns it was given, clipped on a cell boundary rather than through
// one.
func TestDoubleWidthTitleFillsTheReclaimedBudgetByCells(t *testing.T) {
	title := strings.Repeat("修复界面布局问题", 6)
	task := budgetAcceptanceTask(title)
	project := renderFixtureProject(t, "budget-project", "platform", "Platform", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := OutlinerRow{Task: task, ProjectID: project.ID, Prefix: "│   ├── "}

	for _, width := range []int{60, 61, 80, 81} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			plain := ansi.Strip(outliner.RenderOutlinerTaskRow(row, false, width, 0, nil))
			// A double-width cell cannot straddle the last column, so an odd
			// budget legitimately leaves one cell unspent; two would mean a
			// whole character was dropped rather than fitted.
			if got := lipgloss.Width(plain); got > width || got < width-1 {
				t.Fatalf("%d-column row assembled %d cells: %q", width, got, plain)
			}
			if !strings.Contains(plain, "修复") {
				t.Fatalf("%d-column row spent nothing on a double-width title: %q", width, plain)
			}
			if !strings.Contains(plain, "…") {
				t.Fatalf("%d-column row did not clip a title far longer than it: %q", width, plain)
			}
			if !strings.Contains(plain, ansi.Strip(query.FormatDueDate(task))) {
				t.Fatalf("%d-column row let a double-width title eat the deadline: %q", width, plain)
			}
		})
	}

	// The wrap breaks a run of double-width text wherever the column ends
	// rather than on a space, so the title is compared with the layout removed.
	peek := ansi.Strip(strings.Join(peekDocumentBody(outliner, row, 60), ""))
	if !strings.HasPrefix(strings.Join(strings.Fields(peek), ""), title) {
		t.Fatalf("Peek did not recover the whole double-width title:\n%s", peek)
	}
}

// TestSelectedNestedRowKeepsTheSameEssentials pins selection to the budget
// rather than to the paint. A selected row says so in the leading column every
// row already has, so it spends no column on it: it assembles exactly as wide as
// the unselected row and keeps every essential that row drew.
func TestSelectedNestedRowKeepsTheSameEssentials(t *testing.T) {
	task := budgetAcceptanceTask("Confirm the vendor security questionnaire")
	project := renderFixtureProject(t, "budget-project", "platform", "Platform", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	outliner.RebuildRows()
	outliner.MultiSelectMode = true
	outliner.ReplaceSelection([]taskref.TaskIdentity{taskref.For(task, project)})
	row := OutlinerRow{Task: task, ProjectID: project.ID, Prefix: "│   │   └── "}

	const width = 60
	plain := ansi.Strip(outliner.RenderOutlinerTaskRow(row, true, width, 0, nil))
	if got := lipgloss.Width(plain); got != width {
		t.Fatalf("selected row assembled %d columns: %q", got, plain)
	}
	if !strings.HasPrefix(plain, selectionMark) {
		t.Fatalf("selected row did not draw its mark: %q", plain)
	}
	outliner.MultiSelectMode = false
	unselected := ansi.Strip(outliner.RenderOutlinerTaskRow(row, true, width, 0, nil))
	if want := selectionMark + strings.TrimPrefix(unselected, " "); plain != want {
		t.Fatalf("selecting the row moved what it draws:\nunselected %q\nselected   %q", unselected, plain)
	}
	for _, want := range []string{
		ansi.Strip(chrome.RenderPriorityBadge(1)),
		ansi.Strip(query.FormatDueDate(task)),
		"Confirm the vendor",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("selected row traded an essential for its mark, missing %q: %q", want, plain)
		}
	}
}
