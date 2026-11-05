package outliner

import (
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/focus"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// planningReorderKind names the durable planning list a J/K move reorders in a
// planning-order context. The Today section reorders the Daily Plan; Next
// reorders the Focus Queue (the Next order index). Neither touches task
// hierarchy.
type planningReorderKind uint8

const (
	ReorderNextOrder planningReorderKind = iota
	ReorderToday
)

// OutlinerPlanningReorderMsg carries a durable planning-order reorder produced by
// J/K in a dedicated planning mode or an All-mode planning section. The
// Outliner owns the row ordering, so it computes the full new workspace order;
// the shell persists it through the planning transaction path and reloads the
// cross outliner with the cursor kept on the moved task. Because the whole
// reorder is a single planning commit it survives a restart and never mutates
// the task tree.
type OutlinerPlanningReorderMsg struct {
	Kind  planningReorderKind
	Order []config.ProjectTaskLocator
}

// isPlanningReorderMode reports whether J/K reorders a durable planning list
// rather than task hierarchy. Dedicated Today/Next modes and their synthetic
// All-mode groups share the same reorder path.
func (m *OutlinerModel) IsPlanningReorderMode() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	row := m.Rows[m.Cursor]
	if row.IsPinnedSection {
		return false
	}
	if (m.PrimaryMode == chrome.ModeToday || m.PrimaryMode == chrome.ModeNext) && row.IsTaskRow() {
		return true
	}
	return row.IsTodaySection || row.IsNextSection
}

// planningReorderMove builds the durable planning-order reorder for a J/K move
// in a planning projection. The target is the adjacent visible reorderable row,
// and the two visible identities are swapped in place within the complete
// workspace-wide order. Hidden planning members stay put relative to the pair.
func (m *OutlinerModel) PlanningReorderMove(delta int) (tea.Model, tea.Cmd) {
	if !m.IsPlanningReorderMode() {
		return m, nil
	}
	task, project := m.GetTaskRef()
	if task == nil || project == nil {
		return m, nil
	}

	kind := ReorderToday
	order := m.globalTodayOrder()
	row := m.Rows[m.Cursor]
	if m.PrimaryMode == chrome.ModeNext || row.IsNextSection {
		kind = ReorderNextOrder
		order = m.globalNextOrder()
	}

	moved := config.ProjectTaskLocator{ProjectID: project.ID, TaskID: task.ID}
	movedPos := locatorIndex(order, moved)
	if movedPos < 0 {
		// The cursor task is no longer a member of the planning list.
		return m, nil
	}

	// Resolve the target from the adjacent *visible* reorderable planning row, not
	// from the neighbor in the unfiltered durable order — otherwise a scope or
	// filter could swap the cursor with an invisible task.
	target, ok := m.adjacentVisiblePlanningMember(order, delta)
	if !ok {
		return m, nil // visible boundary: nothing to swap with.
	}
	targetPos := locatorIndex(order, target)
	if targetPos < 0 {
		return m, nil
	}
	order[movedPos], order[targetPos] = order[targetPos], order[movedPos]

	msg := OutlinerPlanningReorderMsg{
		Kind:  kind,
		Order: order,
	}
	return m, func() tea.Msg { return msg }
}

// visiblePlanningMembers returns every durable planning member (present in
// order) reachable by scanning m.rows from the cursor in the direction of delta,
// nearest first. The cursor row's own group bounds the active planning run, as
// does a separator. The list is empty at a visible boundary.
//
// The project half of that group stops the scan even though it does not
// partition the durable order, because a planning group's display position
// follows that order: a move reaching past it would lift the cursor's whole
// project above its neighbor rather than reorder tasks within it. A checklist
// row carries its owner's group, so an expanded checklist is stepped over rather
// than read as the end of the run. A member the active order separates from the
// cursor — another priority band under the default sort — ends the run as well,
// since planning order only breaks ties and a swap past it would move no row.
func (m *OutlinerModel) visiblePlanningMembers(
	order []config.ProjectTaskLocator, delta int,
) []config.ProjectTaskLocator {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return nil
	}
	members := make(map[config.ProjectTaskLocator]bool, len(order))
	for _, loc := range order {
		members[loc] = true
	}
	step := 1
	if delta < 0 {
		step = -1
	}
	cursorRow := m.Rows[m.Cursor]
	cursorGroup := cursorRow.Group
	var visible []config.ProjectTaskLocator
	for i := m.Cursor + step; i >= 0 && i < len(m.Rows); i += step {
		row := m.Rows[i]
		if row.IsSeparator || row.Group != cursorGroup {
			break
		}
		if !row.IsTaskRow() || row.Task == nil {
			continue
		}
		identity := m.TaskIdentityForRow(row)
		loc := config.ProjectTaskLocator{ProjectID: identity.ProjectID, TaskID: identity.TaskID}
		if members[loc] {
			if cursorRow.Task != nil && m.compareSectionTasks(row.Task, cursorRow.Task) != 0 {
				break
			}
			visible = append(visible, loc)
		}
	}
	return visible
}

// adjacentVisiblePlanningMember returns the nearest visible planning member in
// the delta direction, reporting false at a visible boundary.
func (m *OutlinerModel) adjacentVisiblePlanningMember(order []config.ProjectTaskLocator, delta int) (config.ProjectTaskLocator, bool) {
	visible := m.visiblePlanningMembers(order, delta)
	if len(visible) == 0 {
		return config.ProjectTaskLocator{}, false
	}
	return visible[0], true
}

// planningReorderEdge sends the cursor's planning member to the far end of the
// visible planning run in one commit. Only the global-order positions the run's
// members already occupy are rewritten, so a member the scope or a filter hides
// keeps its own position.
func (m *OutlinerModel) PlanningReorderEdge(delta int) (tea.Model, tea.Cmd) {
	if !m.IsPlanningReorderMode() {
		return m, nil
	}
	task, project := m.GetTaskRef()
	if task == nil || project == nil {
		return m, nil
	}

	kind := ReorderToday
	order := m.globalTodayOrder()
	row := m.Rows[m.Cursor]
	if m.PrimaryMode == chrome.ModeNext || row.IsNextSection {
		kind = ReorderNextOrder
		order = m.globalNextOrder()
	}

	moved := config.ProjectTaskLocator{ProjectID: project.ID, TaskID: task.ID}
	if locatorIndex(order, moved) < 0 {
		// The cursor task is no longer a member of the planning list.
		return m, nil
	}
	peers := m.visiblePlanningMembers(order, delta)
	if len(peers) == 0 {
		return m, nil // visible boundary: nothing to move past.
	}

	positions := make([]int, 0, len(peers)+1)
	for _, loc := range append([]config.ProjectTaskLocator{moved}, peers...) {
		position := locatorIndex(order, loc)
		if position < 0 {
			return m, nil
		}
		positions = append(positions, position)
	}
	slices.Sort(positions)
	for i, member := range edgeMoveOrder(moved, peers, delta) {
		order[positions[i]] = member
	}

	msg := OutlinerPlanningReorderMsg{
		Kind:  kind,
		Order: order,
	}
	return m, func() tea.Msg { return msg }
}

// globalNextOrder returns the workspace-wide Next sequence as project-qualified
// locators, ordered exactly as BuildNextModeRows renders it. Reordering over
// this full sequence (rather than only the visible scope) keeps the global order
// coherent.
func (m *OutlinerModel) globalNextOrder() []config.ProjectTaskLocator {
	projects := m.ProjectsInCatalogOrder(m.planningProjects())
	input := m.buildOutlinerModeInput(projects)
	input.CandidateTasks = nil
	input.TaskFilter = nil
	input.TaskExclude = nil
	input.IncludeDeferred = true
	rows := BuildNextModeRows(input)
	order := make([]config.ProjectTaskLocator, 0, len(rows))
	for _, r := range rows {
		if r.Task == nil || r.Project == nil {
			continue
		}
		order = append(order, config.ProjectTaskLocator{ProjectID: r.Project.ID, TaskID: r.Task.ID})
	}
	return order
}

// globalTodayOrder returns the workspace-wide Today list in the same durable
// project-qualified order used by the display.
func (m *OutlinerModel) globalTodayOrder() []config.ProjectTaskLocator {
	plan := m.resolveTodayOrder(m.ProjectsInCatalogOrder(m.planningProjects()))
	order := make([]config.ProjectTaskLocator, 0, len(plan))
	for _, ft := range plan {
		if ft.Task == nil || ft.Project == nil {
			continue
		}
		order = append(order, config.ProjectTaskLocator{ProjectID: ft.Project.ID, TaskID: ft.Task.ID})
	}
	return order
}

func (m *OutlinerModel) planningProjects() []*models.Project {
	if m == nil {
		return nil
	}
	if m.Project != nil {
		return []*models.Project{m.Project}
	}
	return m.Population.Projects
}

// locatorIndex returns the position of loc in order, or -1 when absent.
func locatorIndex(order []config.ProjectTaskLocator, loc config.ProjectTaskLocator) int {
	for i, item := range order {
		if item == loc {
			return i
		}
	}
	return -1
}

// PlanningTask is a project-qualified task in a durable planning order.
type PlanningTask = focus.PlanningTask

func (m *OutlinerModel) ReorderTask(delta int) tea.Cmd {
	if len(m.Rows) == 0 || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return nil
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() || row.Task == nil {
		return nil
	}

	task := row.Task
	siblings := m.visibleSiblings(m.Cursor)
	index := siblingIndex(siblings, task.ID)
	target := index + delta
	if index < 0 || target < 0 || target >= len(siblings) {
		return nil
	}
	other := siblings[target]
	project := m.ResolveProjectForRow(row)
	if project == nil {
		return nil
	}
	taskIdentity := taskref.For(task, project)
	targetIdentity := taskref.For(other, project)
	if m.taskItem.Wired() {
		return m.taskItem.ReorderSiblings(taskIdentity, targetIdentity)
	}

	// Isolated Outliner models have no transaction coordinator. Preserve their
	// in-memory structural behavior for focused rendering/navigation tests while
	// production always takes the checked application transaction above.
	task = project.GetTaskByID(task.ID)
	other = project.GetTaskByID(other.ID)
	if task == nil || other == nil {
		return nil
	}
	_ = project.SwapSiblingTasksAt(task, other, time.Now())
	RebuildOutlinerPreservingCursor(m)
	return nil
}

// reorderTaskEdge sends the cursor task to the far end of its visible siblings
// in one transaction. Repeating an adjacent swap through visible peers is a
// rotation of the durable slots those peers occupy, so only the sub-range from
// the cursor to the edge is named and every sibling outside it — hidden by a
// filter, a collapsed parent, or another section — keeps its own slot.
func (m *OutlinerModel) ReorderTaskEdge(delta int) tea.Cmd {
	if len(m.Rows) == 0 || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return nil
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() || row.Task == nil {
		return nil
	}
	siblings := m.visibleSiblings(m.Cursor)
	index := siblingIndex(siblings, row.Task.ID)
	if index < 0 {
		return nil
	}
	peers := siblings[index+1:]
	if delta < 0 {
		peers = reversed(siblings[:index])
	}
	if len(peers) == 0 {
		return nil
	}
	rotated := edgeMoveOrder(row.Task, peers, delta)
	project := m.ResolveProjectForRow(row)
	if project == nil {
		return nil
	}
	if m.taskItem.Wired() {
		order := make([]taskref.TaskIdentity, 0, len(rotated))
		for _, task := range rotated {
			order = append(order, taskref.For(task, project))
		}
		return m.taskItem.ReorderSiblingOrder(taskref.For(row.Task, project), order, reorderEdgeLabel(delta))
	}

	// Isolated Outliner models have no transaction coordinator. Preserve their
	// in-memory structural behavior for focused rendering/navigation tests while
	// production always takes the checked application transaction above.
	resolved := make([]*models.Task, 0, len(rotated))
	for _, task := range rotated {
		canonical := project.GetTaskByID(task.ID)
		if canonical == nil {
			return nil
		}
		resolved = append(resolved, canonical)
	}
	if err := project.ReorderSiblingTasksAt(resolved, time.Now()); err != nil {
		return nil
	}
	RebuildOutlinerPreservingCursor(m)
	return nil
}

// edgeMoveOrder returns the final display order of the run an edge move rewrites:
// the moving element plus the peers between it and the edge, given nearest
// first. The mover ends up past all of them and each peer shifts one slot toward
// the cursor's old position, which is exactly what repeating an adjacent swap
// produces. Every ordering domain shares the permutation, only the slots it is
// written into differ.
func edgeMoveOrder[T any](mover T, peers []T, delta int) []T {
	moved := make([]T, 0, len(peers)+1)
	if delta < 0 {
		moved = append(moved, mover)
		return append(moved, reversed(peers)...)
	}
	moved = append(moved, peers...)
	return append(moved, mover)
}

func reversed[T any](items []T) []T {
	out := make([]T, 0, len(items))
	for i := len(items) - 1; i >= 0; i-- {
		out = append(out, items[i])
	}
	return out
}

// reorderEdgeLabel names the edge an edge move heads for, for status text.
func reorderEdgeLabel(delta int) string {
	if delta < 0 {
		return "top"
	}
	return "bottom"
}

func siblingIndex(siblings []*models.Task, taskID string) int {
	for index, sibling := range siblings {
		if sibling != nil && sibling.ID == taskID {
			return index
		}
	}
	return -1
}
