package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type staticFooterContinuationView struct {
	continuation string
}

func (v *staticFooterContinuationView) Init() tea.Cmd { return nil }

func (v *staticFooterContinuationView) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return v, nil
}

func (v *staticFooterContinuationView) View() tea.View { return tea.NewView("") }

func (v *staticFooterContinuationView) SetSize(int, int) {}

func (v *staticFooterContinuationView) FooterContinuation(int) string {
	return v.continuation
}

// statusBarContinuationApp opens a task tag picker over the Outliner, which is
// the state the continuation exists for: the overlay clipped its explainer and
// the row behind it is still addressing a view that has no input.
func statusBarContinuationApp(t *testing.T) *AppModel {
	t.Helper()
	app := typedTestApp(t)
	app.width = 100
	app.height = 35
	app.crossOutliner.SetSize(app.width, app.height-2)
	return app
}

func TestStatusBarFinishesTheClippedExplainerWhileThePickerIsOpen(t *testing.T) {
	app := statusBarContinuationApp(t)

	hints := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(hints, "capture") {
		t.Fatalf("expected the Outliner hints before the picker opens, got:\n%s", hints)
	}

	app.operationValuePicker = newSemanticTagPickerForTest(t, []TagInfo{{Name: "urgent", TaskCount: 1}}, true)
	withPicker := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(withPicker, "colors its title.") {
		t.Fatalf("expected the clipped summary to finish in the status bar, got:\n%s", withPicker)
	}
	if strings.Contains(withPicker, "capture") {
		t.Fatalf("the continuation must replace hints for a view with no input, got:\n%s", withPicker)
	}

	app.operationValuePicker = nil
	if restored := ansi.Strip(app.renderStatusBar()); restored != hints {
		t.Fatalf("expected the hints to return once the picker closes, got:\n%s", restored)
	}
}

func TestStatusBarOutranksATransientStatusMessageWhileThePickerIsOpen(t *testing.T) {
	app := statusBarContinuationApp(t)
	app.operationValuePicker = newSemanticTagPickerForTest(t, nil, true)
	app.statusMessage = "Tagged 1 task(s)"

	statusBar := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(statusBar, "colors its title.") {
		t.Fatalf("a leftover status message must not blink the sentence away, got:\n%s", statusBar)
	}
}

func TestStatusBarHasNothingToContinueInAddTagMode(t *testing.T) {
	app := statusBarContinuationApp(t)
	picker := newSemanticTagPickerForTest(t, nil, true)
	picker.tagAdder.AddTagMode = true
	app.operationValuePicker = picker

	statusBar := ansi.Strip(app.renderStatusBar())
	if strings.Contains(statusBar, "colors its title.") {
		t.Fatalf("the Add New Tag box prints no explainer to continue, got:\n%s", statusBar)
	}
	if !strings.Contains(statusBar, "capture") {
		t.Fatalf("expected the Outliner hints to stay, got:\n%s", statusBar)
	}
}

func TestOpenPickerDoesNotFallThroughToTheCoveredViewContinuation(t *testing.T) {
	app := statusBarContinuationApp(t)
	view := &staticFooterContinuationView{continuation: "covered view continuation"}
	app.views.Register(ViewCrossProjectOutliner, view)

	if got := app.footerContinuation(); got != view.continuation {
		t.Fatalf("active view continuation = %q, want %q", got, view.continuation)
	}

	picker := newSemanticTagPickerForTest(t, nil, true)
	picker.tagAdder.AddTagMode = true
	app.operationValuePicker = picker
	if got := picker.FooterContinuation(app.width); got != "" {
		t.Fatalf("test precondition failed: add-tag picker continuation = %q", got)
	}
	if got := app.footerContinuation(); got != "" {
		t.Fatalf("open picker fell through to the covered view continuation %q", got)
	}
}

func TestStatusBarSuppressesProjectTagContinuationAtMinimumWidth(t *testing.T) {
	app := statusBarContinuationApp(t)
	app.width = minTerminalWidth
	app.height = minTerminalHeight
	app.crossOutliner.SetSize(app.width, app.height-2)
	app.statusMessage = "ready"

	picker, err := newOperationValuePickerModel(
		OperationExecutionRequest{ID: operationProjectSetTags},
		ValueProviderProjectTags,
		"Project Tags",
		"Project",
		ListSource{},
		true,
		nil,
	)
	if err != nil {
		t.Fatalf("newOperationValuePickerModel: %v", err)
	}
	app.operationValuePicker = picker

	if raw := picker.FooterContinuation(app.width); lipgloss.Width(raw) <= app.width {
		t.Fatalf("test precondition failed: project picker remainder %q fits width %d", raw, app.width)
	}
	if continuation := app.footerContinuation(); continuation != "" {
		t.Fatalf("minimum-width continuation = %q, want it suppressed", continuation)
	}

	statusBar := ansi.Strip(app.renderStatusBar())
	row := strings.TrimRight(strings.Split(statusBar, "\n")[1], " ")
	if strings.HasSuffix(row, "…") {
		t.Fatalf("minimum-width status row ends in a clipped word: %q", row)
	}
	if !strings.Contains(row, "ready") {
		t.Fatalf("expected the regular status row after suppression, got: %q", row)
	}
}
