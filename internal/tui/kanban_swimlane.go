package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// KanbanSwimlanesOff is the sentinel value for swimlaneBy when swimlanes are disabled.
const KanbanSwimlanesOff KanbanGroupBy = -1

// swimlaneEnabled returns true when swimlanes are active.
func (m *KanbanViewModel) swimlaneEnabled() bool {
	return m.swimlaneBy != KanbanSwimlanesOff
}

// activeLaneKey returns the key of the currently active swimlane row.
func (m *KanbanViewModel) activeLaneKey() string {
	if m.activeSwimlane >= 0 && m.activeSwimlane < len(m.swimlaneDefs) {
		return m.swimlaneDefs[m.activeSwimlane].Key
	}
	return ""
}

// swimlaneCurrentVisibleNodes returns visible nodes for the active cell (lane + column).
func (m *KanbanViewModel) swimlaneCurrentVisibleNodes() []*KanbanNode {
	laneKey := m.activeLaneKey()
	colKey := m.activeColumnKey()
	if nodes, ok := m.swimlaneNodes[laneKey]; ok {
		if cellNodes, ok := nodes[colKey]; ok {
			return cellNodes
		}
	}
	return nil
}

// swimlaneCurrentColumnState returns the ColumnState for the active cell.
func (m *KanbanViewModel) swimlaneCurrentColumnState() *ColumnState {
	laneKey := m.activeLaneKey()
	colKey := m.activeColumnKey()
	if cells, ok := m.swimlaneCells[laneKey]; ok {
		if cs, ok := cells[colKey]; ok {
			return cs
		}
	}
	// Fallback: create a zero state to avoid nil panics
	return &ColumnState{}
}

// cycleSwimlaneBy cycles through swimlane modes: Off → Section → Status → Priority → Type → Off,
// skipping the value that equals groupBy and skipping SectionAll (redundant with column navigation).
func (m *KanbanViewModel) cycleSwimlaneBy() {
	// Cycle order: Off(-1) → Section(0) → Status(2) → Priority(3) → Type(4) → Off(-1)
	next := int(m.swimlaneBy) + 1
	if next >= KanbanGroupByCount {
		next = -1 // Back to off
	}

	// Skip GroupBySectionAll (redundant) and the value that matches current groupBy
	for next != -1 && (KanbanGroupBy(next) == m.groupBy || KanbanGroupBy(next) == GroupBySectionAll) {
		next++
		if next >= KanbanGroupByCount {
			next = -1
		}
	}

	if next == -1 {
		m.disableSwimlane()
		m.statusMessage = "Swimlanes: off"
	} else {
		m.enableSwimlane(KanbanGroupBy(next))
		m.statusMessage = "Swimlanes: " + GroupByLabel(KanbanGroupBy(next))
	}

	// Persist to config
	m.persistSwimlaneBy()
}

// enableSwimlane activates swimlane mode with the given group-by dimension.
func (m *KanbanViewModel) enableSwimlane(gb KanbanGroupBy) {
	m.swimlaneBy = gb
	m.swimlaneDefs = ColumnsForGroupBy(gb, m.Project)
	m.activeSwimlane = 0
	m.swimlaneScrollOffset = 0
	m.initSwimlane2DMaps()
	m.loadTasks()
}

// disableSwimlane turns off swimlane mode and clears 2D data structures.
func (m *KanbanViewModel) disableSwimlane() {
	m.swimlaneBy = KanbanSwimlanesOff
	m.swimlaneDefs = nil
	m.activeSwimlane = 0
	m.swimlaneScrollOffset = 0
	m.swimlaneTasks = nil
	m.swimlaneNodes = nil
	m.swimlaneCells = nil
	m.loadTasks()
}

// initSwimlane2DMaps creates fresh 2D maps for all lane×column combinations.
func (m *KanbanViewModel) initSwimlane2DMaps() {
	m.swimlaneTasks = make(map[string]map[string][]*models.Task)
	m.swimlaneNodes = make(map[string]map[string][]*KanbanNode)
	m.swimlaneCells = make(map[string]map[string]*ColumnState)

	for _, lane := range m.swimlaneDefs {
		m.swimlaneTasks[lane.Key] = make(map[string][]*models.Task)
		m.swimlaneNodes[lane.Key] = make(map[string][]*KanbanNode)
		m.swimlaneCells[lane.Key] = make(map[string]*ColumnState)
		for _, col := range m.columnDefs {
			m.swimlaneTasks[lane.Key][col.Key] = []*models.Task{}
			m.swimlaneNodes[lane.Key][col.Key] = []*KanbanNode{}
			m.swimlaneCells[lane.Key][col.Key] = &ColumnState{}
		}
	}
}

// loadTasksWithSwimlanes is called from loadTasks when swimlanes are active.
// It distributes tasks into the 2D grid [laneKey][colKey].
func (m *KanbanViewModel) loadTasksWithSwimlanes() {
	// Reset custom section row focus if no custom sections exist
	if m.customSectionTabs() == nil {
		m.customSectionRowFocused = false
		m.customSectionTab = 0
	}

	// Clear existing swimlane data
	m.initSwimlane2DMaps()

	// Also clear the 1D maps (they're unused but keep them consistent)
	for _, def := range m.columnDefs {
		m.Tasks[def.Key] = []*models.Task{}
		m.Nodes[def.Key] = []*KanbanNode{}
	}

	if m.Project == nil {
		return
	}

	// Distribute tasks into swimlane cells
	for _, task := range m.Project.Tasks {
		// Handle completed tasks based on group-by mode
		if task.Status.IsComplete() {
			if m.groupBy == GroupByStatus && IsRecentlyCompleted(task, CompletedInStatusModeDays) {
				// Allow
			} else {
				continue
			}
		}

		if task.IsSnoozed() {
			continue
		}

		if !m.passesFilters(task) {
			continue
		}

		if m.FocusedTask != nil && !m.isInFocusedSubtree(task) {
			continue
		}

		colKey := TaskColumnKey(m.groupBy, task)
		laneKey := TaskColumnKey(m.swimlaneBy, task)

		// Validate column key
		if _, exists := m.Columns[colKey]; !exists {
			if IsSectionGroupBy(m.groupBy) {
				colKey = KanbanSectionActive
			} else {
				continue
			}
		}

		// Validate lane key — check against swimlaneDefs
		validLane := false
		for _, def := range m.swimlaneDefs {
			if def.Key == laneKey {
				validLane = true
				break
			}
		}
		if !validLane {
			if IsSectionGroupBy(m.swimlaneBy) {
				laneKey = KanbanSectionActive
			} else {
				continue
			}
		}

		m.swimlaneTasks[laneKey][colKey] = append(m.swimlaneTasks[laneKey][colKey], task)
		// Also add to 1D Tasks for filter count calculations
		m.Tasks[colKey] = append(m.Tasks[colKey], task)
	}

	// Build flat nodes for each cell (no hierarchy in swimlane mode — cells are too small)
	for _, lane := range m.swimlaneDefs {
		for _, col := range m.columnDefs {
			tasks := m.swimlaneTasks[lane.Key][col.Key]
			var nodes []*KanbanNode
			for _, task := range tasks {
				nodes = append(nodes, &KanbanNode{
					Task:    task,
					Level:   0,
					Visible: true,
				})
			}
			m.swimlaneNodes[lane.Key][col.Key] = nodes
		}
	}

	// Sort nodes in each cell
	for _, lane := range m.swimlaneDefs {
		for _, col := range m.columnDefs {
			nodes := m.swimlaneNodes[lane.Key][col.Key]
			if len(nodes) > 1 {
				sortField := m.getSortFieldForColumn(col.Key)
				if sortField != SortByDefault || col.Key == KanbanSectionActive {
					m.sortKanbanNodeSlice(nodes, sortField, col.Key)
				}
			}
		}
	}

	// Validate cursor positions for all cells
	for _, lane := range m.swimlaneDefs {
		for _, col := range m.columnDefs {
			cs := m.swimlaneCells[lane.Key][col.Key]
			cs.Validate(len(m.swimlaneNodes[lane.Key][col.Key]))
		}
	}

	// Calculate filter counts
	m.calculateFilterCounts()
}

// moveSwimlaneNext moves focus to the next swimlane row below.
func (m *KanbanViewModel) moveSwimlaneNext() {
	if m.activeSwimlane < len(m.swimlaneDefs)-1 {
		m.activeSwimlane++
		m.ensureActiveSwimlaneVisible()
	}
}

// moveSwimlanePrev moves focus to the previous swimlane row above.
func (m *KanbanViewModel) moveSwimlanePrev() {
	if m.activeSwimlane > 0 {
		m.activeSwimlane--
		m.ensureActiveSwimlaneVisible()
	}
}

// ensureActiveSwimlaneVisible adjusts swimlaneScrollOffset so that activeSwimlane
// is within the visible range.
func (m *KanbanViewModel) ensureActiveSwimlaneVisible() {
	total := len(m.swimlaneDefs)
	maxVisible := m.maxVisibleSwimlanes()
	if total <= maxVisible {
		m.swimlaneScrollOffset = 0
		return
	}
	if m.activeSwimlane < m.swimlaneScrollOffset {
		m.swimlaneScrollOffset = m.activeSwimlane
	}
	if m.activeSwimlane >= m.swimlaneScrollOffset+maxVisible {
		m.swimlaneScrollOffset = m.activeSwimlane - maxVisible + 1
	}
	// Clamp
	if m.swimlaneScrollOffset < 0 {
		m.swimlaneScrollOffset = 0
	}
	if m.swimlaneScrollOffset > total-maxVisible {
		m.swimlaneScrollOffset = total - maxVisible
	}
}

// visibleSwimlaneRange returns the start (inclusive) and end (exclusive) indices
// of swimlane rows that should be rendered.
func (m *KanbanViewModel) visibleSwimlaneRange() (int, int) {
	total := len(m.swimlaneDefs)
	maxVisible := m.maxVisibleSwimlanes()
	if total <= maxVisible {
		return 0, total
	}
	start := m.swimlaneScrollOffset
	if start < 0 {
		start = 0
	}
	if start > total-maxVisible {
		start = total - maxVisible
	}
	end := start + maxVisible
	if end > total {
		end = total
	}
	return start, end
}

// swimlaneCellNodeCount returns the number of nodes in a specific cell.
func (m *KanbanViewModel) swimlaneCellNodeCount(laneKey, colKey string) int {
	if nodes, ok := m.swimlaneNodes[laneKey]; ok {
		if cellNodes, ok := nodes[colKey]; ok {
			return len(cellNodes)
		}
	}
	return 0
}

// swimlaneLaneTaskCount returns the total task count in a swimlane row.
func (m *KanbanViewModel) swimlaneLaneTaskCount(laneKey string) int {
	count := 0
	for _, col := range m.columnDefs {
		count += m.swimlaneCellNodeCount(laneKey, col.Key)
	}
	return count
}

// persistSwimlaneBy saves the swimlane mode to config.
func (m *KanbanViewModel) persistSwimlaneBy() {
	if m.config == nil || m.Project == nil || m.configManager == nil {
		return
	}
	vs := m.config.ProjectViewStates[m.Project.ID]
	if m.swimlaneBy == KanbanSwimlanesOff {
		vs.KanbanSwimlaneBy = nil
	} else {
		val := int(m.swimlaneBy)
		vs.KanbanSwimlaneBy = &val
	}
	m.config.ProjectViewStates[m.Project.ID] = vs
	_ = m.configManager.Save(m.config)
}
