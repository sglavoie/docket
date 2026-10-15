package pulse

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

const (
	operationalTitleSeparator       = " · "
	operationalTitleContextGap      = "  "
	operationalTitlePulseGap        = 2
	operationalTitleMinContextWidth = 14
)

// OperationalTitleSpec describes the semantic regions of a one-row
// operational header. Segment strings may already contain ANSI styling; all
// fitting is based on terminal-cell width rather than bytes or runes.
type OperationalTitleSpec struct {
	Surface string
	Scope   string
	Mode    string
	Context []string
	Pace    *TodayPaceReadout
	Pulse   *WorkspacePulse
}

// RenderOperationalTitle composes identity, interaction context, and passive
// workspace state onto exactly one non-wrapping row. Passive pulse components
// are removed in the product-wide precedence order (Done, Inbox, Overdue, Today)
// before required interaction context is shortened.
func RenderOperationalTitle(spec OperationalTitleSpec, width int) string {
	if width <= 0 {
		return ""
	}

	identity := renderOperationalIdentity(spec, width)
	context := joinOperationalTitleSegments(spec.Context)
	base := joinOperationalTitleRegions(identity, context)

	// Today pace is middle-region context, but it remains optional relative to
	// active interaction state. Its full form outranks the passive workspace
	// pulse; once the pulse ladder is exhausted, pace shortens without restoring
	// a lower-priority pulse.
	if spec.Pace != nil {
		paceVariants := operationalPaceVariants(*spec.Pace)
		fullPaceBase := joinOperationalTitleRegions(base, paceVariants[0])
		if spec.Pulse != nil {
			for _, pulse := range operationalPulseVariants(*spec.Pulse) {
				if pulse == "" {
					break
				}
				if rendered, ok := rightAlignOperationalPulse(fullPaceBase, pulse, width); ok {
					return rendered
				}
			}
		}
		if lipgloss.Width(fullPaceBase) <= width {
			return fullPaceBase
		}
		for _, pace := range paceVariants[1:] {
			candidate := joinOperationalTitleRegions(base, pace)
			if lipgloss.Width(candidate) <= width {
				return candidate
			}
		}
	} else if spec.Pulse != nil {
		for _, pulse := range operationalPulseVariants(*spec.Pulse) {
			if pulse == "" {
				break
			}
			if rendered, ok := rightAlignOperationalPulse(base, pulse, width); ok {
				return rendered
			}
		}
	}
	if lipgloss.Width(base) <= width {
		return base
	}

	if context == "" {
		return renderOperationalIdentity(spec, width)
	}

	// Keep a recognizable prefix of active interaction state by allowing a long
	// scope label to contract first. Surface and mode remain stable anchors.
	contextReserve := min(lipgloss.Width(context), operationalTitleMinContextWidth)
	identityBudget := max(width-lipgloss.Width(operationalTitleContextGap)-contextReserve, 1)
	identity = renderOperationalIdentity(spec, identityBudget)
	contextBudget := width - lipgloss.Width(identity) - lipgloss.Width(operationalTitleContextGap)
	if contextBudget <= 0 {
		return renderOperationalIdentity(spec, width)
	}
	return identity + operationalTitleContextGap + textfit.TruncateToVisualWidth(context, contextBudget)
}

// OperationalTitleFitsWhole reports whether spec renders at width with nothing
// shortened: the whole identity and context, the full Today pace, and the full
// workspace pulse at its gap. A caller offering a richer segment uses it to
// spend only columns the title would otherwise leave blank, so the richer form
// never costs the pulse or pace a rung of its ladder.
func OperationalTitleFitsWhole(spec OperationalTitleSpec, width int) bool {
	if width <= 0 {
		return false
	}
	identity := joinOperationalTitleSegments([]string{spec.Surface, spec.Scope, spec.Mode})
	base := joinOperationalTitleRegions(identity, joinOperationalTitleSegments(spec.Context))
	if spec.Pace != nil {
		base = joinOperationalTitleRegions(base, operationalPaceVariants(*spec.Pace)[0])
	}
	need := lipgloss.Width(base)
	if spec.Pulse != nil {
		if pulse := operationalPulseVariants(*spec.Pulse)[0]; pulse != "" {
			need += operationalTitlePulseGap + lipgloss.Width(pulse)
		}
	}
	return need <= width
}

func renderOperationalIdentity(spec OperationalTitleSpec, width int) string {
	parts := []string{spec.Surface, spec.Scope, spec.Mode}
	identity := joinOperationalTitleSegments(parts)
	if width <= 0 || lipgloss.Width(identity) <= width {
		return identity
	}

	// Scope is the elastic identity field. Keep surface and mode visible and
	// shorten only the scope when the full identity does not fit.
	fixedWidth := lipgloss.Width(spec.Surface) + lipgloss.Width(spec.Mode)
	separatorCount := 0
	if spec.Surface != "" && spec.Scope != "" {
		separatorCount++
	}
	if spec.Scope != "" && spec.Mode != "" {
		separatorCount++
	}
	scopeBudget := width - fixedWidth - separatorCount*lipgloss.Width(operationalTitleSeparator)
	if spec.Scope != "" && scopeBudget > 0 {
		parts[1] = textfit.TruncateToVisualWidth(spec.Scope, scopeBudget)
		identity = joinOperationalTitleSegments(parts)
		if lipgloss.Width(identity) <= width {
			return identity
		}
	}

	return textfit.TruncateToVisualWidth(identity, width)
}

// ContextRowSegment is one axis of interaction context on its way to the row
// under the operational title. Full is what the axis says when there is room
// for it; Compact is the shorter spelling of the same axis, empty where the
// axis has only one form. Neither is ever cut in the middle: the row drops a
// whole segment instead, and names how many it dropped.
type ContextRowSegment struct {
	Full    string
	Compact string
}

func (s ContextRowSegment) compact() string {
	if s.Compact == "" {
		return s.Full
	}
	return s.Compact
}

// RenderContextRow composes interaction context onto exactly one non-wrapping
// row below the operational title, in four steps: every segment in full, every
// segment compact, then whole segments dropped from the right, and last a
// single shortened segment. Every step past the first reserves room for the
// overflow suffix, so a row that dropped something always says so and always
// names where the whole of it is readable.
//
// detailsHint is where the dropped values still are — the readout key — and it
// is passed in rather than spelled here because this package owns the row's
// arithmetic and not the key that opens the readout.
func RenderContextRow(segments []ContextRowSegment, width int, detailsHint string) string {
	if width <= 0 || len(segments) == 0 {
		return ""
	}
	separator := theme.MutedStyle.Render(operationalTitleSeparator)

	full := make([]string, 0, len(segments))
	compact := make([]string, 0, len(segments))
	for _, segment := range segments {
		full = append(full, segment.Full)
		compact = append(compact, segment.compact())
	}
	for _, candidate := range [][]string{full, compact} {
		if line := strings.Join(candidate, separator); lipgloss.Width(line) <= width {
			return line
		}
	}

	for shown := len(compact) - 1; shown > 0; shown-- {
		candidate := strings.Join(compact[:shown], separator) +
			operationalTitleContextGap + contextRowOverflow(len(compact)-shown, detailsHint)
		if lipgloss.Width(candidate) <= width {
			return candidate
		}
	}

	// Not even one whole segment fits beside the count. The first one is kept and
	// shortened rather than dropped, because a row saying only how much is missing
	// says less than a row that also says what the first axis is; every value it
	// cut is still readable whole under the readout key the suffix names.
	suffix := contextRowOverflow(len(compact)-1, detailsHint)
	budget := width - lipgloss.Width(suffix) - lipgloss.Width(operationalTitleContextGap)
	if budget < operationalTitleMinContextWidth {
		return textfit.TruncateToVisualWidth(compact[0], width)
	}
	return textfit.TruncateToVisualWidth(compact[0], budget) + operationalTitleContextGap + suffix
}

// contextRowOverflow names what the row could not draw. The count is omitted
// when nothing was dropped whole, which is the shortened-value case: there the
// hint alone is the honest statement, since every label is still on the row.
func contextRowOverflow(dropped int, detailsHint string) string {
	parts := make([]string, 0, 2)
	if dropped > 0 {
		parts = append(parts, fmt.Sprintf("+%d more", dropped))
	}
	if detailsHint != "" {
		parts = append(parts, detailsHint)
	}
	return theme.MutedStyle.Render(strings.Join(parts, operationalTitleSeparator))
}

// OperationalTitleBadge renders a background-styled title chip without the
// theme's padding cells. The row's separator and context gap own every
// inter-region space, so a padded edge widens one of those gaps by a cell.
func OperationalTitleBadge(style lipgloss.Style, text string) string {
	return style.Padding(0, 0).Render(text)
}

func joinOperationalTitleSegments(parts []string) string {
	nonEmpty := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	return strings.Join(nonEmpty, theme.MutedStyle.Render(operationalTitleSeparator))
}

func joinOperationalTitleRegions(identity, context string) string {
	if identity == "" {
		return context
	}
	if context == "" {
		return identity
	}
	return identity + operationalTitleContextGap + context
}

func operationalPulseVariants(pulse WorkspacePulse) []string {
	variants := make([]string, 0, int(workspacePulseHidden)+1)
	for level := workspacePulseFull; level <= workspacePulseHidden; level++ {
		rendered := formatWorkspacePulseComponents(pulse, level)
		if len(variants) == 0 || rendered != variants[len(variants)-1] {
			variants = append(variants, rendered)
		}
	}
	return variants
}

func operationalPaceVariants(pace TodayPaceReadout) []string {
	rate := strconv.FormatFloat(pace.MedianPerDay, 'f', -1, 64)
	candidates := []string{
		fmt.Sprintf("%d open · median %s done/day (30d)", pace.OpenCount, rate),
		fmt.Sprintf("%d open · median %s done/day", pace.OpenCount, rate),
		fmt.Sprintf("%d open · %s done/day", pace.OpenCount, rate),
	}
	variants := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		variants = append(variants, theme.MutedStyle.Render(candidate))
	}
	return variants
}

func rightAlignOperationalPulse(base, pulse string, width int) (string, bool) {
	if pulse == "" || width <= 0 {
		return base, false
	}
	gap := width - lipgloss.Width(base) - lipgloss.Width(pulse)
	if gap < operationalTitlePulseGap {
		return base, false
	}
	return base + strings.Repeat(" ", gap) + pulse, true
}
