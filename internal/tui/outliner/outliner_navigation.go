package outliner

import (
	"fmt"

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
	m.SetTransientStatus("Task is unavailable in this project")
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

	if !m.MoveCursorToProjectTask(task.ID, projectID) {
		m.SetTransientStatus("Task is hidden by current filters")
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

func (m *OutlinerModel) VisibleSectionHeaders() []OutlinerRow {
	headers := make([]OutlinerRow, 0)
	for _, row := range m.Rows {
		if row.IsSectionHeader {
			headers = append(headers, row)
		}
	}
	return headers
}

// visibleProjectGroupHeaders is the p-side counterpart to
// visibleSectionHeaders. The two header kinds share collapsedSections and are
// told apart by scanned row kind rather than by a key prefix, which is what lets
// one helper serve the five project-header namespaces across all seven modes.
func (m *OutlinerModel) VisibleProjectGroupHeaders() []OutlinerRow {
	headers := make([]OutlinerRow, 0)
	for _, row := range m.Rows {
		// The focused-project header is a project header without a fold, so p
		// passes it over: in a focused projection it is the only one, and p stays
		// unavailable there rather than toggling nothing.
		if row.IsProjectHeader && row.IsCollapsibleHeader() {
			headers = append(headers, row)
		}
	}
	return headers
}

// topLevelSectionAtCursor returns the visible section that owns the cursor's
// current row. Pinned and Today task rows have explicit membership markers;
// those avoid treating the ungrouped Inbox/Next/Backlog rows after Pinned as
// Pinned content. A checklist row carries its owner's markers, so it answers
// the same membership question its owner does.
func (m *OutlinerModel) topLevelSectionAtCursor(sectionKeys map[string]bool) (OutlinerRow, bool) {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return OutlinerRow{}, false
	}
	selected := m.Rows[m.Cursor]
	for i := m.Cursor; i >= 0; i-- {
		header := m.Rows[i]
		if !header.IsSectionHeader {
			continue
		}
		if !sectionKeys[header.EffectiveCollapseKey()] {
			return OutlinerRow{}, false
		}
		if selected.IsTaskRow() || selected.IsChecklistItem {
			if header.IsPinnedSection && !selected.IsPinnedSection {
				return OutlinerRow{}, false
			}
			if header.IsTodaySection && !selected.IsTodaySection {
				return OutlinerRow{}, false
			}
			if header.IsNextSection && !selected.IsNextSection {
				return OutlinerRow{}, false
			}
			if header.PromotedTag != "" && selected.PromotedTag != header.PromotedTag {
				return OutlinerRow{}, false
			}
		}
		return header, true
	}
	return OutlinerRow{}, false
}

// moveCursorToHeaderKey parks the cursor on the grouping header holding key.
// Section and project keys are disjoint by construction, so one lookup over
// both kinds serves the section toggle, the project toggle, and the zoom alike.
func (m *OutlinerModel) moveCursorToHeaderKey(key string) {
	for i, row := range m.Rows {
		// Only a header that owns a fold can be named by one: the focused-project
		// header's key is empty, and an empty key must not become a seat every
		// unmatched seek lands on.
		if row.IsCollapsibleHeader() && row.EffectiveCollapseKey() == key {
			m.Cursor = i
			return
		}
	}
	m.ClampCursor()
}

func (m *OutlinerModel) restoreSectionToggleFocus() bool {
	anchor := m.Folds.takeSectionToggleAnchor()
	if anchor == nil {
		return false
	}
	if m.MoveCursorToIdentity(anchor.task) {
		return true
	}
	m.moveCursorToHeaderKey(anchor.sectionKey)
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

// setSectionRowCollapsed is the single write of a header row's fold, so it is
// also the single place the one header that owns no fold is refused. The
// focused-project header's EffectiveCollapseKey is empty, and an empty key is
// not a namespace: writing it would make one nameless fold that every future
// keyless header would share.
func (m *OutlinerModel) SetSectionRowCollapsed(row OutlinerRow, collapsed bool) {
	if !row.IsCollapsibleHeader() {
		return
	}
	m.Folds.setSection(row.EffectiveCollapseKey(), collapsed)
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
		return false
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

// expandAtCursor gives l/Right one behavior: expand the current collapsed
// section, project header, task, or checklist. Children come before the
// checklist, which is the exact reverse of the order h collapses them in. A
// checklist row has nothing left to expand, so it is a no-op there.
// Indent/outdent remain legacy-only until they move into the action sheet.
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
		m.SetSectionRowCollapsed(row, false)
		m.RebuildRows()
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

// moveCursorToAdjacentHeader steps the cursor to the next (delta 1) or previous
// (delta -1) grouping header, wrapping at either end. It reports false only when
// the projection holds no header at all, which is what lets the caller say so
// instead of leaving the key silent. A projection with exactly one header parks
// the cursor there.
//
// n and N are user cursor navigation like j and k, so they void a pending
// relocation capture on the same terms: a wrap that leaves and returns to the
// settled row is numerically invisible to the reload guard, and parking on the
// only header moves nothing and voids nothing.
func (m *OutlinerModel) MoveCursorToAdjacentHeader(delta int) (tea.Cmd, bool) {
	if m == nil || len(m.Rows) == 0 {
		return nil, false
	}
	previous := m.Cursor
	for step := 1; step <= len(m.Rows); step++ {
		index := ((m.Cursor+delta*step)%len(m.Rows) + len(m.Rows)) % len(m.Rows)
		if !m.Rows[index].IsHeaderRow() {
			continue
		}
		m.Cursor = index
		m.UpdateRangeSelection()
		m.VoidRelocationCaptureAfterNavigation(previous)
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
	headers := m.VisibleSectionHeaders()
	if len(headers) == 0 {
		return false
	}

	allExpanded := true
	headerKeys := make(map[string]bool, len(headers))
	for _, header := range headers {
		key := header.EffectiveCollapseKey()
		headerKeys[key] = true
		if m.IsSectionRowCollapsed(header) {
			allExpanded = false
		}
	}

	cursorAnchor := m.captureModeCursor()
	selectedHeaderKey := ""
	if m.Cursor >= 0 && m.Cursor < len(m.Rows) && m.Rows[m.Cursor].IsSectionHeader {
		selectedHeaderKey = m.Rows[m.Cursor].EffectiveCollapseKey()
	}
	if allExpanded {
		m.Folds.setSectionToggleAnchor(nil)
		if header, containsCursor := m.topLevelSectionAtCursor(headerKeys); containsCursor {
			if row := m.Rows[m.Cursor]; row.IsTaskRow() && row.Task != nil {
				m.Folds.setSectionToggleAnchor(&sectionToggleRestoreAnchor{
					task:       m.TaskIdentityForRow(row),
					sectionKey: header.EffectiveCollapseKey(),
				})
			}
			for _, section := range headers {
				m.SetSectionRowCollapsed(section, true)
			}
			m.RebuildRows()
			m.moveCursorToHeaderKey(header.EffectiveCollapseKey())
			return true
		}
		for _, section := range headers {
			m.SetSectionRowCollapsed(section, true)
		}
		m.RebuildRows()
		m.restoreModeCursor(cursorAnchor)
		return true
	}

	for _, section := range headers {
		m.SetSectionRowCollapsed(section, false)
	}
	m.RebuildRows()
	if m.restoreSectionToggleFocus() {
		return true
	}
	if selectedHeaderKey != "" {
		m.moveCursorToHeaderKey(selectedHeaderKey)
		return true
	}
	m.restoreModeCursor(cursorAnchor)
	return true
}

// toggleVisibleProjectGroups is p: it expands every visible project group header
// when any is collapsed; otherwise it collapses them all. It is the third bulk
// fold beside s and t, and the only one that reaches the header
// appendProjectGroupHeader emits in all seven modes — Today, Next, Inbox, and
// Backlog carry no section headers at all, so project groups are the whole
// structure there.
//
// It diverges from toggleVisibleSections twice, deliberately:
//
// No zoom refusal. sectionCollapsed exempts only m.focus.zoom.key, which is always a
// section key, and an admitted section still emits its project headers — so
// under a zoom p folds rows that are genuinely on screen. Refusing would be a
// lie.
//
// No restore anchor. toggleVisibleSections carries sectionToggleRestoreAnchor
// because collapsing a section can hide a mode's worth of rows. A project fold
// hides one group, and restoreModeCursor already handles that displacement, so
// there is no second anchor field.
func (m *OutlinerModel) ToggleVisibleProjectGroups() bool {
	if m == nil {
		return false
	}
	headers := m.VisibleProjectGroupHeaders()
	if len(headers) == 0 {
		return false
	}

	allExpanded := true
	for _, header := range headers {
		if m.IsSectionRowCollapsed(header) {
			allExpanded = false
		}
	}

	cursorAnchor := m.captureModeCursor()
	selectedHeaderKey := ""
	if m.Cursor >= 0 && m.Cursor < len(m.Rows) && m.Rows[m.Cursor].IsProjectHeader {
		selectedHeaderKey = m.Rows[m.Cursor].EffectiveCollapseKey()
	}
	for _, header := range headers {
		m.SetSectionRowCollapsed(header, allExpanded)
	}
	m.RebuildRows()
	if selectedHeaderKey != "" {
		m.moveCursorToHeaderKey(selectedHeaderKey)
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
