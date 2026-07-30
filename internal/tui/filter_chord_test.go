package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

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
			{ID: "critical", ShortID: 1, Title: "Critical", WorkflowState: models.StateReady, Priority: 1},
			{ID: "normal", ShortID: 2, Title: "Normal", WorkflowState: models.StateNext, Priority: 3},
			{ID: "lowest", ShortID: 3, Title: "Lowest", WorkflowState: models.StateWaiting, Priority: 5, Tags: []string{"deep"}},
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
	row := app.pendingChordHintRow()
	for _, want := range []string{
		"p priority threshold",
		"d due",
		"s states",
		"t tags",
		"b blocked",
		"z snoozed",
		"x clear all",
	} {
		if !strings.Contains(row, want) {
			t.Errorf("f which-key row %q omits %q", row, want)
		}
	}

	if handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'p', Text: "p"}); !handled || cmd != nil {
		t.Fatalf("fp = handled %v cmd %T; want a pending prefix", handled, cmd)
	}
	row = app.pendingChordHintRow()
	for _, want := range []string{"0 all", "1 <=1", "3 <=3", "5 <=5"} {
		if !strings.Contains(row, want) {
			t.Errorf("fp which-key row %q omits %q", row, want)
		}
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
			status: "Filtering priority <= 3",
			verify: func(t *testing.T) {
				if filter.Priority != 3 {
					t.Errorf("Priority = %d, want 3", filter.Priority)
				}
			},
		},
		{
			path:   []string{"f", "p", "5"},
			status: "Filtering priority <= 5",
			verify: func(t *testing.T) {
				if filter.Priority != 5 {
					t.Errorf("Priority = %d, want 5", filter.Priority)
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
	filter.Blocked = true
	filter.Snoozed = true
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
	for _, id := range []string{"critical", "normal", "lowest"} {
		if !before[id] {
			t.Fatalf("task %q missing before filtering; visible = %v", id, before)
		}
	}

	if cmd := pressFilterChord(t, app, "fp3"); cmd == nil {
		t.Fatal("fp3 produced no status command")
	}
	if out.filter.Priority != 3 {
		t.Fatalf("Priority = %d, want 3", out.filter.Priority)
	}
	visible := visibleOutlinerTaskIDs(out)
	if !visible["critical"] || !visible["normal"] {
		t.Errorf("fp3 should keep priority <= 3 tasks; visible = %v", visible)
	}
	if visible["lowest"] {
		t.Errorf("fp3 should hide the priority-5 task; visible = %v", visible)
	}

	pressFilterChord(t, app, "fp5")
	if out.filter.Priority != 5 {
		t.Fatalf("Priority = %d, want 5", out.filter.Priority)
	}
	if visible := visibleOutlinerTaskIDs(out); !visible["lowest"] {
		t.Errorf("fp5 should keep the priority-5 task; visible = %v", visible)
	}

	pressFilterChord(t, app, "fx")
	if out.filter.Priority != 0 {
		t.Fatalf("Priority = %d after fx, want 0", out.filter.Priority)
	}
	if visible := visibleOutlinerTaskIDs(out); !visible["lowest"] {
		t.Errorf("fx should restore all tasks; visible = %v", visible)
	}
}

func TestFilterChordBlockedAndSnoozedFacets(t *testing.T) {
	app := newFilterChordTestApp(t)
	filter := &app.crossOutliner.filter.CommonFilterState

	if cmd := pressFilterChord(t, app, "fb"); cmd == nil {
		t.Fatal("fb produced no status command")
	}
	if !filter.Blocked {
		t.Fatal("fb did not enable the blocked-only facet")
	}

	if cmd := pressFilterChord(t, app, "fz"); cmd == nil {
		t.Fatal("fz produced no status command")
	}
	if !filter.Snoozed {
		t.Fatal("fz did not enable the snoozed-only facet")
	}

	pressFilterChord(t, app, "fx")
	if filter.Blocked || filter.Snoozed {
		t.Fatalf("fx left blocked/snoozed enabled: %+v", filter)
	}
}

func TestFilterChordPickersOpenSeededFromTheActiveFilter(t *testing.T) {
	t.Run("fd due date", func(t *testing.T) {
		app := newFilterChordTestApp(t)
		app.crossOutliner.filter.DueDate = DueDateThisWeek

		cmd := pressFilterChord(t, app, "fd")
		if cmd == nil {
			t.Fatal("fd produced no value-request command")
		}
		app.handleOperationOverlayMessage(cmd())
		if app.operationValuePicker == nil || app.operationValuePicker.provider != ValueProviderFilterDue {
			t.Fatalf("fd did not open the due-date picker: %#v", app.operationValuePicker)
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
			cmd := pressFilterChord(t, app, "fd")
			if cmd == nil {
				t.Fatal("fd produced no value-request command")
			}
			if handled, overlayCmd := app.handleOperationOverlayMessage(cmd()); !handled || overlayCmd != nil {
				t.Fatalf("fd request = handled %v cmd %T; want picker opened synchronously", handled, overlayCmd)
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
		[]string{"f", "d"},
		ctx,
	)
	if resolution != ChordTerminal || descriptor.ID != operationFilterDue {
		t.Fatalf("fd = resolution %v operation %s, want terminal %s", resolution, descriptor.ID, operationFilterDue)
	}
	if binding.HasPresetValue || binding.DisplayKey != "fd" {
		t.Errorf("fd binding = preset %v display %q, want picker display fd", binding.HasPresetValue, binding.DisplayKey)
	}
	helpDocumentsFD := false
	for _, item := range registry.HelpItemsForInputStrategy(InputStrategySimplified, ctx) {
		if item.Key == "fd" && item.Description == "Filter by Due Date" {
			helpDocumentsFD = true
			break
		}
	}
	if !helpDocumentsFD {
		t.Error("derived help omits the terminal fd due-date picker")
	}
	for _, retired := range [][]string{
		{"f", "d", "t"},
		{"f", "d", "w"},
		{"f", "d", "o"},
		{"f", "d", "n"},
	} {
		if _, _, got := registry.InputBindingForSequence(InputStrategySimplified, retired, ctx); got != ChordNoMatch {
			t.Errorf("retired due path %v resolution = %v, want no match", retired, got)
		}
	}
}

func TestFilterOperationsApplyPickerValues(t *testing.T) {
	app := newFilterChordTestApp(t)
	ctx := simplifiedOperationContext()
	filter := &app.crossOutliner.filter.CommonFilterState

	states := app.operationExecutor.Execute(OperationExecutionRequest{
		ID:       operationFilterStates,
		Context:  ctx,
		Source:   OperationSourceDirectKey,
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
		Source:   OperationSourceDirectKey,
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
		Source:   OperationSourceDirectKey,
		Value:    []string{"not-a-state"},
		HasValue: true,
	})
	if invalid.Err == nil {
		t.Error("an unknown workflow state should be rejected")
	}

	tags := app.operationExecutor.Execute(OperationExecutionRequest{
		ID:       operationFilterTags,
		Context:  ctx,
		Source:   OperationSourceDirectKey,
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
	if visible := visibleOutlinerTaskIDs(app.crossOutliner); !visible["lowest"] || visible["critical"] {
		t.Errorf("tag filter should keep only the tagged task; visible = %v", visible)
	}
}

// TestFilterOperationsStayChordOnly locks the direct-grammar contract: the
// filter family refines the view in place through the f chords.
func TestFilterOperationsStayChordOnly(t *testing.T) {
	app := parityAuditApp(t)
	registry := app.GetOperationRegistry()

	descriptors := filterOperationDescriptors()
	if len(descriptors) != 7 {
		t.Fatalf("filter family declares %d operations, want 7", len(descriptors))
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
		1: {ID: "p1", Title: "P1", WorkflowState: models.StateReady, Priority: 1},
		3: {ID: "p3", Title: "P3", WorkflowState: models.StateReady, Priority: 3},
		5: {ID: "p5", Title: "P5", WorkflowState: models.StateReady, Priority: 5},
	}
	for threshold, want := range map[int]map[int]bool{
		0: {1: true, 3: true, 5: true},
		3: {1: true, 3: true, 5: false},
		5: {1: true, 3: true, 5: true},
	} {
		filter := newFilterState()
		filter.Priority = threshold
		for priority, pass := range want {
			if got := passesCommonFilters(tasks[priority], &filter, nil); got != pass {
				t.Errorf("threshold %d priority %d = %v, want %v", threshold, priority, got, pass)
			}
		}
	}

	filter := newFilterState()
	filter.Priority = 3
	badge := commonFilterParts(&filter)
	if len(badge) != 1 || badge[0] != "!<=3" {
		t.Errorf("badge parts = %v, want [!<=3]", badge)
	}
	if described := fmt.Sprint(badge); !strings.Contains(described, "!<=3") {
		t.Errorf("badge %q should describe the threshold", described)
	}
}
