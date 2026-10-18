package tui

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// renderStatusBar composes the shell-owned bottom bar: the active view's status
// strip, when it has one, sits on top of the footer chrome under a static dim
// divider. Growing the status bar is what pins the strip directly above the
// footer, since AppModel.View sizes and pads content against its height.
func (m *AppModel) renderStatusBar() string {
	footer := m.renderFooterChrome()
	provider, ok := m.activeView().(route.ViewStatusStripProvider)
	if !ok {
		return footer
	}
	strip := provider.ViewStatusStrip(m.viewNav.width)
	if strip == "" {
		return footer
	}
	// Deliberately static: the strip is view content, so it is neither one of
	// the flash-capable dividers nor a taker of the footer content wash.
	return chrome.RenderHorizontalDivider(m.viewNav.width) + "\n" + strip + "\n" + footer
}

func (m *AppModel) renderFooterChrome() string {
	// Top border uses the shared separator color, with temporary status flashes.
	separator := m.renderFooterDivider(m.viewNav.width)
	return separator + "\n" + m.renderFooterContent(m.footerChromeContent(), m.viewNav.width)
}

// footerChromeContent is everything the footer says under its divider: what just
// happened, and then what the keys still do. They are two rows rather than one
// occupant, because a message is a report and the controls are the only
// statement of how to leave the surface it was reported from — displacing them
// leaves a user inside a text field whose keys are named nowhere on screen.
//
// Feedback leads so the controls keep the terminal's last rows: the footer is
// pinned to the bottom of the frame, so a line inserted above them grows the
// footer upward and the commit and cancel entries do not move under a message
// arriving and expiring.
//
// It is pure, which is what lets shellFrameMetrics measure the same two blocks
// this composes: nothing here mounts, consumes, or clears.
func (m *AppModel) footerChromeContent() string {
	feedback := m.footerFeedbackLine()
	controls := m.footerControlContent()
	switch {
	case feedback == "":
		return controls
	case controls == "":
		return feedback
	default:
		return feedback + "\n" + controls
	}
}

// footerFeedbackLine is the one line the shell's own status cluster owns: its
// error, else its transient message, else nothing. The error outranks the
// message because they are the same channel reporting the same act, and a
// failure is the half that must not be summarized by a success.
func (m *AppModel) footerFeedbackLine() string {
	switch {
	case m.feedback.err != nil:
		return m.renderFooterRefusal(theme.ErrorStatusStyle, icons.Get("error")+" "+sentenceCaseFeedback(m.feedback.err.Error()))
	case m.feedback.message == "":
		return ""
	case m.feedback.info && m.feedback.severity == flashWarning:
		return m.renderFooterRefusal(theme.InfoStatusStyle, icons.Get("warning")+" "+m.feedback.message)
	case m.feedback.info:
		// Neutral information carries no mark: a leading dot read as a stray
		// bullet, and the neutral color already sets it apart from ✓ and ✕.
		return m.renderFooterMessageRow(theme.InfoStatusStyle, m.feedback.message)
	default:
		return m.renderFooterMessageRow(theme.SuccessStyle, icons.Get("success")+" "+m.feedback.message)
	}
}

// footerRefusalLines gives a reason and its recovery instruction room to wrap
// without consuming the editor or the controls. Longer diagnostics explicitly
// point to the existing full-text history instead of silently losing their tail.
func (m *AppModel) footerRefusalLines(text string) []string {
	width := m.viewNav.width
	if width <= 0 {
		width = 80
	}
	lines := textfit.WrapText(text, width)
	const maxRows = 3
	if len(lines) > maxRows {
		lines = lines[:maxRows]
		lines[maxRows-1] = textfit.TruncateToVisualWidth("F10 → Help → Recent messages for full details", width)
	}
	return lines
}

func (m *AppModel) renderFooterRefusal(style lipgloss.Style, text string) string {
	lines := m.footerRefusalLines(text)
	for i := range lines {
		lines[i] = m.renderFooterMessageRow(style, lines[i])
	}
	return strings.Join(lines, "\n")
}

// sentenceCaseFeedback capitalizes an error's first rune so it reads beside the
// sentence-case successes. A first word carrying a sigil, a digit, a slash, or a
// dot is a slug, key, path, or value and is left exactly as spelled.
func sentenceCaseFeedback(text string) string {
	first, _, _ := strings.Cut(text, " ")
	if first == "" || strings.ContainsAny(first, "0123456789/.!#@$*=%<>~&;?-_") {
		return text
	}
	r, size := utf8.DecodeRuneInString(text)
	if !unicode.IsLower(r) {
		return text
	}
	return string(unicode.ToUpper(r)) + text[size:]
}

// renderFooterMessageRow left-aligns one status line and fills the rest of the
// row with the flash wash, so a message never ends in a hole the wash does not
// cover. The text is fitted before it is styled, because the recovery controls
// a too-wide line must keep are recognized by their spelling.
func (m *AppModel) renderFooterMessageRow(style lipgloss.Style, text string) string {
	width := m.viewNav.width
	if width <= 0 {
		width = 80
	}
	msg := style.Render(fitRecoveryFeedback(text, width))
	gap := max(width-lipgloss.Width(msg), 0)
	spacer := lipgloss.NewStyle().
		Width(gap).
		Background(m.footerFlashBackgroundColor()).
		Render("")
	return lipgloss.JoinHorizontal(lipgloss.Top, msg, spacer)
}

// footerControlContent is the footer's other block: the keys a press would
// reach right now, whoever owns them.
//
// An unfinished chord wins over the surface underneath it, because the which-key
// row describes the very next press; it never competes with feedback, which
// describes the last one.
func (m *AppModel) footerControlContent() string {
	owner, chordSpec := m.activeGuidance()
	if owner == route.GuidanceOwnerPendingChord {
		return chrome.RenderGuidanceText(chordSpec, m.viewNav.width)
	}

	content := m.surfaceFooterContent()
	if owner.SuppressesShellGuidance() {
		// The interaction receiving input has already drawn its own
		// instructions, so this occupant is the second set. It is measured out
		// of the frame either way, so it is blanked at its own height rather
		// than dropped: closing the overlay hands the host back the same rows.
		content = suppressGuidanceRows(content, m.viewNav.width)
	}
	return content
}

// shellFrameMetrics measures the footer this frame will draw, without drawing
// it. It reads the same two blocks footerChromeContent composes, so the height a
// view reserves and the height the shell paints are one measurement taken twice.
//
// It is pure by construction: every call underneath it renders into a string and
// none of them consumes a one-shot notice. ViewStatusStrip is the one status
// surface that does consume — it clears the Outliner's focus notice — and it is
// deliberately not measured here, because the strip is view content the view
// already reserves out of its own budget.
func (m *AppModel) shellFrameMetrics() chrome.ShellFrameMetrics {
	return chrome.ShellFrameMetrics{
		FeedbackRows: footerBlockRows(m.footerFeedbackLine()),
		ControlRows:  footerBlockRows(m.footerControlContent()),
	}
}

// footerBlockRows is what one footer block costs. An absent block costs nothing,
// which lipgloss.Height alone would report as one row.
func footerBlockRows(block string) int {
	if block == "" {
		return 0
	}
	return lipgloss.Height(block)
}

func fitStatusBarSegments(left, right string, width int) (string, string) {
	if width <= 0 {
		width = 80
	}

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	minGap := 0
	if leftWidth > 0 && rightWidth > 0 {
		minGap = 1
	}
	if leftWidth+rightWidth+minGap > width {
		availableLeft := max(width-rightWidth-minGap, 1)
		left = textfit.TruncateToVisualWidth(left, availableLeft)
		leftWidth = lipgloss.Width(left)
		if leftWidth+rightWidth+minGap > width {
			right = textfit.TruncateToVisualWidth(right, max(width-leftWidth-minGap, 1))
		}
	}

	return left, right
}

// buildBreadcrumb constructs a compact route/overlay breadcrumb.
// Each segment uses the view's StatusBarLabel() when available, with fallbacks for common views.
// Project names are truncated to keep the breadcrumb short.
func (m *AppModel) buildBreadcrumb() string {
	const maxProjectLen = 16
	const sep = " > "

	path := m.presentationPath()

	// Build a deduplicated renderer path, keeping the LAST occurrence of each
	// view type (e.g. for stacked project views the deepest project wins).
	lastIdx := make(map[presentation.PresentationID]int)
	for i, vt := range path {
		lastIdx[vt] = i
	}

	var parts []string
	for i, vt := range path {
		if lastIdx[vt] != i {
			continue // skip earlier duplicates
		}
		label := m.breadcrumbLabel(vt, maxProjectLen)
		if label != "" {
			parts = append(parts, label)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	// Deep stacks: abbreviate to last 2 segments with a depth indicator.
	if len(parts) > 3 {
		depth := len(parts)
		abbreviated := "… > " + strings.Join(parts[len(parts)-2:], sep)
		return abbreviated + " " + theme.MutedStyle.Render(fmt.Sprintf("[%d]", depth))
	}

	crumb := strings.Join(parts, sep)
	// Truncate total breadcrumb if it would crowd out the help text.
	maxLen := max(m.viewNav.width/2, 20)
	if lipgloss.Width(crumb) > maxLen {
		crumb = textfit.TruncateToVisualWidth(crumb, maxLen)
	}

	// Esc-preview: show what pressing Esc will return to. The Outliner is the
	// sole root, so suppress the hint there.
	atRoot := m.viewNav.navigation.Primary.Depth() <= 1 && !m.viewNav.navigation.Overlays.Active()
	if len(path) >= 2 && !atRoot {
		prevVT := path[len(path)-2]
		prevLabel := m.breadcrumbLabel(prevVT, maxProjectLen)
		if prevLabel != "" {
			hint := theme.MutedStyle.Render(" ← " + prevLabel)
			if lipgloss.Width(crumb)+lipgloss.Width(hint) <= maxLen {
				crumb += hint
			}
		}
	}

	return crumb
}

// breadcrumbLabel returns a short human-readable label for a single view type.
func (m *AppModel) breadcrumbLabel(vt presentation.PresentationID, maxProjectLen int) string {
	path := m.presentationPath()
	var label string
	if m.activeManageHub() && len(path) > 0 && vt == path[len(path)-1] {
		label = m.manageHub.StatusBarLabel()
	}

	// Use the registered view's StatusBarLabel when available.
	if label == "" {
		if view := m.viewNav.views.Get(vt); view != nil {
			if labeler, ok := view.(route.StatusBarLabeler); ok {
				raw := labeler.StatusBarLabel()
				// Strip "Project: " prefix from project tree label for compactness.
				label = strings.TrimPrefix(raw, "Project: ")
				// Also strip cycle-position suffix (starts with " [") for compactness.
				if idx := strings.Index(label, " ["); idx > 0 {
					label = label[:idx]
				}
				// A segment names a place, not its state: the Outliner's scope and
				// task count already lead its own title row, and cutting them at
				// the length cap below left "Outliner · Acti…" on the widest
				// terminal.
				if idx := strings.Index(label, " · "); idx > 0 {
					label = label[:idx]
				}
				if lipgloss.Width(label) > maxProjectLen {
					label = textfit.TruncateToVisualWidth(label, maxProjectLen)
				}
			}
		}
	}

	return label
}
