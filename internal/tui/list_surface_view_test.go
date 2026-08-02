package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestListSurfaceCompactRendererExposesSelectionDisabledStateContinuationAndHelp(
	t *testing.T,
) {
	t.Parallel()

	model := mustListSurface(t, []ListItem{
		{ID: "alpha", PrimaryText: "Alpha"},
		{ID: "bravo", PrimaryText: "Bravo", DisabledReason: "read only"},
		{ID: "charlie", PrimaryText: "Charlie"},
		{ID: "delta", PrimaryText: "Delta"},
	}, nil, ListSurfaceConfig{SelectionMode: ListSelectionMulti})
	if err := model.SetSelectedIDs([]ListItemID{"alpha"}); err != nil {
		t.Fatalf("SetSelectedIDs() error = %v", err)
	}
	if !model.SetCursorToID("bravo") {
		t.Fatal("failed to move cursor to disabled item")
	}

	rendered := NewListSurfaceRenderer(ListSurfaceRenderConfig{
		Width:  40,
		Height: 3,
	}).Render(model)
	if rendered.Layout != ListSurfaceLayoutCompact || rendered.PreviewVisible {
		t.Fatalf("compact result = %+v", rendered)
	}
	for _, want := range []string{"[x]", "disabled: read only", "↓ 2 more", "space toggle"} {
		if !strings.Contains(rendered.Content, want) {
			t.Fatalf("compact rendering omitted %q:\n%s", want, rendered.Content)
		}
	}
	assertListSurfaceWidth(t, rendered.Content, 40)

	if !model.MoveCursorToEnd() {
		t.Fatal("failed to move cursor to final item")
	}
	rendered = NewListSurfaceRenderer(ListSurfaceRenderConfig{
		Width:    40,
		Height:   3,
		HideHelp: true,
	}).Render(model)
	if !strings.Contains(rendered.Content, "↑ 2 more") {
		t.Fatalf("scrolled rendering omitted upper continuation:\n%s", rendered.Content)
	}
}

func TestListSurfaceWideRendererShowsPreviewAndFallsBackAtMinimumWidth(t *testing.T) {
	t.Parallel()

	model := mustListSurface(t, []ListItem{{
		ID:            "alpha",
		PrimaryText:   "Alpha",
		SecondaryText: "Ready",
		Preview: ListPreview{
			Title:   "Alpha preview",
			Summary: "A compact summary.",
			Body:    "Longer preview body.",
			Fields:  []ListPreviewField{{Label: "Project", Value: "Example"}},
		},
	}}, nil, ListSurfaceConfig{})

	wide := NewListSurfaceRenderer(ListSurfaceRenderConfig{
		Width:       90,
		Height:      6,
		Layout:      ListSurfaceLayoutWide,
		ShowPreview: true,
		HideHelp:    true,
	}).Render(model)
	if wide.Layout != ListSurfaceLayoutWide || !wide.PreviewVisible {
		t.Fatalf("wide result = %+v", wide)
	}
	for _, want := range []string{"Alpha preview", "Project: Example", " │ "} {
		if !strings.Contains(wide.Content, want) {
			t.Fatalf("wide rendering omitted %q:\n%s", want, wide.Content)
		}
	}
	if wide.ListWidth < ListSurfaceDefaultListMinWidth ||
		wide.PreviewWidth < ListSurfaceDefaultPreviewMinWidth {
		t.Fatalf("wide pane widths violate minimums: %+v", wide)
	}
	assertListSurfaceWidth(t, wide.Content, 90)

	compact := NewListSurfaceRenderer(ListSurfaceRenderConfig{
		Width:       48,
		Height:      6,
		Layout:      ListSurfaceLayoutWide,
		ShowPreview: true,
		HideHelp:    true,
	}).Render(model)
	if compact.Layout != ListSurfaceLayoutCompact || compact.PreviewVisible {
		t.Fatalf("narrow fallback result = %+v", compact)
	}
	if strings.Contains(compact.Content, "Alpha preview") {
		t.Fatalf("narrow fallback leaked preview pane:\n%s", compact.Content)
	}
	assertListSurfaceWidth(t, compact.Content, 48)
}

func TestListSurfaceRendererKeepsCursorInsideContinuationRowBudget(t *testing.T) {
	t.Parallel()

	categories := []ListCategory{
		{ID: "first", Label: "First"},
		{ID: "second", Label: "Second"},
	}
	items := make([]ListItem, 0, 8)
	for index := range 8 {
		category := ListCategoryID("first")
		if index >= 4 {
			category = "second"
		}
		items = append(items, ListItem{
			ID:          ListItemID(fmt.Sprintf("item-%d", index)),
			CategoryID:  category,
			PrimaryText: fmt.Sprintf("Item %d", index),
		})
	}
	model := mustListSurface(t, items, categories, ListSurfaceConfig{})
	renderer := NewListSurfaceRenderer(ListSurfaceRenderConfig{
		Width:    30,
		Height:   4,
		Layout:   ListSurfaceLayoutCompact,
		HideHelp: true,
	})

	for index := range items {
		if index > 0 && !model.MoveCursor(1) {
			t.Fatalf("cursor did not move to item %d", index)
		}
		rendered := renderer.Render(model)
		if got := lipgloss.Height(rendered.Content); got > 4 {
			t.Fatalf("item %d rendered height = %d, want <= 4:\n%s", index, got, rendered.Content)
		}
		current, ok := model.CurrentItem()
		if !ok {
			t.Fatalf("item %d has no current item", index)
		}
		if want := "› " + current.PrimaryText; !strings.Contains(ansi.Strip(rendered.Content), want) {
			t.Fatalf("cursor row %q is outside the rendered viewport:\n%s", want, ansi.Strip(rendered.Content))
		}
	}

	rendered := renderer.Render(model)
	if !strings.Contains(rendered.Content, "↑") || strings.Contains(rendered.Content, "↓") {
		t.Fatalf("bottom viewport has incorrect continuations:\n%s", rendered.Content)
	}
}

func TestRetainedListSurfaceDomainAdaptersProvideStableTargets(t *testing.T) {
	t.Parallel()

	alpha := &models.Project{
		ID: "alpha", Name: "Alpha", Context: "work",
	}

	states := ListSourceFromWorkflowStates(models.AllWorkflowStates())
	if len(states.Items) != len(models.AllWorkflowStates()) ||
		states.Items[2].ID != "state:doing" {
		t.Fatalf("state adapter = %+v", states.Items)
	}
	priorities := ListSourceFromPriorities([]PriorityInfo{{Priority: 1, Description: "Critical"}})
	projects := ListSourceFromProjects([]*models.Project{alpha})
	tags := ListSourceFromTags([]TagInfo{{Name: "focus", TaskCount: 2}})
	if priorities.Items[0].ID != "priority:1" ||
		projects.Items[0].ID != "project:alpha" ||
		tags.Items[0].CategoryID != "tag:other" {
		t.Fatalf("priority/project/tag adapters are not stable: %+v %+v %+v",
			priorities.Items, projects.Items, tags.Items)
	}
}

func assertListSurfaceWidth(t *testing.T, rendered string, width int) {
	t.Helper()
	for index, line := range strings.Split(rendered, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line %d width = %d, want <= %d: %q", index, got, width, line)
		}
	}
}
