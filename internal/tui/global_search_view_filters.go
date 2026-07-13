package tui

import (
	"fmt"
	"strings"
)

// renderFilterBadges renders active filters as styled badges.
func (m *GlobalSearchModel) renderFilterBadges() string {
	var badges []string

	if m.parsedFilters == nil {
		if m.hideCompleted {
			return "Filters: " + FilterBadgeStyle.Render("hide:completed")
		}
		return ""
	}

	if len(m.parsedFilters.Priorities) > 0 {
		var priorityValues []string
		for _, priority := range m.parsedFilters.Priorities {
			priorityValues = append(priorityValues, fmt.Sprintf("%d", priority))
		}
		badges = append(badges, fmt.Sprintf("!%s", strings.Join(priorityValues, ",")))
	}

	for _, tag := range m.parsedFilters.Tags {
		badges = append(badges, "#"+tag)
	}

	for _, status := range m.parsedFilters.Statuses {
		badges = append(badges, "$"+string(status))
	}

	for _, taskType := range m.parsedFilters.Types {
		badges = append(badges, "^"+string(taskType))
	}

	if m.parsedFilters.DueDateFilter != "" {
		badges = append(badges, "@"+m.parsedFilters.DueDateFilter)
	}

	if m.parsedFilters.Section != "" {
		badges = append(badges, "/"+m.parsedFilters.Section)
	}

	if m.parsedFilters.ActiveOnly {
		badges = append(badges, "*active")
	}

	for _, filter := range m.parsedFilters.HasPresence {
		badges = append(badges, "has:"+filter)
	}

	for _, filter := range m.parsedFilters.NoPresence {
		badges = append(badges, "no:"+filter)
	}

	for _, filter := range m.parsedFilters.IsState {
		badges = append(badges, "is:"+filter)
	}

	if m.parsedFilters.ProjectSlug != "" {
		badges = append(badges, "p:"+m.parsedFilters.ProjectSlug)
	}

	if m.hideCompleted {
		badges = append(badges, "hide:completed")
	}

	if len(badges) == 0 {
		return ""
	}

	styled := make([]string, 0, len(badges))
	for _, badge := range badges {
		styled = append(styled, FilterBadgeStyle.Render(badge))
	}

	return "Filters: " + strings.Join(styled, " ")
}

// renderSearchHelp renders the search syntax help panel.
func (m *GlobalSearchModel) renderSearchHelp() string {
	prefix := MatchHighlightStyle
	muted := MutedStyle

	var b strings.Builder

	b.WriteString(prefix.Render("has:X"))
	b.WriteString(muted.Render("  Task has this property"))
	b.WriteString("\n")
	hasValues := []string{
		"description", "comment", "checklist", "attachment", "issue",
		"due", "dependency", "relation", "recurrence",
		"url", "parent", "children", "tag",
	}
	b.WriteString(muted.Render("  " + wrapValues(hasValues, m.previewWidth-4)))
	b.WriteString("\n\n")

	b.WriteString(prefix.Render("no:X"))
	b.WriteString(muted.Render("  Negation of has:"))
	b.WriteString("\n")
	b.WriteString(muted.Render("  Same values as has: (e.g. no:due, no:tag)"))
	b.WriteString("\n\n")

	b.WriteString(prefix.Render("is:X"))
	b.WriteString(muted.Render("  Task matches this state"))
	b.WriteString("\n")
	isValues := []string{
		"pinned", "active", "dimmed", "blocked",
		"overdue", "snoozed", "recurring", "leaf", "root",
	}
	b.WriteString(muted.Render("  " + wrapValues(isValues, m.previewWidth-4)))
	b.WriteString("\n\n")

	b.WriteString(prefix.Render("p:slug"))
	b.WriteString(muted.Render("  Filter by project (case-insensitive prefix)"))
	b.WriteString("\n")

	return b.String()
}

// wrapValues joins values with spaces, wrapping to a new line when exceeding maxWidth.
func wrapValues(values []string, maxWidth int) string {
	var lines []string
	line := ""
	for _, value := range values {
		if line == "" {
			line = value
		} else if len(line)+1+len(value) > maxWidth {
			lines = append(lines, line)
			line = value
		} else {
			line += " " + value
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n  ")
}
