package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func TestStateValueProviderAcceptsOnlyCanonicalPresets(t *testing.T) {
	for _, state := range models.AllWorkflowStates() {
		if !operation.ValueProviderState.AcceptsPresetValue(state) {
			t.Errorf("canonical state preset %q was rejected", state)
		}
	}
	for _, retired := range []models.WorkflowState{"ready", "next", "in_progress", "review"} {
		if operation.ValueProviderState.AcceptsPresetValue(retired) {
			t.Errorf("retired state preset %q was accepted", retired)
		}
	}
}

func TestTaskStatePickerContainsExactCanonicalStatesAndTransitionReasons(t *testing.T) {
	request := OperationExecutionRequest{
		ID:      operationTaskSetState,
		Context: taskActionTestContext(),
	}
	picker, err := NewTaskStatePickerForTargets(request, []models.WorkflowState{models.StateDone}, "Completed task")
	if err != nil {
		t.Fatalf("NewTaskStatePickerForTargets: %v", err)
	}
	items := picker.surface.Items()
	if len(items) != len(models.AllWorkflowStates()) {
		t.Fatalf("state items = %d, want %d", len(items), len(models.AllWorkflowStates()))
	}
	for index, state := range models.AllWorkflowStates() {
		if got, ok := items[index].Data.(models.WorkflowState); !ok || got != state {
			t.Errorf("state item %d payload = %#v, want %s", index, items[index].Data, state)
		}
		if state == models.StateInbox && items[index].DisabledReason != "" {
			t.Errorf("reopen target Inbox disabled: %q", items[index].DisabledReason)
		}
		if state != models.StateInbox && items[index].DisabledReason == "" {
			t.Errorf("terminal transition to %s has no disabled reason", state)
		}
	}
}

func newOperationTagPickerForTest(t *testing.T, tags ...string) *OperationValuePickerModel {
	t.Helper()
	infos := make([]query.TagInfo, 0, len(tags))
	for _, tag := range tags {
		infos = append(infos, query.TagInfo{Name: tag})
	}
	source := ListSourceFromTags(infos)
	for index := range source.Items {
		source.Items[index].Data = source.Items[index].PrimaryText
	}
	picker, err := newOperationValuePickerModel(
		OperationExecutionRequest{ID: operationTaskSetTags},
		operation.ValueProviderTags,
		"Set Tags",
		"Task",
		source,
		true,
		nil,
	)
	if err != nil {
		t.Fatalf("newOperationValuePickerModel: %v", err)
	}
	return picker
}

func TestOperationTagPickerCreatesSelectsAndAppliesNewTag(t *testing.T) {
	picker := newOperationTagPickerForTest(t, "zeta")
	picker.surface.SetQuery("stale")
	if rendered := picker.render(90); !strings.Contains(ansi.Strip(rendered), "a add tag") {
		t.Fatalf("Set Tags help does not advertise tag creation:\n%s", rendered)
	}

	_, dismissed, result := picker.handleKey(tea.KeyPressMsg{Code: 'a'})
	if dismissed || result != nil || !picker.tagAdder.AddTagMode {
		t.Fatalf("a result = dismissed %v, message %T, add mode %v", dismissed, result, picker.tagAdder.AddTagMode)
	}
	if rendered := picker.render(90); !strings.Contains(rendered, "Add New Tag") {
		t.Fatalf("add-tag overlay is missing its title:\n%s", rendered)
	}

	picker.tagAdder.AddTagInput.SetValue("  #alpha  ")
	_, dismissed, result = picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if dismissed || result != nil || picker.tagAdder.AddTagMode {
		t.Fatalf("tag submit = dismissed %v, message %T, add mode %v", dismissed, result, picker.tagAdder.AddTagMode)
	}
	if got := picker.surface.Query().Raw; got != "" {
		t.Fatalf("query after tag creation = %q, want cleared", got)
	}
	if !picker.surface.IsSelected("tag:alpha") {
		t.Fatal("new tag is not selected")
	}
	items := picker.surface.Items()
	if len(items) != 2 || items[0].ID != "tag:alpha" || items[1].ID != "tag:zeta" {
		t.Fatalf("sorted tag items = %+v, want alpha then zeta", items)
	}

	_, dismissed, result = picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !dismissed {
		t.Fatal("Enter did not apply the completed tag selection")
	}
	selected, ok := result.(OperationValueSelectedMsg)
	if !ok {
		t.Fatalf("result = %T, want OperationValueSelectedMsg", result)
	}
	if got, want := selected.Value, []string{"alpha"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selected tags = %#v, want %#v", got, want)
	}
}

func TestOperationTagPickerValidatesInputAndEscapesBackToPicker(t *testing.T) {
	picker := newOperationTagPickerForTest(t)
	if rendered := picker.render(90); !strings.Contains(ansi.Strip(rendered), "a add tag") {
		t.Fatalf("empty Set Tags help does not advertise tag creation:\n%s", rendered)
	}

	picker.handleKey(tea.KeyPressMsg{Code: 'a'})
	for _, input := range []string{" ", "!!!"} {
		picker.tagAdder.AddTagInput.SetValue(input)
		_, dismissed, result := picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
		if dismissed || result != nil || !picker.tagAdder.AddTagMode || picker.tagAdder.AddTagError == "" {
			t.Fatalf(
				"invalid input %q = dismissed %v, message %T, add mode %v, error %q",
				input,
				dismissed,
				result,
				picker.tagAdder.AddTagMode,
				picker.tagAdder.AddTagError,
			)
		}
	}
	if len(picker.surface.Items()) != 0 {
		t.Fatalf("invalid input added tag rows: %+v", picker.surface.Items())
	}

	_, dismissed, result := picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	if dismissed || result != nil || picker.tagAdder.AddTagMode {
		t.Fatalf("first Esc = dismissed %v, message %T, add mode %v", dismissed, result, picker.tagAdder.AddTagMode)
	}
	_, dismissed, result = picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	if !dismissed {
		t.Fatal("second Esc did not dismiss Set Tags")
	}
	if _, ok := result.(OperationValueCanceledMsg); !ok {
		t.Fatalf("second Esc result = %T, want OperationValueCanceledMsg", result)
	}
}

func TestBulkOperationTagPickerCreatesAndPromotesTagsAsAddPatch(t *testing.T) {
	picker := newOperationTagPickerForTest(t, "partial")
	if err := picker.configureValueMembership(map[ListItemID]operationValueMembership{
		"tag:partial": operationValueSome,
	}); err != nil {
		t.Fatalf("configureValueMembership: %v", err)
	}

	for _, input := range []string{" #partial ", "fresh"} {
		picker.handleKey(tea.KeyPressMsg{Code: 'a'})
		picker.tagAdder.AddTagInput.SetValue(input)
		_, dismissed, result := picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
		if dismissed || result != nil || picker.tagAdder.AddTagMode {
			t.Fatalf("add %q = dismissed %v, message %T, add mode %v", input, dismissed, result, picker.tagAdder.AddTagMode)
		}
	}
	if items := picker.surface.Items(); len(items) != 2 {
		t.Fatalf("duplicate tag created an extra row: %+v", items)
	}

	value, err := picker.selectedValue()
	if err != nil {
		t.Fatalf("selectedValue: %v", err)
	}
	patch, ok := value.(operation.OperationTagsValue)
	if !ok {
		t.Fatalf("tag value = %T, want operation.OperationTagsValue", value)
	}
	if !reflect.DeepEqual(patch.Add, []string{"fresh", "partial"}) || len(patch.Remove) != 0 {
		t.Fatalf("tag patch = %+v, want add fresh and partial only", patch)
	}
}

func TestBulkOperationTagPickerKeepsSemanticTagsIndependent(t *testing.T) {
	picker := newOperationTagPickerForTest(t, "bug", "idea", "keep")
	if err := picker.configureValueMembership(map[ListItemID]operationValueMembership{
		"tag:bug":  operationValueSome,
		"tag:idea": operationValueNone,
		"tag:keep": operationValueAll,
	}); err != nil {
		t.Fatalf("configureValueMembership: %v", err)
	}
	if !picker.surface.SetCursorToID("tag:idea") {
		t.Fatal("idea tag is missing")
	}
	if err := picker.toggleCurrentMultiValue(); err != nil {
		t.Fatalf("toggle idea: %v", err)
	}

	value, err := picker.selectedValue()
	if err != nil {
		t.Fatalf("selectedValue: %v", err)
	}
	patch, ok := value.(operation.OperationTagsValue)
	if !ok {
		t.Fatalf("tag value = %T, want operation.OperationTagsValue", value)
	}
	if !reflect.DeepEqual(patch.Add, []string{"idea"}) || len(patch.Remove) != 0 {
		t.Fatalf("tag patch = %+v, want add idea without implicit removals", patch)
	}
	if picker.membershipCurrent["tag:keep"] != operationValueAll {
		t.Fatal("ordinary tag selection changed while adding a semantic tag")
	}
}

func TestOperationValuePickerMultiSelectionReturnsStableValues(t *testing.T) {
	source := ListSource{Items: []ListItem{
		{ID: "tag:z", PrimaryText: "z", Data: "z"},
		{ID: "tag:a", PrimaryText: "a", Data: "a"},
	}}
	picker, err := newOperationValuePickerModel(
		OperationExecutionRequest{ID: operationTaskSetTags},
		operation.ValueProviderTags,
		"Set Tags",
		"Task",
		source,
		true,
		[]ListItemID{"tag:z", "tag:a"},
	)
	if err != nil {
		t.Fatalf("newOperationValuePickerModel: %v", err)
	}
	_, dismissed, result := picker.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !dismissed {
		t.Fatal("Enter did not dismiss a completed multi picker")
	}
	selected, ok := result.(OperationValueSelectedMsg)
	if !ok {
		t.Fatalf("result = %T, want OperationValueSelectedMsg", result)
	}
	if got, want := selected.Value, []string{"a", "z"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selected values = %#v, want %#v", got, want)
	}
}

func TestBulkTagPickerRepresentsAllSomeNoneAndEmitsPatch(t *testing.T) {
	common := newBulkPickerTestTask(t, 1, "first", "common", "first-only")
	other := newBulkPickerTestTask(t, 1, "second", "common", "second-only")
	env := newAppBulkPickerEnv(t, []*models.Task{common}, []*models.Task{other})
	model := env.appModel()
	context := PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	context.Capabilities.Add(operation.CapabilitySimplifiedInput)
	context.Selection.Visual = true
	for index, task := range []*models.Task{common, other} {
		project := env.projects[index]
		context.Selection.Tasks = append(context.Selection.Tasks, taskref.For(task, project))
		context.Selection.Entities = append(context.Selection.Entities, taskOperationEntity(task, project))
	}
	context.Entity = context.Selection.Entities[0]
	request := OperationExecutionRequest{
		ID:      operationTaskSetTags,
		Context: context,
	}
	picker, err := model.newOperationValuePicker(request, operation.ValueProviderTags)
	if err != nil {
		t.Fatalf("newOperationValuePicker: %v", err)
	}
	if !picker.surface.IsSelected("tag:common") {
		t.Fatal("tag on every task is not rendered as selected")
	}
	items := picker.surface.Items()
	indeterminate := map[ListItemID]bool{}
	for _, item := range items {
		indeterminate[item.ID] = item.Indeterminate
	}
	if !indeterminate["tag:first-only"] || !indeterminate["tag:second-only"] {
		t.Fatalf("partial tag membership = %v, want both task-specific tags indeterminate", indeterminate)
	}
	if rendered := picker.render(90); !strings.Contains(rendered, "[-]") {
		t.Fatalf("tri-state picker did not render an indeterminate marker:\n%s", rendered)
	}

	if !picker.surface.SetCursorToID("tag:first-only") {
		t.Fatal("first-only tag is missing")
	}
	if toggleErr := picker.toggleCurrentMultiValue(); toggleErr != nil {
		t.Fatalf("promote partial tag to all: %v", toggleErr)
	}
	if !picker.surface.SetCursorToID("tag:common") {
		t.Fatal("common tag is missing")
	}
	if toggleErr := picker.toggleCurrentMultiValue(); toggleErr != nil {
		t.Fatalf("remove common tag: %v", toggleErr)
	}
	value, err := picker.selectedValue()
	if err != nil {
		t.Fatalf("selectedValue: %v", err)
	}
	patch, ok := value.(operation.OperationTagsValue)
	if !ok {
		t.Fatalf("tag value = %T, want operation.OperationTagsValue", value)
	}
	if !reflect.DeepEqual(patch.Add, []string{"first-only"}) ||
		!reflect.DeepEqual(patch.Remove, []string{"common"}) {
		t.Fatalf("tag patch = %+v, want add first-only and remove common", patch)
	}
}

// TestBulkContextRowsReadIndeterminateAndWriteOnlyWhenPicked pins the CONTEXTS
// half of the tri-state picker the README describes: a context only part of the
// selection holds renders `[-]`, a disagreeing selection writes nothing until a
// row is picked, `No context` clears the field across the whole selection the
// way `##0` does, and the literal `0` name stays refused.
func TestBulkContextRowsReadIndeterminateAndWriteOnlyWhenPicked(t *testing.T) {
	first := newBulkPickerTestTask(t, 1, "first")
	first.Context = "home"
	second := newBulkPickerTestTask(t, 1, "second")
	second.Context = "office"
	env := newAppBulkPickerEnv(t, []*models.Task{first}, []*models.Task{second})
	model := env.appModel()
	context := PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	context.Capabilities.Add(operation.CapabilitySimplifiedInput)
	context.Selection.Visual = true
	for index, task := range []*models.Task{first, second} {
		project := env.projects[index]
		context.Selection.Tasks = append(context.Selection.Tasks, taskref.For(task, project))
		context.Selection.Entities = append(context.Selection.Entities, taskOperationEntity(task, project))
	}
	context.Entity = context.Selection.Entities[0]
	picker, err := model.newOperationValuePicker(
		OperationExecutionRequest{ID: operationTaskSetTags, Context: context},
		operation.ValueProviderTags,
	)
	if err != nil {
		t.Fatalf("newOperationValuePicker: %v", err)
	}

	indeterminate := map[ListItemID]bool{}
	for _, item := range picker.surface.Items() {
		indeterminate[item.ID] = item.Indeterminate
	}
	if !indeterminate[taskContextRowID("home")] || !indeterminate[taskContextRowID("office")] {
		t.Fatalf("context membership = %v, want both disagreed contexts indeterminate", indeterminate)
	}
	if picker.contextInitial != "" {
		t.Fatalf("contextInitial = %q, want no agreed context", picker.contextInitial)
	}

	value, err := picker.selectedValue()
	if err != nil {
		t.Fatalf("selectedValue: %v", err)
	}
	patch, ok := value.(operation.OperationTagsValue)
	if !ok {
		t.Fatalf("tag value = %T, want operation.OperationTagsValue", value)
	}
	if patch.ContextSet {
		t.Fatalf("untouched disagreeing contexts wrote %+v, want no context write", patch)
	}

	if refusal := picker.addOrSelectContext("0"); refusal == nil {
		t.Fatal("minting a literal 0 context was accepted")
	}

	if !picker.surface.SetCursorToID(taskContextClearRowID) {
		t.Fatal("the No context row is missing")
	}
	if selectErr := picker.selectContextUnderCursor(); selectErr != nil {
		t.Fatalf("select No context: %v", selectErr)
	}
	value, err = picker.selectedValue()
	if err != nil {
		t.Fatalf("selectedValue after clear: %v", err)
	}
	patch, ok = value.(operation.OperationTagsValue)
	if !ok {
		t.Fatalf("tag value = %T, want operation.OperationTagsValue", value)
	}
	if !patch.ContextSet || patch.Context != "" {
		t.Fatalf("No context patch = %+v, want ContextSet with an empty context", patch)
	}
}
