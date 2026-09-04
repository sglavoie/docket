package tui

import (
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

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
	projects      []*models.Project
	snapshotReady bool

	// loading is true while a read is in flight and loadGen is the generation
	// every result is matched against, so a superseded read publishes nothing.
	// spinner and spinnerVisible are the cold-load animation, held back until
	// crossSpinnerDelay has elapsed so a fast load never flashes one.
	loading        bool
	loadGen        int
	spinnerVisible bool
	spinner        chrome.SpinnerModel

	// reloadProjectID and reloadTaskID name the cursor task captured before a
	// read starts, so the row it moved to can be found again afterwards.
	// reloadHeaderProjectID overrides that with one project sub-header, for the
	// mutations that delete every row the cursor was on. reloadFromTop is the
	// one-shot refusal to restore anything, for a change that makes the old
	// position meaningless rather than merely displaced.
	reloadProjectID       string
	reloadTaskID          string
	reloadHeaderProjectID string
	reloadFromTop         bool

	// fingerprint, focusDepth and taskCount are the inputs the last
	// crossProjectRebuildRows pass ran under; forceRebuild is the one-shot flag
	// that bypasses them.
	fingerprint  string
	focusDepth   int
	taskCount    int
	forceRebuild bool
}

// setProjects installs a new loaded population. It is the only write to the
// set, which is what lets setLoadedProjects below pair it with dropping the
// reading derived from it.
func (p *populationState) setProjects(projects []*models.Project) { p.projects = projects }

// setSnapshotReady records whether the set standing here is an accepted
// snapshot. The three routes that publish a real load pass true; the in-memory
// baseline restore passes back the value it captured, because returning to a
// baseline taken before any load landed must not claim one did.
func (p *populationState) setSnapshotReady(ready bool) { p.snapshotReady = ready }

// beginLoad opens a read and returns the generation it owns. The bump is what
// makes every earlier read safe to abandon: a result whose generation is no
// longer this one publishes nothing.
func (p *populationState) beginLoad() int {
	p.loading = true
	p.loadGen++
	return p.loadGen
}

// supersede rejects whatever is in flight without opening a read of its own. It
// is the generation bump alone, for the callers that are abandoning a load
// rather than replacing it.
func (p *populationState) supersede() { p.loadGen++ }

// settleLoad puts the read down. The spinner goes with it unconditionally: a
// load that is over cannot be the one being waited on, whether it landed, was
// canceled, or was refused.
func (p *populationState) settleLoad() {
	p.loading = false
	p.spinnerVisible = false
}

// resetSpinner arms a fresh animation for a cold load without showing it yet.
func (p *populationState) resetSpinner() { p.spinner = chrome.NewSpinner() }

// showSpinner reveals the cold-load animation once its anti-flicker delay has
// elapsed, restarting it so the first visible frame is the first frame.
func (p *populationState) showSpinner() {
	p.spinnerVisible = true
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
	p.reloadProjectID = projectID
	p.reloadTaskID = taskID
}

// clearReloadCursor drops the captured task without touching the header anchor
// or the from-top flag. It is what a read that published nothing owes: the
// capture describes rows that were never replaced, so restoring it later would
// move a cursor nobody asked to move.
func (p *populationState) clearReloadCursor() {
	p.reloadProjectID = ""
	p.reloadTaskID = ""
}

// anchorReloadHeader names the project sub-header the cursor must land on after
// the next read, overriding any captured task. Set by the mutations that remove
// the rows the cursor was standing on.
func (p *populationState) anchorReloadHeader(projectID string) { p.reloadHeaderProjectID = projectID }

// requestReloadFromTop declares the next read's population different enough that
// the old position means nothing, so nothing should be restored onto it.
func (p *populationState) requestReloadFromTop() { p.reloadFromTop = true }

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
	projectID, taskID, headerProjectID = p.reloadProjectID, p.reloadTaskID, p.reloadHeaderProjectID
	p.clearReloadCursor()
	p.reloadHeaderProjectID = ""
	return projectID, taskID, headerProjectID
}

// clearReloadTarget forgets every reload intent, including the from-top flag.
// An abandoned read leaves no successor to honor them.
func (p *populationState) clearReloadTarget() {
	p.clearReloadCursor()
	p.reloadHeaderProjectID = ""
	p.reloadFromTop = false
}

// invalidate declares the loaded population, or the projection taken over it,
// changed underneath the guard, so the next rebuild runs whatever the guard
// inputs say. It is one-shot: the rebuild that honors it consumes it through
// record.
func (p *populationState) invalidate() { p.forceRebuild = true }

// memoized reports whether the pass about to run would reproduce the rows the
// last one already built. A standing force flag answers no on its own, before
// any input is compared.
func (p *populationState) memoized(fingerprint string, focusDepth, taskCount int) bool {
	return !p.forceRebuild &&
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
	p.forceRebuild = false
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
	projects      []*models.Project
	snapshotReady bool
}

// capture copies the set out so the retained frame can never alias a slice a
// later load replaces underneath it.
func (p *populationState) capture() populationSnapshot {
	if p == nil {
		return populationSnapshot{}
	}
	return populationSnapshot{projects: slices.Clone(p.projects), snapshotReady: p.snapshotReady}
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
	m.population.setProjects(projects)
	m.invalidateDependencyIndex()
}

// restorePopulation reinstalls a retained population and the accepted-snapshot
// flag that stood with it. It goes through setLoadedProjects rather than writing
// the set, because reinstating a population owes the dependency index the same
// drop a load does and there is one place that pairing is stated. The flag
// travels beside the set for the same reason it is captured beside it: returning
// to a baseline taken before any load landed must not claim one did.
func (m *OutlinerModel) restorePopulation(snapshot populationSnapshot) {
	if m == nil {
		return
	}
	m.setLoadedProjects(slices.Clone(snapshot.projects))
	m.population.setSnapshotReady(snapshot.snapshotReady)
}
