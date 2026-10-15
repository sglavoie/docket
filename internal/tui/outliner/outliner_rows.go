package outliner

import (
	"cmp"
	"slices"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

// validateCrossFocusStack keeps only the contiguous prefix that still describes
// one real ancestry branch in one owning project. Existence alone is not
// sufficient: an inline edit can reparent a nested Focus root while leaving its
// task ID alive, and keeping that frame would escape the branch above it.
func (m *OutlinerModel) validateCrossFocusStack() {
	if len(m.Focus.CrossStack) == 0 {
		return
	}

	projects := make(map[string]*models.Project, len(m.Population.Projects))
	for _, project := range m.Population.Projects {
		if project != nil {
			projects[project.ID] = project
		}
	}

	invalidIndex := len(m.Focus.CrossStack)
	var previous *models.Task
	previousProjectID := ""
	for index, entry := range m.Focus.CrossStack {
		project := projects[entry.ProjectID]
		task := (*models.Task)(nil)
		if project != nil {
			task = project.GetTaskByID(entry.TaskID)
		}
		if task == nil || (index > 0 &&
			(entry.ProjectID != previousProjectID || !focusTaskDescendsFrom(project, task, previous.ID))) {
			invalidIndex = index
			break
		}
		previous = task
		previousProjectID = entry.ProjectID
	}
	if invalidIndex == len(m.Focus.CrossStack) {
		return
	}

	restore := m.Focus.CrossStack[invalidIndex]
	m.Cursor = restore.CursorIndex
	m.ViewScrollOffset = restore.ScrollOffset
	m.Focus.truncateCross(invalidIndex)
	m.setFocusRecoveryNotice(invalidIndex)
}

// validateFocusStack applies the same contiguous-branch rule to the
// single-project stack.
func (m *OutlinerModel) validateFocusStack() {
	if m.Project == nil || len(m.Focus.Stack) == 0 {
		return
	}

	invalidIndex := len(m.Focus.Stack)
	var previous *models.Task
	for index, entry := range m.Focus.Stack {
		task := m.Project.GetTaskByID(entry.TaskID)
		if task == nil || (index > 0 && !focusTaskDescendsFrom(m.Project, task, previous.ID)) {
			invalidIndex = index
			break
		}
		previous = task
	}
	if invalidIndex == len(m.Focus.Stack) {
		return
	}

	restore := m.Focus.Stack[invalidIndex]
	m.Cursor = restore.CursorIndex
	m.ViewScrollOffset = restore.ScrollOffset
	m.Focus.truncate(invalidIndex)
	m.setFocusRecoveryNotice(invalidIndex)
}

// focusTaskDescendsFrom follows canonical ParentID links rather than transient
// projection pointers. It therefore stays correct after a reload or reparent.
func focusTaskDescendsFrom(project *models.Project, task *models.Task, ancestorID string) bool {
	if project == nil || task == nil || ancestorID == "" {
		return false
	}
	seen := make(map[string]bool)
	for current := task; current != nil && current.ParentID != nil; {
		parentID := *current.ParentID
		if parentID == ancestorID {
			return true
		}
		if seen[parentID] {
			return false
		}
		seen[parentID] = true
		current = project.GetTaskByID(parentID)
	}
	return false
}

func (m *OutlinerModel) setFocusRecoveryNotice(retainedDepth int) {
	destination := "root"
	if retainedDepth > 0 {
		destination = "parent"
	}
	m.Focus.setNotice("Focus target left this branch — returned to " + destination)
}

// rebuildRows flattens the current task tree into the rows slice, applying the
// active mode and filters, and clamps the cursor to stay within bounds.
func (m *OutlinerModel) RebuildRows() {
	// The multi-select summary counts visible against hidden members, so it is a
	// projection of the rows and not of the selection alone: a fold, a filter, or
	// a mode change moves members across that line without touching the selection.
	// Nothing is refreshed here, because the summary is a context-row segment
	// derived from the live rows at render time rather than a status message some
	// seam has to keep current — which is what makes a stale count unreachable
	// instead of merely refreshed at every seam that remembered to.
	defer m.restoreTimeDrivenCursor()
	m.SyncJumpProjection()
	if m.Project == nil {
		// Mark as a mutation-driven rebuild so the memoization guard in
		// crossProjectRebuildRows is bypassed. The render path calls
		// crossProjectRebuildRows directly (not via rebuildRows), so it
		// still benefits from the guard on no-op events like window resize.
		m.Population.Invalidate()
		m.CrossProjectRebuildRows()
		return
	}
	defer func() {
		m.Population.record(m.singleProjectRowsFingerprint(), len(m.Focus.Stack), len(m.Project.Tasks))
	}()
	openAt := m.OpenAtTask
	// Consume the pending target before any navigation helper can trigger a
	// nested rebuild. A target hidden by the current focus or filters otherwise
	// re-enters this block indefinitely.
	m.OpenAtTask = nil
	m.Rows = m.Rows[:0]
	m.hiddenLayout.reset()
	// Explicit rebuilds invalidate the index; repaint-only frames reuse both.
	m.invalidateDependencyIndex()

	m.validateFocusStack()

	m.buildCanonicalModeRows()

	if openAt != nil {
		m.revealTask(openAt)
	}
}

// effectiveDisplaySection returns the task's display section for non-header
// contexts such as navigation matching and task-form dimming.
func effectiveDisplaySection(t *models.Task) string {
	return theme.TaskStateGroupLabel(t)
}

// filterTaskTreeKeepingAncestors returns a copy of the task tree containing
// matching tasks plus their ancestors. It is reserved for mode membership
// projections, where a qualifying descendant may keep hierarchy context.
func filterTaskTreeKeepingAncestors(tasks []*models.Task, predicate func(*models.Task) bool) []*models.Task {
	var result []*models.Task
	for _, task := range tasks {
		if task == nil {
			continue
		}
		filteredChildren := filterTaskTreeKeepingAncestors(task.Children, predicate)
		if predicate(task) || len(filteredChildren) > 0 {
			clone := taskref.CloneTaskDetachingDependencies(task)
			clone.Children = filteredChildren
			result = append(result, &clone)
		}
	}
	return result
}

// filterTaskTreePromotingChildren returns a copy containing only tasks that
// satisfy predicate. Matching descendants of a rejected task are promoted so
// facet filtering never hides valid work merely because an ancestor failed.
func filterTaskTreePromotingChildren(tasks []*models.Task, predicate func(*models.Task) bool) []*models.Task {
	return excludeTaskTreePromotingChildren(tasks, func(task *models.Task) bool {
		return !predicate(task)
	})
}

// clampCursor ensures the cursor stays within [0, len(rows)-1] and never
// rests on a non-navigable row (a separator). The cursor is allowed to settle
// on section headers (Enter zooms), project sub-headers (Enter scopes), and
// task rows. Separators are visual-only and must never be selected after a
// rebuild, filter, collapse, paging, or mode change.
func (m *OutlinerModel) ClampCursor() {
	m.ClampCursorTo(len(m.Rows))
	if len(m.Rows) == 0 {
		return
	}
	if m.Rows[m.Cursor].IsNavigable() {
		return
	}
	for i := m.Cursor + 1; i < len(m.Rows); i++ {
		if m.Rows[i].IsNavigable() {
			m.Cursor = i
			return
		}
	}
	for i := m.Cursor - 1; i >= 0; i-- {
		if m.Rows[i].IsNavigable() {
			m.Cursor = i
			return
		}
	}
}

// contentHeight returns the number of task rows visible on screen after the
// one-row operational title, the one-row menu bar above it when it is shown,
// and the context row below it when some axis is live. Scope and mode are part
// of that title and context is exactly one further row, so neither ever
// consumes a third breadcrumb.
//
// This is the one definition of the row budget, and every caller wants the
// reduced value: the window, the page size, and the suggestion budgets are all
// measured against the list the chrome leaves behind.
func (m *OutlinerModel) ContentHeight() int {
	height := m.Height() - 1 - m.menuBarRows() - m.contextRowRows()
	if height < 1 {
		return 1
	}
	return height
}

// recentModeCutoffDays bounds Recent: activity older than this is no longer
// something the user is triaging.
const recentModeCutoffDays = 30

// OutlinerModeSection is the representation-neutral grouped output for Due.
type OutlinerModeSection struct {
	Name        string
	CollapseKey string
	Rows        []OutlinerModeRow
	// Lead is the section's order without its stored-order tiebreak, which is
	// what a project block ranks by; a tie falls to the project order. Nil
	// ranks blocks by position alone.
	Lead func(a, b OutlinerModeRow) int
}

const canonicalModeCollapseKeyPrefix = "_mode:"

func canonicalModeCollapseKey(mode chrome.OutlinerModeID, bucket string) string {
	return canonicalModeCollapseKeyPrefix + string(mode) + ":" + bucket
}

// canonicalModeProjectCollapseKeyPrefix namespaces the project groups a
// canonical mode owns. Each grouping surface keeps a disjoint key namespace so a
// fold left behind in one mode can never bind to a row in the mode you enter
// next; collapsedSections is one map that survives every mode change.
const canonicalModeProjectCollapseKeyPrefix = "_mode_project:"

// modeProjectCollapseKey identifies one project's group in a mode that emits no
// bucket header of its own.
func modeProjectCollapseKey(mode chrome.OutlinerModeID, projectID string) string {
	return canonicalModeProjectCollapseKeyPrefix + string(mode) + ":" + projectID
}

// modeSectionProjectCollapseKey identifies one project's group inside one bucket
// of a computed mode. It derives from the section's own key so the bucket
// vocabulary lives in exactly one place.
func modeSectionProjectCollapseKey(sectionCollapseKey, projectID string) string {
	return canonicalModeProjectCollapseKeyPrefix +
		strings.TrimPrefix(sectionCollapseKey, canonicalModeCollapseKeyPrefix) + ":" + projectID
}

type orderedModeTask struct {
	row          OutlinerModeRow
	projectOrder int
	taskOrder    int
}

// orderedModeTasks collects the in-scope candidate tasks in the caller's
// authoritative project-catalog order while retaining each project's stable
// document order as an explicit deterministic tie-break. CandidateTasks
// optionally narrows a focused scope.
func orderedModeTasks(in OutlinerModeInput) []orderedModeTask {
	projects := slices.Clone(in.Projects)

	var result []orderedModeTask
	for projectOrder, project := range projects {
		if project == nil {
			continue
		}
		tasks := project.Tasks
		if candidates, ok := in.CandidateTasks[project.ID]; ok {
			tasks = candidates
		}
		modelOrder := make(map[string]int, len(project.Tasks))
		for i, task := range project.Tasks {
			if task != nil {
				modelOrder[task.ID] = i
			}
		}
		for candidateOrder, task := range tasks {
			if task == nil {
				continue
			}
			order, ok := modelOrder[task.ID]
			if !ok {
				order = len(project.Tasks) + candidateOrder
			}
			result = append(result, orderedModeTask{
				row:          OutlinerModeRow{Task: task, Project: project},
				projectOrder: projectOrder,
				taskOrder:    order,
			})
		}
	}
	return result
}

func compareOrderedModeTasks(left, right orderedModeTask) int {
	if c := cmp.Compare(left.projectOrder, right.projectOrder); c != 0 {
		return c
	}
	if c := cmp.Compare(left.taskOrder, right.taskOrder); c != 0 {
		return c
	}
	return cmp.Compare(left.row.Task.ID, right.row.Task.ID)
}

func isComputedOpenWork(task *models.Task, in OutlinerModeInput) bool {
	if task == nil || (task.IsTerminalState() && !in.IncludeTerminal) {
		return false
	}
	return in.IncludeDeferred || !task.IsDeferredAt(in.now())
}

// BuildDueModeSections returns dated open work in the accepted local
// calendar-day buckets. Next Seven Days is tomorrow through today+7 inclusive.
func BuildDueModeSections(in OutlinerModeInput) []OutlinerModeSection {
	in.Now = in.now()
	sections := []OutlinerModeSection{
		{Name: "Overdue", CollapseKey: canonicalModeCollapseKey(chrome.ModeDue, "overdue")},
		{Name: "Today", CollapseKey: canonicalModeCollapseKey(chrome.ModeDue, "today")},
		{Name: "Next Seven Days", CollapseKey: canonicalModeCollapseKey(chrome.ModeDue, "next-seven-days")},
		{Name: "Later", CollapseKey: canonicalModeCollapseKey(chrome.ModeDue, "later")},
	}
	now := in.now()

	buckets := make([][]orderedModeTask, len(sections))
	for _, candidate := range orderedModeTasks(in) {
		task := candidate.row.Task
		if !isComputedOpenWork(task, in) || task.DueDate == nil {
			continue
		}
		bucket := 3
		switch days := util.DaysBetween(now, *task.DueDate); {
		case days < 0:
			bucket = 0
		case days == 0:
			bucket = 1
		case days <= 7:
			bucket = 2
		}
		buckets[bucket] = append(buckets[bucket], candidate)
	}

	// A deadline leads, priority breaks a same-day tie, and stored order breaks
	// the rest.
	lead := func(l, r OutlinerModeRow) int {
		if days := util.DaysBetween(*l.Task.DueDate, *r.Task.DueDate); days != 0 {
			return -cmp.Compare(days, 0)
		}
		return comparePriorityBand(l.Task, r.Task)
	}
	var result []OutlinerModeSection
	for i := range sections {
		sections[i].Lead = lead
		sort.SliceStable(buckets[i], func(left, right int) bool {
			l, r := buckets[i][left], buckets[i][right]
			if c := lead(l.row, r.row); c != 0 {
				return c < 0
			}
			return compareOrderedModeTasks(l, r) < 0
		})
		for _, candidate := range buckets[i] {
			candidate.row.Breadcrumb = taskAncestorBreadcrumb(candidate.row.Task)
			sections[i].Rows = append(sections[i].Rows, candidate.row)
		}
		if len(sections[i].Rows) > 0 {
			result = append(result, sections[i])
		}
	}
	return result
}

// BuildRecentModeSections returns tasks touched within the cutoff, newest first,
// in local calendar-day buckets. Terminal and deferred membership is the caller's
// decision through IncludeTerminal and IncludeDeferred.
func BuildRecentModeSections(in OutlinerModeInput) []OutlinerModeSection {
	in.Now = in.now()
	sections := []OutlinerModeSection{
		{Name: "Today", CollapseKey: canonicalModeCollapseKey(chrome.ModeRecent, "today")},
		{Name: "Yesterday", CollapseKey: canonicalModeCollapseKey(chrome.ModeRecent, "yesterday")},
		{Name: "Last 7 Days", CollapseKey: canonicalModeCollapseKey(chrome.ModeRecent, "last-seven-days")},
		{Name: "Last 30 Days", CollapseKey: canonicalModeCollapseKey(chrome.ModeRecent, "last-thirty-days")},
	}
	now := in.now()

	buckets := make([][]orderedModeTask, len(sections))
	for _, candidate := range orderedModeTasks(in) {
		task := candidate.row.Task
		if !isComputedOpenWork(task, in) {
			continue
		}
		// Stored instants load in UTC, and DaysBetween compares calendar dates,
		// so the key is read in now's location before the day is taken.
		key := in.recencyKey(task, candidate.row.Project).In(now.Location())
		age := util.DaysBetween(key, now)
		if age > recentModeCutoffDays {
			continue
		}
		bucket := 3
		switch {
		case age <= 0:
			bucket = 0
		case age == 1:
			bucket = 1
		case age <= 7:
			bucket = 2
		}
		buckets[bucket] = append(buckets[bucket], candidate)
	}

	// The newest touch leads, priority breaks an exact tie, and stored order
	// breaks the rest.
	lead := func(l, r OutlinerModeRow) int {
		if c := in.recencyKey(r.Task, r.Project).Compare(in.recencyKey(l.Task, l.Project)); c != 0 {
			return c
		}
		return comparePriorityBand(l.Task, r.Task)
	}
	var result []OutlinerModeSection
	for i := range sections {
		sections[i].Lead = lead
		sort.SliceStable(buckets[i], func(left, right int) bool {
			l, r := buckets[i][left], buckets[i][right]
			if c := lead(l.row, r.row); c != 0 {
				return c < 0
			}
			return compareOrderedModeTasks(l, r) < 0
		})
		for _, candidate := range buckets[i] {
			candidate.row.Breadcrumb = taskAncestorBreadcrumb(candidate.row.Task)
			sections[i].Rows = append(sections[i].Rows, candidate.row)
		}
		if len(sections[i].Rows) > 0 {
			result = append(result, sections[i])
		}
	}
	return result
}

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
	IsPinnedSection        bool                      // True for the synthetic Pinned header and every promoted task row beneath it
	IsTodaySection         bool                      // True for the synthetic Today header and every explicit Today task row beneath it
	IsNextSection          bool                      // True for the synthetic Next header and every explicit Next task row beneath it
	PromotedTag            string                    // Canonical promoted workflow tag for synthetic header/project/task rows
	SectionName            string                    // Section name for header rows
	SectionCount           int                       // Number of tasks in this section
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
	IsNoteRow              bool                      // True for one note from the search projection's notes tier
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
