package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// ActiveFilterCount returns the number of active filters for the status bar pill.
func (m *OutlinerModel) ActiveFilterCount() int {
	return commonFilterFacetCount(&m.filter.CommonFilterState) +
		projectFilterFacetCount(&m.projectFilter)
}

// activeFilterParts returns the list of active filter description strings
// (state, priority, tags, due date, completion age, deferred, and pinned).
func (m *OutlinerModel) activeFilterParts() []string {
	return commonFilterParts(&m.filter.CommonFilterState)
}

func (m *OutlinerModel) isQuickEditRow(row outlinerRow) bool {
	task := m.quickEdit.GetTask()
	if !m.quickEdit.IsActive() || task == nil || row.task == nil || row.task.ID != task.ID {
		return false
	}
	if m.project != nil || m.quickEdit.project == nil {
		return true
	}
	return row.ProjectID == m.quickEdit.project.ID
}

func (m *OutlinerModel) quickEditSuggestionBudget() int {
	return min(quickEditSuggestionLimit, max(m.contentHeight()-2, 1))
}

func (m *OutlinerModel) quickEditExtraLineCount() int {
	if !m.quickEdit.IsActive() {
		return 0
	}
	// A normal task consumes one row. Inline editing replaces it with the
	// metadata row plus the input row, then adds the bounded suggestion pane.
	return 1 + m.quickEdit.SuggestionLineCount(m.quickEditSuggestionBudget())
}

func (m *OutlinerModel) renderOutlinerQuickEditRow(row outlinerRow, rowWidth int) string {
	project := m.resolveProjectForRow(row)
	m.quickEdit.SetContext(project, m.quickEditProjects())

	metadata := SelectedLine(m.quickEditMetadataLine(row, project, rowWidth), rowWidth)
	previewTask := m.quickEditDisplayTask()
	status := m.renderTaskStatusIcon(previewTask)
	prefixBudget := max(min(rowWidth/3, 12), 4)
	prefix := MutedStyle.Render(compactOutlinerTreePrefix(row.prefix, prefixBudget))
	// The quick-edit row is a transient input overlay, not a row grammar: it owns
	// its own selection paint, so the fold indicator takes no cursor background.
	fold := m.renderTaskFoldIndicator(row, nil)
	inputPrefix := RenderCursorPrefix() + prefix + fold + status + " "
	indentWidth := lipgloss.Width(inputPrefix)
	// textinput.View() renders Width()+1 cells: the horizontal scroll window plus
	// the cursor cell after it, so the window must leave that column free.
	m.quickEdit.SetInputWidth(max(rowWidth-indentWidth-1, 1))
	selectedPrefix := SelectedStyle.Render(ansi.Strip(inputPrefix))
	inputLine := fitStyledLineToWidth(selectedPrefix+m.quickEdit.renderSelectedInput(), rowWidth)

	lines := []string{metadata, inputLine}
	if suggestions := m.quickEdit.RenderSuggestions(strings.Repeat(" ", indentWidth), m.quickEditSuggestionBudget()); suggestions != "" {
		for _, line := range strings.Split(suggestions, "\n") {
			lines = append(lines, truncateToVisualWidth(line, rowWidth))
		}
	}
	return strings.Join(lines, "\n")
}

func (m *OutlinerModel) quickEditDisplayTask() *models.Task {
	if preview := m.quickEdit.GetPreview(); preview != nil && preview.preview.ID != "" {
		return &preview.preview
	}
	return m.quickEdit.GetTask()
}

func (m *OutlinerModel) quickEditMetadataLine(row outlinerRow, project *models.Project, width int) string {
	task := m.quickEditDisplayTask()
	if task == nil {
		return "EDIT TASK"
	}
	preview := m.quickEdit.GetPreview()
	effectiveProject := project
	if preview != nil && preview.targetProject != nil {
		effectiveProject = preview.targetProject
	}

	label := "EDIT TASK"
	route := quickEditRouteLabel(task, effectiveProject)
	workflow := quickEditWorkflowLabel(row, task, preview)
	errorText := m.quickEdit.GetPreviewError()

	mandatory := []string{label}
	if errorText != "" {
		mandatory = append(mandatory, "! "+errorText)
	}
	mandatory = append(mandatory, route, workflow)
	if lipgloss.Width(strings.Join(mandatory, " · ")) > width {
		mandatory = quickEditMandatoryMetadata(label, errorText, route, workflow, width)
	}

	segments := append([]string(nil), mandatory...)
	for _, optional := range quickEditOptionalMetadata(task, time.Now()) {
		if lipgloss.Width(strings.Join(append(segments, optional), " · ")) > width {
			break
		}
		segments = append(segments, optional)
	}
	line := strings.Join(segments, " · ")
	return truncateToVisualWidth(line, width)
}

func quickEditMandatoryMetadata(label, errorText, route, workflow string, width int) []string {
	separatorWidth := lipgloss.Width(" · ")
	if errorText == "" {
		routeBudget := max(width-lipgloss.Width(label)-lipgloss.Width(workflow)-2*separatorWidth, 1)
		return []string{label, compactQuickEditRoute(route, routeBudget), workflow}
	}

	error := "! " + errorText
	available := max(width-lipgloss.Width(label)-lipgloss.Width(workflow)-3*separatorWidth, 2)
	routeBudget := min(lipgloss.Width(route), max(available/2, 1))
	errorBudget := min(lipgloss.Width(error), max(available-routeBudget, 1))
	remaining := max(available-routeBudget-errorBudget, 0)
	if errorBudget < lipgloss.Width(error) {
		added := min(remaining, lipgloss.Width(error)-errorBudget)
		errorBudget += added
		remaining -= added
	}
	if routeBudget < lipgloss.Width(route) {
		routeBudget += min(remaining, lipgloss.Width(route)-routeBudget)
	}

	return []string{
		label,
		truncateToVisualWidth(error, errorBudget),
		compactQuickEditRoute(route, routeBudget),
		workflow,
	}
}

// compactQuickEditRoute keeps both ends of the mandatory route visible under
// width pressure: the project at the front and the immediate parent at the
// back. Deeper ancestry collapses between them before either endpoint shrinks.
func compactQuickEditRoute(route string, width int) string {
	if lipgloss.Width(route) <= width {
		return route
	}
	parts := strings.Split(route, " › ")
	if len(parts) < 2 || width < 5 {
		return truncateToVisualWidth(route, width)
	}

	project, parent := parts[0], parts[len(parts)-1]
	middle := " › "
	if len(parts) > 2 {
		middle = " › … › "
	}
	middleWidth := lipgloss.Width(middle)
	if width <= middleWidth+1 {
		return truncateToVisualWidth(route, width)
	}

	endpointBudget := width - middleWidth
	parentFloor := min(lipgloss.Width(parent), max(endpointBudget/2, 1))
	projectBudget := min(lipgloss.Width(project), max(endpointBudget-parentFloor, 1))
	parentBudget := max(endpointBudget-projectBudget, 1)
	return compactQuickEditProjectChip(project, projectBudget) + middle +
		truncateToVisualWidth(parent, parentBudget)
}

func compactQuickEditProjectChip(project string, width int) string {
	if lipgloss.Width(project) <= width || width < 3 || !strings.HasPrefix(project, "[") || !strings.HasSuffix(project, "]") {
		return truncateToVisualWidth(project, width)
	}
	return "[" + truncateToVisualWidth(strings.TrimSuffix(strings.TrimPrefix(project, "["), "]"), width-2) + "]"
}

func quickEditRouteLabel(task *models.Task, project *models.Project) string {
	projectName := projectLabel(project)
	if projectName == "" {
		projectName = "project"
	}
	path := []string{"[" + projectName + "]"}
	if task == nil || task.ParentID == nil || project == nil {
		return strings.Join(append(path, "root"), " › ")
	}
	parents := make([]string, 0, 3)
	seen := map[string]bool{}
	parentID := *task.ParentID
	for parentID != "" && !seen[parentID] {
		seen[parentID] = true
		parent := project.GetTaskByID(parentID)
		if parent == nil {
			break
		}
		parents = append(parents, parent.Title)
		if parent.ParentID == nil {
			break
		}
		parentID = *parent.ParentID
	}
	for left, right := 0, len(parents)-1; left < right; left, right = left+1, right-1 {
		parents[left], parents[right] = parents[right], parents[left]
	}
	if len(parents) == 0 {
		parents = append(parents, "root")
	}
	return strings.Join(append(path, parents...), " › ")
}

func quickEditWorkflowLabel(row outlinerRow, task *models.Task, preview *quickEditResult) string {
	if preview != nil && preview.destinationSet {
		return string(preview.destination)
	}
	if row.IsTodaySection {
		return "today"
	}
	if row.IsNextSection {
		return "next"
	}
	return task.State().String()
}

// quickEditOptionalMetadata is ordered from strongest to weakest. Appending
// until the first width miss yields the required reverse degradation ladder:
// tags, pin, deferral, due/check-back, then priority disappear as width shrinks.
func quickEditOptionalMetadata(task *models.Task, now time.Time) []string {
	if task == nil {
		return nil
	}
	parts := make([]string, 0, 5)
	if task.Priority != models.PriorityUnset {
		parts = append(parts, fmt.Sprintf("!%d", task.Priority))
	}
	if task.DueDate != nil {
		prefix := "@"
		if task.IsInState(models.StateWaiting) {
			prefix = "check "
		}
		parts = append(parts, prefix+task.DueDate.Format("Jan 2"))
	}
	if task.DeferredUntil != nil {
		parts = append(parts, "defer "+formatDeferDate(*task.DeferredUntil, now))
	}
	if task.Pinned {
		parts = append(parts, "pin")
	}
	if len(task.Tags) > 0 {
		tags := make([]string, 0, len(task.Tags))
		for _, tag := range task.Tags {
			tags = append(tags, "#"+tag)
		}
		parts = append(parts, strings.Join(tags, " "))
	}
	return parts
}
