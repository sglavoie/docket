package outliner

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
)

// Peek ends on the row's own legend: each glyph the row drew beside the word
// it abbreviates, read off the row's segments rather than a second table.
func TestInlinePeekEndsOnTheRowLegend(t *testing.T) {
	task := primaryRowsTask("legend", models.StateBacklog)
	task.Priority = 1
	task.Description = "why"
	m := nowCardOutliner(t, 100, 30, task)
	m.Cursor = taskRowIndex(t, m, task.ID)
	doc := m.inlinePeekDocumentForRow(m.Rows[m.Cursor], "", 100, testPeekWindow())
	body := ansi.Strip(strings.Join(doc.Body, "\n"))
	last := body[strings.LastIndex(body, "\n")+1:]
	if !strings.Contains(last, "Row: ") || strings.Contains(last, "!1 High") || !strings.Contains(last, "Has a description") {
		t.Fatalf("legend line = %q in:\n%s", last, body)
	}
	if strings.Contains(last, "date ") {
		t.Fatalf("legend repeats a word key: %q", last)
	}
}

func TestPeekLegendOmitsStateAndPriority(t *testing.T) {
	for _, marks := range []bool{false, true} {
		task := primaryRowsTask("legend", models.StateInbox)
		task.Title = "A task"
		task.Priority = 1
		if marks {
			task.Pinned = true
			task.Attachments = []models.Attachment{{Filename: "brief.md"}}
		}
		m := nowCardOutliner(t, 100, 35, task)
		// Inbox keeps the pinned glyph on the task instead of lifting it into Pinned.
		m.PrimaryMode = chrome.ModeInbox
		m.RebuildRows()
		m.Cursor = taskRowIndex(t, m, task.ID)
		entries := m.CursorRowLegend()
		var state, priority bool
		for _, entry := range entries {
			state = state || strings.HasPrefix(entry.Meaning, "Status: ")
			priority = priority || strings.HasPrefix(entry.Meaning, "Priority: ")
		}
		if !state || !priority {
			t.Fatalf("help lost state/priority: %#v", entries)
		}
		for _, width := range []int{30, 60, 100} {
			doc := m.inlinePeekDocumentForRow(m.Rows[m.Cursor], "", width, testPeekWindow())
			body := ansi.Strip(strings.Join(doc.Body, "\n"))
			if strings.Contains(body, "Row:") != marks {
				t.Fatalf("marks=%v width=%d: unexpected legend presence:\n%s", marks, width, body)
			}
			legend := ansi.Strip(peekRowLegend(entries))
			if strings.Contains(legend, "Inbox") || strings.Contains(legend, "!1") {
				t.Fatalf("redundant legend: %q", legend)
			}
			if marks && (!strings.Contains(legend, "* Pinned") || !strings.Contains(legend, "& Has attachments")) {
				t.Fatalf("lost surviving marks: %q", legend)
			}
			if !strings.Contains(body, "Status:") || !strings.Contains(body, "Priority:") {
				t.Fatalf("lost status values:\n%s", body)
			}
		}
	}
}
