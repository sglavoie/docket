package outliner

import (
	"slices"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
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

// queryReadoutState is the expanded readout's whole sub-state: whether it is
// open, and where its window sits. The rows themselves are still read off the
// live axes each frame by the pure queryReadoutBody that statusStripHeight
// measures, so there is no second copy of the readout here that could go stale
// against the view it describes.
//
// Offset is a line offset into that document and nothing else. It is not a
// cursor: the readout has no selection, the digits address axes rather than
// screen rows, and the outline's own cursor is untouched by every key the
// readout claims. It is session-only and it is put down with the readout,
// because a window into a document built from axes that have since changed is a
// position remembered about nothing.
//
// It is a named field on OutlinerModel rather than promoted fields for the same
// reason the menu bar beside it is: the flag is written in a handful of places
// and read by the gate, the guidance, and the body, and naming the cluster is
// what lets those readers say which sub-state they are asking about.
type queryReadoutState struct {
	Expanded bool
	Offset   int
}

// toggle opens or closes the stack. It is the whole of the operation's handler:
// the strip reads the flag when it next renders.
func (s *queryReadoutState) toggle() {
	if s == nil {
		return
	}
	s.Expanded = !s.Expanded
	s.Offset = 0
}

// close puts the stack down without asking what it was showing. The resets that
// replace the whole projection call it, so a readout cannot survive into a view
// whose axes it was never built from.
func (s *queryReadoutState) close() {
	if s == nil {
		return
	}
	s.Expanded = false
	s.Offset = 0
}
