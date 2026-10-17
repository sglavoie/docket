package outliner

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestChecklistPeekWrapsAndPagesWithoutMovingTheList(t *testing.T) {
	for _, width := range []int{60, 80, 120} {
		task := pagedPeekTask()
		task.Checklist = []models.ChecklistItem{
			{Text: strings.Repeat("Check 日本語 é and long details ", 30)},
			{Text: "A different item", Done: true},
		}
		m := pagedPeekOutliner(t, task, width, 24)
		ownerRow := m.Rows[m.Cursor]
		m.Folds.SetChecklist(m.rowIdentity(ownerRow), true)
		m.RebuildRows()
		for index, row := range m.Rows {
			if row.IsChecklistItem && row.ChecklistIndex == 0 {
				m.Cursor = index
				break
			}
		}
		m.View()
		doc := cursorPeekDocument(t, m)
		if !doc.Paged() {
			t.Fatal("long checklist text did not produce a scrollable document")
		}
		plain := ansi.Strip(strings.Join(doc.Body, "\n"))
		for _, text := range []string{"Not done", "item 1 of 2", "Task: Selected"} {
			if !strings.Contains(plain, text) {
				t.Fatalf("item document lacks %q:\n%s", text, plain)
			}
		}
		for _, line := range doc.Body {
			if lipgloss.Width(line) > width {
				t.Fatalf("checklist detail overflows %d columns: %q", width, line)
			}
		}
		rows, cursor := m.PeekDockRows(), m.Cursor
		m.scrollPeek(doc.window.FullPageRows())
		if peekOffset(t, m) == 0 || m.Cursor != cursor || m.PeekDockRows() != rows {
			t.Fatal("paging did not scroll the item independently of the list")
		}
		m.Cursor++
		other := cursorPeekDocument(t, m)
		if peekOffset(t, m) != 0 || !strings.Contains(ansi.Strip(strings.Join(other.Body, "\n")), "Done · item 2 of 2") {
			t.Fatal("neighbor inherited the first item's document or scroll")
		}
		if m.PeekDockRows() != rows {
			t.Fatal("moving to a short checklist item resized the dock")
		}
	}
}

func TestPeekExplainsTimedOverdueAtItsObservationTime(t *testing.T) {
	now := time.Date(2026, time.October, 16, 10, 0, 0, 0, time.Local)
	due := now.Add(-time.Hour)
	task := &models.Task{WorkflowState: models.StateBacklog, DueDate: &due}
	if got := ansi.Strip(inlinePeekScheduleLine(task, now)); !strings.Contains(got, "09:00") || !strings.Contains(got, "overdue") {
		t.Fatalf("timed overdue not explained: %s", got)
	}
	if got := inlinePeekScheduleLine(task, now.Add(-2*time.Hour)); strings.Contains(got, "overdue") {
		t.Fatalf("future deadline labeled overdue: %s", got)
	}
}
