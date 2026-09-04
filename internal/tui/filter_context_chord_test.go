package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
)

// newContextFilterTestApp builds a scope whose context vocabulary is uneven and
// whose gap is non-empty, so the picker's rows, its counts, and the no-context
// row can each be told apart. "deep" is carried both as a tag and as a context
// to prove the two axes are counted separately rather than unioned.
func newContextFilterTestApp(t *testing.T) *AppModel {
	t.Helper()
	app := newUITestApp()
	project := &models.Project{
		ID:   "project",
		Name: "Project",
		Slug: "project",
		Tasks: []*models.Task{
			{ID: "errand-one", ShortID: 1, Title: "Errand one", WorkflowState: models.StateBacklog, Context: "errand"},
			{ID: "errand-two", ShortID: 2, Title: "Errand two", WorkflowState: models.StateBacklog, Context: "errand"},
			{ID: "deep-one", ShortID: 3, Title: "Deep one", WorkflowState: models.StateBacklog, Context: "deep"},
			{ID: "tagged-only", ShortID: 4, Title: "Tagged only", WorkflowState: models.StateBacklog, Tags: []string{"deep"}},
			{ID: "bare", ShortID: 5, Title: "Bare", WorkflowState: models.StateBacklog},
		},
	}
	project.BuildTaskTree()
	setTestAppProjects(app, project)
	app.crossOutliner.loadedProjects = []*models.Project{project}
	app.crossOutliner.crossForceRebuild = true
	app.crossOutliner.rebuildRows()
	return app
}

// fc is the context facet's own chord. Its picker leads with the gap row,
// because the unset context field is legal and no vocabulary row can name it,
// then offers only names actually carried as contexts.
func TestFilterContextChordPickerLeadsWithTheGapThenTheVocabulary(t *testing.T) {
	app := newContextFilterTestApp(t)

	cmd := pressFilterChord(t, app, "fc")
	if cmd == nil {
		t.Fatal("fc produced no value-request command")
	}
	app.handleOperationOverlayMessage(cmd())
	if app.operationValuePicker == nil || app.operationValuePicker.provider != operation.ValueProviderFilterContexts {
		t.Fatalf("fc did not open the context picker: %#v", app.operationValuePicker)
	}
	if title := app.operationValuePicker.title; title != "Filter by Context" {
		t.Errorf("picker title = %q, want %q", title, "Filter by Context")
	}

	items := app.operationValuePicker.surface.Items()
	wantIDs := []ListItemID{
		"filter-context:" + query.NoContextFilterKey,
		"filter-context:deep",
		"filter-context:errand",
	}
	if len(items) != len(wantIDs) {
		t.Fatalf("context picker items = %v, want %v", items, wantIDs)
	}
	for index, wantID := range wantIDs {
		if items[index].ID != wantID {
			t.Errorf("context picker item %d = %s, want %s", index, items[index].ID, wantID)
		}
	}
	// The counts are the context census alone: "deep" is carried by two tasks
	// but only one of them as a context, and the gap row counts the two tasks
	// no vocabulary row covers.
	wantSecondary := []string{"2 tasks", "1 task", "2 tasks"}
	for index, want := range wantSecondary {
		if items[index].SecondaryText != want {
			t.Errorf("context picker item %d count = %q, want %q", index, items[index].SecondaryText, want)
		}
	}
	if items[0].PrimaryText != "No context" {
		t.Errorf("gap row = %q, want %q", items[0].PrimaryText, "No context")
	}
	if items[2].PrimaryText != "##errand" {
		t.Errorf("vocabulary row = %q, want the ## sigil it answers to", items[2].PrimaryText)
	}
}

func TestFilterContextChordPickerOpensSeededFromBothHalvesOfTheFacet(t *testing.T) {
	app := newContextFilterTestApp(t)
	filter := &app.crossOutliner.filter.CommonFilterState
	filter.Contexts = map[string]bool{"errand": true}
	filter.NoContext = true

	cmd := pressFilterChord(t, app, "fc")
	if cmd == nil {
		t.Fatal("fc produced no value-request command")
	}
	app.handleOperationOverlayMessage(cmd())

	selected := app.operationValuePicker.surface.SelectedIDs()
	slices.Sort(selected)
	want := []ListItemID{"filter-context:errand", "filter-context:" + query.NoContextFilterKey}
	slices.Sort(want)
	if !slices.Equal(selected, want) {
		t.Fatalf("seeded selection = %v, want %v", selected, want)
	}
}

// The rows compose with OR, unlike the AND tags above them, because a task
// carries at most one context and an AND over two names would be unsatisfiable.
func TestFilterContextOperationAppliesOrSemanticsAcrossNamesAndTheGap(t *testing.T) {
	app := newContextFilterTestApp(t)
	ctx := simplifiedOperationContext()
	filter := &app.crossOutliner.filter.CommonFilterState

	both := app.operationExecutor.Execute(operation.OperationExecutionRequest{
		ID:       operationFilterContexts,
		Context:  ctx,
		Value:    []string{"errand", "deep"},
		HasValue: true,
	})
	if both.Err != nil {
		t.Fatalf("context filter error: %v", both.Err)
	}
	if both.Status != "Filtering context: ##deep ##errand" {
		t.Errorf("Status = %q, want the sorted ## chips", both.Status)
	}
	visible := visibleOutlinerTaskIDs(app.crossOutliner)
	for _, id := range []string{"errand-one", "errand-two", "deep-one"} {
		if !visible[id] {
			t.Errorf("two context names should match by OR; %s is hidden: %v", id, visible)
		}
	}
	if visible["bare"] || visible["tagged-only"] {
		t.Errorf("a task carrying neither context should be hidden: %v", visible)
	}

	gap := app.operationExecutor.Execute(operation.OperationExecutionRequest{
		ID:       operationFilterContexts,
		Context:  ctx,
		Value:    []string{query.NoContextFilterKey},
		HasValue: true,
	})
	if gap.Err != nil {
		t.Fatalf("no-context filter error: %v", gap.Err)
	}
	if gap.Status != "Filtering context: ##0" {
		t.Errorf("Status = %q, want the gap chip", gap.Status)
	}
	if len(filter.Contexts) != 0 || !filter.NoContext {
		t.Fatalf("the gap row should replace the vocabulary, not join it: %+v %v", filter.Contexts, filter.NoContext)
	}
	visible = visibleOutlinerTaskIDs(app.crossOutliner)
	if !visible["bare"] || !visible["tagged-only"] {
		t.Errorf("the gap should admit every task with no context: %v", visible)
	}
	if visible["errand-one"] {
		t.Errorf("the gap should exclude tasks carrying a context: %v", visible)
	}

	cleared := app.operationExecutor.Execute(operation.OperationExecutionRequest{
		ID:       operationFilterContexts,
		Context:  ctx,
		Value:    []string{},
		HasValue: true,
	})
	if cleared.Err != nil {
		t.Fatalf("context clear error: %v", cleared.Err)
	}
	if cleared.Status != "Context filter cleared" {
		t.Errorf("Status = %q, want %q", cleared.Status, "Context filter cleared")
	}
	if len(filter.Contexts) != 0 || filter.NoContext {
		t.Fatalf("an empty selection must leave no live predicate: %+v %v", filter.Contexts, filter.NoContext)
	}
}

// f# is tags alone now that contexts have fc. The census it reads is the tag
// census, so a value carried only as a context is not a row here, a value
// carried as both is one tag row rather than two, and the CONTEXTS section and
// its c jump hint drop out with the counts that produced them. Asserting this
// at the picker rather than at taskTagListSource is the point: the source still
// builds a CONTEXTS section when handed context counts, and the contract is
// that f# never hands it any.
func TestFilterTagsPickerNarrowsTagsAloneWithNoContextSection(t *testing.T) {
	app := newContextFilterTestApp(t)

	cmd := pressFilterChord(t, app, "f#")
	if cmd == nil {
		t.Fatal("f# produced no value-request command")
	}
	app.handleOperationOverlayMessage(cmd())
	if app.operationValuePicker == nil || app.operationValuePicker.provider != operation.ValueProviderFilterTags {
		t.Fatalf("f# did not open the tags picker: %#v", app.operationValuePicker)
	}

	items := app.operationValuePicker.surface.Items()
	for _, item := range items {
		if item.CategoryID == taskContextCategory {
			t.Errorf("f# offered a CONTEXTS row: %s", item.ID)
		}
	}
	// "deep" is carried once as a tag and once as a context; only the tag half
	// may reach this picker, and "errand" is a context alone so it has no row.
	assertRowIDs(t, taskTagRowIDs(items, tagDescriptiveCategory), []string{"tag:deep"})
	for _, item := range items {
		if item.ID == "tag:errand" {
			t.Error("a value used only as a context became an f# tag row")
		}
	}

	hint := ansi.Strip(taskTagCategoryJumpHint(operation.ValueProviderFilterTags, items, 120))
	if got := strings.Join(strings.Fields(hint), " "); got != "jump t tags" {
		t.Fatalf("f# jump hint = %q, want the tag section alone", got)
	}
}
