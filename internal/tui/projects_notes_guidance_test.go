package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func TestProjectHubGuidanceFollowsSearchBrowseSelectionAndActions(t *testing.T) {
	hub := openProjectHub(t, projectHubApp(t))

	spec, ok := hub.ContextualGuidance()
	assertGuidanceContains(t, "project search editing", spec, ok,
		"Enter open project", "Esc clear and browse", "↑/↓ or Ctrl+J/K navigate")
	if spec.Rows != 2 {
		t.Fatalf("search-edit guidance rows = %d, want 2", spec.Rows)
	}
	if len(spec.Hints) < 2 || !spec.Hints[0].Required || !spec.Hints[1].Required {
		t.Fatalf("required search controls do not precede browse hints: %+v", spec.Hints)
	}

	_, _ = hub.Update(syntheticSpecialKey(tea.KeyEsc))
	spec, ok = hub.ContextualGuidance()
	assertGuidanceContains(t, "project browse", spec, ok,
		"Enter open project", "Space actions", "n new project", "/ search")
	if spec.Rows != 1 {
		t.Fatalf("browse guidance rows = %d, want 1", spec.Rows)
	}

	hub.SetSearchQuery("alpha")
	hub.applyProjectFilter()
	spec, ok = hub.ContextualGuidance()
	assertGuidanceContains(t, "project applied search", spec, ok, "Esc clear search")
	if !strings.Contains(spec.Supplemental, "search: alpha") {
		t.Fatalf("applied-search supplemental = %q", spec.Supplemental)
	}

	hub.ClearSearch()
	hub.applyProjectFilter()
	hub.toggleProjectHubSelection()
	spec, ok = hub.ContextualGuidance()
	assertGuidanceContains(t, "project selection", spec, ok, "Space bulk actions", "v toggle", "a/A all")
	if spec.Supplemental != "1 selected" {
		t.Fatalf("selection supplemental = %q", spec.Supplemental)
	}

	hub.surface.ClearSelection()
	hub.openProjectHubActions()
	if hub.actions == nil {
		t.Fatal("project action sheet did not open")
	}
	spec, ok = hub.ContextualGuidance()
	assertGuidanceContains(t, "project actions", spec, ok, "Enter run", "Esc close", "/ search")
	_, _ = hub.Update(syntheticKey('/'))
	spec, ok = hub.ContextualGuidance()
	assertGuidanceContains(t, "project action search", spec, ok, "Enter apply search", "Esc cancel search")
	if spec.Rows != 2 {
		t.Fatalf("action-search guidance rows = %d, want 2", spec.Rows)
	}
}

func TestProjectHubGuidanceUsesCausalEmptyRecovery(t *testing.T) {
	hub := openProjectHub(t, projectHubApp(t))
	_, _ = hub.Update(syntheticSpecialKey(tea.KeyEsc))
	if err := hub.surface.Reload(nil, hub.projectHubCategories()); err != nil {
		t.Fatalf("empty Project Hub surface: %v", err)
	}
	spec, ok := hub.ContextualGuidance()
	assertGuidanceContains(t, "empty projects", spec, ok, "n new project")
	if len(spec.Hints) == 0 || !spec.Hints[0].Required {
		t.Fatalf("empty-project recovery is not required: %+v", spec.Hints)
	}
}

func TestNotesGuidanceFollowsBrowseSearchRenameMoveAndDelete(t *testing.T) {
	project := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	service := &combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{
		app.GlobalNoteLocation(): {{Title: "Global note", Filename: "global.md"}},
	}}
	view := NewNotesViewModelWithService(t.TempDir(), AllNotesScope(), service, nil, project)
	view.SetSize(80, 24)

	spec, ok := view.ContextualGuidance()
	assertGuidanceContains(t, "notes browse", spec, ok, "Enter open/edit", "n new", "v scope", "s search")
	if !strings.Contains(spec.Supplemental, "location: Global") {
		t.Fatalf("browse guidance omits location context: %+v", spec)
	}

	_, _ = view.Update(syntheticKey('s'))
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

	_, _ = view.Update(syntheticKey('r'))
	spec, ok = view.ContextualGuidance()
	assertRequiredGuidance(t, "notes rename", spec, ok, "Enter save", "Esc cancel")
	_, _ = view.Update(syntheticSpecialKey(tea.KeyEsc))

	_, _ = view.Update(syntheticKey('m'))
	if view.locationPicker == nil || view.locationPicker.purpose != notesLocationPickerMove {
		t.Fatal("move did not open the location picker")
	}
	spec, ok = view.ContextualGuidance()
	assertRequiredGuidance(t, "notes move", spec, ok, "Enter move", "Esc cancel")
	_, cancelCmd := view.Update(syntheticSpecialKey(tea.KeyEsc))
	_, _ = view.Update(cancelCmd())

	_, _ = view.Update(syntheticKey('x'))
	spec, ok = view.ContextualGuidance()
	assertRequiredGuidance(t, "notes delete", spec, ok, "Y delete", "N/Esc cancel")
}

func TestNotesGuidanceCausalEmptyAndFooterHeightKeepSelectionVisible(t *testing.T) {
	empty := NewNotesViewModelWithService(
		t.TempDir(), AllNotesScope(),
		&combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{}}, nil,
	)
	empty.SetSize(32, 12)
	spec, ok := empty.ContextualGuidance()
	assertRequiredGuidance(t, "empty notes", spec, ok, "n choose location")
	_, _ = empty.Update(syntheticKey('n'))
	spec, ok = empty.ContextualGuidance()
	assertRequiredGuidance(t, "notes creation location", spec, ok, "Enter choose", "Esc cancel")
	assertGuidanceFits(t, spec, 24)

	notes := make([]*models.Note, 0, 24)
	for i := range 24 {
		notes = append(notes, &models.Note{
			Title: fmt.Sprintf("Note %02d", i), Filename: fmt.Sprintf("note-%02d.md", i),
		})
	}
	view := NewNotesViewModelWithService(
		t.TempDir(), GlobalNotesScope(),
		&combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{
			app.GlobalNoteLocation(): notes,
		}}, nil,
	)
	view.SetSize(48, 12)
	view.cursor = len(view.visibleRows) - 1
	_ = view.View()
	_, _ = view.Update(syntheticKey('s'))
	rendered := view.View().Content
	if !strings.Contains(rendered, "Note 23") || view.scrollOffset == 0 {
		t.Fatalf("two-row search footer lost persistent selection: offset=%d\n%s", view.scrollOffset, rendered)
	}
}

func assertGuidanceContains(t *testing.T, state string, spec GuidanceSpec, ok bool, want ...string) {
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

func assertRequiredGuidance(t *testing.T, state string, spec GuidanceSpec, ok bool, want ...string) {
	t.Helper()
	assertGuidanceContains(t, state, spec, ok, want...)
	for _, expected := range want {
		found := false
		for _, hint := range spec.Hints {
			if guidanceHintText(hint) == expected && hint.Required {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s guidance does not require %q: %+v", state, expected, spec.Hints)
		}
	}
}

func assertGuidanceFits(t *testing.T, spec GuidanceSpec, width int) {
	t.Helper()
	lines := guidanceLines(spec, width)
	if len(lines) > max(spec.Rows, 1) {
		t.Fatalf("guidance uses %d lines with row budget %d: %v", len(lines), spec.Rows, lines)
	}
	for _, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("guidance line width = %d, want <= %d: %q", got, width, line)
		}
	}
}
