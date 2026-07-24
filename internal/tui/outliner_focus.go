package tui

import (
	"fmt"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// OutlinerFocus identifies the mutually exclusive base predicate used by an
// Outliner. Core focuses map to persisted perspectives; smart focuses reuse
// the same tree renderer with an additional predicate.
type OutlinerFocus uint8

const (
	OutlinerActive OutlinerFocus = iota
	OutlinerAll
	OutlinerInbox
	OutlinerNext
	OutlinerStandby
	OutlinerWaiting
	OutlinerReview
	OutlinerSomeday
	OutlinerCompleted
	OutlinerDueSoon
	OutlinerHighPriority
	OutlinerBlocked
	OutlinerParked
	OutlinerSnoozed
	OutlinerPinned
	OutlinerRecent
)

var outlinerFocusNames = [...]string{
	"Active",
	"All",
	"Inbox",
	"Next",
	"Standby",
	"Waiting",
	"Review",
	"Someday",
	"Completed",
	"Due Soon",
	"High Priority",
	"Blocked",
	"Parked",
	"Snoozed",
	"Pinned",
	"Recent",
}

// OutlinerLens selects the row presentation used inside the canonical
// Outliner. Tree is the normal hierarchical view; DueDate is a workspace-wide
// flat schedule used only by the cross-project Outliner.
type OutlinerLens uint8

const (
	OutlinerLensTree OutlinerLens = iota
	OutlinerLensDueDate
)

func (focus OutlinerFocus) String() string {
	if int(focus) < len(outlinerFocusNames) {
		return outlinerFocusNames[focus]
	}
	return fmt.Sprintf("OutlinerFocus(%d)", focus)
}

func (focus OutlinerFocus) perspectiveID() (int, bool) {
	switch focus {
	case OutlinerActive:
		return PerspectiveActive, true
	case OutlinerAll:
		return PerspectiveAll, true
	case OutlinerInbox:
		return PerspectiveInbox, true
	case OutlinerNext:
		return PerspectiveNext, true
	case OutlinerStandby:
		return PerspectiveStandby, true
	case OutlinerWaiting:
		return PerspectiveWaiting, true
	case OutlinerReview:
		return PerspectiveReview, true
	case OutlinerSomeday:
		return PerspectiveSomeday, true
	case OutlinerCompleted:
		return PerspectiveCompleted, true
	case OutlinerSnoozed:
		return PerspectiveSnoozed, true
	default:
		return 0, false
	}
}

// OutlinerOpenOptions applies command/startup intent without changing the
// typed TaskWorkspace route, which owns scope only.
type OutlinerOpenOptions struct {
	Focus    OutlinerFocus
	SetFocus bool
	Sort     SortField
	SetSort  bool
	Lens     OutlinerLens
	SetLens  bool
	// Mode selects a catalog mode directly (e.g. the review modes that the Daily
	// Review / Activity launch messages redirect into). SetMode decides the final
	// mode over focus/lens intent; an explicit SetSort can still initialize a tree
	// target's active and durable sort together.
	Mode    OutlinerModeID
	SetMode bool
}

func (m *OutlinerModel) ApplyOpenOptions(opts OutlinerOpenOptions) {
	target, hasModeIntent := m.openOptionsTargetMode(opts)
	if hasModeIntent {
		m.selectOutlinerMode(target.ID)
	} else {
		// Sort-only and no-op option sets still pass through the same seam so a
		// fixed-order mode cannot retain a legacy active manual sort.
		m.applyOutlinerModeTransition(target)
	}

	if opts.SetSort && target.SupportsManualSort() {
		sortState := SortState{Field: opts.Sort, Ascending: true}
		m.sort = sortState
		m.treeSort = sortState
	}
	// selectOutlinerMode already rebuilt, but an explicit tree sort is applied
	// afterward and therefore needs one final render. A no-intent call retains
	// ApplyOpenOptions' historical rebuild behavior.
	if !hasModeIntent || (opts.SetSort && target.SupportsManualSort()) {
		m.rebuildRows()
	}
}

// openOptionsTargetMode resolves the single catalog mode represented by a set
// of command/startup options. Direct Mode intent retains its historical
// precedence. Otherwise Due Date lens intent wins over focus, while an explicit
// tree lens reveals the requested (or underlying) tree mode.
func (m *OutlinerModel) openOptionsTargetMode(opts OutlinerOpenOptions) (OutlinerMode, bool) {
	current := m.activeOutlinerModeEntry()
	if opts.SetMode {
		if mode, ok := OutlinerModeByID(opts.Mode); ok {
			return mode, true
		}
		return current, false
	}
	if opts.SetLens && opts.Lens == OutlinerLensDueDate {
		mode, _ := OutlinerModeByID(ModeDueDateLens)
		return mode, true
	}
	if opts.SetFocus {
		if perspective, ok := opts.Focus.perspectiveID(); ok {
			if mode, found := OutlinerModeForPerspective(perspective); found {
				return mode, true
			}
		}
		if mode, ok := OutlinerModeForSmartFocus(opts.Focus); ok {
			return mode, true
		}
	}
	if opts.SetLens && opts.Lens == OutlinerLensTree {
		if current.ID == ModeDueDateLens {
			if m.smartFocus != nil {
				if mode, ok := OutlinerModeForSmartFocus(*m.smartFocus); ok {
					return mode, true
				}
			}
			if mode, ok := OutlinerModeForPerspective(m.activePerspective); ok {
				return mode, true
			}
		}
		return current, true
	}
	return current, false
}

func (m *OutlinerModel) activatePerspective(perspective int) {
	m.activePerspective = perspective
	m.smartFocus = nil
	m.crossPriorityMode = false
	m.workNextMode = false
	m.workBacklogMode = false
	m.reviewMode = reviewNone
	// A perspective is a tree-rendered mode. Restore the tree lens so a
	// previously selected Due Date lens cannot silently take precedence in
	// activeOutlinerMode (the lens is checked ahead of the perspective).
	m.lens = OutlinerLensTree
}

// applyFocusOrdering preserves the Outliner's hierarchy while honoring the
// durable ordering owned by focus-specific indexes. Explicit user sorts take
// precedence over these defaults.
func (m *OutlinerModel) applyFocusOrdering(tasks []*models.Task, projectID string) []*models.Task {
	if m.sort.Field != SortByDefault || m.runtime == nil {
		return tasks
	}

	ranks := make(map[string]int)
	switch {
	case m.smartFocus == nil && m.activePerspective == PerspectiveNext:
		for index, item := range m.runtime.Planning().FocusQueue {
			if item.ProjectID == projectID {
				ranks[item.TaskID] = index
			}
		}
	case m.smartFocus != nil && *m.smartFocus == OutlinerRecent:
		for index, visit := range m.runtime.Machine().GetRecentTaskVisits() {
			if visit.ProjectID == projectID {
				ranks[visit.TaskID] = index
			}
		}
	default:
		return tasks
	}

	return sortTaskTreeByRank(tasks, ranks)
}

func sortTaskTreeByRank(tasks []*models.Task, ranks map[string]int) []*models.Task {
	if len(tasks) == 0 {
		return tasks
	}
	const unranked = int(^uint(0) >> 1)
	type rankedTask struct {
		task *models.Task
		rank int
	}
	ranked := make([]rankedTask, 0, len(tasks))
	for _, task := range tasks {
		clone := *task
		clone.Children = sortTaskTreeByRank(task.Children, ranks)
		rank, ok := ranks[task.ID]
		if !ok {
			rank = unranked
		}
		for _, child := range clone.Children {
			if childRank := minimumTaskTreeRank(child, ranks, unranked); childRank < rank {
				rank = childRank
			}
		}
		ranked = append(ranked, rankedTask{task: &clone, rank: rank})
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].rank < ranked[j].rank })
	result := make([]*models.Task, len(ranked))
	for index := range ranked {
		result[index] = ranked[index].task
	}
	return result
}

func minimumTaskTreeRank(task *models.Task, ranks map[string]int, fallback int) int {
	result, ok := ranks[task.ID]
	if !ok {
		result = fallback
	}
	for _, child := range task.Children {
		if rank := minimumTaskTreeRank(child, ranks, fallback); rank < result {
			result = rank
		}
	}
	return result
}

func (m *OutlinerModel) baseFocusFilter(project *models.Project) func(*models.Task) bool {
	if m.smartFocus == nil {
		return m.currentPerspective().Filter
	}
	focus := *m.smartFocus
	return func(task *models.Task) bool {
		return m.matchesSmartFocus(task, project, focus, time.Now())
	}
}

func (m *OutlinerModel) matchesSmartFocus(task *models.Task, project *models.Project, focus OutlinerFocus, now time.Time) bool {
	if task == nil {
		return false
	}
	switch focus {
	case OutlinerDueSoon:
		return outlinerDueSoon(task, now, 7)
	case OutlinerHighPriority:
		return !task.IsTerminalState() && task.Priority <= 2
	case OutlinerBlocked:
		return project != nil && !task.IsTerminalState() && task.IsDependencyBlocked(project)
	case OutlinerParked:
		return !task.IsTerminalState() && task.IsParked()
	case OutlinerPinned:
		return !task.IsTerminalState() && task.Pinned
	case OutlinerRecent:
		if project == nil || m.runtime == nil {
			return false
		}
		for _, visit := range m.runtime.Machine().GetRecentTaskVisits() {
			if visit.ProjectID == project.ID && visit.TaskID == task.ID {
				return true
			}
		}
	case OutlinerActive, OutlinerAll, OutlinerInbox, OutlinerNext,
		OutlinerStandby, OutlinerWaiting, OutlinerReview, OutlinerSomeday,
		OutlinerCompleted, OutlinerSnoozed:
		// Core focuses are implemented through perspectives, not this smart-focus
		// predicate. Keep these cases explicit so additions to OutlinerFocus remain
		// exhaustively checked.
		return false
	}
	return false
}

func outlinerDueSoon(task *models.Task, now time.Time, days int) bool {
	if task == nil || task.DueDate == nil || task.IsTerminalState() {
		return false
	}
	if task.HasTime {
		return !task.DueDate.Before(now) && !task.DueDate.After(now.AddDate(0, 0, days))
	}
	today := util.StartOfDay(now)
	due := util.StartOfDay(*task.DueDate)
	return !due.Before(today) && !due.After(today.AddDate(0, 0, days))
}
