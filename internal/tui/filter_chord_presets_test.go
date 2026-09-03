package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestFilterChordPresetsCarryValuesAndStatuses(t *testing.T) {
	app := newFilterChordTestApp(t)
	ctx := simplifiedOperationContext()
	filter := &app.crossOutliner.filter.CommonFilterState

	presets := []struct {
		path   []string
		status string
		verify func(t *testing.T)
	}{
		{
			path:   []string{"f", "!", "3"},
			status: "Filtering Low priority or higher",
			verify: func(t *testing.T) {
				if filter.Priority != 3 {
					t.Errorf("Priority = %d, want 3", filter.Priority)
				}
			},
		},
		{
			path:   []string{"f", "!", "2"},
			status: "Filtering Medium priority or higher",
			verify: func(t *testing.T) {
				if filter.Priority != 2 {
					t.Errorf("Priority = %d, want 2", filter.Priority)
				}
			},
		},
		{
			path:   []string{"f", "!", "0"},
			status: "Priority filter cleared",
			verify: func(t *testing.T) {
				if filter.Priority != 0 {
					t.Errorf("Priority = %d, want 0", filter.Priority)
				}
			},
		},
	}
	for _, preset := range presets {
		t.Run(strings.Join(preset.path, ""), func(t *testing.T) {
			result := executeDirectPath(t, app, ctx, preset.path)
			if result.Err != nil {
				t.Fatalf("Execute() error: %v", result.Err)
			}
			if result.ValueRequired != nil {
				t.Fatal("preset chord asked for a value it already carries")
			}
			if result.Status != preset.status {
				t.Errorf("Status = %q, want %q", result.Status, preset.status)
			}
			preset.verify(t)
		})
	}
}

func TestFilterChordClearResetsEveryFacet(t *testing.T) {
	app := newFilterChordTestApp(t)
	out := app.crossOutliner
	filter := &out.filter.CommonFilterState

	filter.Priority = 2
	filter.DueDate = query.DueDateToday
	filter.Tags = map[string]bool{"deep": true}
	// Both halves of the context facet, because fx must clear the gap row as
	// well as the vocabulary: leaving NoContext set would keep a live predicate
	// behind chrome that reports every filter cleared.
	filter.Contexts = map[string]bool{"errand": true}
	filter.NoContext = true
	filter.Deferred = true
	set, err := models.NewWorkflowStateSet(models.StateWaiting)
	if err != nil {
		t.Fatal(err)
	}
	filter.SetStateFilter(set)

	result := executeDirectPath(t, app, simplifiedOperationContext(), []string{"f", "x"})
	if result.Err != nil {
		t.Fatalf("fx error: %v", result.Err)
	}
	if result.Status != "Filters cleared" {
		t.Errorf("Status = %q, want %q", result.Status, "Filters cleared")
	}
	if query.HasActiveCommonFilters(filter) {
		t.Errorf("common filters still active after fx: %v", query.CommonFilterParts(filter))
	}
}

func TestFilteredEmptyStateAdvertisesWorkingClearChord(t *testing.T) {
	app := newFilterChordTestApp(t)
	out := app.crossOutliner
	out.filter.Tags = map[string]bool{"missing": true}
	out.adoptFilterCarriers()
	out.crossForceRebuild = true
	out.rebuildRows()

	if visible := visibleOutlinerTaskIDs(out); len(visible) != 0 {
		t.Fatalf("missing-tag filter left tasks visible: %v", visible)
	}
	const want = "No tasks match filters. Press fx to clear filters."
	if got := out.outlinerEmptyMessage(); got != want {
		t.Fatalf("filtered empty state = %q, want %q", got, want)
	}

	pressFilterChord(t, app, "fx")
	if visible := visibleOutlinerTaskIDs(out); len(visible) != 3 {
		t.Fatalf("advertised fx chord restored %d tasks, want 3: %v", len(visible), visible)
	}
}

func TestFilterChordPriorityThresholdNarrowsRows(t *testing.T) {
	app := newFilterChordTestApp(t)
	out := app.crossOutliner

	before := visibleOutlinerTaskIDs(out)
	for _, id := range []string{"critical", "medium", "low"} {
		if !before[id] {
			t.Fatalf("task %q missing before filtering; visible = %v", id, before)
		}
	}

	if cmd := pressFilterChord(t, app, "f!2"); cmd == nil {
		t.Fatal("f!2 produced no status command")
	}
	if out.filter.Priority != 2 {
		t.Fatalf("Priority = %d, want 2", out.filter.Priority)
	}
	visible := visibleOutlinerTaskIDs(out)
	if !visible["critical"] || !visible["medium"] {
		t.Errorf("f!2 should keep High and Medium tasks; visible = %v", visible)
	}
	if visible["low"] {
		t.Errorf("f!2 should hide the low-priority task; visible = %v", visible)
	}

	pressFilterChord(t, app, "f!3")
	if out.filter.Priority != 3 {
		t.Fatalf("Priority = %d, want 3", out.filter.Priority)
	}
	if visible := visibleOutlinerTaskIDs(out); !visible["low"] {
		t.Errorf("f!3 should keep the low-priority task; visible = %v", visible)
	}

	pressFilterChord(t, app, "fx")
	if out.filter.Priority != 0 {
		t.Fatalf("Priority = %d after fx, want 0", out.filter.Priority)
	}
	if visible := visibleOutlinerTaskIDs(out); !visible["low"] {
		t.Errorf("fx should restore all tasks; visible = %v", visible)
	}
}

// Deferral is a scheduling instant like a due date, so it is a row on the f@
// picker rather than a chord of its own. The row and the four due-date rows are
// mutually exclusive: choosing one has to clear the other field.
func TestFilterDuePickerDeferredRowOwnsTheDeferredFacet(t *testing.T) {
	app := newFilterChordTestApp(t)
	filter := &app.crossOutliner.filter.CommonFilterState
	filter.DueDate = query.DueDateToday

	result := app.executeFilterOperation(OperationInvocation{
		Descriptor: OperationDescriptor{ID: operationFilterDue},
		Value:      "?deferred",
	})
	if result.Err != nil {
		t.Fatalf("deferred row error: %v", result.Err)
	}
	if !filter.Deferred {
		t.Fatal("the deferred row did not enable the deferred-only facet")
	}
	if filter.DueDate != query.DueDateAll {
		t.Fatalf("DueDate = %v, want the due-date facet cleared", filter.DueDate)
	}

	result = app.executeFilterOperation(OperationInvocation{
		Descriptor: OperationDescriptor{ID: operationFilterDue},
		Value:      "@overdue",
	})
	if result.Err != nil {
		t.Fatalf("overdue row error: %v", result.Err)
	}
	if filter.Deferred {
		t.Fatalf("a due-date row left deferred enabled: %+v", filter)
	}

	pressFilterChord(t, app, "fx")
	if filter.Deferred || filter.DueDate != query.DueDateAll {
		t.Fatalf("fx left the f@ facets active: %+v", filter)
	}
}

func TestFilterChordPickersOpenSeededFromTheActiveFilter(t *testing.T) {
	t.Run("f@ due date", func(t *testing.T) {
		app := newFilterChordTestApp(t)
		app.crossOutliner.filter.DueDate = query.DueDateThisWeek
		app.crossOutliner.adoptFilterCarriers()

		cmd := pressFilterChord(t, app, "f@")
		if cmd == nil {
			t.Fatal("f@ produced no value-request command")
		}
		app.handleOperationOverlayMessage(cmd())
		if app.operationValuePicker == nil || app.operationValuePicker.provider != ValueProviderFilterDue {
			t.Fatalf("f@ did not open the due-date picker: %#v", app.operationValuePicker)
		}
		items := app.operationValuePicker.surface.Items()
		wantIDs := []ListItemID{
			"filter-due:overdue",
			"filter-due:today",
			"filter-due:week",
			"filter-due:none",
			"filter-due:deferred",
		}
		if len(items) != len(wantIDs) {
			t.Fatalf("due-date picker items = %v, want %v", items, wantIDs)
		}
		for index, wantID := range wantIDs {
			if items[index].ID != wantID {
				t.Errorf("due-date picker item %d = %s, want %s", index, items[index].ID, wantID)
			}
		}
		if current, ok := app.operationValuePicker.surface.CurrentItem(); !ok || current.ID != "filter-due:week" {
			t.Errorf("seeded cursor = %v (%v), want filter-due:week", current.ID, ok)
		}
	})

	t.Run("f$ states", func(t *testing.T) {
		app := newFilterChordTestApp(t)
		set, err := models.NewWorkflowStateSet(models.StateWaiting)
		if err != nil {
			t.Fatal(err)
		}
		app.crossOutliner.filter.SetStateFilter(set)
		app.crossOutliner.adoptFilterCarriers()

		cmd := pressFilterChord(t, app, "f$")
		if cmd == nil {
			t.Fatal("f$ produced no value-request command")
		}
		app.handleOperationOverlayMessage(cmd())
		if app.operationValuePicker == nil || app.operationValuePicker.provider != ValueProviderFilterStates {
			t.Fatalf("f$ did not open the states picker: %#v", app.operationValuePicker)
		}
		selected := app.operationValuePicker.surface.SelectedIDs()
		if len(selected) != 1 || selected[0] != "state:waiting" {
			t.Errorf("seeded selection = %v, want [state:waiting]", selected)
		}
	})

	t.Run("fa completion age", func(t *testing.T) {
		app := newFilterChordTestApp(t)
		app.crossOutliner.filter.SetCompletionAgeFilter(query.CompletionAge30Days)
		app.crossOutliner.adoptFilterCarriers()

		cmd := pressFilterChord(t, app, "fa")
		if cmd == nil {
			t.Fatal("fa produced no value-request command")
		}
		app.handleOperationOverlayMessage(cmd())
		if app.operationValuePicker == nil || app.operationValuePicker.provider != ValueProviderFilterCompletionAge {
			t.Fatalf("fa did not open the completion-age picker: %#v", app.operationValuePicker)
		}
		items := app.operationValuePicker.surface.Items()
		wantIDs := []ListItemID{
			"filter-completion-age:all",
			"filter-completion-age:7",
			"filter-completion-age:30",
			"filter-completion-age:90",
		}
		if len(items) != len(wantIDs) {
			t.Fatalf("completion-age picker items = %v, want %v", items, wantIDs)
		}
		for index, wantID := range wantIDs {
			if items[index].ID != wantID {
				t.Errorf("completion-age picker item %d = %s, want %s", index, items[index].ID, wantID)
			}
		}
		if current, ok := app.operationValuePicker.surface.CurrentItem(); !ok || current.ID != "filter-completion-age:30" {
			t.Errorf("seeded cursor = %v (%v), want filter-completion-age:30", current.ID, ok)
		}
	})

	t.Run("f# tags", func(t *testing.T) {
		app := newFilterChordTestApp(t)
		app.crossOutliner.filter.Tags = map[string]bool{"deep": true}
		app.crossOutliner.adoptFilterCarriers()

		cmd := pressFilterChord(t, app, "f#")
		if cmd == nil {
			t.Fatal("f# produced no value-request command")
		}
		app.handleOperationOverlayMessage(cmd())
		if app.operationValuePicker == nil || app.operationValuePicker.provider != ValueProviderFilterTags {
			t.Fatalf("f# did not open the tags picker: %#v", app.operationValuePicker)
		}
		selected := app.operationValuePicker.surface.SelectedIDs()
		if len(selected) != 1 || selected[0] != "tag:deep" {
			t.Errorf("seeded selection = %v, want [tag:deep]", selected)
		}
		// The filter picker builds its rows from taskTagListSource, so it wears
		// the same colors as # with no code of its own. This asserts that
		// inheritance rather than restating it: the fixture carries the review
		// workflow tag and the deep descriptive one, so both rules are exercised.
		items := app.operationValuePicker.surface.Items()
		if len(items) == 0 {
			t.Fatal("f# picker has no rows to color")
		}
		for _, item := range items {
			assertRowWearsTheColorItApplies(t, item)
		}
	})
}

func TestFilterStatePickerShowsProspectiveCountsFromOtherActiveFilters(t *testing.T) {
	app := newUITestApp()
	project := &models.Project{
		ID:   "project",
		Name: "Project",
		Slug: "project",
		Tasks: []*models.Task{
			{ID: "inbox-1", Title: "Inbox one", WorkflowState: models.StateInbox, Priority: 1, Tags: []string{"focus"}},
			{ID: "inbox-2", Title: "Inbox two", WorkflowState: models.StateInbox, Priority: 2, Tags: []string{"focus"}},
			{ID: "inbox-other-tag", Title: "Other tag", WorkflowState: models.StateInbox, Priority: 1, Tags: []string{"other"}},
			{ID: "ready", Title: "Ready", WorkflowState: models.StateBacklog, Priority: 1, Tags: []string{"focus"}},
			{ID: "waiting-too-low", Title: "Waiting", WorkflowState: models.StateWaiting, Priority: 3, Tags: []string{"focus"}},
			{ID: "done", Title: "Done", WorkflowState: models.StateDone, Priority: 1, Tags: []string{"focus"}},
		},
	}
	project.BuildTaskTree()
	setTestAppProjects(app, project)
	outliner := app.crossOutliner
	outliner.loadedProjects = []*models.Project{project}
	outliner.filter.Priority = 2
	outliner.filter.Tags = map[string]bool{"focus": true}
	outliner.adoptFilterCarriers()
	selectedStates, err := models.NewWorkflowStateSet(models.StateWaiting)
	if err != nil {
		t.Fatal(err)
	}
	outliner.filter.SetStateFilter(selectedStates)
	outliner.adoptFilterCarriers()
	outliner.crossForceRebuild = true
	outliner.rebuildRows()

	picker, err := app.newFilterStatesPicker(OperationExecutionRequest{})
	if err != nil {
		t.Fatalf("newFilterStatesPicker() error = %v", err)
	}
	items := make(map[ListItemID]ListItem)
	for _, item := range picker.surface.Items() {
		items[item.ID] = item
	}

	for id, want := range map[ListItemID]string{
		"state:inbox":   "inbox ⋅ 2",
		"state:backlog": "backlog ⋅ 1",
		"state:waiting": "waiting ⋅ 0",
		"state:done":    "done ⋅ 1",
	} {
		item, ok := items[id]
		if !ok {
			t.Fatalf("state picker omitted %s", id)
		}
		if got := ansi.Strip(item.SecondaryText); got != want {
			t.Errorf("%s secondary text = %q, want %q", id, got, want)
		}
	}

	waiting := items["state:waiting"].SecondaryText
	lightRedZero := lipgloss.NewStyle().Foreground(theme.ColorError).Faint(true).Render("0")
	if !strings.HasSuffix(waiting, "⋅ "+lightRedZero) {
		t.Errorf("zero count is not rendered with the prospective zero style: %q", waiting)
	}
	if got := outliner.filter.States.Values(); len(got) != 1 || got[0] != models.StateWaiting {
		t.Errorf("building counts mutated active state filter to %v", got)
	}

	rendered := ansi.Strip(picker.render(120))
	for _, want := range []string{"inbox ⋅ 2", "waiting ⋅ 0"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered state picker omitted %q:\n%s", want, rendered)
		}
	}
}
