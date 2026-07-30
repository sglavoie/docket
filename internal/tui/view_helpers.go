package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// TitleParams holds optional filter/sort info to display inline on the title line.
type TitleParams struct {
	FilterParts   []string
	SortField     SortField
	SortAscending bool
	// Width is the room the title line owns, already net of any right-side
	// reservation the caller makes (such as a counter). 0 = no truncation.
	Width int
}

// renderProjectTitle renders the project title line with context, notes indicator,
// filter badges, and archived status. Shared across all project-level views.
// When params is non-nil, filter/sort info is appended inline, truncated to fit
// within params.Width.
func renderProjectTitle(project *models.Project, runtime *config.RuntimeState, params *TitleParams) string {
	var b strings.Builder
	title := fmt.Sprintf("%s - Tasks", project.Name)
	b.WriteString(TitleStyle.Render(title))
	if project.Context != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" [%s]", project.Context)))
	}
	if storage.NotesExist(project) {
		b.WriteString(" ✎ Notes")
	}
	if project.Archived {
		b.WriteString(" ")
		b.WriteString(ArchivedBadgeStyle.Render("ARCHIVED"))
		if project.ArchivedAt != nil {
			b.WriteString(" ")
			b.WriteString(MutedStyle.Render(fmt.Sprintf("since %s", project.ArchivedAt.Format(util.DateFormat))))
		}
	}

	// Append inline filter/sort info if provided
	if params != nil {
		filterSuffix := buildTitleFilterSuffix(params, lipgloss.Width(b.String()))
		if filterSuffix != "" {
			b.WriteString(filterSuffix)
		}
	}

	return b.String()
}

// buildTitleFilterSuffix builds the filter+sort suffix for the title line.
// It progressively degrades if the suffix would be too wide:
// 1. Full filter badge + sort
// 2. Compact "[N filters]" badge + sort
// 3. Just sort
// 4. Nothing
func buildTitleFilterSuffix(params *TitleParams, baseWidth int) string {
	if len(params.FilterParts) == 0 && params.SortField == SortByDefault {
		return ""
	}

	maxWidth := params.Width
	if maxWidth <= 0 {
		// No width constraint — render full
		maxWidth = 0
	}

	// Build sort badge
	sortBadge := ""
	if params.SortField != SortByDefault {
		direction := "↑"
		if !params.SortAscending {
			direction = "↓"
		}
		sortBadge = MutedStyle.Render(fmt.Sprintf("Sort: %s %s", params.SortField, direction))
	}

	// Build full filter badge
	fullFilterBadge := renderFilterBadge(params.FilterParts)

	// Build compact filter badge
	compactFilterBadge := ""
	if len(params.FilterParts) > 0 {
		compactFilterBadge = FilterBadgeStyle.Render(fmt.Sprintf("[%d filters]", len(params.FilterParts)))
	}

	// Try progressively degraded versions
	candidates := []string{
		joinSuffix(fullFilterBadge, sortBadge),
		joinSuffix(compactFilterBadge, sortBadge),
		joinSuffix("", sortBadge),
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		suffix := "  " + candidate
		if maxWidth == 0 || baseWidth+lipgloss.Width(suffix) <= maxWidth {
			return suffix
		}
	}

	return ""
}

// joinSuffix joins filter and sort parts with spacing.
func joinSuffix(filter, sort string) string {
	if filter != "" && sort != "" {
		return filter + "  " + sort
	}
	if filter != "" {
		return filter
	}
	return sort
}
