package outliner

import (
	"fmt"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
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

// VisibleSections is the s-side counterpart of VisibleProjectGroups: the
// distinct section collapse keys the current projection holds, in
// first-appearance order. It reads the section half every row carries rather
// than scanning for header rows, so the two bulk folds are symmetric and neither
// depends on a heading being drawn. A projection whose mode emits no sections
// carries an empty section half on every row and returns none, so s stays
// unavailable there rather than toggling nothing.
func (m *OutlinerModel) VisibleSections() []string {
	keys := make([]string, 0)
	seen := make(map[string]bool)
	for _, row := range m.Rows {
		key := row.Group.SectionKey
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys
}

// VisibleProjectGroups is the p-side counterpart to visibleSectionHeaders: the
// distinct project groups the current projection holds, in first-appearance
// order. It reads the structural identity every member row carries rather than
// scanning for header rows, which is what lets one helper serve the five
// project-group namespaces across all seven modes and keeps a group nameable
// whether or not its header is drawn. A projection with no project groups — both
// single-project forms — returns none, so p stays unavailable there rather than
// toggling nothing.
func (m *OutlinerModel) VisibleProjectGroups() []OutlinerRowGroup {
	groups := make([]OutlinerRowGroup, 0)
	seen := make(map[OutlinerRowGroup]bool)
	for _, row := range m.Rows {
		if !row.Group.ProjectGrouped() || seen[row.Group] {
			continue
		}
		seen[row.Group] = true
		groups = append(groups, row.Group)
	}
	return groups
}

// topLevelSectionAtCursor names the visible section that owns the cursor's
// current row. Membership is the group the row carries, so a row after Pinned
// that belongs to the mode below it is never read as Pinned content and no lane
// marker has to be consulted to say so. A checklist row carries its owner's
// group, so it answers the same membership question its owner does.
//
// It returns the key rather than the header row because the key is the whole of
// what a caller needs: the fold to write and the group to seat the cursor in are
// both spelled by it, and looking a header up would make the answer depend on a
// heading this projection may not draw.
func (m *OutlinerModel) topLevelSectionAtCursor(sectionKeys map[string]bool) (string, bool) {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return "", false
	}
	key := m.Rows[m.Cursor].Group.SectionKey
	if key == "" || !sectionKeys[key] {
		return "", false
	}
	return key, true
}

// moveCursorToGroupKey parks the cursor on the first row belonging to the group
// the key names, reporting whether one was found. A group's header is emitted
// above its members and carries the same identity, so this lands on the header
// wherever one is drawn and on the group's first task where none is.
//
// An empty key is refused rather than matched: it is not a namespace, and the
// focused-project header carries no group precisely so no unmatched seek can
// land on it.
func (m *OutlinerModel) moveCursorToGroupKey(key string) bool {
	if m == nil || key == "" {
		return false
	}
	for i, row := range m.Rows {
		if slices.Contains(row.Group.CollapseKeys(), key) {
			m.Cursor = i
			return true
		}
	}
	return false
}

// moveCursorToGroupTask parks the cursor on the group's first task row, which is
// where a group lands a cursor that has no better claim on a row inside it. It
// skips the group's own header because that row is what the cursor is leaving:
// an expansion whose whole point is to reach the work would otherwise stop on the
// heading it just opened.
func (m *OutlinerModel) moveCursorToGroupTask(group OutlinerRowGroup) bool {
	if m == nil || group == (OutlinerRowGroup{}) {
		return false
	}
	for i, row := range m.Rows {
		if row.Group == group && row.IsTaskRow() && row.Task != nil {
			m.Cursor = i
			return true
		}
	}
	return false
}

// restoreGroupCursor seats the cursor inside a group that has just been
// expanded: on the task the fold was covering when it still has a row in that
// same group, otherwise on the group's first task. The remembered identity is
// checked against the group's own rows rather than against the whole projection,
// so a memory left behind by a restored checkpoint can only be ignored, never
// followed somewhere else.
func (m *OutlinerModel) restoreGroupCursor(group OutlinerRowGroup, remembered taskref.TaskIdentity, ok bool) bool {
	if m == nil {
		return false
	}
	if ok {
		for i, row := range m.Rows {
			if row.Group != group || !row.IsTaskRow() || row.Task == nil {
				continue
			}
			if m.TaskIdentityForRow(row) == remembered {
				m.Cursor = i
				return true
			}
		}
	}
	return m.moveCursorToGroupTask(group)
}

// rememberCursorGroupFocus records the task the cursor is standing on against
// its own project group, so a fold that is about to hide it can be undone onto
// the same row. A cursor on a header or outside a project group records nothing,
// which is what makes the expansion fall back to the group's first task.
func (m *OutlinerModel) rememberCursorGroupFocus() {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return
	}
	row := m.Rows[m.Cursor]
	if !row.Group.ProjectGrouped() {
		return
	}
	identity, ok := m.cursorTaskIdentity()
	if !ok {
		return
	}
	m.Folds.rememberGroupFocus(row.Group.ProjectKey, identity)
}

func (m *OutlinerModel) restoreSectionToggleFocus() bool {
	anchor := m.Folds.takeSectionToggleAnchor()
	if anchor == nil {
		return false
	}
	if m.MoveCursorToIdentity(anchor.task) {
		return true
	}
	if !m.moveCursorToGroupKey(anchor.sectionKey) {
		m.ClampCursor()
	}
	return true
}

// clearAllFolds opens everything at once: every task subtree and every section
// and project-group key, across every namespace collapsedSections holds rather
// than only the ones this mode happens to draw. It is deliberately one-way,
// which is what the three toggles cannot be — `s`, `p`, and `t` each reverse, so
// reaching a fully open tree from a mixed state takes three presses that may
// each go the wrong way, and no fourth toggle would fix that.
//
// It clears the whole map rather than the visible headers because a fold under a
// fold is invisible by definition, and "all" that stopped at the first closed
// header would leave a user pressing it twice. The keys are namespaced per
// grouping surface and survive a mode change, so this is also the one act that
// ends a fold another mode wrote; that is the point of it rather than a leak.
//
// Checklists are outside it for the reason they are outside `t`: they default
// collapsed, so opening them is a different promise than undoing a fold.
func (m *OutlinerModel) ClearAllFolds() bool {
	if m == nil {
		return false
	}
	changed := m.setAllTaskFolds(true)
	for _, collapsed := range m.Folds.CollapsedSections {
		if collapsed {
			changed = true
			break
		}
	}
	if !changed {
		return false
	}
	m.Folds.clearSections()
	anchor := m.captureModeCursor()
	m.RebuildRows()
	m.restoreModeCursor(anchor)
	return true
}

// setGroupFold is the single write of a grouping fold, whether a header row or a
// group identity named it, so it is also the single place an empty key is
// refused. An empty key is not a namespace: writing it would make one nameless
// fold that every keyless header and every ungrouped projection would share.
//
// It is also where the group's remembered cursor is dropped, so that memory lasts
// exactly as long as the fold that needs it: opening a group forgets the task its
// summary was covering, and a later close writes the task it is covering then.
func (m *OutlinerModel) setGroupFold(key string, collapsed bool) {
	if m == nil || key == "" {
		return
	}
	m.Folds.setSection(key, collapsed)
	if !collapsed {
		m.Folds.forgetGroupFocus(key)
	}
}

// setSectionRowCollapsed is the header-row spelling of that write, so it is also
// where the one header that owns no fold is refused. The focused-project
// header's EffectiveCollapseKey is empty, and setGroupFold would refuse it
// anyway; refusing the row kind here keeps the reason readable at the call site.
func (m *OutlinerModel) SetSectionRowCollapsed(row OutlinerRow, collapsed bool) {
	if !row.IsCollapsibleHeader() {
		return
	}
	m.setGroupFold(row.EffectiveCollapseKey(), collapsed)
}

// SetProjectGroupFolded writes one project group's fold from its structural
// identity, which is how the p axis reaches a group without holding its header
// row. A group carrying no project half owns no fold and is refused.
func (m *OutlinerModel) SetProjectGroupFolded(group OutlinerRowGroup, collapsed bool) {
	m.setGroupFold(group.ProjectKey, collapsed)
}

// IsProjectGroupFolded reports whether the group's own fold is closed, reading
// the same zoom exemption every other fold read goes through.
func (m *OutlinerModel) IsProjectGroupFolded(group OutlinerRowGroup) bool {
	return m.sectionCollapsed(group.ProjectKey)
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

// collapseOrMoveToParent gives h/Left one deterministic tree behavior: collapse
// an expanded section, project header, checklist, or task first; otherwise move
// to the visible parent task. A checklist row is one rung further down, so it
// returns to its owner and leaves the collapse to the next press. From a direct
// child of a focus root the parent has no row, so the ladder pops that one
// frame to reach it.
//
// A task that has run out of hierarchy — no open checklist, no open children, and
// no parent at all — folds its own project group as the last rung, which is what
// keeps one group reachable from inside it when the projection draws no heading
// above it. The rung is the group's, not the section's: a section always draws a
// header of its own, so h on a root task in a single-project form still says
// nothing. A parent that exists but is hidden by narrowing is a hierarchy state
// rather than a group boundary, so it keeps its silence instead of folding.
func (m *OutlinerModel) CollapseOrMoveToParent() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	row := m.Rows[m.Cursor]
	if row.IsHeaderRow() {
		// h says nothing on the focused-project header: it names the projection the
		// cursor is already inside, so the only outline it could fold is the whole
		// one.
		if !row.IsCollapsibleHeader() || m.IsZoomedSectionRow(row) || m.IsSectionRowCollapsed(row) {
			return false
		}
		m.SetSectionRowCollapsed(row, true)
		m.RebuildRows()
		return true
	}
	if row.IsChecklistItem {
		return m.MoveCursorToIdentity(m.TaskIdentityForRow(row))
	}
	if !row.IsTaskRow() || row.Task == nil {
		return false
	}

	identity := m.TaskIdentityForRow(row)
	if m.IsChecklistExpanded(identity) {
		m.Folds.SetChecklist(identity, false)
		m.RebuildRows()
		m.MoveCursorToIdentity(identity)
		return true
	}
	if row.HasVisibleChildren && m.IsExpanded(identity) {
		m.ensureExpandedNodes()
		m.Folds.setNode(identity, false)
		m.RebuildRows()
		m.MoveCursorToIdentity(identity)
		return true
	}
	parentID := parentIDOf(row.Task)
	if parentID == "" {
		return m.foldGroupAtCursor()
	}
	parent := taskref.TaskIdentity{ProjectID: identity.ProjectID, TaskID: parentID}
	if m.MoveCursorToIdentity(parent) {
		return true
	}
	// The focus root is the one parent that is deliberately rowless, so h leaves
	// exactly that frame to reach it. A promoted row whose parent is merely
	// filtered out fails the comparison and stays silent.
	if root, focused := m.FocusRootIdentity(); focused && root == parent {
		m.PopFocusLevel()
		return m.MoveCursorToIdentity(parent)
	}
	return false
}

// foldGroupAtCursor closes the project group the cursor's row belongs to and
// leaves the cursor on the summary that replaces it, remembering the task it was
// standing on so the matching expansion can put it back. It is the one fold a
// task row can write, and it reads the row's own group rather than searching
// upward for a heading, so it answers the same way whether or not one is drawn. A
// row outside a project group owns no such fold and refuses.
func (m *OutlinerModel) foldGroupAtCursor() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	group := m.Rows[m.Cursor].Group
	if !group.ProjectGrouped() || m.IsProjectGroupFolded(group) {
		return false
	}
	m.rememberCursorGroupFocus()
	m.SetProjectGroupFolded(group, true)
	m.RebuildRows()
	if !m.moveCursorToGroupKey(group.ProjectKey) {
		m.ClampCursor()
	}
	return true
}

// expandAtCursor gives l/Right one behavior: expand the current collapsed
// section, project header, task, or checklist. Children come before the
// checklist, which is the exact reverse of the order h collapses them in. A
// checklist row has nothing left to expand, so it is a no-op there.
// Indent/outdent remain legacy-only until they move into the action sheet.
//
// Expanding a project group is the one expansion that also moves the cursor: the
// summary it opens is a stand-in for the group's rows, so l lands on the task the
// matching fold was covering, or on the group's first task when there is none to
// return to. A section keeps its own header once expanded, so the cursor stays on
// it there.
func (m *OutlinerModel) ExpandAtCursor() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	row := m.Rows[m.Cursor]
	if row.IsHeaderRow() {
		// l is silent on the focused-project header for the same reason h is: it
		// carries no collapse state either way.
		if !row.IsCollapsibleHeader() || !m.IsSectionRowCollapsed(row) {
			return false
		}
		remembered, hasRemembered := m.Folds.groupFocus(row.Group.ProjectKey)
		m.SetSectionRowCollapsed(row, false)
		m.RebuildRows()
		if row.Group.ProjectGrouped() && !m.restoreGroupCursor(row.Group, remembered, hasRemembered) {
			m.ClampCursor()
		}
		return true
	}
	if !row.IsTaskRow() || row.Task == nil {
		return false
	}
	identity := m.TaskIdentityForRow(row)
	if row.HasVisibleChildren && !m.IsExpanded(identity) {
		m.ensureExpandedNodes()
		m.Folds.setNode(identity, true)
		m.RebuildRows()
		m.MoveCursorToIdentity(identity)
		return true
	}
	if len(row.Task.Checklist) == 0 || m.IsChecklistExpanded(identity) {
		return false
	}
	m.Folds.SetChecklist(identity, true)
	m.RebuildRows()
	m.MoveCursorToIdentity(identity)
	return true
}

// rowStartsGroup reports whether the row at index is where a block begins, which
// is what n and N step between. A drawn heading is one: a section header and a
// project header — whether it is an expanded group's heading or a folded group's
// summary — both open the block they name. So is the first row of a project group
// that draws no heading at all, because the block still starts there and a key
// that skipped it would leave a whole group unreachable.
//
// Membership is read off the rows' own groups rather than off the headings above
// them, so the three cases are one rule: index starts a block when it carries a
// project group its predecessor did not. A separator, a note row, and a
// focused-project header all carry the zero group, so none of them turns the row
// after it into a second start of the same block.
func (m *OutlinerModel) rowStartsGroup(index int) bool {
	if m == nil || index < 0 || index >= len(m.Rows) {
		return false
	}
	row := m.Rows[index]
	if row.IsHeaderRow() {
		return true
	}
	if !row.Group.ProjectGrouped() {
		return false
	}
	return index == 0 || m.Rows[index-1].Group != row.Group
}

// moveCursorToAdjacentHeader steps the cursor to the next (delta 1) or previous
// (delta -1) group start, wrapping at either end. It reports false only when the
// projection holds no group at all, which is what lets the caller say so instead
// of leaving the key silent. A projection with exactly one start parks the cursor
// there.
//
// n and N are user cursor navigation like j and k, so they void a pending
// relocation capture on the same terms: a wrap that leaves and returns to the
// settled row is numerically invisible to the reload guard, and parking on the
// only start moves nothing and voids nothing.
func (m *OutlinerModel) MoveCursorToAdjacentHeader(delta int) (tea.Cmd, bool) {
	if m == nil || len(m.Rows) == 0 {
		return nil, false
	}
	previous := m.Cursor
	for step := 1; step <= len(m.Rows); step++ {
		index := ((m.Cursor+delta*step)%len(m.Rows) + len(m.Rows)) % len(m.Rows)
		if !m.rowStartsGroup(index) {
			continue
		}
		m.Cursor = index
		m.UpdateRangeSelection()
		m.AdoptNavigatedCursor(previous)
		return listsurface.FlashIfJumped(previous, m.Cursor, &m.CursorFlash), true
	}
	return nil, false
}

// toggleVisibleSections is s: it expands every visible top-level section when
// any is collapsed; otherwise it collapses them all. Project group headers
// answer to p and task subtrees to t, so this bulk operation stays on the one
// axis its key names. A zoom leaves exactly one section visible and overrides
// its fold, so the bulk toggle refuses there instead of writing a fold nothing
// shows.
func (m *OutlinerModel) ToggleVisibleSections() bool {
	if m == nil || m.Focus.Zoom.Active() {
		return false
	}
	sections := m.VisibleSections()
	if len(sections) == 0 {
		return false
	}

	allExpanded := true
	sectionKeys := make(map[string]bool, len(sections))
	for _, key := range sections {
		sectionKeys[key] = true
		if m.sectionCollapsed(key) {
			allExpanded = false
		}
	}

	cursorAnchor := m.captureModeCursor()
	// A cursor already parked on a section header rides that section's own group
	// through the reshape, because the header is the only thing the press leaves
	// standing there. The key comes off the row's group rather than off the header
	// row, so the same capture serves a projection that draws no header for it.
	selectedSectionKey := ""
	if m.Cursor >= 0 && m.Cursor < len(m.Rows) && m.Rows[m.Cursor].IsSectionHeader {
		selectedSectionKey = m.Rows[m.Cursor].Group.SectionKey
	}
	if allExpanded {
		m.Folds.setSectionToggleAnchor(nil)
		if key, containsCursor := m.topLevelSectionAtCursor(sectionKeys); containsCursor {
			if row := m.Rows[m.Cursor]; row.IsTaskRow() && row.Task != nil {
				m.Folds.setSectionToggleAnchor(&sectionToggleRestoreAnchor{
					task:       m.TaskIdentityForRow(row),
					sectionKey: key,
				})
			}
			for _, section := range sections {
				m.setGroupFold(section, true)
			}
			m.RebuildRows()
			if !m.moveCursorToGroupKey(key) {
				m.ClampCursor()
			}
			return true
		}
		for _, section := range sections {
			m.setGroupFold(section, true)
		}
		m.RebuildRows()
		m.restoreModeCursor(cursorAnchor)
		return true
	}

	for _, section := range sections {
		m.setGroupFold(section, false)
	}
	m.RebuildRows()
	if m.restoreSectionToggleFocus() {
		return true
	}
	if selectedSectionKey != "" {
		if !m.moveCursorToGroupKey(selectedSectionKey) {
			m.ClampCursor()
		}
		return true
	}
	m.restoreModeCursor(cursorAnchor)
	return true
}

// toggleVisibleProjectGroups is p: it expands every visible project group when
// any is collapsed; otherwise it collapses them all. It is the third bulk fold
// beside s and t, and the only one that reaches the groups emitted in all seven
// modes — Today, Next, Inbox, and Backlog carry no section headers at all, so
// project groups are the whole structure there.
//
// It diverges from toggleVisibleSections twice, deliberately:
//
// No zoom refusal. sectionCollapsed exempts only m.focus.zoom.key, which is always a
// section key, and an admitted section still emits its project headers — so
// under a zoom p folds rows that are genuinely on screen. Refusing would be a
// lie.
//
// No restore anchor field. toggleVisibleSections carries
// sectionToggleRestoreAnchor because collapsing a section can hide a mode's worth
// of rows. A project fold hides one group, and the per-group memory every fold
// already writes is what puts that cursor back, so there is no second anchor.
func (m *OutlinerModel) ToggleVisibleProjectGroups() bool {
	if m == nil {
		return false
	}
	groups := m.VisibleProjectGroups()
	if len(groups) == 0 {
		return false
	}

	allExpanded := true
	for _, group := range groups {
		if m.IsProjectGroupFolded(group) {
			allExpanded = false
		}
	}

	cursorAnchor := m.captureModeCursor()
	// Same rule as s, one axis over, and read off the cursor row's own group rather
	// than off a header row: whatever the press leaves standing where the cursor was
	// belongs to that group, so the group is what the cursor rides through the
	// reshape. A collapse remembers the task it is hiding first, so the expansion
	// that follows can return to it rather than to whichever row leads the group.
	cursorGroup := OutlinerRowGroup{}
	if m.Cursor >= 0 && m.Cursor < len(m.Rows) {
		cursorGroup = m.Rows[m.Cursor].Group
	}
	cursorTask, hasCursorTask := m.cursorTaskIdentity()
	if allExpanded {
		m.rememberCursorGroupFocus()
	}
	remembered, hasRemembered := m.Folds.groupFocus(cursorGroup.ProjectKey)
	for _, group := range groups {
		m.SetProjectGroupFolded(group, allExpanded)
	}
	m.RebuildRows()
	if cursorGroup.ProjectGrouped() {
		// An expansion keeps the row the cursor was on whenever that row survived,
		// because nothing about it moved out of view; only a collapse needs the
		// group's own landing.
		if !allExpanded && hasCursorTask && m.MoveCursorToIdentity(cursorTask) {
			return true
		}
		if m.restoreGroupCursor(cursorGroup, remembered, hasRemembered) {
			return true
		}
		if !m.moveCursorToGroupKey(cursorGroup.ProjectKey) {
			m.ClampCursor()
		}
		return true
	}
	m.restoreModeCursor(cursorAnchor)
	return true
}

// toggleAllSubtrees is t, and takes the same direction rule its two siblings
// do: any fold anywhere means the press unfolds. The two branches keep their own
// bodies rather than merging, because their cursor-restore policies genuinely
// differ — folding must ride the cursor up its ParentID chain and unfolding must
// not — so merging would only move the branch.
func (m *OutlinerModel) ToggleAllSubtrees() bool {
	if m == nil {
		return false
	}
	if m.AnyTaskFoldCollapsed() {
		return m.expandAllSubtrees()
	}
	return m.collapseAllSubtrees()
}

// collapseAllSubtrees folds every task subtree at once, the bulk counterpart to
// the single fold h walks. Sections and checklists are deliberately outside it:
// s owns the section axis, p the project axis, and a checklist already defaults
// collapsed, so one press means one thing here too. A zoom overrides a section's
// fold but leaves task folds alone, which is why this does not refuse there the
// way the section toggle does.
func (m *OutlinerModel) collapseAllSubtrees() bool {
	if m == nil || !m.setAllTaskFolds(false) {
		return false
	}
	anchor, anchored := m.cursorTaskIdentity()
	parents := m.cursorAncestorIdentities()
	m.RebuildRows()
	if !anchored || !m.MoveCursorToIdentity(anchor) {
		// The cursor's own row just went under a fold, so it rides up to the
		// nearest ancestor that still has one.
		m.moveCursorToNearestIdentity(parents)
	}
	return true
}

// expandAllSubtrees unfolds every task subtree. The cursor's row was visible
// before and stays visible, so it only needs putting back after the rebuild.
func (m *OutlinerModel) expandAllSubtrees() bool {
	if m == nil || !m.setAllTaskFolds(true) {
		return false
	}
	anchor, anchored := m.cursorTaskIdentity()
	m.RebuildRows()
	if anchored {
		m.MoveCursorToIdentity(anchor)
	}
	return true
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
