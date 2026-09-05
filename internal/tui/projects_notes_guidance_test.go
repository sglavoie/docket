package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/route"
)

// The Project Hub's own guidance moved to internal/tui/projecthub with the
// surface. The shared assertions below stay here because the Notes manager,
// which is still the shell's to mount, is their other caller.

func TestNotesGuidanceFollowsBrowseSearchRenameMoveAndDelete(t *testing.T) {
	project := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	service := &combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{
		app.GlobalNoteLocation(): {{Title: "Global note", Filename: "global.md"}},
	}}
	view := notes.NewNotesViewModelWithService(t.TempDir(), route.AllNotesScope(), service, nil, project)
	shell := mountNotesManager(t, view, route.AllNotesScope())
	view.SetSize(80, 24)

	spec, ok := view.ContextualGuidance()
	assertGuidanceContains(t, "notes browse", spec, ok, "Enter open/edit", "a new", "p scope", "/ search")
	if !strings.Contains(spec.Supplemental, "location: Global") {
		t.Fatalf("browse guidance omits location context: %+v", spec)
	}

	_, _ = view.Update(syntheticKey('/'))
	spec, ok = view.ContextualGuidance()
	assertRequiredGuidance(t, "notes search", spec, ok, "Enter apply search", "Esc cancel edit")
	for _, r := range "missing" {
		_, _ = view.Update(syntheticKey(r))
	}
	spec, ok = view.ContextualGuidance()
	if !ok || spec.Supplemental != "no matches" {
		t.Fatalf("live no-match guidance = (%+v, %v)", spec, ok)
	}
	_, _ = view.Update(syntheticSpecialKey(tea.KeyEnter))
	spec, ok = view.ContextualGuidance()
	assertRequiredGuidance(t, "notes applied no-match", spec, ok, "Esc clear search")
	_, _ = view.Update(syntheticSpecialKey(tea.KeyEsc))

	_, _ = view.Update(syntheticKey('e'))
	spec, ok = view.ContextualGuidance()
	assertRequiredGuidance(t, "notes rename", spec, ok, "Enter save", "Esc cancel")
	_, _ = view.Update(syntheticSpecialKey(tea.KeyEsc))

	_, _ = view.Update(syntheticKey('%'))
	if view.LocationPicker == nil || view.LocationPicker.Purpose != notes.NotesLocationPickerMove {
		t.Fatal("move did not open the location picker")
	}
	spec, ok = view.ContextualGuidance()
	assertRequiredGuidance(t, "notes move", spec, ok, "Enter move", "Esc cancel")
	_, cancelCmd := view.Update(syntheticSpecialKey(tea.KeyEsc))
	_, _ = view.Update(cancelCmd())

	// Delete has no direct key: the Space sheet is its one path, and choosing it
	// hands the confirmation back to the manager that owns the row.
	_, _ = view.Update(syntheticKey(' '))
	if shell.operations.ActionSheet == nil {
		t.Fatal("Space did not open the note action sheet")
	}
	handled, sheetCmd := shell.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !handled || sheetCmd == nil {
		t.Fatalf("sheet Enter = handled %t cmd %v", handled, sheetCmd)
	}
	if handled, _ := shell.handleOperationOverlayMessage(sheetCmd()); !handled {
		t.Fatal("the sheet's Delete row was not executed")
	}
	spec, ok = view.ContextualGuidance()
	assertRequiredGuidance(t, "notes delete", spec, ok, "Y delete", "N/Esc cancel")
}

func TestNotesGuidanceCausalEmptyAndFooterHeightKeepSelectionVisible(t *testing.T) {
	empty := notes.NewNotesViewModelWithService(
		t.TempDir(), route.AllNotesScope(),
		&combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{}}, nil,
	)
	mountNotesManager(t, empty, route.AllNotesScope())
	empty.SetSize(32, 12)
	spec, ok := empty.ContextualGuidance()
	assertRequiredGuidance(t, "empty notes", spec, ok, "a choose location")
	_, _ = empty.Update(syntheticKey('a'))
	spec, ok = empty.ContextualGuidance()
	assertRequiredGuidance(t, "notes creation location", spec, ok, "Enter choose", "Esc cancel")
	assertGuidanceFits(t, spec, 24)

	manager := make([]*models.Note, 0, 24)
	for i := range 24 {
		manager = append(manager, &models.Note{
			Title: fmt.Sprintf("Note %02d", i), Filename: fmt.Sprintf("note-%02d.md", i),
		})
	}
	view := notes.NewNotesViewModelWithService(
		t.TempDir(), route.GlobalNotesScope(),
		&combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{
			app.GlobalNoteLocation(): manager,
		}}, nil,
	)
	mountNotesManager(t, view, route.GlobalNotesScope())
	view.SetSize(48, 12)
	view.Cursor = len(view.VisibleRows) - 1
	_ = view.View()
	_, _ = view.Update(syntheticKey('/'))
	rendered := view.View().Content
	if !strings.Contains(rendered, "Note 23") || view.ScrollOffset == 0 {
		t.Fatalf("two-row search footer lost persistent selection: offset=%d\n%s", view.ScrollOffset, rendered)
	}
}

func assertGuidanceContains(t *testing.T, state string, spec chrome.GuidanceSpec, ok bool, want ...string) {
	t.Helper()
	if !ok {
		t.Fatalf("%s guidance is unavailable", state)
	}
	got := guidanceEntryText(spec)
	for _, entry := range want {
		if !strings.Contains(got, entry) {
			t.Errorf("%s guidance = %q, want %q", state, got, entry)
		}
	}
}

func assertRequiredGuidance(t *testing.T, state string, spec chrome.GuidanceSpec, ok bool, want ...string) {
	t.Helper()
	assertGuidanceContains(t, state, spec, ok, want...)
	for _, expected := range want {
		found := false
		for _, hint := range spec.Hints {
			if chrome.GuidanceHintText(hint) == expected && hint.Required {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s guidance does not require %q: %+v", state, expected, spec.Hints)
		}
	}
}

func assertGuidanceFits(t *testing.T, spec chrome.GuidanceSpec, width int) {
	t.Helper()
	lines := chrome.GuidanceLines(spec, width)
	if len(lines) > max(spec.Rows, 1) {
		t.Fatalf("guidance uses %d lines with row budget %d: %v", len(lines), spec.Rows, lines)
	}
	for _, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("guidance line width = %d, want <= %d: %q", got, width, line)
		}
	}
}
