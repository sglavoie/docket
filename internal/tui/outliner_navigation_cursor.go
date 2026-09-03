package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// through its owner so a fold started from one lands on the task it belongs to.
func (m *OutlinerModel) cursorTaskIdentity() (taskref.TaskIdentity, bool) {
	if m == nil || m.cursor < 0 || m.cursor >= len(m.rows) {
		return taskref.TaskIdentity{}, false
	}
	row := m.rows[m.cursor]
	if (!row.isTaskRow() && !row.IsChecklistItem) || row.task == nil {
		return taskref.TaskIdentity{}, false
	}
	return m.taskIdentityForRow(row), true
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
	project := m.resolveProjectForRow(m.rows[m.cursor])
	if project == nil {
		return nil
	}
	var ancestors []taskref.TaskIdentity
	parentID := parentIDOf(m.rows[m.cursor].task)
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
		if m.moveCursorToIdentity(candidate) {
			return
		}
	}
	m.clampCursor()
}

func (m *OutlinerModel) visibleSectionHeaders() []outlinerRow {
	headers := make([]outlinerRow, 0)
	for _, row := range m.rows {
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
func (m *OutlinerModel) visibleProjectGroupHeaders() []outlinerRow {
	headers := make([]outlinerRow, 0)
	for _, row := range m.rows {
		if row.IsProjectHeader {
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
func (m *OutlinerModel) topLevelSectionAtCursor(sectionKeys map[string]bool) (outlinerRow, bool) {
	if m == nil || m.cursor < 0 || m.cursor >= len(m.rows) {
		return outlinerRow{}, false
	}
	selected := m.rows[m.cursor]
	for i := m.cursor; i >= 0; i-- {
		header := m.rows[i]
		if !header.IsSectionHeader {
			continue
		}
		if !sectionKeys[header.effectiveCollapseKey()] {
			return outlinerRow{}, false
		}
		if selected.isTaskRow() || selected.IsChecklistItem {
			if header.IsPinnedSection && !selected.IsPinnedSection {
				return outlinerRow{}, false
			}
			if header.IsTodaySection && !selected.IsTodaySection {
				return outlinerRow{}, false
			}
			if header.IsNextSection && !selected.IsNextSection {
				return outlinerRow{}, false
			}
			if header.PromotedTag != "" && selected.PromotedTag != header.PromotedTag {
				return outlinerRow{}, false
			}
		}
		return header, true
	}
	return outlinerRow{}, false
}

// moveCursorToHeaderKey parks the cursor on the grouping header holding key.
// Section and project keys are disjoint by construction, so one lookup over
// both kinds serves the section toggle, the project toggle, and the zoom alike.
func (m *OutlinerModel) moveCursorToHeaderKey(key string) {
	for i, row := range m.rows {
		if row.isHeaderRow() && row.effectiveCollapseKey() == key {
			m.cursor = i
			return
		}
	}
	m.clampCursor()
}

func (m *OutlinerModel) restoreSectionToggleFocus() bool {
	anchor := m.sectionToggleRestore
	m.sectionToggleRestore = nil
	if anchor == nil {
		return false
	}
	if m.moveCursorToIdentity(anchor.task) {
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
func (m *OutlinerModel) clearAllFolds() bool {
	if m == nil {
		return false
	}
	changed := m.setAllTaskFolds(true)
	for _, collapsed := range m.collapsedSections {
		if collapsed {
			changed = true
			break
		}
	}
	if !changed {
		return false
	}
	m.collapsedSections = make(map[string]bool)
	m.sectionToggleRestore = nil
	anchor := m.captureModeCursor()
	m.rebuildRows()
	m.restoreModeCursor(anchor)
	return true
}

func (m *OutlinerModel) setSectionRowCollapsed(row outlinerRow, collapsed bool) {
	key := row.effectiveCollapseKey()
	if m.collapsedSections == nil {
		m.collapsedSections = make(map[string]bool)
	}
	m.collapsedSections[key] = collapsed
}

func (m *OutlinerModel) moveCursorToIdentity(identity taskref.TaskIdentity) bool {
	for i, row := range m.rows {
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		if m.taskIdentityForRow(row) == identity {
			m.cursor = i
			return true
		}
	}
	m.clampCursor()
	return false
}

// crossSkipNonNavigableRows walks the cursor off a header or other unselectable
// row in the direction the move was already going, falling back to the other
// direction at an edge. It passes cmd straight through so a caller can thread a
// navigation command across the skip; no caller has ever needed the model back,
// so it is not returned.
func (m *OutlinerModel) crossSkipNonNavigableRows(previous int, cmd tea.Cmd) tea.Cmd {
	if len(m.rows) == 0 {
		return cmd
	}
	if m.cursor < previous {
		for m.cursor > 0 && !m.rows[m.cursor].isNavigable() {
			m.cursor--
		}
		if !m.rows[m.cursor].isNavigable() {
			for m.cursor < len(m.rows)-1 && !m.rows[m.cursor].isNavigable() {
				m.cursor++
			}
		}
	} else {
		for m.cursor < len(m.rows)-1 && !m.rows[m.cursor].isNavigable() {
			m.cursor++
		}
		if !m.rows[m.cursor].isNavigable() {
			for m.cursor > 0 && !m.rows[m.cursor].isNavigable() {
				m.cursor--
			}
		}
	}
	return cmd
}

// simplifiedReorderEligible dispatches J/K by the selected row's ordering
// context. Pinned uses manual pin order wherever it renders; Due is automatic;
// Today and Next use durable planning order; All, Inbox, and Backlog use
// same-project sibling order.
func (m *OutlinerModel) simplifiedReorderEligible() bool {
	if m == nil || m.cursor < 0 || m.cursor >= len(m.rows) {
		return false
	}
	if m.multiSelectMode {
		return false
	}
	row := m.rows[m.cursor]
	if !row.isTaskRow() || row.task == nil {
		return false
	}
	// A pinned row carries its own manual order, so it answers before the active
	// mode's reorder policy.
	if m.isPinnedReorderRow() {
		return true
	}
	mode, ok := OutlinerModeByID(m.activeOutlinerMode())
	if !ok || mode.ReorderPolicy == ReorderPolicyNone {
		return false
	}
	if m.isPlanningReorderMode() {
		return true
	}
	switch mode.ID {
	case ModeAll, ModeInbox, ModeBacklog:
		return m.resolveProjectForRow(row) != nil
	default:
		return false
	}
}
