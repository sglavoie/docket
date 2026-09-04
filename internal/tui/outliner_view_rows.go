package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// Tree connector characters for outliner rendering.
const (
	connectorBranch   = "├── "
	connectorLastItem = "└── "
	connectorPipe     = "│   "
	connectorSpace    = "    "
)

// outlinerRow represents a single flattened row for rendering.
type outlinerRow struct {
	task               *models.Task
	prefix             string
	Breadcrumb         string                    // Ancestor path for tasks promoted across a section boundary.
	hasVisibleChildren bool                      // Children rendered in this row's section-local tree.
	IsSectionHeader    bool                      // True if this row is a section header (not a task)
	IsComputedHeader   bool                      // True for a computed-mode grouping header such as a due-date bucket
	IsProjectHeader    bool                      // True if this row is a synthetic, cursorable project sub-header (not a task)
	IsPinnedSection    bool                      // True for the synthetic Pinned header and every promoted task row beneath it
	IsTodaySection     bool                      // True for the synthetic Today header and every explicit Today task row beneath it
	IsNextSection      bool                      // True for the synthetic Next header and every explicit Next task row beneath it
	PromotedTag        string                    // Canonical promoted workflow tag for synthetic header/project/task rows
	SectionName        string                    // Section name for header rows
	SectionCount       int                       // Number of tasks in this section
	StateGroup         models.WorkflowStateGroup // Canonical state group for workflow headers; empty for unrelated/computed headers
	CollapseKey        string                    // Stable identity for shared or project-owned grouping headers
	ProjectID          string                    // Project that owns task/project-header rows; empty for shared section headers
	IsSeparator        bool                      // True if this row is a non-navigable horizontal rule between blocks
	OverdueCount       int                       // Overdue tasks inside the group a project header names
	TodayCount         int                       // Today-planned tasks inside the group a project header names
	IsChecklistItem    bool                      // True for one expanded checklist item beneath its owner task row
	ChecklistIndex     int                       // Position of the item inside the owner task's checklist
	ChecklistDone      bool                      // Ticked state of the item this row names
	MatchSource        outlinerMatchSource       // Field that produced this search hit; matchSourceNone on every ordinary row
	IsNoteRow          bool                      // True for one note from the search projection's notes tier
	NoteRow            notes.NoteRowIdentity     // Workspace identity of the note an IsNoteRow row names
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
func (r outlinerRow) isTaskRow() bool {
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
func (r outlinerRow) isNavigable() bool {
	return !r.IsSeparator
}

// isHeaderRow reports whether the row is one of the two cursorable grouping
// headers. Both fold under h/l and both are what n/N step between. Section
// headers alone would leave n/N dead in the four modes that group by project
// only.
func (r outlinerRow) isHeaderRow() bool {
	return r.IsSectionHeader || r.IsProjectHeader
}

// collapseKey returns the effective key for collapsedSections lookups.
// Cross-project headers set CollapseKey to "projectID:sectionName"; single-project
// headers leave it empty and fall back to SectionName.
func (r outlinerRow) effectiveCollapseKey() string {
	if r.CollapseKey != "" {
		return r.CollapseKey
	}
	return r.SectionName
}

// isSectionRowCollapsed reports whether a section-header row is collapsed.
func (m *OutlinerModel) isSectionRowCollapsed(row outlinerRow) bool {
	return m.sectionCollapsed(row.effectiveCollapseKey())
}

// sectionCollapsed is the one read of persistent fold state. A zoomed section
// ignores its own fold so it can be zoomed without mutating the fold the user
// set, and without the zoom having to write and restore it.
func (m *OutlinerModel) sectionCollapsed(key string) bool {
	return m.collapsedSections[key] && key != m.zoom.key
}

// sectionZoomAdmits reports whether a section may emit rows at all.
func (m *OutlinerModel) sectionZoomAdmits(key string) bool {
	return !m.zoom.active() || m.zoom.key == key
}

// cursorNamesSectionHeader is the one read of the cursor's row kind, shared by
// the Outliner's own command context and the shell's, so the key that dispatches
// and the footer that advertises it can never disagree.
func (m *OutlinerModel) cursorNamesSectionHeader() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.rows) {
		return false
	}
	return m.rows[m.Cursor].IsSectionHeader
}

// cursorNoteIdentity names the note the cursor rests on, and is the second read
// of the cursor's row kind both command contexts share. A note row carries no
// project or task, so this identity is the only thing an operation acting on it
// can resolve a target from.
func (m *OutlinerModel) cursorNoteIdentity() (notes.NoteRowIdentity, bool) {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.rows) {
		return notes.NoteRowIdentity{}, false
	}
	row := m.rows[m.Cursor]
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
func (m *OutlinerModel) isZoomedSectionRow(row outlinerRow) bool {
	return m.zoom.active() && row.IsSectionHeader && row.effectiveCollapseKey() == m.zoom.key
}

// buildTreePrefix computes the connector prefix string from the ancestor
// is-last-child boolean stack. Each true entry means that ancestor was the
// last child of its parent (so we render spaces), while false means we need
// a continuation pipe.
func buildTreePrefix(ancestorIsLast []bool, isLast bool) string {
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
func (m *OutlinerModel) isExpanded(identity taskref.TaskIdentity) bool {
	if len(m.expandedNodes) == 0 {
		return true
	}
	expanded, exists := m.expandedNodes[identity]
	if !exists {
		return true // Default to expanded
	}
	return expanded
}
