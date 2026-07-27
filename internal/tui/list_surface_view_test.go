package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

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
		Height: 2,
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
		Height:   2,
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

func TestListSurfaceDomainAdaptersProvideStableQualifiedTargets(t *testing.T) {
	t.Parallel()

	taskA, err := models.NewTask("same-task", 1, models.TaskTypeTask, "Alpha task")
	if err != nil {
		t.Fatalf("NewTask(alpha) error = %v", err)
	}
	taskB, err := models.NewTask("same-task", 1, models.TaskTypeTask, "Bravo task")
	if err != nil {
		t.Fatalf("NewTask(bravo) error = %v", err)
	}
	alpha := &models.Project{
		ID: "alpha", Name: "Alpha", Context: "work", Tasks: []*models.Task{taskA},
	}
	bravo := &models.Project{
		ID: "bravo", Name: "Bravo", Context: "home", Tasks: []*models.Task{taskB},
	}
	taskSource := ListSourceFromTasks([]*models.Project{alpha, bravo})
	if len(taskSource.Items) != 2 ||
		taskSource.Items[0].ID == taskSource.Items[1].ID {
		t.Fatalf("task adapter did not qualify duplicate IDs: %+v", taskSource.Items)
	}
	if taskSource.Items[0].ID != QualifiedTaskListItemID("alpha", "same-task") {
		t.Fatalf("qualified task ID = %q", taskSource.Items[0].ID)
	}
	payload, ok := taskSource.Items[0].Data.(TaskListSourceData)
	if !ok || payload.Task != taskA || payload.Project != alpha {
		t.Fatalf("task payload = (%+v, %v)", payload, ok)
	}
	if _, err := taskSource.NewModel(ListSurfaceConfig{}); err != nil {
		t.Fatalf("task source validation error = %v", err)
	}

	states := ListSourceFromWorkflowStates(models.AllWorkflowStates())
	if len(states.Items) != len(models.AllWorkflowStates()) ||
		states.Items[3].ID != "state:in_progress" {
		t.Fatalf("state adapter = %+v", states.Items)
	}
	modes := ListSourceFromOutlinerModes(OutlinerModeCatalog())
	if len(modes.Items) != len(OutlinerModeCatalog()) || len(modes.Categories) != 3 {
		t.Fatalf("mode adapter sizes = %d items %d categories",
			len(modes.Items), len(modes.Categories))
	}
	scopes := ListSourceFromContextScopes([]ContextScopeSummary{
		{Scope: AllContextScope(), ActiveProjectCount: 2},
		{Scope: CustomContextScope("Focus"), ActiveProjectCount: 1},
	})
	if scopes.Items[0].PrimaryText != "All Projects" ||
		scopes.Items[1].CategoryID != "scope:custom" {
		t.Fatalf("scope adapter = %+v", scopes.Items)
	}
	priorities := ListSourceFromPriorities([]PriorityInfo{{Priority: 1, Description: "Critical"}})
	projects := ListSourceFromProjects([]*models.Project{alpha})
	tags := ListSourceFromTags([]TagInfo{{Name: "focus", TaskCount: 2, Pinned: true}})
	if priorities.Items[0].ID != "priority:1" ||
		projects.Items[0].ID != "project:alpha" ||
		tags.Items[0].CategoryID != "tag:pinned" {
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
