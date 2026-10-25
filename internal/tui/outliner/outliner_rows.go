package outliner

import (
	"strings"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/notes"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// Tree connector characters for outliner rendering.
const (
	connectorBranch   = "├── "
	connectorLastItem = "└── "
	connectorPipe     = "│   "
	connectorSpace    = "    "
)

// OutlinerRowGroup is the structural identity of one block of a grouped
// projection: the section it belongs to and the project group inside that
// section. Both halves are the collapse keys the fold state already stores, so a
// group names exactly the folds that can hide it and mints no namespace of its
// own; the rendered row index is never part of it.
//
// It is stamped on a group's header rows and on every member row alike, which is
// what lets group discovery, membership, and boundaries be read off the row the
// cursor is on rather than reconstructed by scanning upward for a header. A
// projection that draws no project headers — either single-project form — is not
// project-grouped, so its section alone is the group and both project halves stay
// empty. A row nothing can fold carries the zero value.
type OutlinerRowGroup struct {
	SectionKey string // Collapse key of the owning section; empty when the projection emits none
	ProjectKey string // Collapse key of the project group; empty when the projection is not project-grouped
	ProjectID  string // Project that owns the group; empty alongside ProjectKey
}

// ProjectGrouped reports whether the row sits inside a foldable project group.
// It is the one read that separates a project-grouped projection from the two
// single-project forms, so discovery and the fold write cannot disagree about
// which groups own a key.
func (g OutlinerRowGroup) ProjectGrouped() bool {
	return g.ProjectKey != ""
}

// CollapseKeys names the folds that hide the group, nearest first. A reveal
// walks it instead of scanning for the two headers above a row, so it stays
// correct for a group that draws no header at all.
func (g OutlinerRowGroup) CollapseKeys() []string {
	keys := make([]string, 0, 2)
	if g.ProjectKey != "" {
		keys = append(keys, g.ProjectKey)
	}
	if g.SectionKey != "" {
		keys = append(keys, g.SectionKey)
	}
	return keys
}

// outlinerRow represents a single flattened row for rendering.
type OutlinerRow struct {
	Task                   *models.Task
	Group                  OutlinerRowGroup // Structural group this row belongs to; zero when nothing can fold it
	Prefix                 string
	Breadcrumb             string                    // Ancestor path for tasks promoted across a section boundary.
	HasVisibleChildren     bool                      // Children rendered in this row's section-local tree.
	IsSectionHeader        bool                      // True if this row is a section header (not a task)
	IsComputedHeader       bool                      // True for a computed-mode grouping header such as a due-date bucket
	IsProjectHeader        bool                      // True if this row is a synthetic, cursorable project sub-header (not a task)
	IsFocusedProjectHeader bool                      // True for the one header a focused projection leads with: project identity, no fold
	LeadsMergedGroup       bool                      // True for the one top-level task an expanded singleton project group draws in place of its header
	MergedLead             *models.Task              // The LeadsMergedGroup task this row descends from, whose label its tree indents past
	IsPinnedSection        bool                      // True for the synthetic Pinned header and every promoted task row beneath it
	IsTodaySection         bool                      // True for the synthetic Today header and every explicit Today task row beneath it
	IsNextSection          bool                      // True for the synthetic Next header and every explicit Next task row beneath it
	PromotedTag            string                    // Canonical promoted workflow tag for synthetic header/project/task rows
	SectionName            string                    // Section name for header rows
	SectionCount           int                       // Number of entries in this section
	SectionLiftedCount     int                       // Members drawn in a promoted lane above instead (All only)
	StateGroup             models.WorkflowStateGroup // Canonical state group for workflow headers; empty for unrelated/computed headers
	CollapseKey            string                    // Stable identity for shared or project-owned grouping headers
	ProjectID              string                    // Project that owns task/project-header rows; empty for shared section headers
	IsSeparator            bool                      // True if this row is a non-navigable horizontal rule between blocks
	OverdueCount           int                       // Overdue tasks inside the group a project header names
	TodayCount             int                       // Today-planned tasks inside the group a project header names
	IsChecklistItem        bool                      // True for one expanded checklist item beneath its owner task row
	ChecklistIndex         int                       // Position of the item inside the owner task's checklist
	ChecklistDone          bool                      // Ticked state of the item this row names
	MatchSource            MatchSource               // Field that produced this search hit; matchSourceNone on every ordinary row
	IsNoteRow              bool                      // True for a project note or a note search result
	NoteRow                notes.NoteRowIdentity     // Workspace identity of the note an IsNoteRow row names
}

// isTaskRow reports whether the row carries a task payload (i.e. it is not a
// section header, project sub-header, separator, checklist item, or note). Use
// this in place of the historical `!IsSectionHeader && !IsSeparator` pattern so
// that newly introduced row kinds (e.g. project sub-headers) are consistently
// excluded from task-only logic. A checklist row carries its owner task pointer
// for rendering, so excluding it here is what stops every task operation from
// silently falling through to that owner. A note row carries no task at all, so
// excluding it here is what makes every task-only guard refuse it by row kind
// rather than by a nil check the guard may not have.
func (r OutlinerRow) IsTaskRow() bool {
	return !r.IsSectionHeader && !r.IsProjectHeader && !r.IsSeparator &&
		!r.IsChecklistItem && !r.IsNoteRow
}

// isNavigable reports whether the cursor is allowed to rest on this row.
// Section headers, project sub-headers, note rows, and task rows are all
// navigable, and h/l fold the group a header names. Only separators (purely
// decorative) are skipped by cursor movement and rebuild/clamp paths. A note
// row is the first kind that is navigable while not being a task row: the
// cursor must reach a search result to open it, and every task operation must
// still refuse it. A header cursor names no task either, so every task
// operation refuses it rather than falling back to a row the user is not
// pointing at.
func (r OutlinerRow) IsNavigable() bool {
	return !r.IsSeparator
}

// isHeaderRow reports whether the row is one of the two cursorable grouping
// headers. Both fold under h/l and both are what n/N step between. Section
// headers alone would leave n/N dead in the four modes that group by project
// only.
func (r OutlinerRow) IsHeaderRow() bool {
	return r.IsSectionHeader || r.IsProjectHeader
}

// isCollapsibleHeader reports whether a header row owns a fold. Every header
// does except the focused-project header, which carries project identity and no
// collapse state at all: it names the projection the user is already inside, so
// folding it would fold the whole outline out from under them. It is the one
// read that separates the two, so h/l, the p toggle, and the fold-restoring
// walk cannot disagree about which headers have a key. Its EffectiveCollapseKey
// is empty by construction, and nothing may write that key.
func (r OutlinerRow) IsCollapsibleHeader() bool {
	return r.IsHeaderRow() && !r.IsFocusedProjectHeader
}

// collapseKey returns the effective key for collapsedSections lookups.
// Cross-project headers set CollapseKey to "projectID:sectionName"; single-project
// headers leave it empty and fall back to SectionName.
func (r OutlinerRow) EffectiveCollapseKey() string {
	if r.CollapseKey != "" {
		return r.CollapseKey
	}
	return r.SectionName
}

// isSectionRowCollapsed reports whether a section-header row is collapsed.
func (m *OutlinerModel) IsSectionRowCollapsed(row OutlinerRow) bool {
	return m.sectionCollapsed(row.EffectiveCollapseKey())
}

// sectionCollapsed is the one read of persistent fold state. A zoomed section
// ignores its own fold so it can be zoomed without mutating the fold the user
// set, and without the zoom having to write and restore it.
func (m *OutlinerModel) sectionCollapsed(key string) bool {
	return m.Folds.CollapsedSections[key] && key != m.Focus.Zoom.key
}

// sectionZoomAdmits reports whether a section may emit rows at all.
func (m *OutlinerModel) sectionZoomAdmits(key string) bool {
	return !m.Focus.Zoom.Active() || m.Focus.Zoom.key == key
}

// cursorNamesSectionHeader is the one read of the cursor's row kind, shared by
// the Outliner's own command context and the shell's, so the key that dispatches
// and the footer that advertises it can never disagree.
func (m *OutlinerModel) CursorNamesSectionHeader() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	return m.Rows[m.Cursor].IsSectionHeader
}

// cursorNoteIdentity names the note the cursor rests on, and is the second read
// of the cursor's row kind both command contexts share. A note row carries no
// project or task, so this identity is the only thing an operation acting on it
// can resolve a target from.
func (m *OutlinerModel) CursorNoteIdentity() (notes.NoteRowIdentity, bool) {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return notes.NoteRowIdentity{}, false
	}
	row := m.Rows[m.Cursor]
	if !row.IsNoteRow || row.NoteRow.Filename == "" {
		return notes.NoteRowIdentity{}, false
	}
	return row.NoteRow, true
}

// isZoomedSectionRow names the header of the section the projection is zoomed
// to. The zoom overrides that section's fold, so folding it is refused rather
// than written: l already refuses because the section always reads expanded, and
// h and s would otherwise persist a fold with no visible effect that Esc then
// reveals.
func (m *OutlinerModel) IsZoomedSectionRow(row OutlinerRow) bool {
	return m.Focus.Zoom.Active() && row.IsSectionHeader && row.EffectiveCollapseKey() == m.Focus.Zoom.key
}

// buildTreePrefix computes the connector prefix string from the ancestor
// is-last-child boolean stack. Each true entry means that ancestor was the
// last child of its parent (so we render spaces), while false means we need
// a continuation pipe.
func BuildTreePrefix(ancestorIsLast []bool, isLast bool) string {
	var b strings.Builder
	for _, last := range ancestorIsLast {
		if last {
			b.WriteString(connectorSpace)
		} else {
			b.WriteString(connectorPipe)
		}
	}
	if isLast {
		b.WriteString(connectorLastItem)
	} else {
		b.WriteString(connectorBranch)
	}
	return b.String()
}

// flattenTaskTreeWithFolds recursively walks the task tree and produces a flat
// list of outlinerRows, respecting fold state. Collapsed nodes (not in
// expandedNodes or explicitly false) stop recursion, showing only the parent.
// When expandedNodes is empty (default), all nodes are expanded.
// isExpanded returns true if the node should show its children.
// When the expandedNodes map is empty (default), all nodes are expanded.
func (m *OutlinerModel) IsExpanded(identity taskref.TaskIdentity) bool {
	if len(m.Folds.ExpandedNodes) == 0 {
		return true
	}
	expanded, exists := m.Folds.ExpandedNodes[identity]
	if !exists {
		return true // Default to expanded
	}
	return expanded
}

// checklistCursor names the checklist item the cursor rests on, together with
// the task and project that own it. The owner travels with the index because a
// checklist item has no identity of its own: it is addressed as a position
// inside one task.
type ChecklistCursor struct {
	Task    *models.Task
	Project *models.Project
	Index   int
}

// cursorChecklistItem reports the checklist row under the cursor. A checklist
// row carries its owner's task pointer, so callers that only ask for a task
// cannot tell the two apart; this is the one predicate that can.
func (m *OutlinerModel) CursorChecklistItem() (ChecklistCursor, bool) {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return ChecklistCursor{}, false
	}
	row := m.Rows[m.Cursor]
	if !row.IsChecklistItem || row.Task == nil {
		return ChecklistCursor{}, false
	}
	project := m.Project
	if project == nil {
		project = m.crossResolveProject(row.ProjectID)
	}
	if project == nil {
		return ChecklistCursor{}, false
	}
	return ChecklistCursor{Task: row.Task, Project: project, Index: row.ChecklistIndex}, true
}

// isChecklistExpanded reports whether the owner task's checklist items render as
// their own rows. Absent means collapsed, so the default projection is exactly
// the one that existed before checklist rows.
func (m *OutlinerModel) IsChecklistExpanded(identity taskref.TaskIdentity) bool {
	return m != nil && m.Folds.ExpandedChecklists[identity]
}

// expandChecklistRows splices each expanded task's checklist items in beneath
// their owner. It runs once over the finished projection rather than inside each
// mode strategy, so every mode gains checklist rows from one kernel.
//
// Visual selection suppresses them entirely: `v`, `V` and `Ctrl+A` already
// cannot select a non-task row, but cursor movement during a live range would
// still land on one.
func (m *OutlinerModel) expandChecklistRows() {
	if m == nil || len(m.Folds.ExpandedChecklists) == 0 || m.MultiSelectMode {
		return
	}
	expanded := make([]OutlinerRow, 0, len(m.Rows))
	for _, row := range m.Rows {
		expanded = append(expanded, row)
		if !row.IsTaskRow() || row.Task == nil || len(row.Task.Checklist) == 0 {
			continue
		}
		identity := m.rowIdentity(row)
		if !m.IsChecklistExpanded(identity) {
			continue
		}
		childrenFollow := row.HasVisibleChildren && m.IsExpanded(identity)
		for index, item := range row.Task.Checklist {
			child := row
			child.IsChecklistItem = true
			child.LeadsMergedGroup = false
			if row.LeadsMergedGroup {
				child.MergedLead = row.Task
			}
			child.ChecklistIndex = index
			child.ChecklistDone = item.Done
			child.HasVisibleChildren = false
			child.Breadcrumb = ""
			child.Prefix = checklistRowPrefix(
				row.Prefix,
				index == len(row.Task.Checklist)-1 && !childrenFollow,
			)
			expanded = append(expanded, child)
		}
	}
	m.Rows = expanded
}

// checklistRowPrefix continues the owner's tree prefix one level deeper, turning
// the owner's own connector into the continuation glyph its descendants need.
func checklistRowPrefix(ownerPrefix string, last bool) string {
	base := ownerPrefix
	switch {
	case strings.HasSuffix(base, connectorLastItem):
		base = strings.TrimSuffix(base, connectorLastItem) + connectorSpace
	case strings.HasSuffix(base, connectorBranch):
		base = strings.TrimSuffix(base, connectorBranch) + connectorPipe
	}
	if last {
		return base + connectorLastItem
	}
	return base + connectorBranch
}

// syncChecklistSelectionRows re-projects the rows around a visual-selection
// boundary and re-seats the cursor on the owner task, because entering selection
// removes every checklist row the cursor may currently rest on and leaving it
// puts them back.
func (m *OutlinerModel) syncChecklistSelectionRows() {
	if m == nil || len(m.Folds.ExpandedChecklists) == 0 {
		return
	}
	owner := taskref.TaskIdentity{}
	if m.Cursor >= 0 && m.Cursor < len(m.Rows) {
		owner = m.TaskIdentityForRow(m.Rows[m.Cursor])
	}
	m.RebuildRows()
	if owner.Valid() {
		m.MoveCursorToIdentity(owner)
	}
}
