package tui

import (
	"strconv"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
)

// KanbanGroupBy controls which task field defines the kanban columns.
type KanbanGroupBy int

const (
	GroupBySection    KanbanGroupBy = iota // GTD sections only (default)
	GroupBySectionAll                      // GTD + custom sections
	GroupByStatus                          // Task status
	GroupByPriority                        // Priority level
	GroupByType                            // Task type
)

// KanbanGroupByCount is the number of group-by modes for cycling.
const KanbanGroupByCount = 5

// KanbanColumnDef defines a single kanban column.
type KanbanColumnDef struct {
	Key   string // The value used for task mapping (e.g., "", "Next", "todo", "1")
	Label string // Display label (e.g., "Active", "Next", "Todo", "P1 (Critical)")
}

// CompletedInStatusModeDays controls how many days of completed tasks to show
// in Done/Canceled columns when grouped by status.
const CompletedInStatusModeDays = 7

// IsSectionGroupBy returns true for both GroupBySection and GroupBySectionAll.
func IsSectionGroupBy(gb KanbanGroupBy) bool {
	return gb == GroupBySection || gb == GroupBySectionAll
}

// ColumnsForGroupBy returns ordered column definitions for the given group-by mode.
func ColumnsForGroupBy(gb KanbanGroupBy, project *models.Project) []KanbanColumnDef {
	switch gb {
	case GroupByStatus:
		return []KanbanColumnDef{
			{Key: string(models.StatusTodo), Label: "Todo"},
			{Key: string(models.StatusInProgress), Label: "In Progress"},
			{Key: string(models.StatusReview), Label: "Review"},
			{Key: string(models.StatusStandby), Label: "Standby"},
			{Key: string(models.StatusBlocked), Label: "Blocked"},
			{Key: string(models.StatusDone), Label: "Done"},
			{Key: string(models.StatusCanceled), Label: "Canceled"},
		}
	case GroupByPriority:
		return []KanbanColumnDef{
			{Key: "1", Label: "P1 (Critical)"},
			{Key: "2", Label: "P2 (High)"},
			{Key: "3", Label: "P3 (Normal)"},
			{Key: "4", Label: "P4 (Low)"},
			{Key: "5", Label: "P5 (Someday)"},
		}
	case GroupByType:
		return []KanbanColumnDef{
			{Key: string(models.TaskTypeEpic), Label: "Epic"},
			{Key: string(models.TaskTypeStory), Label: "Story"},
			{Key: string(models.TaskTypeTask), Label: "Task"},
			{Key: string(models.TaskTypeBug), Label: "Bug"},
			{Key: string(models.TaskTypeIdea), Label: "Idea"},
		}
	default: // GroupBySection / GroupBySectionAll
		defs := []KanbanColumnDef{
			{Key: KanbanSectionActive, Label: "Active"},
			{Key: KanbanSectionReview, Label: "Review"},
			{Key: KanbanSectionStandby, Label: "Standby"},
			{Key: KanbanSectionNext, Label: "Next"},
			{Key: KanbanSectionInbox, Label: "Inbox"},
			{Key: KanbanSectionWaiting, Label: "Waiting"},
			{Key: KanbanSectionSomeday, Label: "Someday"},
		}
		// Only append custom sections in GroupBySectionAll mode
		if gb == GroupBySectionAll && project != nil {
			for _, cs := range project.GetCustomSections() {
				defs = append(defs, KanbanColumnDef{Key: cs, Label: cs})
			}
		}
		return defs
	}
}

// TaskColumnKey extracts the column key from a task for the given group-by mode.
func TaskColumnKey(gb KanbanGroupBy, task *models.Task) string {
	switch gb {
	case GroupByStatus:
		return string(task.Status)
	case GroupByPriority:
		p := task.Priority
		if p >= 1 && p <= 5 {
			return strconv.Itoa(p)
		}
		return "0" // No priority
	case GroupByType:
		return string(task.Type)
	default: // GroupBySection / GroupBySectionAll
		return task.Section
	}
}

// ApplyColumnMove mutates the correct task field when moving between columns.
// For status mode, handles completion/reopening logic (CompletedAt).
func ApplyColumnMove(gb KanbanGroupBy, task *models.Task, targetKey string) {
	switch gb {
	case GroupByStatus:
		newStatus := models.TaskStatus(targetKey)
		// Moving from completed to non-completed requires reopen first
		if task.Status.IsComplete() && !newStatus.IsComplete() {
			task.Reopen()
		}
		if task.Status != newStatus {
			task.ApplyTransition(newStatus)
		}
	case GroupByPriority:
		p, err := strconv.Atoi(targetKey)
		if err == nil {
			if p == 0 {
				task.Priority = models.PriorityDefault
			} else {
				task.Priority = p
			}
		}
	case GroupByType:
		newType, ok := models.ParseTaskType(targetKey)
		if ok {
			task.Type = newType
		}
	default: // GroupBySection / GroupBySectionAll
		task.Section = targetKey
	}
}

// GroupByLabel returns a human-readable label for the group-by mode.
func GroupByLabel(gb KanbanGroupBy) string {
	switch gb {
	case GroupBySectionAll:
		return "All Sections"
	case GroupByStatus:
		return "Status"
	case GroupByPriority:
		return "Priority"
	case GroupByType:
		return "Type"
	default:
		return "Section"
	}
}

// IsCompletedStatusKey returns true if the key represents a completed status column.
func IsCompletedStatusKey(key string) bool {
	return key == string(models.StatusDone) || key == string(models.StatusCanceled)
}

// IsRecentlyCompleted returns true if the task was completed within the last N days.
func IsRecentlyCompleted(task *models.Task, days int) bool {
	if task.CompletedAt == nil {
		return false
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	return task.CompletedAt.After(cutoff)
}

// GetColumnHeaderStyle returns the appropriate style for a column header
// based on the group-by mode and column key.
func GetColumnHeaderStyle(gb KanbanGroupBy, key string) lipgloss.Style {
	switch gb {
	case GroupByStatus:
		return GetStatusStyle(key)
	case GroupByPriority:
		p, _ := strconv.Atoi(key)
		return GetPriorityStyle(p)
	case GroupByType:
		return GetTypeStyle(key)
	default:
		return GetViewModeStyleForSection(key)
	}
}

// setGroupBy sets the group-by mode, recomputes column definitions,
// reinitializes maps, resets cursor, and reloads tasks.
func (m *KanbanViewModel) setGroupBy(gb KanbanGroupBy) {
	m.groupBy = gb
	m.columnDefs = ColumnsForGroupBy(gb, m.Project)

	// Reinitialize maps for new columns
	m.Columns = make(map[string]*ColumnState)
	m.Tasks = make(map[string][]*models.Task)
	m.Nodes = make(map[string][]*KanbanNode)
	for _, def := range m.columnDefs {
		m.Columns[def.Key] = &ColumnState{}
		m.Tasks[def.Key] = []*models.Task{}
		m.Nodes[def.Key] = []*KanbanNode{}
	}

	m.ActiveColumn = 0
	m.columnScrollOffset = 0

	// If swimlane matches the new groupBy, disable it to avoid same-axis duplication
	if m.swimlaneEnabled() && m.swimlaneBy == gb {
		m.disableSwimlane()
		return // disableSwimlane calls loadTasks
	}

	// Reinitialize swimlane 2D maps for new column defs
	if m.swimlaneEnabled() {
		m.initSwimlane2DMaps()
	}

	m.loadTasks()
}

// cycleGroupBy cycles to the next group-by mode and persists the choice.
func (m *KanbanViewModel) cycleGroupBy() {
	next := KanbanGroupBy((int(m.groupBy) + 1) % KanbanGroupByCount)
	m.setGroupBy(next)

	// Persist to config
	if m.config != nil && m.Project != nil && m.configManager != nil {
		vs := m.config.ProjectViewStates[m.Project.ID]
		vs.KanbanGroupBy = int(next)
		m.config.ProjectViewStates[m.Project.ID] = vs
		_ = m.configManager.Save(m.config)
	}

	if next == GroupBySection {
		m.statusMessage = ""
	} else {
		m.statusMessage = "Grouped by: " + GroupByLabel(next)
	}
}

// taskColumnKey returns the column key for a task based on current group-by mode.
func (m *KanbanViewModel) taskColumnKey(task *models.Task) string {
	return TaskColumnKey(m.groupBy, task)
}

// columnLabel returns the display label for a column key.
func (m *KanbanViewModel) columnLabel(key string) string {
	for _, def := range m.columnDefs {
		if def.Key == key {
			return def.Label
		}
	}
	return sectionDisplayName(key)
}

// columnIndexForKey returns the index of the column with the given key, or -1 if not found.
func (m *KanbanViewModel) columnIndexForKey(key string) int {
	for i, def := range m.columnDefs {
		if def.Key == key {
			return i
		}
	}
	return -1
}
