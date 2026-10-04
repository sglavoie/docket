package tui

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

const (
	footerDividerIdleGlyph  = '─'
	footerDividerFlashGlyph = '━'
)

// renderFooterDivider turns a status change into three visible phases: a brief
// full-width impact, an eased severity sweep, and a shrinking dissolve. The
// glyph motion remains visible when the explicit --no-color flag is active.
func (m *AppModel) renderFooterDivider(width int) string {
	if width <= 0 {
		width = 80
	}
	accent := m.currentAccentColor()
	if !m.feedback.flash.active() {
		return chrome.RenderHorizontalDivider(width, chrome.WithDividerColor(accent))
	}

	highlight := m.feedback.flash.severity.color()
	frame := min(max(m.feedback.flash.frame, 0), footerFlashFrames-1)
	if frame < footerFlashImpactFrames {
		return chrome.RenderHorizontalDivider(
			width,
			chrome.WithDividerColor(highlight),
			chrome.WithDividerGlyph(footerDividerFlashGlyph),
		)
	}

	segmentWidth := footerFlashSegmentWidth(width)
	if frame < footerFlashImpactFrames+footerFlashSweepFrames {
		sweepFrame := frame - footerFlashImpactFrames
		progress := float64(sweepFrame) / float64(footerFlashSweepFrames-1)
		start := int(math.Round(easeInOutCubic(progress) * float64(width-segmentWidth)))
		return renderFooterSweep(width, start, segmentWidth, accent, highlight)
	}

	settleFrame := frame - footerFlashImpactFrames - footerFlashSweepFrames
	progress := float64(settleFrame+1) / float64(footerFlashSettleFrames+1)
	eased := easeInOutCubic(progress)
	segmentWidth = max(int(math.Round(float64(segmentWidth)*(1-eased))), 1)
	start := width - segmentWidth
	highlight = blendColor(highlight, accent, eased)
	return renderFooterSweep(width, start, segmentWidth, accent, highlight)
}

func renderFooterSweep(width, start, segmentWidth int, accent, highlight color.Color) string {
	start = min(max(start, 0), width)
	segmentWidth = min(max(segmentWidth, 0), width-start)

	var divider strings.Builder
	divider.Grow(width * 4)
	writeFooterDividerRun(&divider, footerDividerIdleGlyph, start, accent)
	for index := range segmentWidth {
		headProgress := 1.0
		if segmentWidth > 1 {
			headProgress = float64(index) / float64(segmentWidth-1)
		}
		trailFade := footerFlashTrailFade * (1 - easeInOutCubic(headProgress))
		cellColor := blendColor(highlight, accent, trailFade)
		writeFooterDividerRun(&divider, footerDividerFlashGlyph, 1, cellColor)
	}
	writeFooterDividerRun(&divider, footerDividerIdleGlyph, width-start-segmentWidth, accent)
	return divider.String()
}

func writeFooterDividerRun(builder *strings.Builder, glyph rune, count int, foreground color.Color) {
	if count <= 0 {
		return
	}
	style := lipgloss.NewStyle().Foreground(foreground)
	builder.WriteString(style.Render(strings.Repeat(string(glyph), count)))
}

// renderFooterContent fills each footer content row with the active flash wash.
// Child foreground and badge styles remain intact; after a child style resets,
// the wash is restored so unstyled text and trailing cells do not develop holes.
func (m *AppModel) renderFooterContent(content string, width int) string {
	if !m.feedback.flash.active() || content == "" {
		return content
	}
	if width <= 0 {
		width = 80
	}

	background := m.footerFlashBackgroundColor()
	prefix, suffix := footerBackgroundStyleParts(background)
	if prefix == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	for index, line := range lines {
		line = textfit.TruncateToVisualWidth(line, width)
		if gap := width - lipgloss.Width(line); gap > 0 {
			line += strings.Repeat(" ", gap)
		}
		line = strings.ReplaceAll(line, ansi.ResetStyle, ansi.ResetStyle+prefix)
		line = strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+prefix)
		lines[index] = prefix + line + suffix
	}
	return strings.Join(lines, "\n")
}

func footerBackgroundStyleParts(background color.Color) (prefix, suffix string) {
	const marker = "x"
	styled := lipgloss.NewStyle().Background(background).Render(marker)
	prefix, suffix, ok := strings.Cut(styled, marker)
	if !ok || prefix == "" {
		return "", ""
	}
	return prefix, suffix
}

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
	// Top border separator for visual distinction; tinted with current view accent.
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
		return m.renderFooterMessageRow(theme.ErrorStatusStyle.Render(icons.Get("error") + " " + sentenceCaseFeedback(m.feedback.err.Error())))
	case m.feedback.message == "":
		return ""
	case m.feedback.info:
		return m.renderFooterMessageRow(theme.InfoStatusStyle.Render("· " + m.feedback.message))
	default:
		return m.renderFooterMessageRow(theme.SuccessStyle.Render(icons.Get("success") + " " + m.feedback.message))
	}
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
// cover.
func (m *AppModel) renderFooterMessageRow(msg string) string {
	width := m.viewNav.width
	if width <= 0 {
		width = 80
	}
	msg = textfit.TruncateToVisualWidth(msg, width)
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

// activeGuidance resolves the one layer of the composed frame whose
// instructions are live, in the order handleGlobalKeyPress visits its layers: a
// standing confirmation, then a shell-mounted modal, then an unfinished chord,
// then the presentation itself. Whatever it names is the only layer allowed to
// speak, because it is the only one a key press reaches.
//
// The chord spec is returned with the owner so the caller renders exactly what
// the resolution read, rather than asking a second time.
func (m *AppModel) activeGuidance() (route.ActiveGuidanceOwner, chrome.GuidanceSpec) {
	if m.shellConfirmationArmed() {
		return route.GuidanceOwnerConfirmation, chrome.GuidanceSpec{}
	}
	if m.shellModalMounted() {
		return route.GuidanceOwnerModal, chrome.GuidanceSpec{}
	}
	if spec, ok := m.pendingChordGuidance(); ok {
		return route.GuidanceOwnerPendingChord, spec
	}
	if !m.activePresentationPublishesToShell() {
		return route.GuidanceOwnerPresentation, chrome.GuidanceSpec{}
	}
	return route.GuidanceOwnerShell, chrome.GuidanceSpec{}
}

// shellConfirmationArmed reports whether one of the three shell-owned
// confirmations is standing over the view that raised it.
//
// A confirmation the active view owns and answers for through its own
// ContextualGuidance is deliberately not here: there the surface is still the
// layer speaking, and a draft-discard prompt whose exits reach the footer must
// keep reaching it.
func (m *AppModel) shellConfirmationArmed() bool {
	return m.project.ConfirmationArmed() ||
		m.taskMutation.confirmationArmed() ||
		m.operations.Confirm.IsActive()
}

// shellModalMounted covers exactly what handleActiveKeyOverlay routes to:
// whatever that dispatcher hands the next key press is the interaction that owns
// guidance. The action-sheet flow answers through its own OverlayMounted rather
// than a second copy of its four levels, so a level added there is covered here
// without a second edit.
func (m *AppModel) shellModalMounted() bool {
	return m.checkInSetup != nil ||
		m.operations.OverlayMounted() ||
		m.lenses.Mounted() ||
		m.conflictBrowser != nil
}

// activePresentationPublishesToShell asks whether the footer is still this
// presentation's voice. A view answering SelfGuidedPresentation decides per
// frame, since a PresentationID names a view and one view may own two
// presentations and only one of them may have drawn its acts inside its own
// content; otherwise the static chrome policy answers, and it answers by whether
// a host is still standing under this presentation holding that footer.
//
// A fullscreen surface keeps it even though its acts are drawn in its content:
// the footer then holds that same surface's breadcrumb and its `Esc back · ?
// help`, which is the one statement of either and names keys the surface itself
// answers.
//
// An unclassified presentation publishes, deliberately: a footer that may be the
// only instructions on screen is never blanked on a guess.
func (m *AppModel) activePresentationPublishesToShell() bool {
	if view := m.activeView(); view != nil {
		if provider, ok := view.(route.SelfGuidedPresentation); ok && provider.PublishesOwnGuidance() {
			return false
		}
	}
	policy, ok := route.ChromePolicyForPresentation(m.currentView())
	if !ok {
		return true
	}
	return !policy.HostGuidanceStandsBeneath()
}

// suppressGuidanceRows empties an occupant without resizing it. The frame was
// laid out against this height, so returning fewer rows would move the content
// above it and closing the overlay would move it back.
//
// The rows it returns are blank but present, and that distinction is the whole
// of it: the empty string is how this footer spells a block that is not there at
// all, so a one-row occupant blanked to "" would be composed and measured as
// nothing and the footer would shrink under a message it is still carrying.
// Each held row is filled to the footer's own width for the same reason
// renderFooterMessageRow fills its own: a row the footer is spending is a row it
// covers, so the wash and the background find no hole where the occupant stood.
//
// An absent occupant stays absent: there is no row to hold, so blanking one
// would charge the frame for guidance nobody wrote.
func suppressGuidanceRows(content string, width int) string {
	rows := footerBlockRows(content)
	if rows == 0 {
		return ""
	}
	blank := strings.Repeat(" ", max(width, 1))
	return strings.TrimSuffix(strings.Repeat(blank+"\n", rows), "\n")
}

// surfaceFooterContent builds what the shell footer says on behalf of the
// active surface: its contextual guidance, else the pinned-task bar, else the
// breadcrumb row with its help hint. It is one occupant, so suppression takes
// it whole — while a modal owns every key, the breadcrumb's Esc preview names a
// destination that press would not reach.
func (m *AppModel) surfaceFooterContent() string {
	// Show the current view's contextual guidance if it provides any (e.g. the
	// capture views). This sits above the pinned-task bar and generic breadcrumb
	// but below the transient loading/error/status overlays handled by the
	// caller, so a temporary status message still replaces capture controls and
	// the controls return once it clears.
	if view := m.activeView(); view != nil {
		if provider, ok := view.(route.ContextualGuidanceProvider); ok {
			if spec, ok := provider.ContextualGuidance(); ok {
				return chrome.RenderGuidanceText(spec, m.viewNav.width)
			}
		}
	}

	// Show pinned tasks bar if any tasks are pinned.
	if pinnedBar := m.renderPinnedTasksBar(m.viewNav.width); pinnedBar != "" {
		return pinnedBar
	}

	status := m.buildBreadcrumb()

	// Check if view supports help shortcut hint; only show if view implements HelpProvider.
	currentView := m.currentView()
	showHelp := false
	if view := m.activeView(); view != nil {
		if _, ok := view.(help.HelpProvider); ok {
			showHelp = true
		}
	}

	help := ""
	if showHelp {
		if hint := pipelineNextHint(currentView); hint != "" {
			help = theme.HelpStyle.Render(hint)
		} else {
			help = strings.Join(
				chrome.RenderGuidanceLines(chrome.EscapeGuidanceSpec(), m.viewNav.width),
				theme.HelpStyle.Render(chrome.FooterHelpSeparator),
			)
		}
	}

	accent := m.currentAccentColor()
	accentBarStyle := lipgloss.NewStyle().
		Foreground(accent).
		Background(m.footerFlashBackgroundColor()).
		Padding(0, 1)

	left := accentBarStyle.Render(status)

	// Add filter pill if current view has active filters.
	if view := m.activeView(); view != nil {
		if fp, ok := view.(route.ActiveFilterProvider); ok {
			if count := fp.ActiveFilterCount(); count > 0 {
				filterLabel := "filters"
				if count == 1 {
					filterLabel = "filter"
				}
				left += " " + theme.FilterBadgeStyle.Render(fmt.Sprintf("%d %s", count, filterLabel))
			}
		}
	}

	// Prepend mode badge when the current view is in a named mode.
	if view := m.activeView(); view != nil {
		if mi, ok := view.(route.ModeIndicator); ok {
			if label, c := mi.ActiveMode(); label != "" {
				badge := lipgloss.NewStyle().
					Bold(true).
					Background(c).
					Foreground(theme.ColorBackground).
					Padding(0, 1).
					Render(label)
				left = badge + " " + left
			}
		}
	}

	right := help
	left, right = fitStatusBarSegments(left, right, m.viewNav.width)
	gap := max(m.viewNav.width-lipgloss.Width(left)-lipgloss.Width(right), 0)
	spacer := lipgloss.NewStyle().
		Width(gap).
		Background(m.footerFlashBackgroundColor()).
		Render("")

	return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
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

// The footer divider takes a severity color the moment warning or error text
// arrives, lands as a full-width impact, then sweeps and dissolves back into the
// view accent so a message that needs attention at the bottom of a busy screen
// is not missed. Routine outcomes change the text and nothing else.
// Named flash, not pulse: workspacePulse is the title row's counts chip.
const (
	footerFlashFrames        = 20
	footerFlashFrameInterval = 25 * time.Millisecond
	footerFlashImpactFrames  = 4
	footerFlashSweepFrames   = 13
	footerFlashSettleFrames  = footerFlashFrames - footerFlashImpactFrames - footerFlashSweepFrames
	footerFlashTrailFade     = 0.8
	footerFlashBackgroundMix = 0.14
)

type footerFlashSeverity int

const (
	flashNone footerFlashSeverity = iota
	flashInfo
	flashSuccess
	flashWarning
	flashError
)

// color resolves against the live palette rather than the palette in effect
// when the flash was armed, so a theme toggle mid-flash cannot strand a frame
// on the previous theme's color.
func (s footerFlashSeverity) color() color.Color {
	switch s {
	case flashSuccess:
		return theme.ColorSuccess
	case flashWarning:
		return theme.ColorWarning
	case flashError:
		return theme.ColorError
	default:
		return theme.ColorPrimary
	}
}

// animates reports whether this severity earns the divider animation and the
// footer wash beneath it. Only a warning and an error do.
//
// A success or an informational line is the expected answer to the key that was
// just pressed: the text says what happened and the reader already knew to look
// for it, so motion at the bottom of the screen buys nothing and spends the one
// thing the warning needs — that the same motion is unusual. Severity is still
// recorded for every status, because that record is exactly what tells the two
// apart here.
func (s footerFlashSeverity) animates() bool {
	return s == flashWarning || s == flashError
}

type footerFlashState struct {
	severity footerFlashSeverity
	frame    int
	seq      uint64
}

func (s footerFlashState) active() bool { return s.severity != flashNone }

// statusStamper is the second status surface: views embedding
// InteractiveViewState own a status line the shell does not write.
type statusStamper interface {
	StatusStamp() uint64
}

// viewStatusOwner is that same status line seen by the expiry timer: it stamps
// generations, declares whether the current message is transient, and clears
// itself when its tick comes back.
//
// Every method here must be exported. An unexported method name is qualified by
// its declaring package in a Go method set, so once InteractiveViewState moved
// down to internal/tui/listsurface, a lowercase clearStatus() in this literal
// became unsatisfiable by any view — a silent ok == false that no build, vet, or
// lint pass reports. The assertion below is what makes that failure loud.
type viewStatusOwner interface {
	statusStamper
	StatusIsTransient() bool
	StatusText() string
	ClearStatus()
}

// The Outliner is the view this interface exists for, so its conformance is
// asserted at compile time rather than discovered as a dead type switch.
var _ viewStatusOwner = (*outliner.OutlinerModel)(nil)

// Notes keeps its own status line and error styling, so it answers the same
// interface directly rather than through InteractiveViewState.
var _ viewStatusOwner = (*notes.NotesViewModel)(nil)

// footerFlashProbe is the status identity compared across one Update. The
// shell's own message carries a sequence, but the error beside it has ~87
// assignment sites and no clock, so it is compared by text. The view stamp is
// only meaningful alongside the view it came from, hence the presentation ID.
type footerFlashProbe struct {
	statusText    string
	statusSeq     uint64
	errText       string
	view          presentation.PresentationID
	viewStatusSeq uint64
	// pathStamps is the generation every stamping view along the live path held,
	// so a view this Update surfaced is compared against its own earlier stamp:
	// closing capture onto the Outliner changes the view and the Outliner's
	// status in one Update, and neither the capture's stamp nor none at all is
	// the right thing to measure that status against.
	pathStamps []viewStatusStamp
}

type viewStatusStamp struct {
	view presentation.PresentationID
	seq  uint64
}

// stampFor reports the generation view held when the probe was taken, and
// whether it was on the path at all. A view's own stamp outranks the active
// one, since a bottom panel such as capture is the active view without being
// the presentation it stands over.
func (p footerFlashProbe) stampFor(view presentation.PresentationID) (uint64, bool) {
	for _, stamp := range p.pathStamps {
		if stamp.view == view {
			return stamp.seq, true
		}
	}
	if view == p.view {
		return p.viewStatusSeq, true
	}
	return 0, false
}

func (m *AppModel) captureFooterFlashProbe() footerFlashProbe {
	probe := footerFlashProbe{
		statusSeq:  m.feedback.seq,
		statusText: m.feedback.message,
		view:       m.currentView(),
	}
	if m.feedback.err != nil {
		probe.errText = m.feedback.err.Error()
	}
	if stamper, ok := m.activeView().(statusStamper); ok {
		probe.viewStatusSeq = stamper.StatusStamp()
	}
	if m.viewNav.views != nil {
		for _, view := range m.statusStampingPath() {
			if stamper, ok := m.viewNav.views.Get(view).(statusStamper); ok {
				probe.pathStamps = append(probe.pathStamps, viewStatusStamp{view: view, seq: stamper.StatusStamp()})
			}
		}
	}
	return probe
}

// armFooterFlashSince starts a flash when this Update changed status text to
// something worth interrupting for. Cleared text arms nothing: neither
// clearStatus nor the auto-clear timers advance a sequence, and a vanished error
// fails the non-empty test.
//
// Routine text changes nothing about the divider and still supersede: a success,
// an informational line, and a view-owned notice are all quiet, and the one they
// replace stops animating under them. The two questions are deliberately
// separate — which change happened, then whether that change animates — because
// a change that arms nothing is not the same as no change at all, and only the
// first of those retires what was running.
func (m *AppModel) armFooterFlashSince(before footerFlashProbe) tea.Cmd {
	after := m.captureFooterFlashProbe()

	severity := flashNone
	switch {
	case after.errText != "" && after.errText != before.errText:
		severity = flashError
	case after.statusSeq != before.statusSeq:
		severity = m.feedback.severity
	case after.view == before.view && after.viewStatusSeq != before.viewStatusSeq:
		severity = flashInfo
	}
	if severity == flashNone {
		// No status changed, so nothing was superseded either: a flash already
		// running belongs to text still on screen and keeps its remaining frames.
		return nil
	}
	if !severity.animates() {
		m.feedback.retireFlash()
		return nil
	}

	return footerFlashTick(m.feedback.armFlash(severity))
}

// statusStampingPath is every presentation whose own status line can be live:
// the navigation path, or the Outliner a path-less launch stands on.
func (m *AppModel) statusStampingPath() []presentation.PresentationID {
	if path := m.presentationPath(); len(path) > 0 {
		return path
	}
	return []presentation.PresentationID{m.currentView()}
}

// scheduleViewStatusExpiry arms the auto-clear for a view-owned status message
// this Update introduced. It reads the same before/after comparison as the
// flash, since a new message is what both observe, but it decides for itself: a
// view notice is quiet and still expires on time, so how long a message lasts
// never follows from whether it animated. A sticky message schedules nothing and
// stays until its view clears it; the Outliner writes none, because a live state
// descriptor belongs on the context row rather than in the channel a piece of
// feedback is about to want back.
func (m *AppModel) scheduleViewStatusExpiry(before footerFlashProbe) tea.Cmd {
	if m.viewNav.views == nil {
		return nil
	}
	// Every view on the path is asked, not just the one on top: a notice can be
	// written onto the Outliner while capture or help stands over it, and its
	// timer has to start when the notice does rather than when the cover lifts.
	var expiries []tea.Cmd
	for _, view := range m.statusStampingPath() {
		owner, ok := m.viewNav.views.Get(view).(viewStatusOwner)
		if !ok || !owner.StatusIsTransient() {
			continue
		}
		seq := owner.StatusStamp()
		if held, known := before.stampFor(view); known && held == seq {
			continue
		}
		expiries = append(expiries, status.ViewStatusTimeout(view, seq, owner.StatusText()))
	}
	return tea.Batch(expiries...)
}

// advanceFooterFlash steps one frame and reports the command that schedules the
// next. A stale tick from a superseded flash is rejected by sequence.
func (m *AppModel) advanceFooterFlash(seq uint64) tea.Cmd {
	if !m.feedback.advanceFlash(seq) {
		return nil
	}
	return footerFlashTick(seq)
}

// blendColor mixes from toward to. Palette entries are opaque, so a per-channel
// lerp over 8-bit components is exact; a translucent input cannot be lerped
// meaningfully through alpha-premultiplied values, so it steps instead.
func blendColor(from, to color.Color, t float64) color.Color {
	if from == nil {
		return to
	}
	if to == nil {
		return from
	}
	t = min(max(t, 0), 1)

	fromR, fromG, fromB, fromA := rgba8(from)
	toR, toG, toB, toA := rgba8(to)
	if fromA != 0xff || toA != 0xff {
		if t < 0.5 {
			return from
		}
		return to
	}

	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x",
		lerp8(fromR, toR, t),
		lerp8(fromG, toG, t),
		lerp8(fromB, toB, t),
	))
}

func rgba8(c color.Color) (r, g, b, a uint8) {
	red, green, blue, alpha := c.RGBA()
	return uint8(red >> 8), uint8(green >> 8), uint8(blue >> 8), uint8(alpha >> 8)
}

func lerp8(from, to uint8, t float64) uint8 {
	return uint8(math.Round(float64(from) + (float64(to)-float64(from))*t))
}

// easeInOutCubic gives the sweep a deliberate start and soft landing instead
// of moving a fixed number of cells on every frame.
func easeInOutCubic(t float64) float64 {
	t = min(max(t, 0), 1)
	if t < 0.5 {
		return 4 * t * t * t
	}
	remaining := -2*t + 2
	return 1 - remaining*remaining*remaining/2
}

func footerFlashSegmentWidth(width int) int {
	return min(max(width/6, 12), min(20, width))
}

// footerFlashBackgroundColor adds a restrained severity wash beneath the
// footer content. It arrives with the divider impact and eases all the way back
// to the theme background, leaving the moving divider as the dominant cue.
func (m *AppModel) footerFlashBackgroundColor() color.Color {
	background := theme.ColorBackground
	if !m.feedback.flash.active() {
		return background
	}

	frame := min(max(m.feedback.flash.frame, 0), footerFlashFrames-1)
	progress := float64(frame) / float64(footerFlashFrames-1)
	mix := footerFlashBackgroundMix * (1 - easeInOutCubic(progress))
	return blendColor(background, m.feedback.flash.severity.color(), mix)
}

// The shell half of the advisory opt-out. advisories.go owns what an advisory
// is and the two helpers every ambient line routes through; this file owns the
// session value those helpers read, which is where the machine-local setting and
// the process-level `--no-advisories` flag meet.
//
// The suppression is session state on the shell rather than a read of the
// machine file, because two inputs decide it: the machine-local flag the menu
// toggle writes, and the process-level `--no-advisories` flag, which forces
// advisories hidden for one run and writes nothing. The toggle commits, so
// toggling back after a flagged launch persists "shown" — the state a user can
// see is the state that was saved.

// The session value itself lives on the feedback cluster beside the conflict
// count, because both are ambient status-strip inputs and both travel into an
// Outliner as the same kind of injected accessor; see feedback_controller.go.

// WithHiddenAdvisories starts the session with advisories suppressed. RunApp
// translates the `--no-advisories` flag into this option; the machine-local
// setting is read in NewAppModel, and either one hiding is enough.
func WithHiddenAdvisories() AppOption {
	return func(m *AppModel) { m.feedback.hideAdvisories = true }
}

// coreToggleAdvisories flips the machine-local setting and the session value
// together. It mirrors coreToggleMenuBar: chrome follows the terminal you are
// sitting at, so the durable home is machine.json rather than the workspace.
func (m *AppModel) coreToggleAdvisories() error {
	if m.domain.runtime == nil || m.domain.runtime.Machine() == nil {
		return fmt.Errorf("machine chrome state is unavailable")
	}
	machine := *m.domain.runtime.Machine()
	machine.AdvisoriesHidden = !m.feedback.hideAdvisories
	if err := m.domain.runtime.CommitMachine(&machine); err != nil {
		return fmt.Errorf("save advisory visibility: %w", err)
	}
	m.feedback.hideAdvisories = machine.AdvisoriesHidden
	if m.feedback.hideAdvisories {
		m.setInfoStatus("Advisories hidden")
	} else {
		m.setInfoStatus("Advisories shown")
	}
	return nil
}

// The shell half of the outstanding-conflict indicator. conflict_indicator.go
// owns the line the Outliner's status strip draws; this file owns the count it
// draws, which is cached here so no render reads the filesystem.
//
// The store is read straight from the storage layer rather than through an
// internal/app port, following the direct storage.LoadProject reads the TUI
// already makes: nothing here validates, mutates, or commits, so there is no
// application-layer policy for a port to hold.

// countOutstandingConflicts reports how many conflict bundles the workspace has
// recorded. An absent directory is zero, not an error, and a read failure is
// also zero: an indicator that cannot be counted must not be asserted.
func countOutstandingConflicts(dataDir string) int {
	if dataDir == "" {
		return 0
	}
	records, err := storage.NewConflictStore(config.ConflictsDir(dataDir)).List()
	if err != nil {
		logging.Warn("counting outstanding conflicts: %v", err)
		return 0
	}
	return len(records)
}

// refreshOutstandingConflicts recounts the workspace's conflict bundles. It runs
// at launch and on the existing 60s periodic tick, which is the same cadence the
// rest of the shell's workspace state is refreshed on; it arms no timer of its
// own.
func (m *AppModel) refreshOutstandingConflicts() {
	if m == nil || m.domain.runtime == nil {
		return
	}
	m.feedback.outstandingConflicts = countOutstandingConflicts(m.domain.runtime.DataDir())
}

// The count itself lives on the feedback cluster, which also supplies the
// accessor injected into every Outliner; see feedback_controller.go.

// rememberFeedbackSince follows the same generations as status expiry, keeping
// the complete latest feedback after its display timer or navigation clears it.
// Ambient advisory lines are not status messages and never enter this record.
func (m *AppModel) rememberFeedbackSince(before footerFlashProbe) {
	after := m.captureFooterFlashProbe()
	if after.errText != "" && after.errText != before.errText {
		m.feedback.lastMessage = after.errText
		return
	}
	if m.feedback.message != "" && (after.statusSeq != before.statusSeq || after.statusText != before.statusText) {
		m.feedback.lastMessage = m.feedback.message
		return
	}
	if m.viewNav.views == nil {
		return
	}
	for _, view := range m.statusStampingPath() {
		owner, ok := m.viewNav.views.Get(view).(viewStatusOwner)
		if !ok || !owner.StatusIsTransient() || owner.StatusText() == "" {
			continue
		}
		if held, known := before.stampFor(view); known && held == owner.StatusStamp() {
			continue
		}
		m.feedback.lastMessage = owner.StatusText()
	}
}
