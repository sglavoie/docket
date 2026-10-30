package outliner

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/route"
)

func TestFullscreenPeekIncludesTheWholeTask(t *testing.T) {
	at := time.Date(2026, 10, 30, 12, 30, 0, 0, time.UTC)
	task := &models.Task{
		ID: "task-uuid", ShortID: 7, Title: "Complete task information", WorkflowState: models.StateWaiting,
		Description: "Full description", ResumeNote: "Start with the draft", Context: "office",
		Tags: []string{"feature"}, Pinned: true, Review: true, Hidden: true,
		CreatedAt: at, UpdatedAt: at, StateChangedAt: at, PinnedAt: &at, LastReviewedAt: &at,
		NextCheckIn: &at, CheckInRepeatDays: 7,
		Checklist:   []models.ChecklistItem{{Text: "Read the draft", Done: true}, {Text: "Write the conclusion"}},
		Comments:    []*models.Comment{{Text: "The whole conversation is readable here.", CreatedAt: at}},
		Attachments: []models.Attachment{{Filename: "brief.md", OriginalPath: "/documents/brief.md", AddedAt: at}},
	}
	for _, width := range []int{60, 80, 120} {
		doc := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Width: width, Indent: "  ", Detailed: true, Window: peekDockGeometry{bodyRows: 12}, Planning: "Today", ProjectSlug: "project"})
		plain := ansi.Strip(strings.Join(doc.Body, "\n"))
		for _, value := range []string{
			"Complete task information", "Plan: Today", "Status: Waiting", "Pinned", "Review",
			"Resume: Start with the draft", "Project: project", "Context: office", "Check-in:",
			"Full description", "[x] Read the draft", "[ ] Write the conclusion",
			"The whole conversation is readable here.", "/documents/brief.md", "Tags: #feature",
			"ID: #7 · task-uuid", "Hidden: Yes", "Created:", "Updated:", "State changed:", "Pinned at:", "Last reviewed:",
		} {
			if !strings.Contains(plain, value) {
				t.Fatalf("width %d missing %q:\n%s", width, value, plain)
			}
		}
		for _, line := range doc.Body {
			if lipgloss.Width(line) > width {
				t.Fatalf("width %d overflow: %q", width, ansi.Strip(line))
			}
		}
		compact := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Width: width, Window: testPeekWindow()})
		if strings.Contains(ansi.Strip(strings.Join(compact.Body, "\n")), "Write the conclusion") {
			t.Fatal("compact Peek started repeating checklist bodies")
		}
	}
}

func TestFullscreenPeekScrollingLeavesTheOutlineInPlace(t *testing.T) {
	m := dockFixtureOutliner(t, 60, 20)
	for i, row := range m.Rows {
		if row.IsTaskRow() && row.Task.Title == "Long" {
			m.Cursor = i
		}
	}
	m.View()
	cursor, offset := m.Cursor, m.ViewScrollOffset
	m.ToggleFullscreenPeek()
	m.View()
	if !m.PeekFullscreen || !m.PeekMode || !m.SimplifiedInlineControlOwnsInput() {
		t.Fatal("B did not open and own the full-screen reader")
	}
	m.handleFullscreenPeekKey("end")
	_, doc, _ := m.peekCursorDocument()
	if got := m.peekOffsetForRow(m.Rows[m.Cursor], doc); got != doc.MaxOffset() || got == 0 {
		t.Fatalf("End did not reach the document's end: %d / %d", got, doc.MaxOffset())
	}
	for _, key := range []string{"j", "down", "pgdown", "ctrl+d", "ctrl+f", "e", "v", "w", "a"} {
		m.handleFullscreenPeekKey(key)
	}
	m.handleFullscreenPeekKey("home")
	m.HandlePointer(route.PointerEvent{X: 2, Y: 2, WheelDelta: 1})
	if got := m.peekOffsetForRow(m.Rows[m.Cursor], doc); got != 1 {
		t.Fatalf("wheel did not scroll the document: %d", got)
	}
	m.handleFullscreenPeekKey("esc")
	if m.PeekMode || m.PeekFullscreen || m.Cursor != cursor || m.ViewScrollOffset != offset {
		t.Fatal("reader changed the list or reopened a dock that was closed")
	}
	m.MultiSelectMode = true
	m.ToggleFullscreenPeek()
	if m.PeekFullscreen {
		t.Fatal("fullscreen fell back to the cursor under a selection")
	}
}

func TestFullscreenPeekKeepsAnOpenDockUsableOnAPlaceholder(t *testing.T) {
	m := dockFixtureOutliner(t, 60, 20)
	_, plain := peekableRowIndexes(t, m)
	if plain < 0 {
		t.Fatal("fixture has no placeholder row")
	}
	m.Cursor = plain
	m.ToggleFullscreenPeek()
	if m.PeekFullscreen {
		t.Fatal("opened details on a row with no document and no dock")
	}
	m.PeekMode = true
	m.ToggleFullscreenPeek()
	if !m.PeekFullscreen || !strings.Contains(ansi.Strip(m.View().Content), peekDockPlaceholderText) {
		t.Fatal("an open dock advertised B but did not expand its placeholder")
	}
	m.handleFullscreenPeekKey("esc")
	if m.PeekFullscreen || !m.PeekMode {
		t.Fatal("placeholder did not restore its compact dock")
	}
}
