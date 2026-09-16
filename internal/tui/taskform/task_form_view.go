package taskform

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// Form layout thresholds.
const (
	// formBodyPreferredWidth is the width the tab bodies are laid out at when the
	// terminal can spare it: the title textarea's own width, and therefore the
	// widest natural row. It is a ceiling, not a floor — the body is rendered at
	// min(formBodyPreferredWidth, terminal width), so a wide terminal draws
	// exactly what it always drew while a narrow one shortens its controls
	// instead of spilling past the frame.
	formBodyPreferredWidth = 68
	// helpPaneGap is the spacer column count between the body and the help pane.
	helpPaneGap = 2
)

// formMinWidth / formMinHeight are the minimum dimensions below which the form
// cannot render at all. Both are the app's own floor rather than a second
// literal: the body resizes from the available width, so the form needs no more
// room than the surfaces it is opened from, and it is allotted terminalHeight-2
// rows (see AppModel.handleWindowSize). Above formMinHeight the body travels
// through scrollFormBody so it never overflows the height. Below either
// dimension a "terminal too small" notice is shown, mirroring the app-level
// guard in app_view.go.
const (
	formMinWidth  = chrome.MinTerminalWidth
	formMinHeight = chrome.MinTerminalHeight - 2
)

func (m *TaskFormModel) View() tea.View {
	if m.width < formMinWidth || m.height < formMinHeight {
		return tea.NewView(m.renderMinSizeNotice())
	}
	if m.activeOverlay != formOverlayNone {
		return tea.NewView(m.renderOverlayView())
	}

	// ── Title ────────────────────────────────────────────────────────────────
	titleText := "Create Task"
	subtitle := ""
	if m.hostTitle != "" {
		titleText = m.hostTitle
	}
	title := chrome.RenderFullscreenTitleWithPulse(titleText, subtitle, m.CurrentWorkspacePulse(), m.width)

	// ── Tab bar ───────────────────────────────────────────────────────────────
	tabBar := chrome.RenderTabBar(formTabNames, m.activeTab, m.width-chrome.CreatorTabBarPadding, 1)

	// ── Per-tab content ───────────────────────────────────────────────────────
	// One width reaches every renderer, so a control's budget is the row's
	// remainder rather than a per-file guess at the terminal.
	bodyWidth := min(formBodyPreferredWidth, m.width)
	var tabBody formBodyBuilder
	switch m.activeTab {
	case formTabBasics:
		m.renderBasicsTab(&tabBody, bodyWidth)
	case formTabSchedule:
		m.renderScheduleTab(&tabBody, bodyWidth)
	case formTabChildren:
		m.renderChildrenTab(&tabBody, bodyWidth)
	case formTabExtras:
		m.renderExtrasTab(&tabBody, bodyWidth)
	}
	// Normalize trailing blank rows so the body's line count reflects only its
	// visible content before it is windowed to the available height.
	tabContent := strings.TrimRight(tabBody.String(), "\n")

	// The two-column gate measures the WHOLE tab content, before the viewport
	// windows it, so scrolling a tall body never moves the help pane in and out
	// of the layout as the focus travels.
	// Two-column: form content left, help pane right. The pane is measured as
	// rendered rather than as requested, because it is reference text drawn at
	// the width its own entries need — a gate reading an intended width instead
	// would let a wider pane overflow the join.
	helpPane := m.renderFormHelpPane()
	if m.width < lipgloss.Width(tabContent)+helpPaneGap+lipgloss.Width(helpPane) {
		helpPane = ""
	}

	// ── Error banner ──────────────────────────────────────────────────────────
	// A refusal names the field the viewport is about to scroll to, so it is the
	// one piece of text the form may not shorten: it wraps to the terminal and
	// pays for every row it takes out of the body's budget below.
	var errBanner string
	if m.err != nil {
		errBanner = theme.ValidationErrorStyle.Render(strings.Join(textfit.WrapText(m.err.Error(), m.width), "\n"))
	}

	// ── Footer ────────────────────────────────────────────────────────────────
	guidance := chrome.RenderGuidance(m.guidanceSpec(), m.width)

	// Window the interior body so the fullscreen title, tab bar, error banner,
	// and footer stay visible even when the body is inherently taller than the
	// allotted height. Chrome = title + tab bar + two structural blank rows +
	// rendered guidance, plus the error banner when present (it occupies the
	// pre-separator slot). The banner is chrome rather than body for exactly this
	// reason: a refusal names a field the viewport then scrolls to, and the two
	// are read together or not at all.
	chrome := lipgloss.Height(title) + lipgloss.Height(tabBar) + 2 +
		lipgloss.Height(guidance)
	if m.err != nil {
		chrome += lipgloss.Height(errBanner)
	}
	bodyRows := m.height - chrome

	// The viewport travels the form body alone. The help pane is anchored to the
	// top because it describes the focused field, so scrolling it away at the one
	// moment its text changed would be the opposite of what the travel is for.
	body := scrollFormBody(tabContent, tabBody.marks, m.focusIndex, bodyRows)
	if helpPane != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, "  ", clampFormHelpPane(helpPane, bodyRows))
	}
	// A help pane taller than the windowed body is the one thing the viewport
	// cannot shorten, so the clamp stays as the last defensive cut.
	body = clampFormBody(body, bodyRows)

	var out strings.Builder
	out.WriteString(title)
	out.WriteString("\n")
	out.WriteString(tabBar)
	out.WriteString("\n\n")
	out.WriteString(body)
	if m.err != nil {
		out.WriteString("\n\n")
		out.WriteString(errBanner)
	} else {
		out.WriteString("\n")
	}
	out.WriteString("\n")
	out.WriteString(guidance)

	return tea.NewView(out.String())
}

// renderOverlayView gives an open inline overlay the form's whole allotted
// area. The overlay owns every key while it is open, so the tab bar and the
// footer would name routes that are not live — and the rows they cost are
// exactly the rows a picker's own list needs at the minimum size. Hidden, not
// shrunk: a frame that loses its bottom border, or a state list that loses a
// state, is worse than a form that waits one Esc to come back.
func (m *TaskFormModel) renderOverlayView() string {
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Render(clampFormBody(m.renderFormOverlay(), m.height))
}

func (m *TaskFormModel) guidanceSpec() chrome.GuidanceSpec {
	if m.ConfirmDiscard {
		return chrome.DiscardGuidanceSpec()
	}
	commit := "Ctrl+S save"
	if m.err != nil {
		commit = "Ctrl+S retry"
	}
	optional := []string{"[/] tabs", "Tab next", "Shift+Tab previous"}
	switch m.activeTab {
	case formTabChildren:
		optional = []string{"c checklist", "Ctrl+A add item", "Enter edit item", "Space toggle", "Ctrl+D delete", "Ctrl+J/K reorder"}
		optional = append([]string{"S subtasks", "Ctrl+T add subtask"}, optional...)
	case formTabExtras:
		optional = []string{"a attachments", "Ctrl+L add", "Enter edit", "Ctrl+D delete", "Ctrl+J/K reorder"}
		optional = append([]string{"c comments", "Ctrl+Y add comment"}, optional...)
	}
	return chrome.RequiredExitGuidanceSpec(commit, chrome.DraftEscapeHint(m.escRaisesDiscard()), optional...)
}

// clampFormBody limits a body to maxRows lines so the surrounding fullscreen
// title, tab bar, and footer always remain visible. It is the last defensive cut
// rather than the form's height policy: the form's own fields travel through
// scrollFormBody, which reaches every one of them, and what reaches this are the
// two blocks no focus indexes — an inline overlay and a help pane taller than
// the window it sits beside — plus the compact composer's already-budgeted
// layout. maxRows is floored at 1 to stay defensive; the min-size guard in View
// keeps the real height well above that.
func clampFormBody(body string, maxRows int) string {
	if maxRows < 1 {
		maxRows = 1
	}
	lines := strings.Split(body, "\n")
	if len(lines) <= maxRows {
		return body
	}
	kept := append(lines[:maxRows-1:maxRows-1], theme.MutedStyle.Render("↓ more (resize terminal to view all)"))
	return strings.Join(kept, "\n")
}

// renderMinSizeNotice renders a centered "terminal too small" message that fills
// the form's allotted area. It answers the one size the form genuinely cannot
// serve — smaller than the app's own floor, which every other surface already
// refuses — and mirrors the app-level guard in app_view.go.
func (m *TaskFormModel) renderMinSizeNotice() string {
	msg := fmt.Sprintf("Terminal too small (need %d×%d, have %d×%d)",
		formMinWidth, formMinHeight, m.width, m.height)
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(theme.ColorMuted).
		Render(msg)
}
