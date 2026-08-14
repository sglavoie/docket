package tui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func newFilterChordTestApp(t *testing.T) *AppModel {
	t.Helper()
	app := newUITestApp()
	project := &models.Project{
		ID:   "project",
		Name: "Project",
		Slug: "project",
		Tasks: []*models.Task{
			{ID: "critical", ShortID: 1, Title: "Critical", WorkflowState: models.StateBacklog, Priority: 1},
			{ID: "medium", ShortID: 2, Title: "Medium", WorkflowState: models.StateBacklog, Priority: 2, Tags: []string{reviewTaskTag}},
			{ID: "low", ShortID: 3, Title: "Low", WorkflowState: models.StateWaiting, Priority: 3, Tags: []string{"deep"}},
		},
	}
	project.BuildTaskTree()
	setTestAppProjects(app, project)
	app.crossOutliner.loadedProjects = []*models.Project{project}
	app.crossOutliner.crossForceRebuild = true
	app.crossOutliner.rebuildRows()
	return app
}

func pressFilterChord(t *testing.T, app *AppModel, keys string) tea.Cmd {
	t.Helper()
	var cmd tea.Cmd
	var handled bool
	for _, key := range keys {
		handled, cmd = app.handleGlobalKeyPress(tea.KeyPressMsg{Code: key, Text: string(key)})
		if !handled {
			t.Fatalf("key %q of chord %q was not handled", key, keys)
		}
	}
	if len(app.pendingChord) != 0 {
		t.Fatalf("chord %q left pending prefix %v", keys, app.pendingChord)
	}
	return cmd
}

func TestFilterChordWhichKeyHints(t *testing.T) {
	app := newFilterChordTestApp(t)

	if handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'f', Text: "f"}); !handled || cmd != nil {
		t.Fatalf("f = handled %v cmd %T; want a pending prefix", handled, cmd)
	}
	row := pendingChordHintRowForTest(app)
	for _, want := range []string{
		"p priority threshold",
		"c completion age",
		"@ due",
		"s states",
		"t tags",
		"r review",
		"d deferred",
		"x clear all",
	} {
		if !strings.Contains(row, want) {
			t.Errorf("f which-key row %q omits %q", row, want)
		}
	}

	if handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'p', Text: "p"}); !handled || cmd != nil {
		t.Fatalf("fp = handled %v cmd %T; want a pending prefix", handled, cmd)
	}
	row = pendingChordHintRowForTest(app)
	for _, want := range []string{"0 clear", "1 high", "2 medium+", "3 low+"} {
		if !strings.Contains(row, want) {
			t.Errorf("fp which-key row %q omits %q", row, want)
		}
	}
}

func TestFilterChordWhichKeyHintsShowEffectiveValues(t *testing.T) {
	app := newFilterChordTestApp(t)
	taskFilter := &app.crossOutliner.filter.CommonFilterState
	taskFilter.Priority = models.PriorityHigh
	taskFilter.DueDate = DueDateToday
	taskFilter.Tags = map[string]bool{reviewTaskTag: true, "deep": true}
	taskFilter.Deferred = true
	terminal, err := models.NewWorkflowStateSet(models.StateDone, models.StateCanceled)
	if err != nil {
		t.Fatal(err)
	}
	taskFilter.setStateFilter(terminal, true)
	taskFilter.setCompletionAgeFilter(CompletionAge90Days)

	if handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'f', Text: "f"}); !handled {
		t.Fatal("f did not open its pending chord")
	}
	row := pendingChordHintRowForTest(app)
	for _, want := range []string{
		"p priority:high+", "s states:2", "c completed:90d+", "@ due:today",
		"t tags:2", "r review:on", "d deferred:on", "x clear:5",
	} {
		if !strings.Contains(row, want) {
			t.Errorf("decorated f row %q omits %q", row, want)
		}
	}
	if got := app.crossOutliner.ActiveFilterCount(); got != 5 {
		t.Errorf("task filter count = %d, want 5 independently clearable facets", got)
	}

	app.pendingChord = []string{"F"}
	projectFilter := &app.crossOutliner.projectFilter
	projectFilter.Tags = map[string]bool{"client": true}
	projectFilter.Untagged = true
	projectFilter.ParkedMode = projectPopulationInclude
	projectFilter.ArchivedMode = projectPopulationInclude
	row = pendingChordHintRowForTest(app)
	for _, want := range []string{
		"t project tags:3", "p parked:on", "P only parked",
		"a archived:on", "A only archived", "x clear:3",
	} {
		if !strings.Contains(row, want) {
			t.Errorf("decorated F row %q omits %q", row, want)
		}
	}

	projectFilter.ParkedMode = projectPopulationOnly
	projectFilter.ArchivedMode = projectPopulationOnly
	row = pendingChordHintRowForTest(app)
	for _, want := range []string{
		"p include parked", "P parked:only", "a include archived", "A archived:only",
	} {
		if !strings.Contains(row, want) {
			t.Errorf("isolated F row %q omits %q", row, want)
		}
	}
	if got := app.crossOutliner.ActiveFilterCount(); got != 8 {
		t.Errorf("combined filter count = %d, want 8 independent task/project facets", got)
	}
}

func TestReviewFilterChordTogglesCanonicalTagFacet(t *testing.T) {
	app := newFilterChordTestApp(t)
	filter := &app.crossOutliner.filter.CommonFilterState
	pressFilterChord(t, app, "fr")
	if !filter.Tags[reviewTaskTag] {
		t.Fatalf("fr filter = %v", filter.Tags)
	}
	if visible := visibleOutlinerTaskIDs(app.crossOutliner); !visible["medium"] || visible["critical"] || visible["low"] {
		t.Fatalf("Review filter visible = %v", visible)
	}
	pressFilterChord(t, app, "fr")
	if filter.Tags[reviewTaskTag] {
		t.Fatalf("second fr filter = %v", filter.Tags)
	}
}

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
			path:   []string{"f", "p", "3"},
			status: "Filtering Low priority or higher",
			verify: func(t *testing.T) {
				if filter.Priority != 3 {
					t.Errorf("Priority = %d, want 3", filter.Priority)
				}
			},
		},
		{
			path:   []string{"f", "p", "2"},
			status: "Filtering Medium priority or higher",
			verify: func(t *testing.T) {
				if filter.Priority != 2 {
					t.Errorf("Priority = %d, want 2", filter.Priority)
				}
			},
		},
		{
			path:   []string{"f", "p", "0"},
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
	filter.DueDate = DueDateToday
	filter.Tags = map[string]bool{"deep": true}
	filter.Deferred = true
	set, err := models.NewWorkflowStateSet(models.StateWaiting)
	if err != nil {
		t.Fatal(err)
	}
	filter.setStateFilter(set, true)

	result := executeDirectPath(t, app, simplifiedOperationContext(), []string{"f", "x"})
	if result.Err != nil {
		t.Fatalf("fx error: %v", result.Err)
	}
	if result.Status != "Filters cleared" {
		t.Errorf("Status = %q, want %q", result.Status, "Filters cleared")
	}
	if hasActiveCommonFilters(filter) {
		t.Errorf("common filters still active after fx: %v", commonFilterParts(filter))
	}
}

func TestFilteredEmptyStateAdvertisesWorkingClearChord(t *testing.T) {
	app := newFilterChordTestApp(t)
	out := app.crossOutliner
	out.filter.Tags = map[string]bool{"missing": true}
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

	if cmd := pressFilterChord(t, app, "fp2"); cmd == nil {
		t.Fatal("fp2 produced no status command")
	}
	if out.filter.Priority != 2 {
		t.Fatalf("Priority = %d, want 2", out.filter.Priority)
	}
	visible := visibleOutlinerTaskIDs(out)
	if !visible["critical"] || !visible["medium"] {
		t.Errorf("fp2 should keep High and Medium tasks; visible = %v", visible)
	}
	if visible["low"] {
		t.Errorf("fp2 should hide the low-priority task; visible = %v", visible)
	}

	pressFilterChord(t, app, "fp3")
	if out.filter.Priority != 3 {
		t.Fatalf("Priority = %d, want 3", out.filter.Priority)
	}
	if visible := visibleOutlinerTaskIDs(out); !visible["low"] {
		t.Errorf("fp3 should keep the low-priority task; visible = %v", visible)
	}

	pressFilterChord(t, app, "fx")
	if out.filter.Priority != 0 {
		t.Fatalf("Priority = %d after fx, want 0", out.filter.Priority)
	}
	if visible := visibleOutlinerTaskIDs(out); !visible["low"] {
		t.Errorf("fx should restore all tasks; visible = %v", visible)
	}
}

func TestFilterChordDeferredFacet(t *testing.T) {
	app := newFilterChordTestApp(t)
	filter := &app.crossOutliner.filter.CommonFilterState

	if cmd := pressFilterChord(t, app, "fd"); cmd == nil {
		t.Fatal("fd produced no status command")
	}
	if !filter.Deferred {
		t.Fatal("fd did not enable the deferred-only facet")
	}

	pressFilterChord(t, app, "fx")
	if filter.Deferred {
		t.Fatalf("fx left deferred enabled: %+v", filter)
	}
}

func TestFilterChordPickersOpenSeededFromTheActiveFilter(t *testing.T) {
	t.Run("f@ due date", func(t *testing.T) {
		app := newFilterChordTestApp(t)
		app.crossOutliner.filter.DueDate = DueDateThisWeek

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

	t.Run("fs states", func(t *testing.T) {
		app := newFilterChordTestApp(t)
		set, err := models.NewWorkflowStateSet(models.StateWaiting)
		if err != nil {
			t.Fatal(err)
		}
		app.crossOutliner.filter.setStateFilter(set, true)

		cmd := pressFilterChord(t, app, "fs")
		if cmd == nil {
			t.Fatal("fs produced no value-request command")
		}
		app.handleOperationOverlayMessage(cmd())
		if app.operationValuePicker == nil || app.operationValuePicker.provider != ValueProviderFilterStates {
			t.Fatalf("fs did not open the states picker: %#v", app.operationValuePicker)
		}
		selected := app.operationValuePicker.surface.SelectedIDs()
		if len(selected) != 1 || selected[0] != "state:waiting" {
			t.Errorf("seeded selection = %v, want [state:waiting]", selected)
		}
	})

	t.Run("fc completion age", func(t *testing.T) {
		app := newFilterChordTestApp(t)
		app.crossOutliner.filter.setCompletionAgeFilter(CompletionAge30Days)

		cmd := pressFilterChord(t, app, "fc")
		if cmd == nil {
			t.Fatal("fc produced no value-request command")
		}
		app.handleOperationOverlayMessage(cmd())
		if app.operationValuePicker == nil || app.operationValuePicker.provider != ValueProviderFilterCompletionAge {
			t.Fatalf("fc did not open the completion-age picker: %#v", app.operationValuePicker)
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

	t.Run("ft tags", func(t *testing.T) {
		app := newFilterChordTestApp(t)
		app.crossOutliner.filter.Tags = map[string]bool{"deep": true}

		cmd := pressFilterChord(t, app, "ft")
		if cmd == nil {
			t.Fatal("ft produced no value-request command")
		}
		app.handleOperationOverlayMessage(cmd())
		if app.operationValuePicker == nil || app.operationValuePicker.provider != ValueProviderFilterTags {
			t.Fatalf("ft did not open the tags picker: %#v", app.operationValuePicker)
		}
		selected := app.operationValuePicker.surface.SelectedIDs()
		if len(selected) != 1 || selected[0] != "tag:deep" {
			t.Errorf("seeded selection = %v, want [tag:deep]", selected)
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
	selectedStates, err := models.NewWorkflowStateSet(models.StateWaiting)
	if err != nil {
		t.Fatal(err)
	}
	outliner.filter.setStateFilter(selectedStates, true)
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
	lightRedZero := lipgloss.NewStyle().Foreground(ColorError).Faint(true).Render("0")
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

func TestFilterDueChordPickerAppliesEveryPublicFacet(t *testing.T) {
	tests := []struct {
		name   string
		itemID ListItemID
		want   DueDateFilter
	}{
		{name: "overdue", itemID: "filter-due:overdue", want: DueDateOverdue},
		{name: "today", itemID: "filter-due:today", want: DueDateToday},
		{name: "this week", itemID: "filter-due:week", want: DueDateThisWeek},
		{name: "no due date", itemID: "filter-due:none", want: DueDateNone},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newFilterChordTestApp(t)
			cmd := pressFilterChord(t, app, "f@")
			if cmd == nil {
				t.Fatal("f@ produced no value-request command")
			}
			if handled, overlayCmd := app.handleOperationOverlayMessage(cmd()); !handled || overlayCmd != nil {
				t.Fatalf("f@ request = handled %v cmd %T; want picker opened synchronously", handled, overlayCmd)
			}
			picker := app.operationValuePicker
			if picker == nil || !picker.surface.SetCursorToID(test.itemID) {
				t.Fatalf("due-date picker cannot select %s", test.itemID)
			}
			_, dismissed, selected := picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
			if !dismissed {
				t.Fatal("Enter did not finish the due-date picker")
			}
			handled, statusCmd := app.handleOperationOverlayMessage(selected)
			if !handled || statusCmd == nil {
				t.Fatalf("due-date selection = handled %v cmd %T; want applied status", handled, statusCmd)
			}
			if got := app.crossOutliner.filter.DueDate; got != test.want {
				t.Errorf("DueDate = %v, want %v", got, test.want)
			}
		})
	}
}

func TestFilterDueChordIsOneTerminalPickerBinding(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	ctx := simplifiedOperationContext()

	binding, descriptor, resolution := registry.InputBindingForSequence(
		InputStrategySimplified,
		[]string{"f", "@"},
		ctx,
	)
	if resolution != ChordTerminal || descriptor.ID != operationFilterDue {
		t.Fatalf("f@ = resolution %v operation %s, want terminal %s", resolution, descriptor.ID, operationFilterDue)
	}
	if binding.HasPresetValue || binding.DisplayKey != "f@" {
		t.Errorf("f@ binding = preset %v display %q, want picker display f@", binding.HasPresetValue, binding.DisplayKey)
	}
	helpDocumentsDueFacet := false
	for _, item := range registry.HelpItemsForInputStrategy(InputStrategySimplified, ctx) {
		if item.Key == "f@" && item.Description == "Filter by Due Date" {
			helpDocumentsDueFacet = true
			break
		}
	}
	if !helpDocumentsDueFacet {
		t.Error("derived help omits the terminal f@ due-date picker")
	}
	for _, retired := range [][]string{
		{"f", "@", "t"},
		{"f", "@", "w"},
		{"f", "@", "o"},
		{"f", "@", "n"},
	} {
		if _, _, got := registry.InputBindingForSequence(InputStrategySimplified, retired, ctx); got != ChordNoMatch {
			t.Errorf("retired due path %v resolution = %v, want no match", retired, got)
		}
	}
}

func TestFilterCompletionAgeChordSelectsTerminalLifecycleStates(t *testing.T) {
	app := newFilterChordTestApp(t)
	filter := &app.crossOutliner.filter.CommonFilterState
	states, err := models.NewWorkflowStateSet(models.StateWaiting)
	if err != nil {
		t.Fatal(err)
	}
	filter.setStateFilter(states, true)

	cmd := pressFilterChord(t, app, "fc")
	if cmd == nil {
		t.Fatal("fc produced no value-request command")
	}
	if handled, overlayCmd := app.handleOperationOverlayMessage(cmd()); !handled || overlayCmd != nil {
		t.Fatalf("fc request = handled %v cmd %T; want picker opened synchronously", handled, overlayCmd)
	}
	picker := app.operationValuePicker
	if picker == nil || !picker.surface.SetCursorToID("filter-completion-age:90") {
		t.Fatal("completion-age picker cannot select 90 days")
	}
	_, dismissed, selected := picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !dismissed {
		t.Fatal("Enter did not finish the completion-age picker")
	}
	handled, statusCmd := app.handleOperationOverlayMessage(selected)
	if !handled || statusCmd == nil {
		t.Fatalf("completion-age selection = handled %v cmd %T; want applied status", handled, statusCmd)
	}
	if !filter.hasCompletionAgeFilter() || filter.CompletionAge != CompletionAge90Days {
		t.Errorf("completion-age filter = active %v value %v, want active 90 days", filter.hasCompletionAgeFilter(), filter.CompletionAge)
	}
	wantStates, err := models.NewWorkflowStateSet(models.StateDone, models.StateCanceled)
	if err != nil {
		t.Fatal(err)
	}
	if !filter.States.Equal(wantStates) {
		t.Errorf("fc lifecycle states = %v, want Done and Canceled", filter.States.Values())
	}
	if got := commonFilterParts(filter); !slices.Equal(got, []string{"state:done,canceled", "completed:90d+"}) {
		t.Errorf("fc badge parts = %v, want terminal states and completion age", got)
	}

	pressFilterChord(t, app, "fx")
	if filter.hasCompletionAgeFilter() || filter.hasStateFilter() {
		t.Fatalf("fx left cleanup facets active: %+v", filter)
	}
}

func TestFilterOperationsApplyPickerValues(t *testing.T) {
	app := newFilterChordTestApp(t)
	ctx := simplifiedOperationContext()
	filter := &app.crossOutliner.filter.CommonFilterState

	states := app.operationExecutor.Execute(OperationExecutionRequest{
		ID:       operationFilterStates,
		Context:  ctx,
		Value:    []string{"done", "waiting"},
		HasValue: true,
	})
	if states.Err != nil {
		t.Fatalf("states filter error: %v", states.Err)
	}
	if states.Status != "Filtering 2 state(s)" {
		t.Errorf("Status = %q, want %q", states.Status, "Filtering 2 state(s)")
	}
	if !filter.States.Contains(models.StateDone) || !filter.States.Contains(models.StateWaiting) {
		t.Errorf("state filter = %v, want done+waiting", filter.States.Values())
	}

	cleared := app.operationExecutor.Execute(OperationExecutionRequest{
		ID:       operationFilterStates,
		Context:  ctx,
		Value:    []string{},
		HasValue: true,
	})
	if cleared.Err != nil {
		t.Fatalf("state clear error: %v", cleared.Err)
	}
	if cleared.Status != "State filter cleared" {
		t.Errorf("Status = %q, want %q", cleared.Status, "State filter cleared")
	}
	if filter.hasStateFilter() {
		t.Error("state filter should be inactive after selecting no states")
	}

	invalid := app.operationExecutor.Execute(OperationExecutionRequest{
		ID:       operationFilterStates,
		Context:  ctx,
		Value:    []string{"not-a-state"},
		HasValue: true,
	})
	if invalid.Err == nil {
		t.Error("an unknown workflow state should be rejected")
	}

	tags := app.operationExecutor.Execute(OperationExecutionRequest{
		ID:       operationFilterTags,
		Context:  ctx,
		Value:    []string{"deep"},
		HasValue: true,
	})
	if tags.Err != nil {
		t.Fatalf("tag filter error: %v", tags.Err)
	}
	if tags.Status != "Filtering 1 tag(s)" {
		t.Errorf("Status = %q, want %q", tags.Status, "Filtering 1 tag(s)")
	}
	if !filter.Tags["deep"] {
		t.Errorf("tag filter = %v, want deep", filter.Tags)
	}
	if visible := visibleOutlinerTaskIDs(app.crossOutliner); !visible["low"] || visible["critical"] {
		t.Errorf("tag filter should keep only the tagged task; visible = %v", visible)
	}
}

// TestFilterOperationsStayChordOnly locks the direct-grammar contract: the
// filter family refines the view in place through the f chords.
func TestFilterOperationsStayChordOnly(t *testing.T) {
	app := parityAuditApp(t)
	registry := app.GetOperationRegistry()

	descriptors := filterOperationDescriptors()
	if len(descriptors) != 8 {
		t.Fatalf("filter family declares %d operations, want 8", len(descriptors))
	}
	for _, declared := range descriptors {
		_, ok := registry.Descriptor(declared.ID)
		if !ok {
			t.Fatalf("filter operation %s is not registered", declared.ID)
		}
	}
}

func TestPriorityThresholdSemantics(t *testing.T) {
	tasks := map[int]*models.Task{
		0: {ID: "p0", Title: "Unset", WorkflowState: models.StateBacklog, Priority: 0},
		1: {ID: "p1", Title: "P1", WorkflowState: models.StateBacklog, Priority: 1},
		2: {ID: "p2", Title: "P2", WorkflowState: models.StateBacklog, Priority: 2},
		3: {ID: "p3", Title: "P3", WorkflowState: models.StateBacklog, Priority: 3},
	}
	for threshold, want := range map[int]map[int]bool{
		0: {0: true, 1: true, 2: true, 3: true},
		1: {0: false, 1: true, 2: false, 3: false},
		2: {0: false, 1: true, 2: true, 3: false},
		3: {0: false, 1: true, 2: true, 3: true},
	} {
		filter := newFilterState()
		filter.Priority = threshold
		for priority, pass := range want {
			if got := passesCommonFilters(tasks[priority], &filter); got != pass {
				t.Errorf("threshold %d priority %d = %v, want %v", threshold, priority, got, pass)
			}
		}
	}

	filter := newFilterState()
	filter.Priority = 3
	badge := commonFilterParts(&filter)
	if len(badge) != 1 || badge[0] != "priority:Low+" {
		t.Errorf("badge parts = %v, want [priority:Low+]", badge)
	}
	if described := fmt.Sprint(badge); !strings.Contains(described, "priority:Low+") {
		t.Errorf("badge %q should describe the threshold", described)
	}
}
