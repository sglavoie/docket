package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/operationhost"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/route"
)

func session8GuidanceText(spec chrome.GuidanceSpec) string {
	entries := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		entries = append(entries, chrome.GuidanceHintText(hint))
	}
	return strings.Join(entries, " | ") + " | " + spec.Supplemental
}

func assertSession8Guidance(t *testing.T, state string, spec chrome.GuidanceSpec, ok bool, want ...string) {
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

// The `?` viewer's own guidance states moved to
// internal/tui/help/help_guidance_test.go with the package, since every state
// they set is HelpViewModel's own field. What stays here is the guidance of the
// overlays the shell still owns.

func TestContextActionSheetGuidanceTracksBrowseSearchEmptyAndFailure(t *testing.T) {
	registry, executor := contextActionTestEngine(t, []operation.OperationDescriptor{
		contextActionTestDescriptor("test.act", "Act"),
	}, nil)
	sheet, err := listsurface.NewContextActionSheetModel(
		registry, executor, contextActionTestContext(), []operation.OperationID{"test.act"},
		listsurface.ContextActionSheetConfig{Host: listsurface.ContextActionHostOutliner},
	)
	if err != nil {
		t.Fatalf("NewContextActionSheetModel: %v", err)
	}

	spec, ok := sheet.GuidanceSpec()
	assertSession8Guidance(t, "browse", spec, ok, "Enter run", "Esc close")
	// One action reads whole at a glance, so the footer does not offer to search it.
	if strings.Contains(session8GuidanceText(spec), "/ search") {
		t.Fatalf("a one-action sheet advertises search: %s", session8GuidanceText(spec))
	}

	sheet.Searching = true
	spec, ok = sheet.GuidanceSpec()
	assertSession8Guidance(t, "search", spec, ok, "Enter apply search", "Esc cancel search")
	if spec.Rows != 2 {
		t.Fatalf("search rows = %d, want 2", spec.Rows)
	}

	sheet.Searching = false
	sheet.Surface.SetQuery("missing")
	spec, ok = sheet.GuidanceSpec()
	assertSession8Guidance(t, "no matches", spec, ok, "/ search", "Esc clear search")
	if strings.Contains(session8GuidanceText(spec), "Enter run") {
		t.Fatalf("empty action sheet advertises a missing selection: %s", session8GuidanceText(spec))
	}

	sheet.Notice = "selected target is no longer available"
	rendered := ansi.Strip(sheet.Render(80))
	if !strings.Contains(rendered, sheet.Notice) || !strings.Contains(rendered, "Esc clear search") {
		t.Fatalf("stale-target sheet lost notice or recovery guidance:\n%s", rendered)
	}
}

func TestOperationValuePickerGuidanceUsesListSurfaceStates(t *testing.T) {
	picker := newOperationTagPickerForTest(t, "alpha", "beta")

	spec, ok := picker.GuidanceSpec()
	assertSession8Guidance(t, "browse", spec, ok, "Space toggle", "a add tag", "Enter apply", "Esc back")

	picker.SetSearching(true)
	picker.Surface().SetQuery("alp")
	spec, ok = picker.GuidanceSpec()
	assertSession8Guidance(t, "search", spec, ok, "Enter apply search", "Esc clear search")
	if spec.Rows != 2 {
		t.Fatalf("search rows = %d, want 2", spec.Rows)
	}

	picker.SetSearching(false)
	picker.Surface().SetQuery("missing")
	spec, ok = picker.GuidanceSpec()
	assertSession8Guidance(t, "no matches", spec, ok, "/ search", "a add tag", "Esc back")
	if strings.Contains(session8GuidanceText(spec), "Enter apply") {
		t.Fatalf("empty value picker advertises apply: %s", session8GuidanceText(spec))
	}

	picker.TagAdder().AddTagMode = true
	spec, ok = picker.GuidanceSpec()
	assertSession8Guidance(t, "add tag", spec, ok, "Enter add tag", "Esc cancel")
	for _, hint := range spec.Hints {
		if !hint.Required {
			t.Fatalf("add-tag control is optional: %+v", hint)
		}
	}
}

func TestOperationOverlaysRenderLocalGuidanceWithoutWorkspacePulse(t *testing.T) {
	prompt := operationhost.NewOperationTextPromptModel(
		operation.OperationExecutionRequest{}, "Rename", "Current", "Current", "new name",
		operationhost.NameTextPromptOptions(),
	)
	picker := newOperationTagPickerForTest(t, "alpha")

	rendered := []struct {
		name string
		text string
		want []string
	}{
		{name: "text prompt", text: prompt.Render(80), want: []string{"Enter apply", "Esc back"}},
		{name: "value picker", text: picker.Render(80), want: []string{"Enter apply", "Esc back"}},
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
	spec := chrome.ConfirmationGuidanceSpec()
	assertSession8Guidance(t, "confirm", spec, true, "y confirm", "n/Esc cancel")
	for _, hint := range spec.Hints {
		if !hint.Required {
			t.Fatalf("confirmation control is optional: %+v", hint)
		}
	}

	var dialog chrome.ConfirmDialog
	dialog.Show("delete", "Delete this item?")
	rendered := ansi.Strip(dialog.RenderOverlay(24))
	if !strings.Contains(rendered, "y confirm") || !strings.Contains(rendered, "n/Esc cancel") {
		t.Fatalf("narrow confirmation lost a mandatory outcome:\n%s", rendered)
	}
}

// A view that draws its own footer keeps every row of it while the shell reports
// what just happened. Session 4 replaced the footer swap this used to assert:
// the old composition removed the surface's structured guidance to make room for
// a transient message, which left a multi-row control set unnamed for as long as
// the message stood. The feedback now takes a body row above the divider
// instead, so the frame is the same height and the controls are all still there.
func TestSurfaceFooterFeedbackStandsAboveEveryStructuredGuidanceRow(t *testing.T) {
	footer := chrome.RenderGuidance(chrome.GuidanceSpec{
		Hints: chrome.RequiredGuidanceHints("Enter apply", "Esc cancel", "Tab navigate"),
		Rows:  2,
	}, 24)
	content := "first\nsecond\n" + footer
	composed := composeSurfaceFooterFeedback(content, "Copied title")
	plain := ansi.Strip(composed)

	for _, want := range []string{"Enter apply", "Esc cancel", "Tab navigate"} {
		if !strings.Contains(plain, want) {
			t.Errorf("feedback displaced a control the surface published: %q missing from\n%s", want, plain)
		}
	}
	if !strings.Contains(plain, "Copied title") {
		t.Fatalf("composition lost the feedback it was given:\n%s", plain)
	}
	if got, want := strings.Count(composed, "\n"), strings.Count(content, "\n"); got != want {
		t.Fatalf("frame height changed: %d rows of separators, want %d", got, want)
	}

	lines := strings.Split(plain, "\n")
	divider := surfaceFooterDividerIndex(lines)
	if divider < 1 {
		t.Fatalf("composed frame lost its footer divider:\n%s", plain)
	}
	if !strings.Contains(lines[divider-1], "Copied title") {
		t.Fatalf("feedback is not the row directly above the footer:\n%s", plain)
	}
	// The row it spent is the one nearest the footer, which on a self-padding
	// full-height view is padding. Rows further up are the surface's own content
	// and stay where they are.
	if !strings.Contains(plain, "first") {
		t.Fatalf("composition spent a row other than the one above the footer:\n%s", plain)
	}
}

// The composition is a no-op where there is nothing to spend, because a frame
// that cannot afford both keeps the controls: they persist and the message does
// not.
func TestSurfaceFooterFeedbackLeavesAFrameItCannotPayFrom(t *testing.T) {
	footer := chrome.RenderGuidance(chrome.GuidanceSpec{
		Hints: chrome.RequiredGuidanceHints("Esc back"),
		Rows:  1,
	}, 18)
	if got := composeSurfaceFooterFeedback(footer, "Copied title"); got != footer {
		t.Fatalf("footer-only frame was rewritten:\n%s", ansi.Strip(got))
	}
	if got := composeSurfaceFooterFeedback("only content", "Copied title"); got != "only content" {
		t.Fatalf("frame without a footer divider was rewritten:\n%s", ansi.Strip(got))
	}
	if got := composeSurfaceFooterFeedback("first\nsecond\n"+footer, ""); got != "first\nsecond\n"+footer {
		t.Fatalf("empty feedback still moved the frame:\n%s", ansi.Strip(got))
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

	views := []struct {
		name string
		view route.View
	}{
		{name: "tag selector", view: listsurface.NewTagSelectorModel(task, source, nil)},
		{name: "parent selector", view: listsurface.NewParentSelectorModel([]*models.Task{parent}, nil, true, presentation.ViewCrossProjectOutliner)},
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
	t.Run("single tag selector", func(t *testing.T) {
		model := listsurface.NewTagSelectorModel(nil, &models.Project{ID: "project", Name: "Project"}, nil)
		model.Update(syntheticKeyStr("/"))
		model.Update(syntheticKey('a'))
		if model.AddTagMode || model.SearchQuery() != "a" {
			t.Fatalf("search text opened add-tag mode: add=%v query=%q", model.AddTagMode, model.SearchQuery())
		}
		if _, cmd := model.Update(syntheticKeyStr("enter")); cmd != nil || model.IsSearchMode() {
			t.Fatalf("search Enter should apply the query without committing: active=%v cmd=%T", model.IsSearchMode(), cmd)
		}
	})
}
