package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// TestCaptureFooterSpecContainsRequiredActions pins the four capture footer
// states to their required action vocabulary. The state selection priority is
// discard confirmation → autocomplete → expanded syntax → normal capture, and
// each state must advertise its full action set so no control is hidden.
func TestCaptureFooterSpecContainsRequiredActions(t *testing.T) {
	cases := []struct {
		name               string
		confirmDiscard     bool
		autocompleteActive bool
		helpExpanded       bool
		required           []string
	}{
		{
			name:     "normal",
			required: []string{"Enter create", "Ctrl+O create & open", "Ctrl+D create & edit", "Ctrl+V paste", "? syntax", "Esc cancel"},
		},
		{
			name:               "autocomplete",
			autocompleteActive: true,
			required:           []string{"Tab/Enter accept", "Ctrl+J/K navigate", "Esc dismiss"},
		},
		{
			name:         "help",
			helpExpanded: true,
			required:     []string{"Enter create", "Ctrl+O create & open", "Ctrl+D create & edit", "Ctrl+V paste", "? close syntax", "Esc close syntax"},
		},
		{
			name:           "confirmation",
			confirmDiscard: true,
			required:       []string{"Y discard", "N/Esc keep editing"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := captureFooterSpec(tc.confirmDiscard, tc.autocompleteActive, tc.helpExpanded)
			for _, want := range tc.required {
				if !strings.Contains(spec.Left, want) {
					t.Fatalf("%s footer missing %q, got %q", tc.name, want, spec.Left)
				}
			}
		})
	}
}

// TestCaptureFooterSpecStatePriority verifies the selection precedence when
// multiple state flags are set simultaneously: discard confirmation wins over
// autocomplete, which wins over expanded syntax.
func TestCaptureFooterSpecStatePriority(t *testing.T) {
	if got := captureFooterSpec(true, true, true); got.Left != captureFooterConfirm {
		t.Fatalf("discard confirmation must take precedence, got %q", got.Left)
	}
	if got := captureFooterSpec(false, true, true); got.Left != captureFooterAutocomplete {
		t.Fatalf("autocomplete must take precedence over help, got %q", got.Left)
	}
	if got := captureFooterSpec(false, false, true); got.Left != captureFooterHelp {
		t.Fatalf("expanded syntax expected, got %q", got.Left)
	}
}

// TestAppViewLinkedTaskCreatorUsesAppShellFooter guards the single-footer
// contract for the Linked Task Creator: it is a normal (non-full-height) stack
// view, so AppModel.View() must supply exactly one app-shell footer. This
// session removed the inline child footer that previously produced a second
// competing separator. Rendering through app.View() exercises the composition
// path.
func TestAppViewLinkedTaskCreatorUsesAppShellFooter(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.width = 80
	app.height = 24
	sourceTask := project.Tasks[0]

	handled, _ := app.handleTaskMessages(CreateLinkedTaskMsg{SourceTask: sourceTask, Project: project, LinkType: LinkTypeBlocker})
	if !handled {
		t.Fatal("CreateLinkedTaskMsg should be handled")
	}

	content := app.View().Content

	if !strings.Contains(content, "New Blocker") {
		t.Fatal("composed output should keep the Linked Task Creator title/body")
	}
	if got := countFullWidthDividers(content); got != 1 {
		t.Fatalf("expected exactly one app-shell footer separator, got %d\n%s", got, ansi.Strip(content))
	}
}

// TestAppViewLinkedTaskCreatorComposesWithinTerminalAcrossSizes composes the
// Linked Task Creator through AppModel at the supported minimum and every
// baseline size, with the guide both closed and expanded. The scenario runner
// cannot reliably construct a source/link context at 60x20, so this composed Go
// test is the minimum-size coverage for the Linked Task Creator: it asserts the
// single app-shell footer with its capture actions stays visible, no required
// text is truncated, every line fits the width, and the render fits the
// terminal height (its fixed source-task header notwithstanding).
func TestAppViewLinkedTaskCreatorComposesWithinTerminalAcrossSizes(t *testing.T) {
	sizes := append([]baselineSize{narrowBaseline}, baselineSizes...)
	for _, sz := range sizes {
		for _, expand := range []bool{false, true} {
			name := fmt.Sprintf("%s expanded=%v", sz.label, expand)
			t.Run(name, func(t *testing.T) {
				app, project := newTestAppModelWithProject()
				app.handleWindowSize(tea.WindowSizeMsg{Width: sz.width, Height: sz.height})
				sourceTask := project.Tasks[0]

				handled, _ := app.handleTaskMessages(CreateLinkedTaskMsg{
					SourceTask: sourceTask, Project: project, LinkType: LinkTypeBlocker,
				})
				if !handled {
					t.Fatal("CreateLinkedTaskMsg should be handled")
				}
				ltc, ok := app.views.Get(ViewLinkedTaskCreator).(*LinkedTaskCreatorModel)
				if !ok {
					t.Fatal("expected a LinkedTaskCreatorModel to be registered")
				}
				if expand {
					ltc.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
					if !ltc.helpExpanded {
						t.Fatal("expected guide to expand")
					}
				}

				content, issues := withRenderAudit(func() string { return app.View().Content })
				if len(issues) != 0 {
					t.Fatalf("%s: required-text truncation recorded: %+v", name, issues)
				}

				plain := ansi.Strip(content)
				assertRenderedLinesWithinVisualWidth(t, name, content, sz.width)
				if h := lipgloss.Height(content); h > sz.height {
					t.Fatalf("%s: composed height %d exceeds terminal height %d\n%s", name, h, sz.height, plain)
				}
				if !strings.Contains(plain, "Enter create") {
					t.Fatalf("%s: app-shell footer capture actions missing\n%s", name, plain)
				}
				if !strings.Contains(plain, "New Blocker") {
					t.Fatalf("%s: linked task creator title missing\n%s", name, plain)
				}
				if got := countFullWidthDividers(content); got != 1 {
					t.Fatalf("%s: expected exactly one app-shell footer separator, got %d\n%s", name, got, plain)
				}
			})
		}
	}
}

// TestCaptureFooterWrapsAt60ColumnsWithoutTruncation verifies that the full
// normal-state capture footer survives the 60-column minimum width: every
// action wraps onto additional footer rows rather than being clipped. A render
// audit confirms no required text was truncated, and all six actions remain
// present in the composed output.
func TestCaptureFooterWrapsAt60ColumnsWithoutTruncation(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.width = 60
	app.height = 20

	handled, _ := app.handleViewMessages(ShowQuickCaptureMsg{Project: project})
	if !handled {
		t.Fatal("ShowQuickCaptureMsg should be handled")
	}

	content, issues := withRenderAudit(func() string { return app.View().Content })
	if len(issues) != 0 {
		t.Fatalf("expected no required-text truncation at 60 columns, got %d issue(s): %+v", len(issues), issues)
	}

	plain := ansi.Strip(content)
	for _, action := range []string{"Enter create", "Ctrl+O create & open", "Ctrl+D create & edit", "Ctrl+V paste", "? syntax", "Esc cancel"} {
		if !strings.Contains(plain, action) {
			t.Fatalf("expected capture action %q at 60 columns, got:\n%s", action, plain)
		}
	}
}

// TestCaptureFooterSurvivesPinnedTasks confirms that a pinned task cannot
// replace the capture controls: the contextual footer sits above the pinned
// bar in the status-bar precedence, so with Quick Capture active the footer
// shows the capture actions rather than the pinned task strip.
func TestCaptureFooterSurvivesPinnedTasks(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.width = 80
	app.height = 24

	const sentinel = "PINNED_SENTINEL_TASK"
	project.Tasks[0].Title = sentinel
	project.Tasks[0].Pinned = true

	handled, _ := app.handleViewMessages(ShowQuickCaptureMsg{Project: project})
	if !handled {
		t.Fatal("ShowQuickCaptureMsg should be handled")
	}

	// Sanity: the project genuinely has a pinned task available to the bar.
	if len(app.collectPinnedTasks()) == 0 {
		t.Fatal("expected a pinned task to be collectable for the test setup")
	}

	statusBar := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(statusBar, "Enter create") {
		t.Fatalf("expected capture controls in status bar, got:\n%s", statusBar)
	}
	if strings.Contains(statusBar, sentinel) {
		t.Fatalf("pinned task must not replace capture controls, got:\n%s", statusBar)
	}
}

// TestCaptureFooterYieldsToTemporaryStatusThenReturns verifies the status-bar
// precedence at the transient boundary: a temporary app status message
// outranks the contextual footer, and once it clears the capture controls
// return.
func TestCaptureFooterYieldsToTemporaryStatusThenReturns(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.width = 80
	app.height = 24

	handled, _ := app.handleViewMessages(ShowQuickCaptureMsg{Project: project})
	if !handled {
		t.Fatal("ShowQuickCaptureMsg should be handled")
	}

	app.statusMessage = "task saved"
	withStatus := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(withStatus, "task saved") {
		t.Fatalf("expected temporary status message to show, got:\n%s", withStatus)
	}
	if strings.Contains(withStatus, "Enter create") {
		t.Fatalf("temporary status message must replace capture controls, got:\n%s", withStatus)
	}

	app.statusMessage = ""
	afterStatus := ansi.Strip(app.renderStatusBar())
	if !strings.Contains(afterStatus, "Enter create") {
		t.Fatalf("expected capture controls to return after status clears, got:\n%s", afterStatus)
	}
}
