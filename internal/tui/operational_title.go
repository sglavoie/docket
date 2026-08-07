package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	operationalTitleSeparator       = " · "
	operationalTitleContextGap      = "  "
	operationalTitlePulseGap        = 2
	operationalTitleMinContextWidth = 14
)

// operationalTitleSpec describes the semantic regions of a one-row
// operational header. Segment strings may already contain ANSI styling; all
// fitting is based on terminal-cell width rather than bytes or runes.
type operationalTitleSpec struct {
	Surface string
	Scope   string
	Mode    string
	Context []string
	Pulse   *WorkspacePulse
}

// renderOperationalTitle composes identity, interaction context, and passive
// workspace state onto exactly one non-wrapping row. Passive pulse components
// are removed in the product-wide precedence order (Inbox, Overdue, Today)
// before required interaction context is shortened.
func renderOperationalTitle(spec operationalTitleSpec, width int) string {
	if width <= 0 {
		return ""
	}

	identity := renderOperationalIdentity(spec, width)
	context := joinOperationalTitleSegments(spec.Context)
	base := joinOperationalTitleRegions(identity, context)

	if spec.Pulse != nil {
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
	return identity + operationalTitleContextGap + truncateToVisualWidth(context, contextBudget)
}

func renderOperationalIdentity(spec operationalTitleSpec, width int) string {
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
		parts[1] = truncateToVisualWidth(spec.Scope, scopeBudget)
		identity = joinOperationalTitleSegments(parts)
		if lipgloss.Width(identity) <= width {
			return identity
		}
	}

	return truncateToVisualWidth(identity, width)
}

// operationalTitleBadge renders a background-styled title chip without the
// theme's padding cells. The row's separator and context gap own every
// inter-region space, so a padded edge widens one of those gaps by a cell.
func operationalTitleBadge(style lipgloss.Style, text string) string {
	return style.Padding(0, 0).Render(text)
}

// operationalTitleModeSegment renders the mode identity segment.
func operationalTitleModeSegment(mode OutlinerMode) string {
	return operationalTitleBadge(GetOutlinerModeStyle(mode.ID), mode.Label)
}

func joinOperationalTitleSegments(parts []string) string {
	nonEmpty := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	return strings.Join(nonEmpty, MutedStyle.Render(operationalTitleSeparator))
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
	candidates := []string{
		formatWorkspacePulseComponents(pulse, true, true),
		formatWorkspacePulseComponents(pulse, false, true),
		formatWorkspacePulseComponents(pulse, false, false),
		"",
	}
	variants := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		rendered := ""
		if candidate != "" {
			rendered = MutedStyle.Render(candidate)
		}
		if len(variants) == 0 || rendered != variants[len(variants)-1] {
			variants = append(variants, rendered)
		}
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
