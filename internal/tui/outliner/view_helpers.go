package outliner

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// TitleParams holds optional filter/sort info to display inline on the title line.
type TitleParams struct {
	FilterParts   []string
	SortField     query.SortField
	SortAscending bool
	// Width is the room the title line owns, already net of any right-side
	// reservation the caller makes (such as a counter). 0 = no truncation.
	Width int
}

// buildTitleFilterSuffix builds the filter+sort suffix for the title line.
// It progressively degrades if the suffix would be too wide:
// 1. Full filter badge + sort
// 2. Compact "[N filters]" badge + sort
// 3. Just sort
// 4. Nothing
func buildTitleFilterSuffix(params *TitleParams, baseWidth int) string {
	if len(params.FilterParts) == 0 && params.SortField == query.SortByDefault {
		return ""
	}

	maxWidth := params.Width
	if maxWidth <= 0 {
		// No width constraint — render full
		maxWidth = 0
	}

	// Build sort badge
	sortBadge := ""
	if params.SortField != query.SortByDefault {
		direction := "↑"
		if !params.SortAscending {
			direction = "↓"
		}
		sortBadge = theme.MutedStyle.Render(fmt.Sprintf("Sort: %s %s", params.SortField, direction))
	}

	// Build full filter badge
	fullFilterBadge := query.RenderFilterBadge(params.FilterParts)

	// Build compact filter badge
	compactFilterBadge := ""
	if len(params.FilterParts) > 0 {
		compactFilterBadge = pulse.OperationalTitleBadge(theme.FilterBadgeStyle, fmt.Sprintf("[%d filters]", len(params.FilterParts)))
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

// projectFocusContext returns the full single-project Focus path. Width fitting
// belongs to the operational-title renderer so pulse components can yield
// before this active interaction context is shortened.
func (m *OutlinerModel) ProjectFocusContext() string {
	if len(m.Focus.Stack) == 0 || m.Project == nil {
		return ""
	}
	task := m.Project.GetTaskByID(m.Focus.Stack[len(m.Focus.Stack)-1].TaskID)
	if task == nil {
		return ""
	}
	return taskFocusContext(m.Project, task)
}

// taskFocusContext renders the canonical project-qualified hierarchy for the
// operational row. Resolving ParentID through the owning project keeps the
// breadcrumb complete even when a detached projection clone lacks Parent.
func taskFocusContext(project *models.Project, task *models.Task) string {
	if project == nil || task == nil {
		return ""
	}
	projectName := strings.TrimSpace(project.Name)
	if projectName == "" {
		projectName = strings.TrimSpace(project.Slug)
	}
	if projectName == "" {
		projectName = project.ID
	}

	path := []string{task.Title}
	seen := map[string]bool{task.ID: true}
	for parent := focusParent(project, task); parent != nil && !seen[parent.ID]; parent = focusParent(project, parent) {
		seen[parent.ID] = true
		path = append(path, parent.Title)
	}
	for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
		path[left], path[right] = path[right], path[left]
	}
	return "Focus: [" + projectName + "] › " + strings.Join(path, " › ")
}

func focusParent(project *models.Project, task *models.Task) *models.Task {
	if task == nil {
		return nil
	}
	if task.Parent != nil {
		return task.Parent
	}
	if task.ParentID != nil {
		return project.GetTaskByID(*task.ParentID)
	}
	return nil
}
