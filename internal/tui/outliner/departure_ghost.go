package outliner

import (
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// departurePlacement is where a task stood, along the axes a departure can
// move it on: its project, its plan, its status, and whether it is hidden.
// The zero value is a task the view could not resolve.
type departurePlacement struct {
	resolved  bool
	projectID string
	plan      string
	state     models.WorkflowState
	hidden    bool
}

// departurePlacement reads where the task stands now. It looks the task up
// by its ID in every loaded project, not only by its identity, because a
// move to another project keeps the task ID and changes the project half.
func (m *OutlinerModel) departurePlacement(identity taskref.TaskIdentity) departurePlacement {
	task, project := m.TaskAndProjectForIdentity(identity)
	if task == nil {
		task, project = m.taskByIDAnywhere(identity.TaskID)
	}
	if task == nil || project == nil {
		return departurePlacement{}
	}
	return departurePlacement{
		resolved:  true,
		projectID: project.ID,
		plan:      m.planningMembership(project.ID, task.ID),
		state:     task.State(),
		hidden:    models.TaskEffectivelyHidden(project, task),
	}
}

// taskByIDAnywhere finds a task by its ID among the loaded projects.
func (m *OutlinerModel) taskByIDAnywhere(taskID string) (*models.Task, *models.Project) {
	if taskID == "" {
		return nil, nil
	}
	if m.Project != nil {
		if task := m.Project.GetTaskByID(taskID); task != nil {
			return task, m.Project
		}
	}
	for _, project := range m.Population.Projects {
		if project == nil {
			continue
		}
		if task := project.GetTaskByID(taskID); task != nil {
			return task, project
		}
	}
	return nil, nil
}

// planningMembership is the plan token a task belongs to, "today", "next", or
// nothing.
func (m *OutlinerModel) planningMembership(projectID, taskID string) string {
	if m.Runtime == nil {
		return ""
	}
	locator := config.ProjectTaskLocator{ProjectID: projectID, TaskID: taskID}
	planning := m.Runtime.Planning()
	switch {
	case slices.Contains(planning.GetToday(), locator):
		return "today"
	case slices.Contains(planning.GetNext(), locator):
		return "next"
	}
	return ""
}

// departureDestination names where a departed task went, for the ghost to
// show in place of a strike, and the ink to show it in. It compares where the
// task stood at the keypress with where it stands after the commit and names
// the one change a reader asked for, most specific first: a project, then a
// plan it joined, then a status, then hiding. A cancel, a delete, a task the
// view can no longer find, and a departure no axis explains name nothing, so
// they keep the strike.
func (m *OutlinerModel) departureDestination(capture *OutlinerRelocationCapture) (string, lipgloss.Style) {
	before := capture.departureFrom
	if !before.resolved {
		return "", lipgloss.Style{}
	}
	after := m.departurePlacement(capture.Task)
	if !after.resolved || after.state == models.StateCanceled || after.state == models.StateDone {
		return "", lipgloss.Style{}
	}
	accent := lipgloss.NewStyle().Foreground(theme.ColorAccent)
	if after.projectID != before.projectID {
		project := m.crossResolveProject(after.projectID)
		if project == nil && m.Project != nil && m.Project.ID == after.projectID {
			project = m.Project
		}
		if project != nil && project.Name != "" {
			return project.Name, theme.ProjectColorStyle(project)
		}
	}
	if after.plan != "" && after.plan != before.plan {
		return chrome.PlanStatusValue(after.plan), accent
	}
	if after.state != before.state || after.plan != before.plan {
		return models.WorkflowStateGroupForState(after.state).Label(), accent
	}
	if after.hidden && !before.hidden {
		return "Hidden", theme.MutedStyle
	}
	return "", lipgloss.Style{}
}

// captureDepartureLine records what the cursor row looks like, in plain text,
// and where the last render drew it, so a departure can leave it behind. It
// reads the last drawn window and never computes one; a cursor that frame did
// not draw records nothing.
func (m *OutlinerModel) captureDepartureLine(capture *OutlinerRelocationCapture) {
	if !m.motion || capture == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return
	}
	viewport := m.LastViewport
	if viewport.VisibleRows <= 0 || viewport.rowWidth <= 0 ||
		m.Cursor < viewport.startIdx || m.Cursor >= viewport.endIdx {
		return
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() || row.Task == nil {
		return
	}
	offset := m.Cursor - viewport.startIdx
	if viewport.pinned {
		offset++
	}
	// A ghost still on screen is not counted: the keypress dropped it, so the
	// next frame draws the rows where they stand without it.
	layout := m.computeRowLayout(m.Rows, viewport.rowWidth, &m.hiddenLayout)
	// The ghost lands its check on the state glyph, so it draws the glyph even
	// in a lane that otherwise leaves it to the header.
	ghostRow := row
	ghostRow.Group.SectionKey = ""
	line := ansi.Strip(m.RenderOutlinerTaskRow(ghostRow, false, viewport.rowWidth, m.Cursor, layout))
	capture.departureLine = strings.TrimRight(line, " ")
	capture.departureOffset = offset
	capture.departureIcon = theme.StateBadgeIcon(row.Task.State())
	capture.departureFrom = m.departurePlacement(capture.Task)
}

// renderDepartureGhost paints the ghost's line for its phase. A completion
// first lands its check: the row as it stood, the check in bold success ink.
// Then the row is struck through, in the success color for a completion and in
// the muted color otherwise, then faint. A relocation is never struck: the row
// goes muted with its destination after it, then both go faint. The
// indentation is never struck.
func (m *OutlinerModel) renderDepartureGhost(rowWidth int) string {
	ghost := m.ghost
	label := ""
	if ghost.relocated() {
		label = "  → " + ghost.destination
	}
	labelWidth := lipgloss.Width(label)
	if labelWidth >= rowWidth {
		label, labelWidth = "", 0
	}
	line := textfit.TruncateToVisualWidth(ghost.line, rowWidth-labelWidth)
	trimmed := strings.TrimLeft(line, " ")
	lead := line[:len(line)-len(trimmed)]
	var rendered string
	switch ghost.look() {
	case ghostLabeled:
		rendered = lead + theme.MutedStyle.Render(trimmed) + ghost.destinationStyle.Bold(true).Render(label)
	case ghostLabelFaded:
		rendered = lead + lipgloss.NewStyle().Foreground(theme.ColorMuted).Faint(true).Render(trimmed+label)
	case ghostChecked:
		rendered = lead + renderLandedCheck(trimmed, ghost.icon)
	case ghostFaded:
		rendered = lead + lipgloss.NewStyle().Foreground(theme.ColorMuted).Strikethrough(true).Faint(true).Render(trimmed)
	default:
		style := lipgloss.NewStyle().Foreground(theme.ColorMuted).Strikethrough(true)
		if ghost.done {
			style = style.Foreground(theme.ColorSuccess)
		}
		rendered = lead + style.Render(trimmed)
	}
	if pad := rowWidth - lipgloss.Width(rendered); pad > 0 {
		rendered += strings.Repeat(" ", pad)
	}
	return rendered
}

// ghostSlot decides where this frame draws the ghost, if anywhere: its
// recorded offset, inside the lines the window draws or directly below them.
// A full window lends the slot of its last row, which is why a ghost is never
// drawn where that row is the cursor's.
func (m *OutlinerModel) ghostSlot(viewport outlinerViewport) (offset int, dropLast bool, ok bool) {
	if m.ghost == nil {
		return 0, false, false
	}
	drawn := viewport.drawnRows()
	offset = m.ghost.screenOffset
	if offset < 0 || offset > drawn || offset >= viewport.VisibleRows {
		return 0, false, false
	}
	if drawn < viewport.VisibleRows {
		return offset, false, true
	}
	if viewport.endIdx-1 == m.Cursor || viewport.endIdx-1 < viewport.startIdx || offset >= drawn {
		return 0, false, false
	}
	return offset, true, true
}
