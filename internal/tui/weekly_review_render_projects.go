package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// renderProjectLine renders a single project line with metric badges.
func (m *WeeklyReviewModel) renderProjectLine(wp WeeklyReviewProject, section WeeklyReviewSection, isSelected bool) string {
	cursor := "  "
	if isSelected {
		cursor = "  "
	}

	var badges []string
	switch section {
	case WeeklySectionStaleProjects:
		badges = append(badges, MutedStyle.Render(fmt.Sprintf("%dd idle", wp.NoActivityDays)))
	case WeeklySectionInboxBacklog:
		badges = append(badges, DueTodayStyle.Render(fmt.Sprintf("%d inbox", wp.InboxCount)))
	case WeeklySectionBlockedWork:
		badges = append(badges, BlockedStyle.Render(fmt.Sprintf("%d blocked", wp.BlockedCount)))
	case WeeklySectionOvercommitted:
		badges = append(badges, OverdueStyle.Render(fmt.Sprintf("%d in-progress", wp.InProgressCount)))
	default:
	}

	// Add secondary context badges
	if section != WeeklySectionStaleProjects && wp.NoActivityDays >= 7 {
		badges = append(badges, MutedStyle.Render(fmt.Sprintf("%dd idle", wp.NoActivityDays)))
	}
	if section != WeeklySectionInboxBacklog && wp.InboxCount > 0 {
		badges = append(badges, MutedStyle.Render(fmt.Sprintf("%d inbox", wp.InboxCount)))
	}
	if wp.OverdueCount > 0 {
		badges = append(badges, OverdueStyle.Render(fmt.Sprintf("%d overdue", wp.OverdueCount)))
	}

	badgeStr := ""
	if len(badges) > 0 {
		badgeStr = "  " + strings.Join(badges, "  ")
	}

	maxNameWidth := 25
	if m.width > 0 {
		maxNameWidth = min(maxNameWidth, max(m.width-lipgloss.Width(cursor)-lipgloss.Width(badgeStr), 1))
	}
	name := truncateToVisualWidth(wp.Project.Name, maxNameWidth)
	styledName := projectColorStyle(wp.Project).Render(name)
	if isSelected {
		// The selected-row foreground remains the primary identity cue while the
		// cursor is on this project; the durable project color returns when the
		// row is no longer selected.
		styledName = name
	}

	line := fmt.Sprintf("%s%s%s", cursor, styledName, badgeStr)
	if m.width > 0 && lipgloss.Width(line) > m.width {
		excess := lipgloss.Width(line) - m.width
		name = truncateToVisualWidth(wp.Project.Name, max(maxNameWidth-excess, 1))
		styledName = projectColorStyle(wp.Project).Render(name)
		if isSelected {
			styledName = name
		}
		line = fmt.Sprintf("%s%s%s", cursor, styledName, badgeStr)
	}
	if m.width > 0 {
		// At very narrow widths the metric badges alone can exceed the available
		// budget even after the project name reaches its one-column minimum.
		line = fitStyledLineToWidth(line, m.width)
	}

	if isSelected {
		selStyle := SelectedStyle
		if m.cursorFlash {
			selStyle = selStyle.Background(ColorFlashBg)
		}
		if m.width > 0 {
			line = selStyle.Width(m.width).Render(line)
		} else {
			line = selStyle.Render(line)
		}
	}

	return line
}
