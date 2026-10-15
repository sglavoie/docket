package outliner

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// ActiveFilterCount returns the number of active filters for the status bar pill.
func (m *OutlinerModel) ActiveFilterCount() int {
	return query.CommonFilterFacetCount(&m.Filter.CommonFilterState) +
		query.ProjectFilterFacetCount(&m.ProjectFilter)
}

// activeFilterParts returns the list of active filter description strings
// (state, priority, tags, due date, completion age, deferred, and pinned).
func (m *OutlinerModel) ActiveFilterParts() []string {
	return query.CommonFilterParts(&m.Filter.CommonFilterState)
}

func (m *OutlinerModel) isQuickEditRow(row OutlinerRow) bool {
	task := m.QuickEdit.GetTask()
	if !m.QuickEdit.IsActive() || task == nil || row.Task == nil || row.Task.ID != task.ID {
		return false
	}
	if m.Project != nil || m.QuickEdit.GetProject() == nil {
		return true
	}
	return row.ProjectID == m.QuickEdit.GetProject().ID
}

func (m *OutlinerModel) QuickEditSuggestionBudget() int {
	return min(grammar.QuickEditSuggestionLimit, max(m.ContentHeight()-2, 1))
}

func (m *OutlinerModel) quickEditExtraLineCount() int {
	if !m.QuickEdit.IsActive() {
		return 0
	}
	// A normal task consumes one row. Inline editing replaces it with the
	// metadata row plus the input row, then adds the bounded suggestion pane.
	return 1 + m.QuickEdit.SuggestionLineCount(m.QuickEditSuggestionBudget())
}

// quickEditSuggestionIndentShare bounds how much of the row the completion pane
// gives back to the caret's own column. A deep subtree can push the title well
// into the row, and a dropdown that followed it there would have a handful of
// columns left for the values it exists to show. Alignment is worth having and
// worth losing: past this share the pane leaves the caret's column and keeps its
// width.
const quickEditSuggestionIndentShare = 3

// RenderOutlinerQuickEditRow draws the row being edited in place. colLayout is
// the projection's own geometry rather than nothing: the caret has to land in
// the column the committed title will be drawn in, and that column includes the
// badge padding the rows around it are wearing.
func (m *OutlinerModel) RenderOutlinerQuickEditRow(row OutlinerRow, rowWidth int, colLayout *chrome.ColumnLayout) string {
	project := m.ResolveProjectForRow(row)
	m.QuickEdit.SetContext(project, m.quickEditProjects())

	metadata := chrome.SelectedLine(m.QuickEditMetadataLine(row, project, rowWidth), rowWidth)
	// The draft, not the stored task: the state badge, the priority badge, and the
	// pin the editor draws are the ones the buffer currently spells, so a `$doing`
	// or a `*pin` typed into the line is answered in the same column the committed
	// row will answer in. The quick-edit row is a transient input overlay rather
	// than a row grammar, so it owns its own selection paint and passes the zero
	// cursorPaint, which leaves the gutter unbackgrounded.
	previewRow := row
	if previewTask := m.quickEditDisplayTask(); previewTask != nil {
		previewRow.Task = previewTask
	}
	inputPrefix := m.buildTaskRowSegments(previewRow, rowWidth, colLayout, cursorPaint{}).titlePrefix()
	indentWidth := lipgloss.Width(inputPrefix)
	// textinput.View() renders Width()+1 cells: the horizontal scroll window plus
	// the cursor cell after it, so the window must leave that column free. The
	// prefix is the row's own, so the columns the input is given are exactly the
	// columns the title had — which is what keeps the caret on screen as the row
	// budget changes.
	m.QuickEdit.SetInputWidth(max(rowWidth-indentWidth-1, 1))
	selectedPrefix := theme.SelectedStyle.Render(ansi.Strip(inputPrefix))
	inputLine := textfit.FitStyledLineToWidth(selectedPrefix+m.QuickEdit.RenderSelectedInput(), rowWidth)

	lines := []string{metadata, inputLine}
	suggestionIndent := min(indentWidth, max(rowWidth/quickEditSuggestionIndentShare, 0))
	if suggestions := m.QuickEdit.RenderSuggestions(strings.Repeat(" ", suggestionIndent), m.QuickEditSuggestionBudget()); suggestions != "" {
		for _, line := range strings.Split(suggestions, "\n") {
			lines = append(lines, textfit.TruncateToVisualWidth(line, rowWidth))
		}
	}
	return strings.Join(lines, "\n")
}

func (m *OutlinerModel) quickEditDisplayTask() *models.Task {
	if preview := m.QuickEdit.GetPreview(); preview != nil && preview.Preview.ID != "" {
		return &preview.Preview
	}
	return m.QuickEdit.GetTask()
}

func (m *OutlinerModel) QuickEditMetadataLine(row OutlinerRow, project *models.Project, width int) string {
	task := m.quickEditDisplayTask()
	if task == nil {
		return "EDIT TASK"
	}
	preview := m.QuickEdit.GetPreview()
	effectiveProject := project
	if preview != nil && preview.TargetProject != nil {
		effectiveProject = preview.TargetProject
	}

	label := "EDIT TASK"
	route := quickEditRouteLabel(task, effectiveProject)
	workflow := quickEditWorkflowLabel(row, task, preview)
	errorText := m.QuickEdit.GetPreviewError()

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
	return textfit.TruncateToVisualWidth(line, width)
}

// quickEditRouteFloor is the columns the route keeps while a validation message
// is on screen. compactQuickEditRoute collapses deep ancestry between its two
// endpoints above this, so the floor is what leaves a recognizable project chip
// and parent rather than a bare ellipsis. A task with no parent has a route of
// the project chip alone, and the floor keeps that chip's brackets.
const quickEditRouteFloor = 8

func quickEditMandatoryMetadata(label, errorText, route, workflow string, width int) []string {
	separatorWidth := lipgloss.Width(" · ")
	if errorText == "" {
		routeBudget := max(width-lipgloss.Width(label)-lipgloss.Width(workflow)-2*separatorWidth, 1)
		return []string{label, compactQuickEditRoute(route, routeBudget), workflow}
	}

	// A refusal is the one thing on this line the user cannot read anywhere else:
	// the route and the workflow restate what the row already shows, while the
	// message names why the buffer will not commit. So it is served whole first
	// and the route shrinks to the floor beneath it, rather than the two splitting
	// the width evenly and both arriving as stubs.
	error := "! " + errorText
	available := max(width-lipgloss.Width(label)-lipgloss.Width(workflow)-3*separatorWidth, 2)
	routeFloor := min(lipgloss.Width(route), quickEditRouteFloor)
	errorBudget := min(lipgloss.Width(error), max(available-routeFloor, 1))
	routeBudget := max(available-errorBudget, 1)

	return []string{
		label,
		textfit.TruncateToVisualWidth(error, errorBudget),
		compactQuickEditRoute(route, routeBudget),
		workflow,
	}
}

// compactQuickEditRoute keeps both ends of the mandatory route visible under
// width pressure: the project at the front and the immediate parent at the
// back. Deeper ancestry collapses between them before either endpoint shrinks.
// A route without a parent is the project chip alone, which shrinks inside its
// brackets rather than losing them.
func compactQuickEditRoute(route string, width int) string {
	if lipgloss.Width(route) <= width {
		return route
	}
	parts := strings.Split(route, " › ")
	if len(parts) == 1 {
		return compactQuickEditProjectChip(route, width)
	}
	if width < 5 {
		return textfit.TruncateToVisualWidth(route, width)
	}

	project, parent := parts[0], parts[len(parts)-1]
	middle := " › "
	if len(parts) > 2 {
		middle = " › … › "
	}
	middleWidth := lipgloss.Width(middle)
	if width <= middleWidth+1 {
		return textfit.TruncateToVisualWidth(route, width)
	}

	endpointBudget := width - middleWidth
	parentFloor := min(lipgloss.Width(parent), max(endpointBudget/2, 1))
	projectBudget := min(lipgloss.Width(project), max(endpointBudget-parentFloor, 1))
	parentBudget := max(endpointBudget-projectBudget, 1)
	return compactQuickEditProjectChip(project, projectBudget) + middle +
		textfit.TruncateToVisualWidth(parent, parentBudget)
}

func compactQuickEditProjectChip(project string, width int) string {
	if lipgloss.Width(project) <= width || width < 3 || !strings.HasPrefix(project, "[") || !strings.HasSuffix(project, "]") {
		return textfit.TruncateToVisualWidth(project, width)
	}
	return "[" + textfit.TruncateToVisualWidth(strings.TrimSuffix(strings.TrimPrefix(project, "["), "]"), width-2) + "]"
}

func quickEditRouteLabel(task *models.Task, project *models.Project) string {
	projectName := grammar.ProjectLabel(project)
	if projectName == "" {
		projectName = "project"
	}
	// No parent shows nothing: the chip alone is the whole route, since a word
	// standing in for the absence of a parent reads as a task named that.
	path := []string{"[" + projectName + "]"}
	if task == nil || task.ParentID == nil || project == nil {
		return path[0]
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
	return strings.Join(append(path, parents...), " › ")
}

func quickEditWorkflowLabel(row OutlinerRow, task *models.Task, preview *grammar.QuickEditResult) string {
	if preview != nil && preview.DestinationSet {
		return string(preview.Destination)
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
// dependencies, tags, pin, deferral, due/check-back, then priority disappear as
// width shrinks. The dependency count leads because it is the one line whose
// full value is never here anyway: it is a count, and the references themselves
// are read elsewhere.
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
		label := task.DueDate.Format("Jan 2")
		if clock := chrome.FormatDueClock(task); clock != "" {
			label += " " + clock
		}
		parts = append(parts, prefix+label)
	}
	if task.DeferredUntil != nil {
		parts = append(parts, "defer "+chrome.FormatDeferDate(*task.DeferredUntil, now))
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
	if len(task.Dependencies) > 0 {
		parts = append(parts, fmt.Sprintf("~%d", len(task.Dependencies)))
	}
	return parts
}

// =============================================================================
// Outliner Row Editor Messages
// =============================================================================

// StartTaskLineEditMsg activates the selected Outliner row editor. Task and
// Project are the frozen operation target so shell dispatch cannot retarget the
// edit if the cursor moves before the message is delivered.
type StartTaskLineEditMsg struct {
	Task    *models.Task
	Project *models.Project
}

func (m *OutlinerModel) DesktopTextBuffer() *chrome.DesktopTextBuffer {
	if m.MenuBar.Open || m.QueryReadout.Expanded || m.RelationPickerOpen() {
		return nil
	}
	if m.QuickEdit != nil && m.QuickEdit.IsActive() && !m.QuickEdit.Pending {
		buffer := chrome.DesktopInput(&m.QuickEdit.Input, m.QuickEdit.RefreshDraft)
		// The row editor reuses one input for different tasks.
		buffer.Key = [2]any{buffer.Key, m.QuickEdit.GetTask()}
		return buffer
	}
	if m.IsSearchMode() && m.Search.historyPicker == nil {
		return &chrome.DesktopTextBuffer{Key: &m.BaseSearchableView, Text: m.SearchInputValue(), Column: m.SearchInputCursor(), Restore: func(text string, _, column int) {
			m.SetSearchInputValue(text)
			m.SetSearchInputCursor(column)
			m.Search.history.resetRecall()
			RebuildOutlinerPreservingCursor(m)
			m.RefreshSearchAutocomplete()
		}}
	}
	return nil
}
