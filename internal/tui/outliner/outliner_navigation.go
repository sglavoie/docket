package outliner

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// resolveProjectForRow returns the project that owns the given row.
// In single-project mode it returns m.project; in cross-project mode it
// resolves via crossResolveProject using the row's ProjectID.
func (m *OutlinerModel) ResolveProjectForRow(row OutlinerRow) *models.Project {
	if m.Project != nil {
		return m.Project
	}
	return m.crossResolveProject(row.ProjectID)
}

// NavigateToTask navigates to a task in the current view. If the task is
// inside a collapsed parent, ancestors are expanded first.
func (m *OutlinerModel) NavigateToTask(task *models.Task) {
	m.navigateToTask(task)
}

func (m *OutlinerModel) navigateToTask(task *models.Task) bool {
	if task == nil {
		return false
	}
	projectID := ""
	if m.Project != nil {
		projectID = m.Project.ID
	} else {
		for _, project := range m.Population.Projects {
			if project != nil && project.GetTaskByID(task.ID) == task {
				projectID = project.ID
				break
			}
		}
	}
	for i, row := range m.Rows {
		if row.Task != nil && row.Task.ID == task.ID && (m.Project != nil || row.ProjectID == projectID) {
			m.Cursor = i
			return true
		}
	}
	if projectID == "" {
		return false
	}

	if bucket := theme.TaskStateGroupLabel(task); bucket != "" && m.Project != nil {
		m.Folds.clearSection(bucket)
	}
	m.clearSectionZoom()
	m.ensureExpandedNodes()
	for ancestor := task.Parent; ancestor != nil; ancestor = ancestor.Parent {
		m.Folds.setNode(taskref.TaskIdentity{ProjectID: projectID, TaskID: ancestor.ID}, true)
	}
	m.RebuildRows()

	for i, row := range m.Rows {
		if row.Task != nil && row.Task.ID == task.ID && (m.Project != nil || row.ProjectID == projectID) {
			m.Cursor = i
			return true
		}
	}
	return false
}

// revealTask honors explicit task navigation even when persisted view
// state would hide the target. The initial attempt preserves the current view;
// only a genuinely hidden target resets visibility constraints.
func (m *OutlinerModel) revealTask(task *models.Task) {
	if m.navigateToTask(task) {
		return
	}

	m.Focus.clear()
	m.clearSectionZoom()
	m.clearSearchProjectionWithoutRestore()
	// The row facets are replaced wholesale by what the target needs to be
	// visible: the reveal writes clauses like any other facet, so the carriers,
	// the chips, and the predicate follow it.
	clauses := make([]models.QueryClause, 0, 2)
	if task.IsTerminalState() {
		clauses = query.AppendFilterClause(clauses, models.QueryFieldState,
			models.QueryOperatorIs, task.State().String())
	}
	if task.IsDeferred() {
		clauses = query.AppendFilterClause(clauses, models.QueryFieldDeferred,
			models.QueryOperatorPresent, "")
	}
	m.ReplaceFilterClauses(query.FilterQueryTaskFields(), clauses)
	m.SelectOutlinerMode(chrome.ModeAll)
	for i, row := range m.Rows {
		if row.Task != nil && row.Task.ID == task.ID {
			m.Cursor = i
			return
		}
	}
	if project := m.projectHoldingTask(task); project != nil &&
		m.expandFoldsRevealing(task, taskref.For(task, project)) {
		return
	}
	m.SetTransientStatus("Task is unavailable in this project")
}

// projectHoldingTask is the project whose tree holds task: the scoped one, else
// the loaded project that owns it.
func (m *OutlinerModel) projectHoldingTask(task *models.Task) *models.Project {
	if m.Project != nil {
		return m.Project
	}
	for _, project := range m.Population.Projects {
		if project != nil && project.GetTaskByID(task.ID) == task {
			return project
		}
	}
	return nil
}

// NavigateToTaskInProject navigates to a task in the cross-project outliner.
// Project identity is part of the match so duplicate task IDs in different
// projects cannot move the cursor to the wrong row.
func (m *OutlinerModel) NavigateToTaskInProject(task *models.Task, project *models.Project) {
	if task == nil {
		return
	}
	if m.Project != nil {
		m.NavigateToTask(task)
		return
	}

	projectID := ""
	if project != nil {
		projectID = project.ID
	}
	if projectID == "" {
		for _, p := range m.Population.Projects {
			if p != nil && p.GetTaskByID(task.ID) == task {
				projectID = p.ID
				break
			}
		}
	}
	if projectID == "" {
		m.SetTransientStatus("Task project not found")
		return
	}

	if m.MoveCursorToProjectTask(task.ID, projectID) {
		return
	}

	section := effectiveDisplaySection(task)
	m.Folds.clearSection("_xp:" + section)
	m.Focus.clearCross()
	m.clearSectionZoom()
	m.ensureExpandedNodes()
	for ancestor := task.Parent; ancestor != nil; ancestor = ancestor.Parent {
		m.Folds.setNode(taskref.TaskIdentity{ProjectID: projectID, TaskID: ancestor.ID}, true)
	}
	m.RebuildRows()

	if m.MoveCursorToProjectTask(task.ID, projectID) {
		return
	}
	// A lane fold, such as the Backlog one a launch seeds, is not spelled from
	// the task, so the shared reveal reads it off the rebuilt rows instead.
	identity := taskref.TaskIdentity{ProjectID: projectID, TaskID: task.ID}
	if !m.expandFoldsRevealing(task, identity) {
		m.SetTransientStatus(m.hiddenTaskNotice(task, time.Now()))
	}
}

// hiddenTaskNotice names why a task the cursor was sent to has no row, in the
// order the projection itself hides one: a pending deferral before anything the
// view narrows, since a deferred task is absent from every view that does not
// ask for it and "filters" would send the reader to a key that shows nothing.
func (m *OutlinerModel) hiddenTaskNotice(task *models.Task, now time.Time) string {
	switch {
	case task.DeferredUntil != nil && task.IsDeferredAt(now):
		return fmt.Sprintf("Deferred until %s, so it stays hidden until then",
			chrome.FormatDeferDate(*task.DeferredUntil, now))
	case m.HasSearchProjection() && m.SearchQuery() != "":
		return "Task is hidden by the current search"
	case m.HasActiveLens():
		return "Task is hidden by the active lens"
	case query.HasActiveCommonFilters(&m.Filter.CommonFilterState):
		return "Task is hidden by current filters"
	case m.Focus.Active():
		return "Task is outside the current Focus"
	case m.PrimaryMode != chrome.ModeAll:
		return fmt.Sprintf("Task is not part of %s", m.ActiveOutlinerModeEntry().Label)
	case task.IsTerminalState():
		return "Task is finished, so All no longer shows it"
	default:
		return "Task is hidden in this view"
	}
}

func (m *OutlinerModel) MoveCursorToProjectTask(taskID, projectID string) bool {
	for i, row := range m.Rows {
		if row.Task != nil && row.ProjectID == projectID && row.Task.ID == taskID {
			m.Cursor = i
			return true
		}
	}
	return false
}

// GoToParentTaskAtCursor moves from a child task to its immediate parent, or
// from a checklist item to its owning task. A Focus root has no row, so the
// jump first leaves that frame; a parent hidden by other narrowing is revealed
// in the ordinary Outliner rather than leaving the cursor on the child.
func (m *OutlinerModel) GoToParentTaskAtCursor() bool {
	if m == nil || m.MultiSelectMode || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	row := m.Rows[m.Cursor]
	if row.Task == nil || (!row.IsTaskRow() && !row.IsChecklistItem) {
		return false
	}
	origin := m.TaskIdentityForRow(row)
	task, project := m.TaskAndProjectForIdentity(origin)
	if task == nil || project == nil {
		return false
	}
	parent := task
	if !row.IsChecklistItem {
		parent = project.GetTaskByID(parentIDOf(task))
	}
	if parent == nil {
		return false
	}
	identity := taskref.For(parent, project)
	m.ClearSearch()
	if m.MoveCursorToIdentity(identity) {
		m.rebaseReloadTargetAfterRelocation()
		return true
	}
	if root, focused := m.FocusRootIdentity(); focused && root == identity {
		m.PopFocusLevel()
		if m.MoveCursorToIdentity(identity) {
			m.rebaseReloadTargetAfterRelocation()
			return true
		}
	}
	if !m.revealTaskForNavigation(parent, project, false) {
		return false
	}
	m.rebaseReloadTargetAfterRelocation()
	return true
}

func (m *OutlinerModel) StatusBarLabel() string {
	if m.Project != nil {
		return fmt.Sprintf("Outliner: %s", m.Project.Name)
	}
	return m.crossStatusBarLabel()
}

// through its owner so a fold started from one lands on the task it belongs to.
func (m *OutlinerModel) cursorTaskIdentity() (taskref.TaskIdentity, bool) {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return taskref.TaskIdentity{}, false
	}
	row := m.Rows[m.Cursor]
	if (!row.IsTaskRow() && !row.IsChecklistItem) || row.Task == nil {
		return taskref.TaskIdentity{}, false
	}
	return m.TaskIdentityForRow(row), true
}

// cursorAncestorIdentities lists the cursor task's ancestors nearest-first. It
// walks ParentID through the owning project rather than the row's Parent
// pointer, because a projection row can hold a clone whose links no longer
// reach the loaded tree — the same reason visibleSiblings reads parentIDOf.
func (m *OutlinerModel) cursorAncestorIdentities() []taskref.TaskIdentity {
	identity, ok := m.cursorTaskIdentity()
	if !ok {
		return nil
	}
	project := m.ResolveProjectForRow(m.Rows[m.Cursor])
	if project == nil {
		return nil
	}
	var ancestors []taskref.TaskIdentity
	parentID := parentIDOf(m.Rows[m.Cursor].Task)
	for depth := 0; parentID != "" && depth <= models.MaxNestingDepth; depth++ {
		ancestors = append(ancestors, taskref.TaskIdentity{ProjectID: identity.ProjectID, TaskID: parentID})
		parent := project.GetTaskByID(parentID)
		if parent == nil {
			break
		}
		parentID = parentIDOf(parent)
	}
	return ancestors
}

// moveCursorToNearestIdentity parks the cursor on the first candidate that still
// has a row, clamping when none does.
func (m *OutlinerModel) moveCursorToNearestIdentity(candidates []taskref.TaskIdentity) {
	for _, candidate := range candidates {
		if m.MoveCursorToIdentity(candidate) {
			return
		}
	}
	m.ClampCursor()
}

// ExpandAncestorsOfTask opens the task subtree folds between task and its
// project root and touches nothing else, reporting whether any of them moved. It
// is the narrow half of navigateToTask: a reveal that must leave the scope, the
// mode, the query, the sections and every unrelated fold exactly as it found
// them still owes its target an open branch, because a collapsed ancestor is the
// one way a freshly committed child is missing from a projection that admits it.
func (m *OutlinerModel) ExpandAncestorsOfTask(task *models.Task, projectID string) bool {
	if m == nil || task == nil || projectID == "" {
		return false
	}
	changed := false
	for ancestor := task.Parent; ancestor != nil; ancestor = ancestor.Parent {
		identity := taskref.TaskIdentity{ProjectID: projectID, TaskID: ancestor.ID}
		if m.IsExpanded(identity) {
			continue
		}
		m.Folds.setNode(identity, true)
		changed = true
	}
	return changed
}

func (m *OutlinerModel) MoveCursorToIdentity(identity taskref.TaskIdentity) bool {
	for i, row := range m.Rows {
		if !row.IsTaskRow() || row.Task == nil {
			continue
		}
		if m.TaskIdentityForRow(row) == identity {
			m.Cursor = i
			return true
		}
	}
	m.ClampCursor()
	return false
}

// MoveCursorToChecklistItem seats the cursor on one expanded checklist row of
// identity. It is the checklist counterpart of MoveCursorToIdentity and exists
// because a checklist row carries its owner's task pointer: that method would
// stop on the owner row instead, one row above the item the caller named.
func (m *OutlinerModel) MoveCursorToChecklistItem(identity taskref.TaskIdentity, index int) bool {
	if m == nil {
		return false
	}
	for i, row := range m.Rows {
		if !row.IsChecklistItem || row.Task == nil || row.ChecklistIndex != index {
			continue
		}
		if m.TaskIdentityForRow(row) != identity {
			continue
		}
		m.Cursor = i
		return true
	}
	m.ClampCursor()
	return false
}

// crossSkipNonNavigableRows walks the cursor off a header or other unselectable
// row in the direction the move was already going, falling back to the other
// direction at an edge. It passes cmd straight through so a caller can thread a
// navigation command across the skip; no caller has ever needed the model back,
// so it is not returned.
func (m *OutlinerModel) CrossSkipNonNavigableRows(previous int, cmd tea.Cmd) tea.Cmd {
	if len(m.Rows) == 0 {
		return cmd
	}
	if m.Cursor < previous {
		for m.Cursor > 0 && !m.Rows[m.Cursor].IsNavigable() {
			m.Cursor--
		}
		if !m.Rows[m.Cursor].IsNavigable() {
			for m.Cursor < len(m.Rows)-1 && !m.Rows[m.Cursor].IsNavigable() {
				m.Cursor++
			}
		}
	} else {
		for m.Cursor < len(m.Rows)-1 && !m.Rows[m.Cursor].IsNavigable() {
			m.Cursor++
		}
		if !m.Rows[m.Cursor].IsNavigable() {
			for m.Cursor > 0 && !m.Rows[m.Cursor].IsNavigable() {
				m.Cursor--
			}
		}
	}
	return cmd
}

// simplifiedReorderEligible dispatches J/K by the selected row's ordering
// context. Pinned uses manual pin order wherever it renders; Due is automatic;
// Today and Next use durable planning order; All, Inbox, and Backlog use
// same-project sibling order.
func (m *OutlinerModel) SimplifiedReorderEligible() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	if m.MultiSelectMode {
		return false
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() || row.Task == nil {
		return false
	}
	// A pinned row carries its own manual order, so it answers before the active
	// mode's reorder policy.
	if m.IsPinnedReorderRow() {
		return true
	}
	mode, ok := chrome.OutlinerModeByID(m.ActiveOutlinerMode())
	if !ok || mode.ReorderPolicy == chrome.ReorderPolicyNone {
		return false
	}
	if m.IsPlanningReorderMode() {
		return true
	}
	switch mode.ID {
	case chrome.ModeAll, chrome.ModeInbox, chrome.ModeBacklog:
		return m.ResolveProjectForRow(row) != nil
	default:
		return false
	}
}

// rebuildOutlinerPreservingCursor rebuilds the rows around the cursor's mode
// anchor rather than around its numeric row, so a rebuild that reorders or
// resizes the projection still leaves the cursor on the row it was on. It is a
// function rather than a method because a nil Outliner is an ordinary caller
// state: the shell refreshes views without first proving one is mounted.
func RebuildOutlinerPreservingCursor(outliner *OutlinerModel) {
	if outliner == nil {
		return
	}
	anchor := outliner.captureModeCursor()
	outliner.RebuildRows()
	outliner.restoreModeCursor(anchor)
	outliner.ClampCursor()
}

// RefreshOutlinerPreservingCursor is RebuildOutlinerPreservingCursor for a
// caller that has changed no task since the last rebuild. The memo guard then
// decides, so the rows are rebuilt only when a view input it reads — planning,
// filters, folds, scope, the loaded set — has moved since, and a refresh that
// would reproduce the rows already built costs nothing. A single-project
// Outliner keeps no such guard and rebuilds as before.
func RefreshOutlinerPreservingCursor(outliner *OutlinerModel) {
	if outliner == nil {
		return
	}
	if outliner.Project != nil {
		RebuildOutlinerPreservingCursor(outliner)
		return
	}
	anchor := outliner.captureModeCursor()
	outliner.CrossProjectRebuildRows()
	outliner.restoreModeCursor(anchor)
	outliner.ClampCursor()
}

// cursorTaskIdentity names the task the cursor rests on, reading a checklist row

// cursorProject resolves the project the Outliner cursor sits inside, whatever
// row kind it rests on. Task rows and project headers both name their project
// directly; a scoped Outliner has only one project to name. `O` reads it to
// decide which project the outline editor dumps.
func (m *OutlinerModel) CursorProject() *models.Project {
	if m == nil {
		return nil
	}
	if m.Cursor >= 0 && m.Cursor < len(m.Rows) {
		if projectID := m.Rows[m.Cursor].ProjectID; projectID != "" {
			if m.Project != nil && m.Project.ID == projectID {
				return m.Project
			}
			if project := m.crossResolveProject(projectID); project != nil {
				return project
			}
		}
	}
	return m.Project
}
