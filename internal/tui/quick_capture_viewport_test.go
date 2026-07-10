package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/testutil"
)

// ctrlKey builds a Ctrl+<r> key press.
func ctrlKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl}
}

// pressQC feeds a key to a QuickCaptureModel and returns the (same) model.
func pressQC(t *testing.T, m *QuickCaptureModel, msg tea.Msg) *QuickCaptureModel {
	t.Helper()
	updated, _ := m.Update(msg)
	return updated.(*QuickCaptureModel)
}

// newExpandedQuickCapture builds a Quick Capture model, sizes it, toggles the
// syntax guide open, and renders once so the guide viewport height is
// established from the composed budget.
func newExpandedQuickCapture(t *testing.T, width, height int) *QuickCaptureModel {
	t.Helper()
	project := testutil.NewTestProject("qc viewport")
	m := NewQuickCaptureModel(project, nil, false)
	m.SetSize(width, height)
	m = pressQC(t, m, tea.KeyPressMsg{Code: '?', Text: "?"})
	if !m.helpExpanded {
		t.Fatal("expected help to open after '?'")
	}
	_ = m.View().Content // establish viewport height from the budget
	return m
}

// TestQuickCaptureHelpOpensAtTopAndClosesWithQuestion verifies the guide opens
// at offset zero and that '?' toggles it closed.
func TestQuickCaptureHelpOpensAtTopAndClosesWithQuestion(t *testing.T) {
	m := newExpandedQuickCapture(t, 80, 16)
	if off := m.syntaxVP.YOffset(); off != 0 {
		t.Fatalf("help must open at the top (offset 0), got %d", off)
	}

	// Scroll away, then re-open: openHelp resets to the top again.
	m = pressQC(t, m, ctrlKey('j'))
	m = pressQC(t, m, ctrlKey('j'))
	m = pressQC(t, m, tea.KeyPressMsg{Code: '?', Text: "?"}) // close
	if m.helpExpanded {
		t.Fatal("'?' should close the expanded guide")
	}
	m = pressQC(t, m, tea.KeyPressMsg{Code: '?', Text: "?"}) // re-open
	_ = m.View().Content
	if off := m.syntaxVP.YOffset(); off != 0 {
		t.Fatalf("re-opening the guide must reset to the top, got offset %d", off)
	}
}

// TestQuickCaptureSyntaxScrollDoesNotMutateTextarea verifies Ctrl+J/K and the
// page keys move the guide viewport offset while leaving the textarea untouched.
func TestQuickCaptureSyntaxScrollDoesNotMutateTextarea(t *testing.T) {
	project := testutil.NewTestProject("qc scroll")
	m := NewQuickCaptureModel(project, nil, false)
	m.SetSize(80, 14)
	const text = "hello world"
	m.input.SetValue(text)
	m.updatePreview()
	m = pressQC(t, m, tea.KeyPressMsg{Code: '?', Text: "?"})
	_ = m.View().Content

	if !m.syntaxOverflows() {
		t.Fatalf("test setup expects the guide to overflow at 80x14")
	}

	base := m.syntaxVP.YOffset()
	m = pressQC(t, m, ctrlKey('j'))
	if got := m.syntaxVP.YOffset(); got <= base {
		t.Fatalf("Ctrl+J should scroll the guide down: offset %d not > %d", got, base)
	}
	if m.input.Value() != text {
		t.Fatalf("Ctrl+J must not mutate the textarea, got %q", m.input.Value())
	}

	afterLine := m.syntaxVP.YOffset()
	m = pressQC(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	if got := m.syntaxVP.YOffset(); got <= afterLine {
		t.Fatalf("PgDn should scroll the guide down a page: offset %d not > %d", got, afterLine)
	}
	if m.input.Value() != text {
		t.Fatalf("PgDn must not mutate the textarea, got %q", m.input.Value())
	}

	afterPage := m.syntaxVP.YOffset()
	m = pressQC(t, m, ctrlKey('k'))
	if got := m.syntaxVP.YOffset(); got >= afterPage {
		t.Fatalf("Ctrl+K should scroll the guide up: offset %d not < %d", got, afterPage)
	}
	if m.input.Value() != text {
		t.Fatalf("Ctrl+K must not mutate the textarea, got %q", m.input.Value())
	}
}

// TestQuickCaptureEscClosesHelpBeforeCancel verifies Esc dismisses the expanded
// guide first, and only a subsequent Esc reaches the cancel/discard path.
func TestQuickCaptureEscClosesHelpBeforeCancel(t *testing.T) {
	project := testutil.NewTestProject("qc esc")
	m := NewQuickCaptureModel(project, nil, false)
	m.SetSize(80, 20)
	m.input.SetValue("draft task")
	m.updatePreview()
	m = pressQC(t, m, tea.KeyPressMsg{Code: '?', Text: "?"})
	_ = m.View().Content

	m = pressQC(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.helpExpanded {
		t.Fatal("first Esc must close the expanded guide")
	}
	if m.confirmDiscard {
		t.Fatal("first Esc must not trigger discard while the guide was open")
	}

	m = pressQC(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !m.confirmDiscard {
		t.Fatal("second Esc must trigger discard confirmation for non-empty input")
	}
}

// TestQuickCaptureAutocompleteHidesHelpAndRestoresOffset verifies that opening
// autocomplete while the guide is expanded hands Ctrl+J/K to autocomplete,
// leaves the guide offset untouched, and restores it once autocomplete closes.
func TestQuickCaptureAutocompleteHidesHelpAndRestoresOffset(t *testing.T) {
	m := newExpandedQuickCapture(t, 80, 14)
	m = pressQC(t, m, ctrlKey('j'))
	m = pressQC(t, m, ctrlKey('j'))
	guideOffset := m.syntaxVP.YOffset()
	if guideOffset == 0 {
		t.Fatal("test setup expects a non-zero guide offset before autocomplete")
	}

	// Activate the type autocomplete (always has suggestions).
	m = pressQC(t, m, tea.KeyPressMsg{Code: '^', Text: "^"})
	if !m.isAutocompleteActive() {
		t.Fatal("expected autocomplete to activate after '^'")
	}
	if !m.helpExpanded {
		t.Fatal("help should remain logically expanded while autocomplete is shown")
	}

	// The guide viewport is hidden but its offset is preserved.
	if got := m.syntaxVP.YOffset(); got != guideOffset {
		t.Fatalf("guide offset must be preserved while autocomplete is active: %d != %d", got, guideOffset)
	}
	// Rendering with autocomplete active shows the dropdown, not the guide.
	frame := ansi.Strip(m.View().Content)
	if strings.Contains(frame, syntaxGuideTitle) {
		t.Fatalf("guide viewport should be hidden while autocomplete is active:\n%s", frame)
	}

	// Ctrl+J is owned by autocomplete, not the guide.
	cursorBefore := m.ac.Cursor
	m = pressQC(t, m, ctrlKey('j'))
	if m.ac.Cursor != cursorBefore+1 {
		t.Fatalf("Ctrl+J should navigate autocomplete: cursor %d, want %d", m.ac.Cursor, cursorBefore+1)
	}
	if got := m.syntaxVP.YOffset(); got != guideOffset {
		t.Fatalf("Ctrl+J must not scroll the guide while autocomplete owns it: %d != %d", got, guideOffset)
	}

	// Dismiss autocomplete; the guide returns at its preserved offset.
	m = pressQC(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.isAutocompleteActive() {
		t.Fatal("Esc should dismiss autocomplete")
	}
	if !m.helpExpanded {
		t.Fatal("help should still be expanded after autocomplete dismissal")
	}
	_ = m.View().Content
	if got := m.syntaxVP.YOffset(); got != guideOffset {
		t.Fatalf("guide offset must be restored after autocomplete closes: %d != %d", got, guideOffset)
	}
}

// TestQuickCaptureCreateControlsActiveInHelpMode verifies Enter, Ctrl+O, and
// Ctrl+D still reach task creation while the syntax guide is expanded (they set
// the empty-title error rather than being swallowed by the guide).
func TestQuickCaptureCreateControlsActiveInHelpMode(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{"enter", tea.KeyPressMsg{Code: tea.KeyEnter}},
		{"ctrl+o", ctrlKey('o')},
		{"ctrl+d", ctrlKey('d')},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newExpandedQuickCapture(t, 80, 20)
			m.err = nil
			m = pressQC(t, m, tc.key)
			if m.err == nil {
				t.Fatalf("%s must reach createTask in help mode (expected empty-title error)", tc.name)
			}
			if !m.helpExpanded {
				t.Fatalf("%s must not close the guide", tc.name)
			}
		})
	}
}

// TestQuickCaptureResizePreservesAndClampsOffset verifies a resize keeps a valid
// offset but clamps a stale offset that no longer fits the (larger) viewport.
func TestQuickCaptureResizePreservesAndClampsOffset(t *testing.T) {
	m := newExpandedQuickCapture(t, 80, 14)

	// Scroll to the bottom of the small viewport.
	for i := 0; i < 6; i++ {
		m = pressQC(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	smallOffset := m.syntaxVP.YOffset()
	if smallOffset == 0 {
		t.Fatal("test setup expects a non-zero offset after paging to the bottom")
	}

	// Re-rendering at the same size preserves the offset.
	_ = m.View().Content
	if got := m.syntaxVP.YOffset(); got != smallOffset {
		t.Fatalf("same-size re-render must preserve offset: %d != %d", got, smallOffset)
	}

	// Growing to a viewport tall enough for the whole guide clamps the offset
	// back to zero (nothing left to scroll).
	m.SetSize(140, 40)
	_ = m.View().Content
	if m.syntaxOverflows() {
		t.Fatal("guide should fit fully at 140x40")
	}
	if got := m.syntaxVP.YOffset(); got != 0 {
		t.Fatalf("resize must clamp a stale offset to 0 when the guide fits, got %d", got)
	}
}

// TestQuickCaptureComposesWithinTerminalAcrossSizes composes Quick Capture
// through AppModel at the supported minimum and every baseline size, with the
// guide both closed and expanded, and asserts the single app-shell footer stays
// visible, no required text is truncated, and the render fits the terminal.
func TestQuickCaptureComposesWithinTerminalAcrossSizes(t *testing.T) {
	sizes := append([]baselineSize{narrowBaseline}, baselineSizes...)
	for _, sz := range sizes {
		for _, expand := range []bool{false, true} {
			name := fmt.Sprintf("%s expanded=%v", sz.label, expand)
			t.Run(name, func(t *testing.T) {
				app, project := newTestAppModelWithProject()
				// Drive sizing through the window-size path so registered views are
				// budgeted to terminal height minus the two-row footer, matching real
				// usage (the registry otherwise re-applies its 80x24 default).
				app.handleWindowSize(tea.WindowSizeMsg{Width: sz.width, Height: sz.height})

				if handled, _ := app.handleViewMessages(ShowQuickCaptureMsg{Project: project}); !handled {
					t.Fatal("ShowQuickCaptureMsg should be handled")
				}
				qc, ok := app.views.Get(ViewQuickCapture).(*QuickCaptureModel)
				if !ok {
					t.Fatal("expected a QuickCaptureModel to be registered")
				}
				if expand {
					qc.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
					if !qc.helpExpanded {
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
				if got := countFullWidthDividers(content); got != 1 {
					t.Fatalf("%s: expected exactly one app-shell footer separator, got %d\n%s", name, got, plain)
				}
			})
		}
	}
}
