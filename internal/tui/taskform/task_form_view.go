package taskform

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// Form layout thresholds.
const (
	// tabContentMinWidth is the natural rendered width of the widest tab body
	// (the Basics tab). The form body is fixed-width and is NOT clamped to the
	// terminal, so it is the minimum width the form needs to render without
	// overflowing even in single-column mode. The two-column help-pane gate
	// measures the actual body width per render (see View), but this constant
	// is the baseline used for the min-size guard below.
	tabContentMinWidth = 68
	// helpPaneGap is the spacer column count between the body and the help pane.
	helpPaneGap = 2
)

// formMinWidth / formMinHeight are the minimum dimensions below which the form
// cannot render without overflowing. The form body is fixed-width
// (tabContentMinWidth), so width genuinely cannot go below it. Height, however,
// only needs to match the app's own floor: the form is allotted terminalHeight-2
// rows (see AppModel.handleWindowSize), so formMinHeight is derived from
// chrome.MinTerminalHeight rather than a second literal. Above formMinHeight the body
// is clamped to the available rows (see View) so it never overflows the height.
// Below either dimension a "terminal too small" notice is shown, mirroring the
// app-level guard in app_view.go.
const (
	formMinWidth  = tabContentMinWidth
	formMinHeight = chrome.MinTerminalHeight - 2
)

func (m *TaskFormModel) View() tea.View {
	if m.width < formMinWidth || m.height < formMinHeight {
		return tea.NewView(m.renderMinSizeNotice())
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

	// ── Per-tab content (or inline overlay) ───────────────────────────────────
	var body string
	if m.activeOverlay != formOverlayNone {
		body = m.renderFormOverlay()
	} else {
		var tabContent string
		switch m.activeTab {
		case formTabBasics:
			tabContent = m.renderBasicsTab()
		case formTabSchedule:
			tabContent = m.renderScheduleTab()
		case formTabChildren:
			tabContent = m.renderChildrenTab()
		case formTabExtras:
			tabContent = m.renderExtrasTab()
		}

		helpWidth := max(min(m.width*35/100, 30), 20)
		if m.width >= lipgloss.Width(tabContent)+helpPaneGap+helpWidth {
			// Two-column: form content left, help pane right. Only shown when
			// the (fixed-width) body, the gap, and the help pane all genuinely
			// fit within the terminal — otherwise the join overflows the width.
			helpPane := m.renderFormHelpPane(helpWidth)
			body = lipgloss.JoinHorizontal(lipgloss.Top, tabContent, "  ", helpPane)
		} else {
			body = tabContent
		}
	}
	// Normalize trailing blank rows so the body's line count reflects only its
	// visible content before it is clamped to the available height.
	body = strings.TrimRight(body, "\n")

	// ── Error banner ──────────────────────────────────────────────────────────
	var errBanner string
	if m.err != nil {
		errBanner = theme.ValidationErrorStyle.Render(m.err.Error())
	}

	// ── Footer ────────────────────────────────────────────────────────────────
	guidance := chrome.RenderGuidance(m.guidanceSpec(), m.width)

	// Clamp the interior body so the fullscreen title, tab bar, and footer stay
	// visible even when the body is inherently taller than the allotted height.
	// Chrome = title + tab bar + two structural blank rows + rendered guidance,
	// plus the error banner when present (it occupies the pre-separator slot).
	chrome := lipgloss.Height(title) + lipgloss.Height(tabBar) + 2 +
		lipgloss.Height(guidance)
	if m.err != nil {
		chrome += lipgloss.Height(errBanner)
	}
	body = clampFormBody(body, m.height-chrome)

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

func (m *TaskFormModel) guidanceSpec() chrome.GuidanceSpec {
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
	return chrome.RequiredActionGuidanceSpec(commit, optional...)
}

// clampFormBody limits the form body to maxRows lines so the surrounding
// fullscreen title, tab bar, and footer always remain visible. When the body is
// taller than maxRows it is clipped and the final visible row is replaced with a
// muted overflow indicator. maxRows is floored at 1 to stay defensive; the
// min-size guard in View keeps the real height well above that.
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
// the form's allotted area. The edit form body is inherently tall (up to 25
// rows) and fixed-width; on smaller terminals this notice replaces the
// overflowing form. It mirrors the app-level guard in app_view.go.
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
