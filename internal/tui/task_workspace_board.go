package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// taskWorkspaceBoardMode is the Board adapter registered with the shared
// controller. Column focus, scroll offsets, and card geometry are renderer
// state; query inputs, cursor, selection, filters, and organization remain in
// TaskWorkspaceState.
type taskWorkspaceBoardMode struct{ view *TaskWorkspaceView }

func (m taskWorkspaceBoardMode) WorkspaceLayout() TaskWorkspaceLayout { return WorkspaceBoard }
func (m taskWorkspaceBoardMode) ApplyWorkspaceSnapshot(state *TaskWorkspaceState, snapshot TaskWorkspaceSnapshot) {
	m.view.applySnapshot(state, snapshot, WorkspaceBoard)
}

func workspaceBoardMode(axis TaskWorkspaceBoardAxis) KanbanMode {
	switch axis {
	case WorkspaceBoardBySection:
		return KanbanModeSection
	case WorkspaceBoardByPriority:
		return KanbanModePriority
	case WorkspaceBoardByScore:
		return KanbanModeScoreBuckets
	default:
		return KanbanModeStatus
	}
}

func workspaceBoardAxisLabel(axis TaskWorkspaceBoardAxis) string {
	return KanbanModeNames[workspaceBoardMode(axis)]
}

func workspaceBoardColumnDefs(axis TaskWorkspaceBoardAxis) []KanbanColumn {
	switch axis {
	case WorkspaceBoardBySection:
		return []KanbanColumn{
			{ID: "inbox", Label: models.SectionInbox, Icon: "@"},
			{ID: "next", Label: models.SectionNext, Icon: "@"},
			{ID: "active", Label: "Active", Icon: "@"},
			{ID: "waiting", Label: models.SectionWaiting, Icon: "@"},
			{ID: "someday", Label: models.SectionSomeday, Icon: "@"},
			{ID: "review", Label: models.SectionReview, Icon: "@"},
			{ID: "standby", Label: models.SectionStandby, Icon: "@"},
			{ID: "completed", Label: models.SectionCompleted, Icon: "@"},
		}
	case WorkspaceBoardByPriority:
		return priorityColumnDefs()
	case WorkspaceBoardByScore:
		return scoreColumnDefs()
	default:
		return statusColumnDefs(nil)
	}
}

func workspaceBoardColumnID(item TaskWorkspaceItem, state *TaskWorkspaceState, snapshot TaskWorkspaceSnapshot) string {
	if item.Task == nil || item.Project == nil {
		return ""
	}
	if state.BoardAxis == WorkspaceBoardBySection {
		section := models.SectionForState(item.Task.State())
		if section == "" {
			return "active"
		}
		return sectionColumnID(section)
	}
	return columnForTask(item.Task, workspaceBoardMode(state.BoardAxis), snapshot.Automation, item.Project.ID)
}

func buildWorkspaceBoardColumns(
	state *TaskWorkspaceState,
	snapshot TaskWorkspaceSnapshot,
	previous []KanbanColumn,
) []KanbanColumn {
	columns := workspaceBoardColumnDefs(state.BoardAxis)
	previousByID := make(map[string]KanbanColumn, len(previous))
	for _, column := range previous {
		previousByID[column.ID] = column
	}
	index := make(map[string]int, len(columns))
	for i := range columns {
		index[columns[i].ID] = i
		if old, ok := previousByID[columns[i].ID]; ok {
			columns[i].Cursor = old.Cursor
			columns[i].ScrollOffset = old.ScrollOffset
		}
	}
	for _, item := range snapshot.Items {
		columnID := workspaceBoardColumnID(item, state, snapshot)
		columnIndex, ok := index[columnID]
		if !ok {
			columns = append(columns, KanbanColumn{ID: columnID, Label: columnID, Icon: "·"})
			columnIndex = len(columns) - 1
			index[columnID] = columnIndex
		}
		columns[columnIndex].Tasks = append(columns[columnIndex].Tasks, KanbanCard{Task: item.Task, Project: item.Project})
	}
	for i := range columns {
		if len(columns[i].Tasks) == 0 {
			columns[i].Cursor, columns[i].ScrollOffset = 0, 0
			continue
		}
		columns[i].Cursor = min(max(columns[i].Cursor, 0), len(columns[i].Tasks)-1)
		columns[i].ScrollOffset = min(max(columns[i].ScrollOffset, 0), len(columns[i].Tasks)-1)
	}
	return columns
}

func (m *TaskWorkspaceView) applyBoardSnapshot(state *TaskWorkspaceState, snapshot TaskWorkspaceSnapshot) {
	previous := m.boardColumns
	if m.boardAxis != state.BoardAxis {
		previous = nil
	}
	m.boardAxis = state.BoardAxis
	m.boardColumns = buildWorkspaceBoardColumns(state, snapshot, previous)

	order := make([]TaskIdentity, 0, len(snapshot.Items))
	for _, column := range m.boardColumns {
		for _, card := range column.Tasks {
			if card.Task != nil && card.Project != nil {
				order = append(order, TaskIdentity{ProjectID: card.Project.ID, TaskID: card.Task.ID})
			}
		}
	}
	state.applyPresentationOrder(order)
	m.syncBoardCursorFromState()
	m.ensureBoardCursorVisible()
}

func (m *TaskWorkspaceView) syncBoardCursorFromState() {
	id, ok := m.controller.State.Cursor()
	if !ok {
		return
	}
	for columnIndex := range m.boardColumns {
		for cardIndex, card := range m.boardColumns[columnIndex].Tasks {
			if card.Project != nil && card.Task != nil && card.Project.ID == id.ProjectID && card.Task.ID == id.TaskID {
				m.boardActiveColumn = columnIndex
				m.boardColumns[columnIndex].Cursor = cardIndex
				return
			}
		}
	}
}

func (m *TaskWorkspaceView) boardCursorIdentity() (TaskIdentity, bool) {
	if m.boardActiveColumn < 0 || m.boardActiveColumn >= len(m.boardColumns) {
		return TaskIdentity{}, false
	}
	column := &m.boardColumns[m.boardActiveColumn]
	if column.Cursor < 0 || column.Cursor >= len(column.Tasks) {
		return TaskIdentity{}, false
	}
	card := column.Tasks[column.Cursor]
	if card.Task == nil || card.Project == nil {
		return TaskIdentity{}, false
	}
	return TaskIdentity{ProjectID: card.Project.ID, TaskID: card.Task.ID}, true
}

func (m *TaskWorkspaceView) setBoardCursor(columnIndex, cardIndex int) {
	if columnIndex < 0 || columnIndex >= len(m.boardColumns) || len(m.boardColumns[columnIndex].Tasks) == 0 {
		return
	}
	m.boardActiveColumn = columnIndex
	m.boardColumns[columnIndex].Cursor = min(max(cardIndex, 0), len(m.boardColumns[columnIndex].Tasks)-1)
	if id, ok := m.boardCursorIdentity(); ok {
		m.controller.State.SetCursor(id)
	}
	m.ensureBoardCursorVisible()
}

func (m *TaskWorkspaceView) moveBoardCursor(delta int) {
	if m.boardActiveColumn < 0 || m.boardActiveColumn >= len(m.boardColumns) {
		return
	}
	column := &m.boardColumns[m.boardActiveColumn]
	if len(column.Tasks) == 0 {
		return
	}
	m.setBoardCursor(m.boardActiveColumn, column.Cursor+delta)
}

func (m *TaskWorkspaceView) jumpBoardCursor(last bool) {
	if m.boardActiveColumn < 0 || m.boardActiveColumn >= len(m.boardColumns) || len(m.boardColumns[m.boardActiveColumn].Tasks) == 0 {
		return
	}
	index := 0
	if last {
		index = len(m.boardColumns[m.boardActiveColumn].Tasks) - 1
	}
	m.setBoardCursor(m.boardActiveColumn, index)
}

func (m *TaskWorkspaceView) moveBoardColumn(delta int) {
	if delta == 0 || len(m.boardColumns) == 0 {
		return
	}
	row := 0
	if m.boardActiveColumn >= 0 && m.boardActiveColumn < len(m.boardColumns) {
		row = m.boardColumns[m.boardActiveColumn].Cursor
	}
	for next := m.boardActiveColumn + delta; next >= 0 && next < len(m.boardColumns); next += delta {
		if len(m.boardColumns[next].Tasks) > 0 {
			m.setBoardCursor(next, row)
			return
		}
	}
}

// moveBoardCard applies the adjacent-column meaning of J/K in Board. The
// controller supplies one identity set to the mutation adapter, so selections
// remain one application ChangeSet and one undo entry.
func (m *TaskWorkspaceView) moveBoardCard(delta int) tea.Cmd {
	if delta == 0 || m.boardActiveColumn < 0 || m.boardActiveColumn >= len(m.boardColumns) {
		return nil
	}
	targetIndex := m.boardActiveColumn + delta
	if targetIndex < 0 || targetIndex >= len(m.boardColumns) {
		return nil
	}
	target := m.boardColumns[targetIndex]
	switch m.controller.State.BoardAxis {
	case WorkspaceBoardByStatus:
		return m.controller.Transition(models.TargetStateForStatus(models.TaskStatus(target.ID)))
	case WorkspaceBoardBySection:
		state, ok := stateForSectionName(kanbanColIDToSection(target.ID))
		if !ok {
			return nil
		}
		return m.controller.Transition(state)
	case WorkspaceBoardByPriority:
		return m.controller.SetPriority(kanbanColIDToPriority(target.ID))
	case WorkspaceBoardByScore:
		if len(m.controller.State.Selected()) > 1 {
			return func() tea.Msg { return StatusMsg("Score Buckets are derived; edit one task's score inputs at a time") }
		}
		return m.itemCommand(func(item TaskWorkspaceItem) tea.Msg {
			return ShowScoreEditorMsg{Task: item.Task, Project: item.Project}
		})
	default:
		return nil
	}
}

func (m *TaskWorkspaceView) boardBodyRows() int {
	rows := m.contentHeight() - 2 // column header and divider
	if m.controller.State.PreviewVisible {
		rows -= 4 // divider plus three detail rows
	}
	return max(rows, 1)
}

func (m *TaskWorkspaceView) ensureBoardCursorVisible() {
	if m.boardActiveColumn < 0 || m.boardActiveColumn >= len(m.boardColumns) {
		return
	}
	column := &m.boardColumns[m.boardActiveColumn]
	rows := m.boardBodyRows()
	if column.Cursor < column.ScrollOffset {
		column.ScrollOffset = column.Cursor
	} else if column.Cursor >= column.ScrollOffset+rows {
		column.ScrollOffset = column.Cursor - rows + 1
	}
	column.ScrollOffset = min(max(column.ScrollOffset, 0), max(len(column.Tasks)-rows, 0))
}

func (m *TaskWorkspaceView) renderBoard() string {
	if len(m.boardColumns) == 0 {
		return renderEmptyState("No Board columns are available.", "v choose an organization axis")
	}
	layout := computeKanbanColumnLayout(m.width, len(m.boardColumns), m.boardColumnOffset)
	if m.boardActiveColumn < layout.startCol {
		m.boardColumnOffset = m.boardActiveColumn
		layout = computeKanbanColumnLayout(m.width, len(m.boardColumns), m.boardColumnOffset)
	} else if m.boardActiveColumn >= layout.endCol {
		m.boardColumnOffset = max(m.boardActiveColumn-layout.numVisible+1, 0)
		layout = computeKanbanColumnLayout(m.width, len(m.boardColumns), m.boardColumnOffset)
	}
	m.boardColumnOffset = layout.startCol

	const columnSeparator = "│"
	const dividerSeparator = "┼"
	activeVisible := m.boardActiveColumn - layout.startCol
	accent := lipgloss.NewStyle().Foreground(ColorPrimary)
	headers := make([]string, 0, layout.numVisible)
	for i := layout.startCol; i < layout.endCol; i++ {
		column := m.boardColumns[i]
		label := truncateToVisualWidth(fmt.Sprintf(" %s %s (%d)", column.Icon, column.Label, len(column.Tasks)), layout.colWidth)
		style := MutedStyle.Width(layout.colWidth)
		if i == m.boardActiveColumn {
			style = accent.Bold(true).Width(layout.colWidth)
		}
		headers = append(headers, style.Render(label))
	}
	leftHint, rightHint := "", ""
	if layout.leftHintWidth > 0 {
		leftHint = MutedStyle.Render(IconScrollLeft)
	}
	if layout.rightHintWidth > 0 {
		rightHint = MutedStyle.Render(IconScrollRight)
	}
	var b strings.Builder
	b.WriteString(leftHint + joinWithActiveColBorders(headers, columnSeparator, activeVisible, accent) + rightHint + "\n")
	dividers := make([]string, layout.numVisible)
	for i := range dividers {
		dividers[i] = strings.Repeat("─", layout.colWidth)
	}
	b.WriteString(strings.Repeat(" ", layout.leftHintWidth) + joinWithActiveColBorders(dividers, dividerSeparator, activeVisible, accent) + "\n")

	selected := make(map[TaskIdentity]bool)
	for _, id := range m.controller.State.Selected() {
		selected[id] = true
	}
	bodyRows := m.boardBodyRows()
	for row := 0; row < bodyRows; row++ {
		cells := make([]string, 0, layout.numVisible)
		for columnIndex := layout.startCol; columnIndex < layout.endCol; columnIndex++ {
			column := &m.boardColumns[columnIndex]
			cardIndex := column.ScrollOffset + row
			if cardIndex >= len(column.Tasks) {
				cells = append(cells, strings.Repeat(" ", layout.colWidth))
				continue
			}
			card := column.Tasks[cardIndex]
			id := TaskIdentity{ProjectID: card.Project.ID, TaskID: card.Task.ID}
			prefix := "  "
			if selected[id] {
				prefix = "✓ "
			}
			project := ""
			if m.controller.State.Scope.Kind == ScopeApp {
				project = card.Project.Name + " · "
			}
			line := truncateToVisualWidth(prefix+project+card.Task.Title, layout.colWidth)
			focused := columnIndex == m.boardActiveColumn && cardIndex == column.Cursor
			switch {
			case focused:
				cells = append(cells, SelectedLine(line, layout.colWidth))
			case selected[id]:
				cells = append(cells, ActiveStyle.Width(layout.colWidth).Render(line))
			default:
				cells = append(cells, lipgloss.NewStyle().Width(layout.colWidth).Render(line))
			}
		}
		b.WriteString(strings.Repeat(" ", layout.leftHintWidth) + joinWithActiveColBorders(cells, columnSeparator, activeVisible, accent))
		if row < bodyRows-1 || m.controller.State.PreviewVisible {
			b.WriteString("\n")
		}
	}
	if m.controller.State.PreviewVisible {
		b.WriteString(m.renderBoardDetail())
	}
	return b.String()
}

func (m *TaskWorkspaceView) renderBoardDetail() string {
	item, ok := m.currentItem()
	if !ok || item.Task == nil || item.Project == nil {
		return strings.Repeat("─", m.width) + "\n\n\n"
	}
	task := item.Task
	metadata := []string{item.Project.Name, task.State().String(), fmt.Sprintf("P%d", task.Priority)}
	if task.DueDate != nil {
		metadata = append(metadata, "Due "+task.DueDate.Format("2006-01-02"))
	}
	details := ""
	if len(task.Tags) > 0 {
		details = "Tags: " + strings.Join(task.Tags, ", ")
	}
	return "\n" + strings.Repeat("─", m.width) + "\n" +
		truncateToVisualWidth(" "+task.Title, m.width) + "\n" +
		truncateToVisualWidth(" "+strings.Join(metadata, " · "), m.width) + "\n" +
		truncateToVisualWidth(" "+details, m.width)
}

func (m *TaskWorkspaceView) boardContains(id TaskIdentity) bool {
	return slices.ContainsFunc(m.boardColumns, func(column KanbanColumn) bool {
		return slices.ContainsFunc(column.Tasks, func(card KanbanCard) bool {
			return card.Project != nil && card.Task != nil && card.Project.ID == id.ProjectID && card.Task.ID == id.TaskID
		})
	})
}
