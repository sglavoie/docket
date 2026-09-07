package outliner

import (
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// outlinerScopeFrame captures the outliner state that a project-scope push
// replaces, so popping the scope restores navigation and presentation axes that
// remain scope-owned. Scope is this frame and never a clause of a query: a
// search runs inside the frame it was typed in, a lens moves the frame, and the
// frames stack. Task facets are intentionally absent and remain global across
// mode and scope changes. The sole exception is Stale, which explicit mode
// selection and frame restoration both clear whenever they activate Recent.
//
// Fold state is deliberately NOT snapshotted and remains shared across scopes:
//   - collapsedSections / expandedNodes are keyed by project and section, so a
//     scope change cannot corrupt another project's folds.
//   - computed section fold maps are not per-scope state.
//
// The section zoom is snapshotted precisely because it is not fold state: it is
// a narrowing of the projection the push replaces, like the focus stacks beside
// it, so the pop owes the user the zoom they left.
type OutlinerScopeFrame struct {
	scopedProjectID  string
	cursor           int
	viewScrollOffset int

	primaryMode chrome.OutlinerModeID

	// Active render ordering plus the durable tree-mode sort.
	Sort     query.SortState
	treeSort query.SortState

	// Focus stacks (single-project and cross-project).
	focusStack      []FocusEntry
	crossFocusStack []CrossProjectFocusEntry

	// Section zoom the push dropped.
	zoom sectionZoom
}

// scopeState is the one project focus the cross-project Outliner is standing
// in, the single base frame behind it, and the parked-project exception that
// can be the reason it is reachable at all.
//
// Two concerns, one owner, because the parked exception is not separable from
// the scope it exists to serve. A parked project is absent from
// TaskWorkspaceProjects(), so scoping into one is only possible while the
// exception holds it inside the loaded snapshot: the exception is requested
// before the push, re-decided by every later load that carries it, and shed by
// the same Esc that pops the scope. Splitting them would put "may this scope
// exist" in one type and "does it exist" in another.
//
// The stack is named a stack and behaves as one frame: the scope model admits
// one base plus at most one project focus, so a second push replaces the focus
// in place rather than nesting. It stays a slice because that is what the
// presentation snapshot captures and what a defensive repair of older nested
// state truncates, and the methods below are what keep "at most one frame" a
// property of this type rather than of every caller.
//
// Whether a scope write also owes m.population.invalidate() and a rebuild stays
// the call site's, exactly as it is for the focus stacks: this state cannot see
// the population's memoization guard and deliberately does not try to.
//
// The zero value is the authoritative all-projects scope with no frame behind
// it and no exception in play.
type scopeState struct {
	// projectID names the single project the global outliner is currently scoped
	// to. Empty means the authoritative all-projects scope. Only meaningful in
	// cross-project mode (m.project == nil); scoping filters the loaded data set
	// down to one project without switching to a separate single-project
	// OutlinerModel.
	ProjectID string

	// stack records the state to restore when a project scope is popped. Each
	// push saves the previous scope's cursor, scroll, mode, sort, and focus so Esc
	// returns exactly where the user left off. The bottom of the stack is the base
	// all-projects frame.
	Stack []OutlinerScopeFrame

	// pendingParkedID names a parked project whose focus load is in flight. The
	// scope is pushed only once that load lands, so a failed or superseded request
	// leaves the base snapshot exactly as it was.
	PendingParkedID string

	// parkedProjectID names the parked project the current snapshot admits as an
	// explicit exception. Every later reload re-requests it, so an ordinary
	// mutation refresh cannot silently drop the focused tree.
	ParkedProjectID string

	// parkedReloadPending records that a parked focus was just dropped and the
	// shell still owes a reload to shed the parked project from the snapshot. The
	// Esc cascade cannot return a Cmd, so the shell claims it afterwards.
	ParkedReloadPending bool
}

// scoped reports whether a single-project focus is active. It is the one
// definition of "the view is scoped", shared by the row population, the scope
// chrome, and every push, pop, and repair below.
func (s *scopeState) scoped() bool {
	if s == nil {
		return false
	}
	return s.ProjectID != ""
}

// setProject installs the focused project, or the base scope when empty.
func (s *scopeState) setProject(projectID string) {
	if s == nil {
		return
	}
	s.ProjectID = projectID
}

// clearProject returns to the authoritative all-projects scope.
func (s *scopeState) clearProject() { s.setProject("") }

// setBase installs the sole restorable frame, replacing whatever stood behind
// the scope before.
func (s *scopeState) setBase(frame OutlinerScopeFrame) {
	if s == nil {
		return
	}
	s.Stack = []OutlinerScopeFrame{frame}
}

// retainBase keeps the original base frame and drops anything above it,
// reporting whether there was one to keep. Replacing a project focus goes
// through here rather than capturing a new frame, because the frame Esc owes the
// user is the one taken at the first push, not the focus this one is leaving.
func (s *scopeState) retainBase() bool {
	if s == nil || len(s.Stack) == 0 {
		return false
	}
	s.Stack = s.Stack[:1]
	return true
}

// takeBase reads the base frame and empties the stack in one step, reporting
// whether a frame was there. Every restoration is single-use: the frame it hands
// back describes a scope that no longer exists the moment it is applied.
func (s *scopeState) takeBase() (OutlinerScopeFrame, bool) {
	if s == nil || len(s.Stack) == 0 {
		return OutlinerScopeFrame{}, false
	}
	base := s.Stack[0]
	s.Stack = nil
	return base, true
}

// clearStack drops the frame without restoring it, for a reset that replaces
// every axis the frame carries rather than returning to it.
func (s *scopeState) clearStack() {
	if s == nil {
		return
	}
	s.Stack = nil
}

// scopeSnapshot is the part of this state a restoration point carries: the
// focus, the frame behind it, and the parked exception that may be the reason
// the focus is reachable. The in-flight request and the owed reload are
// deliberately absent, because both name work rather than a view — reinstating
// them would re-open a load that already settled or re-owe a reload nobody is
// waiting for.
type scopeSnapshot struct {
	projectID       string
	Stack           []OutlinerScopeFrame
	ParkedProjectID string
}

// capture reads those three out, copying the stack so the retained frame cannot
// alias one a later push replaces.
func (s *scopeState) capture() scopeSnapshot {
	if s == nil {
		return scopeSnapshot{}
	}
	return scopeSnapshot{
		projectID:       s.ProjectID,
		Stack:           slices.Clone(s.Stack),
		ParkedProjectID: s.ParkedProjectID,
	}
}

// restore writes a captured scope back at once. The three go together because
// restoring any one alone leaves a view standing in a scope with no way back out
// of it, holding a frame for a scope it is not in, or focused on a parked project
// the snapshot no longer admits.
func (s *scopeState) restore(snapshot scopeSnapshot) {
	if s == nil {
		return
	}
	s.ProjectID = snapshot.projectID
	s.Stack = slices.Clone(snapshot.Stack)
	s.ParkedProjectID = snapshot.ParkedProjectID
}

// focusRequest names the parked project the next load must admit. An in-flight
// request wins over the adopted one so a changed focus is not fought by the
// project it replaces; an adopted focus is re-requested by every later reload so
// an ordinary mutation refresh cannot drop the focused tree.
func (s *scopeState) FocusRequest() string {
	if s == nil {
		return ""
	}
	if s.PendingParkedID != "" {
		return s.PendingParkedID
	}
	return s.ParkedProjectID
}

// requestParked records that a focus load for one parked project is in flight.
func (s *scopeState) requestParked(projectID string) {
	if s == nil {
		return
	}
	s.PendingParkedID = projectID
}

// clearPendingParked abandons the in-flight request alone, leaving whatever
// exception the current snapshot already holds.
func (s *scopeState) clearPendingParked() {
	if s == nil {
		return
	}
	s.PendingParkedID = ""
}

// setParkedException records the parked project the current snapshot admits, or
// clears it when the base workspace admits the project on its own.
func (s *scopeState) setParkedException(projectID string) {
	if s == nil {
		return
	}
	s.ParkedProjectID = projectID
}

// clearParkedException puts the exception and any in-flight request down without
// deciding a reload. It is the repair a vanished scope target performs: there is
// nothing left to re-request, and nothing owed for shedding a project that is
// already gone.
func (s *scopeState) clearParkedException() {
	if s == nil {
		return
	}
	s.PendingParkedID = ""
	s.ParkedProjectID = ""
}

// takeParkedException consumes the exception, returning the project it was
// holding. The reload obligation is deliberately left to the caller: shedding
// the project from the live snapshot owes one, while settling it against a
// population that was already read under the live facets owes nothing.
func (s *scopeState) takeParkedException() string {
	if s == nil {
		return ""
	}
	focused := s.ParkedProjectID
	s.clearParkedException()
	return focused
}

// oweParkedReload records the reload the shell must claim to shed a dropped
// parked project from the snapshot.
func (s *scopeState) oweParkedReload() {
	if s == nil {
		return
	}
	s.ParkedReloadPending = true
}

// clearParkedReload settles that obligation, either because the reload is
// starting now or because the population in hand already is what it would have
// gone back for.
func (s *scopeState) clearParkedReload() {
	if s == nil {
		return
	}
	s.ParkedReloadPending = false
}

// FocusEntry is the retained internal focus-frame representation. It records
// the complete viewport state needed to restore the parent view exactly.
type FocusEntry struct {
	TaskID       string // UUID of the task that was focused
	CursorIndex  int    // Cursor position at the moment of focus
	ScrollOffset int    // Persistent viewport offset at the moment of focus
}

// CrossProjectFocusEntry records the state at a single focus level for the
// cross-project outliner. It extends FocusEntry with a ProjectID field to
// scope focus resolution to the correct project, preventing UUID collisions
// across projects from causing incorrect focus resolution.
type CrossProjectFocusEntry struct {
	TaskID       string // UUID of the task that was focused
	ProjectID    string // ID of the project owning the task
	CursorIndex  int    // Cursor position at the moment of focus
	ScrollOffset int    // Persistent viewport offset at the moment of focus
}

// sectionZoom narrows the Outliner to one section of the active mode, plus the
// viewport Esc restores. A zero value means no zoom.
type sectionZoom struct {
	key    string // Collapse key of the zoomed section
	Name   string // Display name, for chrome only
	cursor int    // Cursor row the zoom was entered from
	scroll int    // Persistent viewport offset the zoom was entered from
}

func (z sectionZoom) Active() bool { return z.key != "" }

// focusState is everything the Outliner has narrowed to and can hand back: the
// two focus stacks, the section zoom, and the one-shot notice a frame that
// stopped resolving leaves behind.
//
// Esc is the argument for one owner. The unwind ladder walks task focus and
// then the zoom before it reaches project scope, and every wholesale reset
// takes all of them down together, so a caller that put one of them away and
// forgot another would leave the view narrowed by something no Esc can name.
// The notice belongs here for the same reason from the other end: only
// validateFocusStack and validateCrossFocusStack write it, and they write it
// exactly when they shorten a stack.
//
// Two stacks rather than one, because a cross-project frame carries the
// owning project and a single-project frame cannot: which stack is live is
// m.project's answer, never a field here. Whether a cross-stack write also owes
// m.population.invalidate() stays the call site's, not this type's — the
// memoization guard is the population's, this state deliberately cannot see it,
// and validateCrossFocusStack truncates mid-rebuild without claiming one.
//
// The zero value is inert: no frames, no zoom, no notice.
type focusState struct {
	// stack is the single-project focus stack and crossStack the cross-project
	// one. Each frame retains the cursor and scroll offset Esc restores, so
	// nested focus levels unwind without moving the viewport.
	Stack      []FocusEntry
	CrossStack []CrossProjectFocusEntry

	// notice is the transient message shown when a focus target left its branch.
	// ViewStatusStrip consumes it, so it survives exactly one frame.
	Notice string

	// zoom is the section the projection is narrowed to; the zero value is none.
	Zoom sectionZoom
}

// active reports whether either stack holds a frame. It is the one definition
// of "the view is focused", shared by the empty-state hint, the launch reset's
// axis inventory, and the search projection's own focus question.
func (s *focusState) Active() bool {
	if s == nil {
		return false
	}
	return len(s.Stack) > 0 || len(s.CrossStack) > 0
}

// push records one single-project focus frame.
func (s *focusState) push(entry FocusEntry) {
	if s == nil {
		return
	}
	s.Stack = append(s.Stack, entry)
}

// pushCross records one cross-project focus frame.
func (s *focusState) pushCross(entry CrossProjectFocusEntry) {
	if s == nil {
		return
	}
	s.CrossStack = append(s.CrossStack, entry)
}

// pop removes the top single-project frame and returns it.
func (s *focusState) pop() (FocusEntry, bool) {
	if s == nil || len(s.Stack) == 0 {
		return FocusEntry{}, false
	}
	top := s.Stack[len(s.Stack)-1]
	s.Stack = s.Stack[:len(s.Stack)-1]
	return top, true
}

// popCross removes the top cross-project frame and returns it.
func (s *focusState) popCross() (CrossProjectFocusEntry, bool) {
	if s == nil || len(s.CrossStack) == 0 {
		return CrossProjectFocusEntry{}, false
	}
	top := s.CrossStack[len(s.CrossStack)-1]
	s.CrossStack = s.CrossStack[:len(s.CrossStack)-1]
	return top, true
}

// truncate keeps the first depth single-project frames, dropping the rest.
func (s *focusState) truncate(depth int) {
	if s == nil || depth < 0 || depth > len(s.Stack) {
		return
	}
	s.Stack = s.Stack[:depth]
}

// truncateCross keeps the first depth cross-project frames, dropping the rest.
func (s *focusState) truncateCross(depth int) {
	if s == nil || depth < 0 || depth > len(s.CrossStack) {
		return
	}
	s.CrossStack = s.CrossStack[:depth]
}

// clear drops the single-project stack.
func (s *focusState) clear() {
	if s == nil {
		return
	}
	s.Stack = nil
}

// clearCross drops the cross-project stack.
func (s *focusState) clearCross() {
	if s == nil {
		return
	}
	s.CrossStack = nil
}

// setStack adopts a captured single-project stack. Deep-copying is the
// caller's, on the same terms as foldState's adopters.
func (s *focusState) setStack(stack []FocusEntry) {
	if s == nil {
		return
	}
	s.Stack = stack
}

// restore writes a captured frame's three narrowings back at once. The scope
// frame and the presentation snapshot both capture the trio together, because
// unwinding one of them without the others would leave the view narrowed by a
// remnant of the frame it thought it had left.
func (s *focusState) Restore(stack []FocusEntry, crossStack []CrossProjectFocusEntry, zoom sectionZoom) {
	if s == nil {
		return
	}
	s.Stack = stack
	s.CrossStack = crossStack
	s.Zoom = zoom
}

// enterZoom narrows to one section, retaining the viewport Esc restores.
func (s *focusState) enterZoom(key, name string, cursor, scroll int) {
	if s == nil {
		return
	}
	s.Zoom = sectionZoom{key: key, Name: name, cursor: cursor, scroll: scroll}
}

// exitZoom widens back and returns the viewport the zoom was entered from.
func (s *focusState) exitZoom() (cursor, scroll int) {
	if s == nil {
		return 0, 0
	}
	cursor, scroll = s.Zoom.cursor, s.Zoom.scroll
	s.Zoom = sectionZoom{}
	return cursor, scroll
}

// clearZoom drops the zoom and the viewport it retained, for a transition that
// reshapes the projection on its own terms.
func (s *focusState) clearZoom() {
	if s == nil {
		return
	}
	s.Zoom = sectionZoom{}
}

// setNotice records the one-shot message a shortened stack owes the user.
func (s *focusState) setNotice(notice string) {
	if s == nil {
		return
	}
	s.Notice = notice
}

// clearNotice consumes it. The strip is rendered once per frame and reserves
// its own height from the same body, so consuming here is what keeps the
// notice from outliving the frame that reported it.
func (s *focusState) clearNotice() {
	if s == nil {
		return
	}
	s.Notice = ""
}

// sectionToggleRestoreAnchor identifies a task temporarily hidden by an
// s-driven all-section collapse. The next s-driven expansion restores focus to
// it when the task remains visible in the active view.
type sectionToggleRestoreAnchor struct {
	task       taskref.TaskIdentity
	sectionKey string
}

// foldState is the whole of what the Outliner has folded shut: the task
// subtrees `t` writes, the section and project-group keys `s` and `p` share,
// the checklists their owners open, and the one anchor an s-driven collapse
// leaves behind so the next expansion can put the cursor back.
//
// Three maps rather than one, and that is the point of the type rather than an
// accident it inherited. Their polarities disagree deliberately —
// expandedNodes defaults expanded so an absent key shows a subtree,
// collapsedSections defaults expanded so an absent key shows a section, and
// expandedChecklists defaults collapsed so an absent key hides its items — and
// their key spaces disagree too: two are project-qualified task identities and
// one is a string namespaced per grouping surface. Merging them would need a
// polarity per key. Naming them together is what makes clearAllFolds a single
// act over three axes without pretending they are one map.
//
// The anchor belongs here rather than beside the cursor because it names a
// section key: it exists only for the window in which those keys are collapsed,
// and clearSections below is what makes that literally true.
//
// The zero value is inert. Every map allocates on first write and reads as
// empty until then, which is exactly the all-open state a fresh Outliner opens
// in, so a struct-literal test model needs no constructor.
type foldState struct {
	// expandedNodes is the project-qualified task fold state: true=expanded, and
	// an absent key reads as expanded too, so the empty map is the canonical
	// all-expanded state rather than merely the initial one.
	ExpandedNodes map[taskref.TaskIdentity]bool

	// collapsedSections is the section and project-group fold state. Its string
	// keys are namespaced per grouping surface and are never cleared on a mode
	// change, so a fold written in one mode is still folded on return to it.
	CollapsedSections map[string]bool

	// expandedChecklists is the project-qualified checklist fold state, and it
	// defaults to collapsed: an absent key hides the owner's items, which is the
	// projection that existed before checklist rows.
	ExpandedChecklists map[taskref.TaskIdentity]bool

	// sectionToggleRestore preserves a task focus across one s-driven
	// collapse/expand cycle. It is deliberately session-only.
	SectionToggleRestore *sectionToggleRestoreAnchor
}

// setNodes adopts a captured task-fold map. Deep-copying is the caller's,
// because a caller restoring a frame that may be restored again owes the copy
// and one restoring a map it just built does not.
func (s *foldState) setNodes(nodes map[taskref.TaskIdentity]bool) {
	if s == nil {
		return
	}
	s.ExpandedNodes = nodes
}

// resetNodes returns every task subtree to expanded by dropping the map rather
// than writing true over each node: an absent key already reads as expanded, so
// the empty map is that state exactly and carries no stale identities with it.
func (s *foldState) resetNodes() {
	s.setNodes(make(map[taskref.TaskIdentity]bool))
}

// setNode writes one task's fold state, allocating the map on first write so no
// caller has to ask whether it exists yet.
func (s *foldState) setNode(identity taskref.TaskIdentity, expanded bool) {
	if s == nil {
		return
	}
	if s.ExpandedNodes == nil {
		s.ExpandedNodes = make(map[taskref.TaskIdentity]bool)
	}
	s.ExpandedNodes[identity] = expanded
}

// setSections adopts a captured section-fold map on the same terms as setNodes.
func (s *foldState) setSections(sections map[string]bool) {
	if s == nil {
		return
	}
	s.CollapsedSections = sections
}

// setSection writes one section or project-group key.
func (s *foldState) setSection(key string, collapsed bool) {
	if s == nil {
		return
	}
	if s.CollapsedSections == nil {
		s.CollapsedSections = make(map[string]bool)
	}
	s.CollapsedSections[key] = collapsed
}

// clearSection forgets one key entirely rather than writing false over it, so a
// reveal leaves no entry behind for a later namespace collision to read.
func (s *foldState) clearSection(key string) {
	if s == nil {
		return
	}
	delete(s.CollapsedSections, key)
}

// clearSections drops every section and project-group fold, and the anchor with
// them. The anchor names a section key to return the cursor to, so it cannot
// outlive the folds that made it necessary.
func (s *foldState) clearSections() {
	if s == nil {
		return
	}
	s.setSections(make(map[string]bool))
	s.SectionToggleRestore = nil
}

// setChecklists adopts a captured checklist-fold map on the same terms as
// setNodes.
func (s *foldState) setChecklists(checklists map[taskref.TaskIdentity]bool) {
	if s == nil {
		return
	}
	s.ExpandedChecklists = checklists
}

// setChecklist opens or closes one owner's checklist. Closing deletes rather
// than writing false, because absent is what collapsed means on this axis.
func (s *foldState) SetChecklist(identity taskref.TaskIdentity, expanded bool) {
	if s == nil {
		return
	}
	if !expanded {
		delete(s.ExpandedChecklists, identity)
		return
	}
	if s.ExpandedChecklists == nil {
		s.ExpandedChecklists = make(map[taskref.TaskIdentity]bool)
	}
	s.ExpandedChecklists[identity] = true
}

// setSectionToggleAnchor records, or drops, the task an s-driven collapse hid.
func (s *foldState) setSectionToggleAnchor(anchor *sectionToggleRestoreAnchor) {
	if s == nil {
		return
	}
	s.SectionToggleRestore = anchor
}

// takeSectionToggleAnchor consumes the anchor. It is one-shot by construction:
// the anchor describes one collapse/expand cycle, and reading it without
// clearing it would let the next expansion move a cursor the user has since
// placed somewhere else.
func (s *foldState) takeSectionToggleAnchor() *sectionToggleRestoreAnchor {
	if s == nil {
		return nil
	}
	anchor := s.SectionToggleRestore
	s.SectionToggleRestore = nil
	return anchor
}

// resetAll opens everything on all three axes and drops the anchor. It is the
// whole-projection reset, so it replaces each map rather than emptying it in
// place: a frame captured before the reset must not alias the state after it.
func (s *foldState) ResetAll() {
	if s == nil {
		return
	}
	s.resetNodes()
	s.clearSections()
	s.setChecklists(make(map[taskref.TaskIdentity]bool))
}

// populationState is the cross-project population the Outliner reads whenever
// m.project == nil, together with everything that replaces it and everything
// memoized over it. Four concerns, one owner, because none of them is
// separable from the others: the loaded project set, the lifecycle of the async
// read that installs a new one, where the cursor lands once that read replaces
// the rows underneath it, and the rebuild guard whose whole job is to notice
// that the set changed.
//
// The guard is the reason this is a named sub-state rather than promoted
// fields. Everything that changes what the population would produce has to say
// so, and it says so by calling invalidate(); "who invalidates the population"
// is then a call graph rather than an assignment grep. The zero value is a cold
// guard whose first pass always rebuilds, since an empty fingerprint cannot
// match a real one.
type populationState struct {
	// projects is the successfully loaded set every cross-project read walks.
	// snapshotReady records that a load was actually accepted, which is what lets
	// a legitimately empty workspace render as empty rather than as still
	// loading.
	Projects      []*models.Project
	SnapshotReady bool

	// loading is true while a read is in flight and loadGen is the generation
	// every result is matched against, so a superseded read publishes nothing.
	// spinner and spinnerVisible are the cold-load animation, held back until
	// crossSpinnerDelay has elapsed so a fast load never flashes one.
	Loading        bool
	LoadGen        int
	SpinnerVisible bool
	spinner        chrome.SpinnerModel

	// reloadProjectID and reloadTaskID name the cursor task captured before a
	// read starts, so the row it moved to can be found again afterwards.
	// reloadHeaderProjectID overrides that with one project sub-header, for the
	// mutations that delete every row the cursor was on. reloadFromTop is the
	// one-shot refusal to restore anything, for a change that makes the old
	// position meaningless rather than merely displaced.
	ReloadProjectID       string
	ReloadTaskID          string
	ReloadHeaderProjectID string
	reloadFromTop         bool

	// fingerprint, focusDepth and taskCount are the inputs the last
	// crossProjectRebuildRows pass ran under; forceRebuild is the one-shot flag
	// that bypasses them.
	fingerprint  string
	focusDepth   int
	taskCount    int
	ForceRebuild bool
}

// setProjects installs a new loaded population. It is the only write to the
// set, which is what lets setLoadedProjects below pair it with dropping the
// reading derived from it.
func (p *populationState) setProjects(projects []*models.Project) { p.Projects = projects }

// setSnapshotReady records whether the set standing here is an accepted
// snapshot. The three routes that publish a real load pass true; the in-memory
// baseline restore passes back the value it captured, because returning to a
// baseline taken before any load landed must not claim one did.
func (p *populationState) setSnapshotReady(ready bool) { p.SnapshotReady = ready }

// beginLoad opens a read and returns the generation it owns. The bump is what
// makes every earlier read safe to abandon: a result whose generation is no
// longer this one publishes nothing.
func (p *populationState) beginLoad() int {
	p.Loading = true
	p.LoadGen++
	return p.LoadGen
}

// supersede rejects whatever is in flight without opening a read of its own. It
// is the generation bump alone, for the callers that are abandoning a load
// rather than replacing it.
func (p *populationState) supersede() { p.LoadGen++ }

// settleLoad puts the read down. The spinner goes with it unconditionally: a
// load that is over cannot be the one being waited on, whether it landed, was
// canceled, or was refused.
func (p *populationState) settleLoad() {
	p.Loading = false
	p.SpinnerVisible = false
}

// resetSpinner arms a fresh animation for a cold load without showing it yet.
func (p *populationState) resetSpinner() { p.spinner = chrome.NewSpinner() }

// showSpinner reveals the cold-load animation once its anti-flicker delay has
// elapsed, restarting it so the first visible frame is the first frame.
func (p *populationState) showSpinner() {
	p.SpinnerVisible = true
	p.resetSpinner()
}

// advanceSpinner steps the animation one tick and hands back the Cmd that asks
// for the next one.
func (p *populationState) advanceSpinner(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.spinner, cmd = p.spinner.Update(msg)
	return cmd
}

// captureReloadCursor records the task the cursor holds, so the read about to
// start can be told where the user was standing when it began.
func (p *populationState) captureReloadCursor(projectID, taskID string) {
	p.ReloadProjectID = projectID
	p.ReloadTaskID = taskID
}

// clearReloadCursor drops the captured task without touching the header anchor
// or the from-top flag. It is what a read that published nothing owes: the
// capture describes rows that were never replaced, so restoring it later would
// move a cursor nobody asked to move.
func (p *populationState) clearReloadCursor() {
	p.ReloadProjectID = ""
	p.ReloadTaskID = ""
}

// anchorReloadHeader names the project sub-header the cursor must land on after
// the next read, overriding any captured task. Set by the mutations that remove
// the rows the cursor was standing on.
func (p *populationState) AnchorReloadHeader(projectID string) { p.ReloadHeaderProjectID = projectID }

// requestReloadFromTop declares the next read's population different enough that
// the old position means nothing, so nothing should be restored onto it.
func (p *populationState) RequestReloadFromTop() { p.reloadFromTop = true }

// consumeReloadFromTop reports whether the read now starting must land at the
// top, and spends the flag if so. The stale capture goes with it, because a
// from-top read is exactly the one whose captured row must not come back.
func (p *populationState) consumeReloadFromTop() bool {
	if !p.reloadFromTop {
		return false
	}
	p.clearReloadCursor()
	p.reloadFromTop = false
	return true
}

// takeReloadTarget hands back where the cursor should land and spends every
// target in one act, so the restore that follows cannot be replayed by the next
// read. The header anchor outranks the captured task; the caller decides that,
// because only it knows what to do when neither resolves.
func (p *populationState) takeReloadTarget() (projectID, taskID, headerProjectID string) {
	projectID, taskID, headerProjectID = p.ReloadProjectID, p.ReloadTaskID, p.ReloadHeaderProjectID
	p.clearReloadCursor()
	p.ReloadHeaderProjectID = ""
	return projectID, taskID, headerProjectID
}

// rebaseReloadCursor re-aims the read already in flight at one task, dropping
// the header anchor the projection it was captured in had asked for. The
// from-top flag is deliberately left standing: it is an intent for the next
// read, not a description of the one running.
func (p *populationState) rebaseReloadCursor(projectID, taskID string) {
	p.captureReloadCursor(projectID, taskID)
	p.ReloadHeaderProjectID = ""
}

// clearReloadTarget forgets every reload intent, including the from-top flag.
// An abandoned read leaves no successor to honor them.
func (p *populationState) clearReloadTarget() {
	p.clearReloadCursor()
	p.ReloadHeaderProjectID = ""
	p.reloadFromTop = false
}

// invalidate declares the loaded population, or the projection taken over it,
// changed underneath the guard, so the next rebuild runs whatever the guard
// inputs say. It is one-shot: the rebuild that honors it consumes it through
// record.
func (p *populationState) Invalidate() { p.ForceRebuild = true }

// memoized reports whether the pass about to run would reproduce the rows the
// last one already built. A standing force flag answers no on its own, before
// any input is compared.
func (p *populationState) memoized(fingerprint string, focusDepth, taskCount int) bool {
	return !p.ForceRebuild &&
		fingerprint == p.fingerprint &&
		focusDepth == p.focusDepth &&
		taskCount == p.taskCount
}

// record stores the guard inputs the running pass is rebuilding under and
// consumes the force flag, so one invalidate() buys exactly one rebuild.
func (p *populationState) record(fingerprint string, focusDepth, taskCount int) {
	p.fingerprint = fingerprint
	p.focusDepth = focusDepth
	p.taskCount = taskCount
	p.ForceRebuild = false
}

// recordFocusDepth re-records the depth alone. Focus-stack validation runs after
// record has already stored the depth the guard was entered with, and a repaired
// stack is the depth the next pass must compare against or that pass rebuilds
// for nothing.
func (p *populationState) recordFocusDepth(focusDepth int) { p.focusDepth = focusDepth }

// populationSnapshot is the part of this state a restoration point carries: the
// loaded set and whether it was ever an accepted one. The read lifecycle, the
// reload targets and the memo guard are all deliberately absent — a frame
// describes a population, not a load, and reinstating a captured "loading" or a
// captured generation would restart a read that already settled.
//
// The set is retained by value, which is what makes returning from a lens that
// widened the population a pure in-memory step rather than a second disk read
// that could fail, be slow, or land after the user moved on.
type populationSnapshot struct {
	Projects      []*models.Project
	SnapshotReady bool
}

// capture copies the set out so the retained frame can never alias a slice a
// later load replaces underneath it.
func (p *populationState) capture() populationSnapshot {
	if p == nil {
		return populationSnapshot{}
	}
	return populationSnapshot{Projects: slices.Clone(p.Projects), SnapshotReady: p.SnapshotReady}
}

// setLoadedProjects installs a new loaded population. It exists so replacing
// that population and dropping the reading derived from it are one act: a
// caller that assigned the slice directly would leave an index describing
// projects the Outliner no longer holds until the next rebuild pass. The
// dependency index itself stays on the model, because it is built over the
// single-project fallback too and so is not the population's to hold.
func (m *OutlinerModel) setLoadedProjects(projects []*models.Project) {
	if m == nil {
		return
	}
	m.Population.setProjects(projects)
	m.invalidateDependencyIndex()
}

// restorePopulation reinstalls a retained population and the accepted-snapshot
// flag that stood with it. It goes through setLoadedProjects rather than writing
// the set, because reinstating a population owes the dependency index the same
// drop a load does and there is one place that pairing is stated. The flag
// travels beside the set for the same reason it is captured beside it: returning
// to a baseline taken before any load landed must not claim one did.
//
// The membership is the frame's and the objects are the workspace's, which is
// what refreshCapturedProjects below is for: a frame retained across a reload
// names the same projects, and reinstating the objects it captured with them
// would put the task data that stood before the reload back on screen.
func (m *OutlinerModel) restorePopulation(snapshot populationSnapshot) {
	if m == nil {
		return
	}
	m.setLoadedProjects(m.refreshCapturedProjects(snapshot.Projects))
	m.Population.setSnapshotReady(snapshot.SnapshotReady)
}

// refreshCapturedProjects re-resolves a retained set against the projects
// standing now, by ID. A restoration point is a description of a view, and the
// only part of a population it may speak for is which projects were in it: a
// reload replaces every object it read, so a frame captured before one holds
// objects whose tasks, titles, and states are a reading the workspace has since
// superseded.
//
// A captured project the live set no longer holds keeps its own object, because
// there is no newer reading of it here to prefer — a lens that narrowed the
// population is exactly that case, and its baseline is entitled to the projects
// the lens excluded. Nil entries travel unchanged so the restored set is the
// captured one member for member.
func (m *OutlinerModel) refreshCapturedProjects(captured []*models.Project) []*models.Project {
	if len(captured) == 0 {
		return slices.Clone(captured)
	}
	live := make(map[string]*models.Project, len(m.Population.Projects))
	for _, project := range m.Population.Projects {
		if project != nil && project.ID != "" {
			live[project.ID] = project
		}
	}
	refreshed := make([]*models.Project, len(captured))
	for index, project := range captured {
		refreshed[index] = project
		if project == nil {
			continue
		}
		if current, loaded := live[project.ID]; loaded {
			refreshed[index] = current
		}
	}
	return refreshed
}

// queryReadoutState is the expanded readout's whole sub-state. One flag is all
// there is: the rows themselves are read off the live axes each frame by the
// pure queryReadoutBody that statusStripHeight measures, so there is no second
// copy of the readout here that could go stale against the view it describes.
//
// It is a named field on OutlinerModel rather than a promoted bool for the same
// reason the menu bar beside it is: the flag is written in exactly two places
// and read by the gate, the guidance, and the body, and naming the cluster is
// what lets those readers say which sub-state they are asking about.
type queryReadoutState struct {
	Expanded bool
}

// toggle opens or closes the stack. It is the whole of the operation's handler:
// the strip reads the flag when it next renders.
func (s *queryReadoutState) toggle() {
	if s == nil {
		return
	}
	s.Expanded = !s.Expanded
}

// close puts the stack down without asking what it was showing. The resets that
// replace the whole projection call it, so a readout cannot survive into a view
// whose axes it was never built from.
func (s *queryReadoutState) close() {
	if s == nil {
		return
	}
	s.Expanded = false
}
