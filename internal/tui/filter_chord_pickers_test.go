package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
)

func TestFilterDueChordPickerAppliesEveryPublicFacet(t *testing.T) {
	tests := []struct {
		name   string
		itemID ListItemID
		token  string
		want   query.DueDateFilter
	}{
		{name: "overdue", itemID: "filter-due:overdue", token: "@overdue", want: query.DueDateOverdue},
		{name: "today", itemID: "filter-due:today", token: "@today", want: query.DueDateToday},
		{name: "this week", itemID: "filter-due:week", token: "@week", want: query.DueDateThisWeek},
		{name: "no due date", itemID: "filter-due:none", token: "@0", want: query.DueDateNone},
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
			// The row is a shortcut for a token, so what it carries is the canonical
			// spelling of the narrowing and not a private enum.
			current, ok := picker.surface.CurrentItem()
			if !ok || current.Data != test.token {
				t.Fatalf("row %s carries %#v, want the token %q", test.itemID, current.Data, test.token)
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
		operation.InputStrategySimplified,
		[]string{"f", "@"},
		ctx,
	)
	if resolution != operation.ChordTerminal || descriptor.ID != operationFilterDue {
		t.Fatalf("f@ = resolution %v operation %s, want terminal %s", resolution, descriptor.ID, operationFilterDue)
	}
	if binding.HasPresetValue || binding.DisplayKey != "f@" {
		t.Errorf("f@ binding = preset %v display %q, want picker display f@", binding.HasPresetValue, binding.DisplayKey)
	}
	helpDocumentsDueFacet := false
	for _, item := range registry.HelpItemsForInputStrategy(operation.InputStrategySimplified, ctx) {
		if item.Key == "f@" && item.Description == "Filter by Due or Deferred" {
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
		if _, _, got := registry.InputBindingForSequence(operation.InputStrategySimplified, retired, ctx); got != operation.ChordNoMatch {
			t.Errorf("retired due path %v resolution = %v, want no match", retired, got)
		}
	}
}

func TestFilterStaleChordIsOneGeneratedHelpTerminal(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	ctx := simplifiedOperationContext()

	binding, descriptor, resolution := registry.InputBindingForSequence(
		operation.InputStrategySimplified,
		[]string{"f", "s"},
		ctx,
	)
	if resolution != operation.ChordTerminal || descriptor.ID != operationFilterStale {
		t.Fatalf("fs = resolution %v operation %s, want terminal %s", resolution, descriptor.ID, operationFilterStale)
	}
	if binding.HasPresetValue || binding.DisplayKey != "fs" {
		t.Errorf("fs binding = preset %v display %q, want toggle display fs", binding.HasPresetValue, binding.DisplayKey)
	}
	helpDocumentsStaleFacet := false
	for _, item := range registry.HelpItemsForInputStrategy(operation.InputStrategySimplified, ctx) {
		if item.Key == "fs" && item.Description == "Filter Stale" {
			helpDocumentsStaleFacet = true
			break
		}
	}
	if !helpDocumentsStaleFacet {
		t.Error("help_filter projection omits the terminal fs Stale facet")
	}
}

func TestFilterStaleChordAndHelpAreUnavailableInRecent(t *testing.T) {
	app := newFilterChordTestApp(t)
	app.crossOutliner.selectOutlinerMode(chrome.ModeRecent)
	ctx := app.operationCommandContext()

	if _, _, resolution := app.operationRegistry.InputBindingForSequence(
		operation.InputStrategySimplified,
		[]string{"f", "s"},
		ctx,
	); resolution != operation.ChordNoMatch {
		t.Fatalf("fs resolution = %v, want no match in Recent", resolution)
	}
	for _, item := range app.operationRegistry.HelpItemsForInputStrategy(operation.InputStrategySimplified, ctx) {
		if item.Key == "fs" || item.Description == "Filter Stale" {
			t.Fatalf("Recent help retained Stale item: %+v", item)
		}
	}

	if handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 'f', Text: "f"}); !handled || cmd != nil {
		t.Fatalf("f = handled %v cmd %T; want a pending prefix", handled, cmd)
	}
	if row := pendingChordHintRowForTest(app); strings.Contains(row, "s stale") {
		t.Fatalf("Recent f which-key row retained Stale: %q", row)
	}

	app.statusMessage = ""
	if handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: 's', Text: "s"}); !handled {
		t.Fatal("s after f was not consumed by the Outliner grammar")
	}
	if app.crossOutliner.filter.Stale {
		t.Fatal("fs enabled the Stale facet in Recent")
	}
	if want := "fs is not a filter action"; app.statusMessage != want {
		t.Fatalf("Recent fs status = %q, want %q", app.statusMessage, want)
	}
}

// The filter and sort second keys are the sigils their facets already spell, so
// the letters they replaced must lead nowhere rather than keep acting. A letter
// still bound to something else would silently apply the wrong facet.
func TestRetiredFilterAndSortLetterPathsResolveToNoMatch(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	ctx := simplifiedOperationContext()

	for _, retired := range [][]string{
		{"f", "p"},
		{"f", "r"},
		{"f", "t"},
		{"f", "d"},
		{"F", "t"},
		{"o", "p"},
	} {
		if _, _, got := registry.InputBindingForSequence(operation.InputStrategySimplified, retired, ctx); got != operation.ChordNoMatch {
			t.Errorf("retired path %v resolution = %v, want no match", retired, got)
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
	filter.SetStateFilter(states)

	cmd := pressFilterChord(t, app, "fa")
	if cmd == nil {
		t.Fatal("fa produced no value-request command")
	}
	if handled, overlayCmd := app.handleOperationOverlayMessage(cmd()); !handled || overlayCmd != nil {
		t.Fatalf("fa request = handled %v cmd %T; want picker opened synchronously", handled, overlayCmd)
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
	if !filter.HasCompletionAgeFilter() || filter.CompletionAge != query.CompletionAge90Days {
		t.Errorf("completion-age filter = active %v value %v, want active 90 days", filter.HasCompletionAgeFilter(), filter.CompletionAge)
	}
	wantStates, err := models.NewWorkflowStateSet(models.StateDone, models.StateCanceled)
	if err != nil {
		t.Fatal(err)
	}
	if !filter.States.Equal(wantStates) {
		t.Errorf("fa lifecycle states = %v, want Done and Canceled", filter.States.Values())
	}
	wantParts := []string{"$canceled", "$done", "?completed:90d"}
	if got := query.CommonFilterParts(filter); !slices.Equal(got, wantParts) {
		t.Errorf("fa badge parts = %v, want terminal states and completion age", got)
	}

	pressFilterChord(t, app, "fx")
	if filter.HasCompletionAgeFilter() || filter.HasStateFilter() {
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
	if filter.HasStateFilter() {
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
