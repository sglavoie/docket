package outliner

import (
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/query"
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

	// unreadable names the projects an accepted read asked for and could not
	// read, each entry standing until a later accepted read opens that project.
	// It is the other half of that answer: a project absent from projects because
	// the workspace failed to read it is not a project the workspace stopped
	// offering, and nothing else here records the difference.
	unreadable map[string]bool

	// fingerprint, focusDepth and taskCount are the inputs the last
	// crossProjectRebuildRows pass ran under; forceRebuild is the one-shot flag
	// that bypasses them.
	fingerprint  string
	focusDepth   int
	taskCount    int
	ForceRebuild bool
	revision     uint64
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

// recordLoadFailures folds one accepted read's answers into the record, project
// by project. A read speaks for the projects it attempted and for no others, so
// the two halves it publishes are the whole of what it may write: a project in
// the population it installed is one it opened, and its failure is cleared, while
// a project in its failures is one it could not open, and that is recorded. A
// project the read never asked for is untouched, which is what makes a narrower
// read — one project's lens, a filtered reload — no evidence at all about the
// projects outside it. Replacing the record whole was the defect: it made an
// unrelated success stand in for a recovery nothing observed.
//
// It is written by the accepted publishes alone. A read that publishes no
// population — superseded, abandoned, or refused — records nothing here either,
// so every answer standing belongs to an accepted read of the project it names.
func (p *populationState) recordLoadFailures(loaded []*models.Project, failures []ProjectLoadFailure) {
	if p == nil {
		return
	}
	for _, project := range loaded {
		if project != nil && project.ID != "" {
			delete(p.unreadable, project.ID)
		}
	}
	for _, failure := range failures {
		if failure.ProjectID == "" {
			continue
		}
		if p.unreadable == nil {
			p.unreadable = make(map[string]bool, len(failures))
		}
		p.unreadable[failure.ProjectID] = true
	}
	if len(p.unreadable) == 0 {
		p.unreadable = nil
	}
}

// isUnreadable reports whether the last accepted read that asked for this
// project failed to open it.
func (p *populationState) isUnreadable(projectID string) bool {
	if p == nil || projectID == "" {
		return false
	}
	return p.unreadable[projectID]
}

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
func (p *populationState) Invalidate() { p.ForceRebuild = true; p.revision++ }

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
//
// focusProjectID is the parked exception the restored view will stand in, which
// the frame's own scope names. It travels with the population because it is half
// of the rule that decides which projects the workspace still admits, and the
// scope it comes from is not restored until after this write.
func (m *OutlinerModel) restorePopulation(snapshot populationSnapshot, focusProjectID string) {
	if m == nil {
		return
	}
	m.setLoadedProjects(m.refreshCapturedProjects(snapshot.Projects, focusProjectID))
	m.Population.setSnapshotReady(snapshot.SnapshotReady)
}

// refreshCapturedProjects re-resolves a retained set against the projects
// standing now, by ID. A restoration point is a description of a view, and the
// only part of a population it may speak for is which projects were in it: a
// reload replaces every object it read, so a frame captured before one holds
// objects whose tasks, titles, and states are a reading the workspace has since
// superseded.
//
// Membership and object are two separate questions here, and asking them in
// that order is the whole of this function. Membership is the restored frame's
// own: a project the workspace no longer offers this view — deleted, archived,
// or parked while the frame was away — is not a reading the frame may speak for
// at all, and whether the destination happened to load it decides nothing. A
// lens that widened the population, or a destination reached under axes of its
// own, can perfectly well be holding a project the frame being restored excludes
// by lifecycle, so the eligibility the catalog answers is asked of every
// captured candidate rather than of absent ones alone. Otherwise a withdrawal is
// caught only when the destination also stopped loading it, which makes the rule
// depend on the other view's axes instead of on this frame's.
//
// Only once a candidate is a member does its object get chosen, and there the
// live set is preferred, since it is the reading that superseded the captured
// one. An absent member is then one of two remaining things. A lens that
// narrowed the population merely excluded it: no newer reading of it is here to
// prefer, and the baseline is entitled to hand it back with the tasks it
// captured, under the description the catalog holds now. Or it is the project an
// accepted read did go and ask for and could not open, with nothing having
// opened it since: the catalog still lists it, so availability alone calls it
// eligible, but the workspace has already answered about it and the answer was
// that its tasks cannot be read. Handing the captured ones back would publish a
// reading the workspace has just refused to confirm, so it leaves the set too,
// and a restoration is the captured membership intersected with what is still
// offered and still readable.
func (m *OutlinerModel) refreshCapturedProjects(captured []*models.Project, focusProjectID string) []*models.Project {
	if len(captured) == 0 {
		return slices.Clone(captured)
	}
	live := make(map[string]*models.Project, len(m.Population.Projects))
	for _, project := range m.Population.Projects {
		if project != nil && project.ID != "" {
			live[project.ID] = project
		}
	}
	available, judged := m.availableProjectCatalog(focusProjectID)
	refreshed := make([]*models.Project, 0, len(captured))
	for _, project := range captured {
		if project == nil {
			continue
		}
		current, stillAvailable := available[project.ID]
		if judged && !stillAvailable {
			continue
		}
		if loadedNow, loaded := live[project.ID]; loaded {
			refreshed = append(refreshed, loadedNow)
			continue
		}
		// The read failure is an observation about this project's tasks, which no
		// lifecycle axis and no missing runtime can speak for, and it outranks even
		// the focus the catalog admits by name — an exception says the view may read
		// that project, never that the read succeeded. It survives every later read
		// that did not ask for this project, so a narrower one cannot expire it, and
		// only a read that opened the project can. It is asked of retained candidates
		// alone: a project the live population holds was opened by the read that
		// installed it, and the object standing there is that reading.
		if m.Population.isUnreadable(project.ID) {
			continue
		}
		refreshed = append(refreshed, retainedProjectWithCurrentMetadata(project, current))
	}
	return refreshed
}

// availableProjectCatalog answers which of the catalog's projects the view being
// restored could still read, mapped by ID to the object carrying their current
// metadata. It asks query.ProjectLifecycleAdmits, the population half of the
// facets and the one authority on it, over the withdrawal reading of the frame's
// own axes: a project is gone when the catalog no longer holds it, or when it
// has since become the thing that frame's parked or archived axis excludes. The
// descriptive facets are deliberately not consulted: a tag restriction is lens
// criteria, and a project a criterion merely excluded is exactly the one a
// baseline is entitled to bring back.
//
// The frame's own axes are what it reads, which is why the population is
// restored after adoptPopulationNarrowing and is handed the focus its scope
// names: the scope itself is not written back until later. That focus is the
// parked exception the restored view stands in, and it is admitted by name,
// because a view standing in a parked project reads it whatever its parked axis
// says.
//
// The second return is false for a nil runtime alone — a model with no catalog
// to ask cannot call a project gone, so it judges nothing and keeps every
// captured member as it always did.
func (m *OutlinerModel) availableProjectCatalog(focusProjectID string) (map[string]*models.Project, bool) {
	if m == nil || m.Runtime == nil {
		return nil, false
	}
	catalog := m.Runtime.AllProjects()
	available := make(map[string]*models.Project, len(catalog))
	axes := withdrawalAxes(m.ProjectFilter)
	for _, project := range catalog {
		if project == nil || project.ID == "" {
			continue
		}
		admitting := axes
		if project.ID == focusProjectID {
			admitting.ParkedMode = query.ProjectPopulationInclude
		}
		if !query.ProjectLifecycleAdmits(project, admitting) {
			continue
		}
		available[project.ID] = project
	}
	return available, true
}

// withdrawalAxes reads a frame's population axes in the one direction a
// restoration cares about. An axis is asked whether the workspace has taken a
// project out of this view's reach, not whether the view would load it from
// scratch: a member that stopped being parked or archived is still the member
// the frame captured, and a frame is entitled to the membership it holds even
// when its own axes and its population disagree, which they do for as long as an
// axis change is still loading. So an isolating axis widens to Include here and
// only an excluding one refuses.
func withdrawalAxes(filter query.ProjectFilterState) query.ProjectFilterState {
	filter.ParkedMode = withdrawalMode(filter.ParkedMode)
	filter.ArchivedMode = withdrawalMode(filter.ArchivedMode)
	return filter
}

func withdrawalMode(mode query.ProjectPopulationMode) query.ProjectPopulationMode {
	if mode == query.ProjectPopulationOnly {
		return query.ProjectPopulationInclude
	}
	return mode
}

// retainedProjectWithCurrentMetadata keeps the captured reading of a project's
// tasks under the catalog's current description of the project itself. The two
// halves have different owners: only a disk read can refresh the tasks and none
// happened here, while the name, tags, pin, and path are the catalog's and are
// current for free. The copy is what keeps it honest — the captured object is
// the frame's and is left exactly as it was retained.
func retainedProjectWithCurrentMetadata(retained, current *models.Project) *models.Project {
	if retained == nil || current == nil {
		return retained
	}
	refreshed := *retained
	coordinator.CopyCatalogMetadata(&refreshed, current)
	return &refreshed
}
