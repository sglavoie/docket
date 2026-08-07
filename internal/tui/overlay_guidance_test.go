package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func session8GuidanceText(spec GuidanceSpec) string {
	entries := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		entries = append(entries, guidanceHintText(hint))
	}
	return strings.Join(entries, " | ") + " | " + spec.Supplemental
}

func assertSession8Guidance(t *testing.T, state string, spec GuidanceSpec, ok bool, want ...string) {
	t.Helper()
	if !ok {
		t.Fatalf("%s guidance unavailable", state)
	}
	text := session8GuidanceText(spec)
	for _, entry := range want {
		if !strings.Contains(text, entry) {
			t.Errorf("%s guidance missing %q: %s", state, entry, text)
		}
	}
}

func TestHelpGuidanceTracksBrowseSearchTabAndEmptyStates(t *testing.T) {
	model := NewHelpViewModel([]HelpItem{{Key: "j", Description: "Move", Category: "Navigate"}}, ViewCrossProjectOutliner, "Outliner")
	model.SetSize(90, 24)

	spec, ok := model.ContextualGuidance()
	assertSession8Guidance(t, "browse", spec, ok, "? close", "/ filter", "\\ find tab", "j/k scroll")
	if spec.Rows != 1 {
		t.Fatalf("browse rows = %d, want 1", spec.Rows)
	}

	model.searchMode = true
	spec, ok = model.ContextualGuidance()
	assertSession8Guidance(t, "filter edit", spec, ok, "Enter apply filter", "Esc cancel edit", "\\ find tab")
	if spec.Rows != 2 {
		t.Fatalf("filter-edit rows = %d, want 2", spec.Rows)
	}

	model.searchMode = false
	model.tabSearchMode = true
	spec, ok = model.ContextualGuidance()
	assertSession8Guidance(t, "tab find", spec, ok, "Enter apply tab", "Esc cancel find", "/ edit filter")

	model.tabSearchMode = false
	model.filteredItems = nil
	spec, ok = model.ContextualGuidance()
	assertSession8Guidance(t, "empty", spec, ok, "no matching help items", "? close")
}

func TestConflictGuidanceGatesProjectOnlyAndRetryResolution(t *testing.T) {
	retry := false
	bundle := &storage.ConflictBundle{Record: storage.ConflictRecord{
		ProjectName:            "Project One",
		RequiresOperationRetry: &retry,
		Conflicts:              []storage.MergeConflict{{}, {}},
	}}
	model := NewConflictResolutionModel(config.EmptyRuntimeState(""), bundle)

	spec, ok := model.ContextualGuidance()
	assertSession8Guidance(t, "project-only", spec, ok,
		"l keep local", "r adopt remote", "e hand-merge", "j/k navigate", "Esc close")

	retry = true
	spec, ok = model.ContextualGuidance()
	assertSession8Guidance(t, "retry", spec, ok, "r clear and retry operation", "j/k navigate", "Esc close")
	text := session8GuidanceText(spec)
	if strings.Contains(text, "keep local") || strings.Contains(text, "hand-merge") {
		t.Fatalf("retry guidance advertises disabled resolutions: %s", text)
	}

	empty := NewConflictResolutionModel(config.EmptyRuntimeState(""), nil)
	spec, ok = empty.ContextualGuidance()
	assertSession8Guidance(t, "missing conflict", spec, ok, "Esc close")
}

func TestContextActionSheetGuidanceTracksBrowseSearchEmptyAndFailure(t *testing.T) {
	registry, executor := contextActionTestEngine(t, []OperationDescriptor{
		contextActionTestDescriptor("test.act", "Act"),
	}, nil)
	sheet, err := NewContextActionSheetModel(
		registry, executor, contextActionTestContext(), []OperationID{"test.act"},
		ContextActionSheetConfig{Host: ContextActionHostOutliner},
	)
	if err != nil {
		t.Fatalf("NewContextActionSheetModel: %v", err)
	}

	spec, ok := sheet.guidanceSpec()
	assertSession8Guidance(t, "browse", spec, ok, "Enter run", "/ search", "Esc close")

	sheet.searching = true
	spec, ok = sheet.guidanceSpec()
	assertSession8Guidance(t, "search", spec, ok, "Enter apply search", "Esc cancel search")
	if spec.Rows != 2 {
		t.Fatalf("search rows = %d, want 2", spec.Rows)
	}

	sheet.searching = false
	sheet.surface.SetQuery("missing")
	spec, ok = sheet.guidanceSpec()
	assertSession8Guidance(t, "no matches", spec, ok, "/ search", "Esc clear search")
	if strings.Contains(session8GuidanceText(spec), "Enter run") {
		t.Fatalf("empty action sheet advertises a missing selection: %s", session8GuidanceText(spec))
	}

	sheet.notice = "selected target is no longer available"
	rendered := ansi.Strip(sheet.render(80))
	if !strings.Contains(rendered, sheet.notice) || !strings.Contains(rendered, "Esc clear search") {
		t.Fatalf("stale-target sheet lost notice or recovery guidance:\n%s", rendered)
	}
}

func TestOperationValuePickerGuidanceUsesListSurfaceStates(t *testing.T) {
	picker := newOperationTagPickerForTest(t, "alpha", "beta")

	spec, ok := picker.guidanceSpec()
	assertSession8Guidance(t, "browse", spec, ok, "Space toggle", "a add tag", "Enter apply", "Esc back")

	picker.searching = true
	picker.surface.SetQuery("alp")
	spec, ok = picker.guidanceSpec()
	assertSession8Guidance(t, "search", spec, ok, "Enter apply search", "Esc clear search")
	if spec.Rows != 2 {
		t.Fatalf("search rows = %d, want 2", spec.Rows)
	}

	picker.searching = false
	picker.surface.SetQuery("missing")
	spec, ok = picker.guidanceSpec()
	assertSession8Guidance(t, "no matches", spec, ok, "/ search", "a add tag", "Esc back")
	if strings.Contains(session8GuidanceText(spec), "Enter apply") {
		t.Fatalf("empty value picker advertises apply: %s", session8GuidanceText(spec))
	}

	picker.tagAdder.AddTagMode = true
	spec, ok = picker.guidanceSpec()
	assertSession8Guidance(t, "add tag", spec, ok, "Enter add tag", "Esc cancel")
	for _, hint := range spec.Hints {
		if !hint.Required {
			t.Fatalf("add-tag control is optional: %+v", hint)
		}
	}
}

func TestOperationOverlaysRenderLocalGuidanceWithoutWorkspacePulse(t *testing.T) {
	prompt := newOperationTextPromptModel(
		OperationExecutionRequest{}, "Rename", "Current", "Current", "new name",
	)
	picker := newOperationTagPickerForTest(t, "alpha")

	rendered := []struct {
		name string
		text string
		want []string
	}{
		{name: "text prompt", text: prompt.render(80), want: []string{"Enter apply", "Esc back"}},
		{name: "value picker", text: picker.render(80), want: []string{"Enter apply", "Esc back"}},
	}
	for _, test := range rendered {
		t.Run(test.name, func(t *testing.T) {
			plain := ansi.Strip(test.text)
			for _, want := range test.want {
				if !strings.Contains(plain, want) {
					t.Errorf("overlay missing %q:\n%s", want, plain)
				}
			}
			if strings.Contains(plain, "workspace:") {
				t.Fatalf("transient operation overlay rendered a workspace pulse:\n%s", plain)
			}
		})
	}
}

func TestConfirmationGuidanceIsMandatoryAndNormalized(t *testing.T) {
	spec := confirmationGuidanceSpec()
	assertSession8Guidance(t, "confirm", spec, true, "y confirm", "n/Esc cancel")
	for _, hint := range spec.Hints {
		if !hint.Required {
			t.Fatalf("confirmation control is optional: %+v", hint)
		}
	}

	var dialog ConfirmDialog
	dialog.Show("delete", "Delete this item?")
	rendered := ansi.Strip(dialog.RenderOverlay(24))
	if !strings.Contains(rendered, "y confirm") || !strings.Contains(rendered, "n/Esc cancel") {
		t.Fatalf("narrow confirmation lost a mandatory outcome:\n%s", rendered)
	}
}

func TestReplaceFooterRemovesEveryStructuredGuidanceRow(t *testing.T) {
	oldFooter := renderGuidance(GuidanceSpec{
		Hints: requiredGuidanceHints("Enter apply", "Esc cancel", "Tab navigate"),
		Rows:  2,
	}, 18)
	newFooter := renderGuidance(GuidanceSpec{
		Hints: requiredGuidanceHints("Esc dismiss status"),
		Rows:  1,
	}, 18)
	replaced := ansi.Strip(replaceFooter("content\n"+oldFooter, newFooter))
	if strings.Contains(replaced, "Enter apply") || strings.Contains(replaced, "Tab navigate") {
		t.Fatalf("old multi-row guidance survived replacement:\n%s", replaced)
	}
	if !strings.Contains(replaced, "content") || !strings.Contains(replaced, "Esc dismiss status") {
		t.Fatalf("replacement lost content or new status:\n%s", replaced)
	}
}

func TestTransientSelectorsRenderGuidanceWithoutWorkspacePulse(t *testing.T) {
	parent, err := models.NewGenericTask("parent-1", 1, "Parent")
	if err != nil {
		t.Fatalf("NewGenericTask: %v", err)
	}
	task, err := models.NewGenericTask("task-1", 2, "Task")
	if err != nil {
		t.Fatalf("NewGenericTask: %v", err)
	}
	source := &models.Project{ID: "source", Name: "Source", Slug: "source"}
	destination := &models.Project{ID: "destination", Name: "Destination", Slug: "destination"}

	views := []struct {
		name string
		view View
	}{
		{name: "tag filter", view: NewTagFilterSelectorModel(ShowTagFilterMsg{Tags: []TagInfo{{Name: "alpha"}}})},
		{name: "tag selector", view: NewTagSelectorModel(task, source, nil)},
		{name: "bulk tag selector", view: NewBulkTagSelectorModel(nil)},
		{name: "parent selector", view: NewParentSelectorModel([]*models.Task{parent}, nil, true, ViewCrossProjectOutliner)},
		{name: "project selector", view: NewProjectSelectorModel([]*models.Project{source, destination}, task, source)},
	}
	for _, test := range views {
		t.Run(test.name, func(t *testing.T) {
			test.view.SetSize(80, 24)
			rendered := ansi.Strip(test.view.View().Content)
			if !strings.Contains(rendered, "Esc") {
				t.Fatalf("selector has no cancellation guidance:\n%s", rendered)
			}
			if strings.Contains(rendered, "workspace:") {
				t.Fatalf("transient selector rendered its own workspace pulse:\n%s", rendered)
			}
		})
	}
}

func TestTagSelectorSearchControlsMatchGuidanceBeforeCommitOrCancel(t *testing.T) {
	t.Run("filter selector", func(t *testing.T) {
		model := NewTagFilterSelectorModel(ShowTagFilterMsg{Tags: []TagInfo{{Name: "alpha"}}})
		model.Update(syntheticKeyStr("/"))
		model.Update(syntheticKey('z'))
		if !model.IsSearchMode() || model.SearchQuery() != "z" {
			t.Fatalf("filter selector did not enter search: active=%v query=%q", model.IsSearchMode(), model.SearchQuery())
		}
		if _, cmd := model.Update(syntheticKeyStr("esc")); cmd != nil || model.IsSearchMode() {
			t.Fatalf("first Esc should clear search without canceling: active=%v cmd=%T", model.IsSearchMode(), cmd)
		}
		if _, cmd := model.Update(syntheticKeyStr("esc")); cmd == nil {
			t.Fatal("second Esc should cancel the filter selector")
		}
	})

	t.Run("single tag selector", func(t *testing.T) {
		model := NewTagSelectorModel(nil, &models.Project{ID: "project", Name: "Project"}, nil)
		model.Update(syntheticKeyStr("/"))
		model.Update(syntheticKey('a'))
		if model.AddTagMode || model.SearchQuery() != "a" {
			t.Fatalf("search text opened add-tag mode: add=%v query=%q", model.AddTagMode, model.SearchQuery())
		}
		if _, cmd := model.Update(syntheticKeyStr("enter")); cmd != nil || model.IsSearchMode() {
			t.Fatalf("search Enter should apply the query without committing: active=%v cmd=%T", model.IsSearchMode(), cmd)
		}
	})

	t.Run("bulk tag selector", func(t *testing.T) {
		model := NewBulkTagSelectorModel(nil)
		model.Update(syntheticKeyStr("/"))
		model.Update(syntheticKey('a'))
		if model.AddTagMode || model.SearchQuery() != "a" {
			t.Fatalf("search text opened bulk add-tag mode: add=%v query=%q", model.AddTagMode, model.SearchQuery())
		}
		if _, cmd := model.Update(syntheticKeyStr("esc")); cmd != nil || model.IsSearchMode() {
			t.Fatalf("search Esc should clear without canceling: active=%v cmd=%T", model.IsSearchMode(), cmd)
		}
	})
}
