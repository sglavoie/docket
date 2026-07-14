package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/icons"
)

func (m *AppModel) renderStatusBar() string {
	// Top border separator for visual distinction; tinted with current view accent.
	separator := renderHorizontalDivider(m.width, WithDividerColor(m.currentAccentColor()))

	// Helper to render left-aligned message with spacer.
	renderLeftAlignedMsg := func(msg string) string {
		width := m.width
		if width <= 0 {
			width = 80
		}
		msg = truncateToVisualWidth(msg, width)
		gap := max(width-lipgloss.Width(msg), 0)
		spacer := ThemeSpacer(gap)
		return lipgloss.JoinHorizontal(lipgloss.Top, msg, spacer)
	}

	// Show loading spinner if active.
	if m.isLoading {
		spinnerView := m.spinner.View()
		msg := LoadingStyle.Render(spinnerView + " " + m.loadingMessage)
		return separator + "\n" + renderLeftAlignedMsg(msg)
	}

	// Show error message if present.
	if m.err != nil {
		msg := ErrorStatusStyle.Render(icons.Get("error") + " " + m.err.Error())
		return separator + "\n" + renderLeftAlignedMsg(msg)
	}

	// Today-cleared banner takes precedence over the regular status message
	// for the duration of its TTL — Peak-End Rule punctuation for daily completion.
	if m.todayClearedBanner != "" {
		msg := TodayClearedBannerStyle.Render(m.todayClearedBanner)
		return separator + "\n" + renderLeftAlignedMsg(msg)
	}

	// Show temporary status message if present.
	if m.statusMessage != "" {
		var msg string
		if m.statusIsInfo {
			msg = InfoStatusStyle.Render("· " + m.statusMessage)
		} else {
			msg = SuccessStyle.Render(icons.Get("success") + " " + m.statusMessage)
		}
		return separator + "\n" + renderLeftAlignedMsg(msg)
	}

	// Show the current view's contextual footer if it provides one (e.g. the
	// capture views). This sits above the pinned-task bar and generic breadcrumb
	// but below the transient loading/error/status overlays handled above, so a
	// temporary status message still replaces capture controls and the controls
	// return once it clears. renderContextualFooter emits its own separator, so
	// return it directly rather than prefixing the local one.
	if view := m.views.Get(m.currentView()); view != nil {
		if provider, ok := view.(ContextualFooterProvider); ok {
			if spec, ok := provider.ContextualFooter(); ok {
				return renderContextualFooter(spec, m.width, m.currentAccentColor())
			}
		}
	}

	// Show pinned tasks bar if any tasks are pinned.
	if pinnedBar := m.renderPinnedTasksBar(m.width); pinnedBar != "" {
		return separator + "\n" + pinnedBar
	}

	status := m.buildBreadcrumb()

	// Check if view supports help shortcut hint; only show if view implements HelpProvider.
	currentView := m.currentView()
	showHelp := false
	if view := m.views.Get(currentView); view != nil {
		if _, ok := view.(HelpProvider); ok {
			showHelp = true
		}
	}

	help := ""
	if showHelp && m.isNotesFullscreen() {
		showHelp = false
	}
	if showHelp {
		if hint := pipelineNextHint(currentView); hint != "" {
			help = hint
		} else {
			help = ";; hub · : cmd · ? help"
		}
		if !m.openLoops.HasTasks {
			if onboarding := pipelineNextHintOnboarding(currentView); onboarding != "" {
				help = onboarding
			}
		}
	}

	accent := m.currentAccentColor()
	accentBarStyle := lipgloss.NewStyle().
		Foreground(accent).
		Background(ColorBackground).
		Padding(0, 1)

	left := accentBarStyle.Render(status)
	if onboarding := onboardingStatus(m.runtime.Settings().GetOnboardingProgress()); onboarding != "" {
		left += ": " + InfoStatusStyle.Render(onboarding)
	}

	// Add filter pill if current view has active filters.
	if view := m.views.Get(currentView); view != nil {
		if fp, ok := view.(ActiveFilterProvider); ok {
			if count := fp.ActiveFilterCount(); count > 0 {
				filterLabel := "filters"
				if count == 1 {
					filterLabel = "filter"
				}
				left += " " + FilterBadgeStyle.Render(fmt.Sprintf("%d %s", count, filterLabel))
			}
		}
	}

	// Prepend mode badge when the current view is in a named mode.
	if view := m.views.Get(currentView); view != nil {
		if mi, ok := view.(ModeIndicator); ok {
			if label, c := mi.ActiveMode(); label != "" {
				badge := lipgloss.NewStyle().
					Bold(true).
					Background(c).
					Foreground(ColorBackground).
					Padding(0, 1).
					Render(label)
				left = badge + " " + left
			}
		}
	}

	right := HelpStyle.Render(help)
	left, right = fitStatusBarSegments(left, right, m.width)
	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right), 0)
	spacer := ThemeSpacer(gap)

	return separator + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
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
		left = truncateToVisualWidth(left, availableLeft)
		leftWidth = lipgloss.Width(left)
		if leftWidth+rightWidth+minGap > width {
			right = truncateToVisualWidth(right, max(width-leftWidth-minGap, 1))
		}
	}

	return left, right
}

// buildBreadcrumb constructs a compact "Dashboard > project > view" label from the view stack.
// Each segment uses the view's StatusBarLabel() when available, with fallbacks for common views.
// Project names are truncated to keep the breadcrumb short.
func (m *AppModel) buildBreadcrumb() string {
	const maxProjectLen = 16
	const sep = " > "

	// Build a deduplicated view stack, keeping the LAST occurrence of each
	// view type (e.g. for stacked ViewProjectTree the deepest project wins).
	lastIdx := make(map[ViewType]int)
	for i, vt := range m.viewStack {
		lastIdx[vt] = i
	}

	var parts []string
	for i, vt := range m.viewStack {
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
		return abbreviated + " " + MutedStyle.Render(fmt.Sprintf("[%d]", depth))
	}

	crumb := strings.Join(parts, sep)
	// Truncate total breadcrumb if it would crowd out the help text.
	maxLen := max(m.width/2, 20)
	if lipgloss.Width(crumb) > maxLen {
		crumb = truncateToVisualWidth(crumb, maxLen)
	}

	// Esc-preview: show what pressing Esc will return to.
	if len(m.viewStack) >= 2 && m.currentView() != ViewDashboard {
		prevVT := m.viewStack[len(m.viewStack)-2]
		prevLabel := m.breadcrumbLabel(prevVT, maxProjectLen)
		if prevLabel != "" {
			hint := MutedStyle.Render(" ← " + prevLabel)
			if lipgloss.Width(crumb)+lipgloss.Width(hint) <= maxLen {
				crumb += hint
			}
		}
	}

	return crumb
}

// breadcrumbLabel returns a short human-readable label for a single view type.
func (m *AppModel) breadcrumbLabel(vt ViewType, maxProjectLen int) string {
	// ViewDashboard has no StatusBarLabel; use a fixed string.
	if vt == ViewDashboard {
		return "Dashboard"
	}

	var label string

	// Use the registered view's StatusBarLabel when available.
	if view := m.views.Get(vt); view != nil {
		if labeler, ok := view.(StatusBarLabeler); ok {
			raw := labeler.StatusBarLabel()
			// Strip "Project: " prefix from project tree label for compactness.
			label = strings.TrimPrefix(raw, "Project: ")
			// Also strip cycle-position suffix (starts with " [") for compactness.
			if idx := strings.Index(label, " ["); idx > 0 {
				label = label[:idx]
			}
			if lipgloss.Width(label) > maxProjectLen {
				label = truncateToVisualWidth(label, maxProjectLen)
			}
		}
	}

	// Fallback labels for views that don't implement StatusBarLabeler.
	if label == "" {
		switch vt {
		case ViewProjectTree:
			if m.currentProject != nil {
				name := m.currentProject.Name
				if lipgloss.Width(name) > maxProjectLen {
					name = truncateToVisualWidth(name, maxProjectLen)
				}
				label = name
			} else {
				label = "Project"
			}
		case ViewFilter:
			label = "Filter"
		case ViewDailyReview:
			label = "Daily Review"
		case ViewWeeklyReview:
			label = "Weekly Review"
		default:
		}
	}

	if label == "" {
		return ""
	}

	// Prepend pipeline stage when the view belongs to a named stage.
	if stage := pipelineStage(vt); stage != "" {
		return stage + ": " + label
	}
	return label
}
