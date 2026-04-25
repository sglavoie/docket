package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

func (m *AppModel) renderStatusBar() string {
	// Top border separator for visual distinction; tinted with current view accent.
	separator := renderHorizontalDivider(m.width, WithDividerColor(m.currentAccentColor()))

	// Helper to render left-aligned message with spacer.
	renderLeftAlignedMsg := func(msg string) string {
		gap := max(m.width-lipgloss.Width(msg), 0)
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
		msg := ErrorStatusStyle.Render("✗ " + m.err.Error())
		return separator + "\n" + renderLeftAlignedMsg(msg)
	}

	// Show temporary status message if present.
	if m.statusMessage != "" {
		var msg string
		if m.statusIsInfo {
			msg = InfoStatusStyle.Render("· " + m.statusMessage)
		} else {
			msg = SuccessStyle.Render("✓ " + m.statusMessage)
		}
		return separator + "\n" + renderLeftAlignedMsg(msg)
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
		help = "Press : for commands, ? for help"
	}

	accent := m.currentAccentColor()
	accentBarStyle := lipgloss.NewStyle().
		Foreground(accent).
		Background(ColorBackground).
		Padding(0, 1)

	var left string
	if m.currentView() == ViewDashboard && !m.config.GetTutorDismissed() {
		left = accentBarStyle.Render("Dashboard") + ": Run " + InfoStatusStyle.Render("tutor") + " command to start. Dismiss with " + InfoStatusStyle.Render("|")
	} else {
		left = accentBarStyle.Render(status)
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

	right := HelpStyle.Render(help)
	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	spacer := ThemeSpacer(gap)

	return separator + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
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

	crumb := strings.Join(parts, sep)
	// Truncate total breadcrumb if it would crowd out the help text.
	maxLen := max(m.width/2, 20)
	if len(crumb) > maxLen {
		crumb = crumb[:maxLen-1] + "…"
	}
	return crumb
}

// breadcrumbLabel returns a short human-readable label for a single view type.
func (m *AppModel) breadcrumbLabel(vt ViewType, maxProjectLen int) string {
	// ViewDashboard has no StatusBarLabel; use a fixed string.
	if vt == ViewDashboard {
		return "Dashboard"
	}

	// Use the registered view's StatusBarLabel when available.
	if view := m.views.Get(vt); view != nil {
		if labeler, ok := view.(StatusBarLabeler); ok {
			raw := labeler.StatusBarLabel()
			// Strip "Project: " prefix from project tree label for compactness.
			label := strings.TrimPrefix(raw, "Project: ")
			// Also strip cycle-position suffix (starts with " [") for compactness.
			if idx := strings.Index(label, " ["); idx > 0 {
				label = label[:idx]
			}
			if len(label) > maxProjectLen {
				label = label[:maxProjectLen-1] + "…"
			}
			return label
		}
	}

	// Fallback labels for views that don't implement StatusBarLabeler.
	switch vt {
	case ViewProjectTree:
		if m.currentProject != nil {
			name := m.currentProject.Name
			if len(name) > maxProjectLen {
				name = name[:maxProjectLen-1] + "…"
			}
			return name
		}
		return "Project"
	case ViewFilter:
		return "Filter"
	case ViewDailyReview:
		return "Daily Review"
	case ViewWeeklyReview:
		return "Weekly Review"
	default:
	}
	return ""
}
