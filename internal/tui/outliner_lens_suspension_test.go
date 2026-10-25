package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/query"
)

// A temporary search under a lens is a visit, not an edit. These assertions are
// about what the visit costs: the lens stops narrowing while it stands, nothing
// about it is written anywhere, and one Esc puts back the view the slash was
// pressed in — mode, query, facets, folds, cursor, selection, and Peek — against
// whatever data the workspace holds by then.

func TestSlashSuspendsTheLensAndEscRestoresItUnchanged(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", "needle"))

	installedMode := outlinerView.ActiveOutlinerMode()
	installedFilters := outlinerView.ActiveFilterCount()
	installedFingerprint := outlinerView.Lens.Active.SavedState.Fingerprint()

	outlinerView.ActivateSearch()
	if outlinerView.HasActiveLens() {
		t.Fatal("the slash left the lens owning the projection")
	}
	if !outlinerView.HasSuspendedLens() {
		t.Fatal("the slash recorded no suspension")
	}
	if got := outlinerView.ActiveLensLabel(); got != "Lens: Waiting (suspended)" {
		t.Fatalf("suspended label = %q, want the lens named as suspended", got)
	}

	// Everything a visit does: type a query, move, select, and open Peek. None of
	// it is the lens's, so none of it may survive the restore.
	typeIntoOutlinerSearch(t, outlinerView, "beta")
	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: 'j', Text: "j"})
	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: 'v', Text: "v"})
	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: 'b', Text: "b"})

	outlinerView.ClearSearch()

	if !outlinerView.HasActiveLens() || outlinerView.HasSuspendedLens() {
		t.Fatalf("Esc did not put the lens back: active=%v suspended=%v",
			outlinerView.HasActiveLens(), outlinerView.HasSuspendedLens())
	}
	if outlinerView.HasSearchProjection() || outlinerView.SearchQuery() != "" {
		t.Fatalf("the temporary search outlived its restore: projection=%v query=%q",
			outlinerView.HasSearchProjection(), outlinerView.SearchQuery())
	}
	if outlinerView.Lens.Query != "needle" {
		t.Fatalf("restored lens query = %q, want the one the lens installed", outlinerView.Lens.Query)
	}
	if got := outlinerView.ActiveOutlinerMode(); got != installedMode {
		t.Fatalf("restored mode = %q, want %q", got, installedMode)
	}
	if got := outlinerView.ActiveFilterCount(); got != installedFilters {
		t.Fatalf("restored facet count = %d, want %d", got, installedFilters)
	}
	if outlinerView.Lens.Active.SavedState.Fingerprint() != installedFingerprint {
		t.Fatal("the suspension moved the fingerprint the modified marker is read against")
	}
	if outlinerView.LensModified() {
		t.Fatal("a visit marked the lens modified")
	}
	if outlinerView.PeekMode || outlinerView.IsMultiSelectActive() || outlinerView.SelectedCount() != 0 {
		t.Fatalf("the visit's Peek and selection outlived it: peek=%v select=%v selected=%d",
			outlinerView.PeekMode, outlinerView.IsMultiSelectActive(), outlinerView.SelectedCount())
	}
	if got := outlinerView.ActiveLensLabel(); got != "Lens: Waiting" {
		t.Fatalf("restored label = %q, want the plain lens name", got)
	}
}

// A second slash edits the query already being typed, so it finds no lens to
// take and must leave the first one's suspension standing: one Esc, one restore.
func TestRepeatedSlashKeepsOneLensSuspension(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", "needle"))

	outlinerView.ActivateSearch()
	typeIntoOutlinerSearch(t, outlinerView, "beta")
	outlinerView.ActivateSearch()

	if !outlinerView.HasSuspendedLens() {
		t.Fatal("the second slash dropped the suspension the first one took")
	}
	if got := outlinerView.SearchInputValue(); got != "beta" {
		t.Fatalf("the second slash restated the query as %q, want the held beta", got)
	}

	outlinerView.ClearSearch()
	if !outlinerView.HasActiveLens() || outlinerView.Lens.Query != "needle" {
		t.Fatalf("one Esc did not restore the lens: active=%v query=%q",
			outlinerView.HasActiveLens(), outlinerView.Lens.Query)
	}
	if outlinerView.HasSuspendedLens() {
		t.Fatal("the restore left a second suspension behind")
	}
}

// The frame names no task and no project, so a restore aims at a cursor anchor
// and lands on the nearest surviving row when the workspace moved underneath it.
// This is the asynchronous-refresh case: a reload replaces the population while
// the search stands, and the lens still comes back applied to current data.
func TestASuspendedLensRestoresAgainstCurrentData(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", ""))

	outlinerView.ActivateSearch()
	typeIntoOutlinerSearch(t, outlinerView, "needle")
	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	// The reload the shell would publish: a fresh population, one Waiting task
	// short of the one the lens was standing on.
	survivor := primaryRowsTask("beta-waiting", models.StateWaiting)
	survivor.Title = "Beta needle"
	triage := primaryRowsTask("beta-inbox", models.StateInbox)
	triage.Title = "Beta triage"
	outlinerView.Population.Projects = []*models.Project{
		primaryRowsProject("alpha", primaryRowsTask("doing", models.StateDoing)),
		primaryRowsProject("beta", survivor, triage),
	}
	outlinerView.Population.Invalidate()
	outlinerView.CrossProjectRebuildRows()

	outlinerView.ClearSearch()

	if !outlinerView.HasActiveLens() {
		t.Fatal("a reload during the search cost the suspension its lens")
	}
	if got := searchVisibleTaskIDs(outlinerView); len(got) != 1 || got[0] != "beta-waiting" {
		t.Fatalf("restored rows = %v, want the lens re-applied to the current population", got)
	}
	if outlinerView.Cursor < 0 || outlinerView.Cursor >= len(outlinerView.Rows) {
		t.Fatalf("restored cursor %d is outside the %d rebuilt rows", outlinerView.Cursor, len(outlinerView.Rows))
	}
}

// Suspension is not a lens being put down, so the machine's resume pointer is
// never written: the next launch opens the lens whatever the search did, and a
// restart in the middle of one is indistinguishable from a restart beside it.
func TestSuspendingALensNeverWritesTheActiveLensPointer(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	var persisted []string
	outlinerView.Lens.Persist = func(id string) error {
		persisted = append(persisted, id)
		return nil
	}
	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", "needle"))
	afterApply := len(persisted)

	outlinerView.ActivateSearch()
	typeIntoOutlinerSearch(t, outlinerView, "beta")
	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	outlinerView.ClearSearch()

	if got := persisted[afterApply:]; len(got) != 0 {
		t.Fatalf("suspending and restoring wrote the active-lens pointer: %v", got)
	}
}

// The marker is a comparison the suspended view cannot make for itself, so it is
// read while the lens is still live and reported from the frame. A dirty lens
// stays dirty across the visit, and the divergence that made it dirty is still
// there when it comes back.
func TestASuspendedDirtyLensKeepsItsModifiedMarker(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", "needle"))
	outlinerView.Filter.Priority = int(models.PriorityHigh)
	adoptFilterCarriers(outlinerView)
	if !outlinerView.LensModified() {
		t.Fatal("the fixture did not diverge the lens")
	}

	outlinerView.ActivateSearch()
	if got := outlinerView.ActiveLensLabel(); got != "Lens: Waiting* (suspended)" {
		t.Fatalf("suspended label = %q, want the divergence marked", got)
	}

	outlinerView.ClearSearch()
	if !outlinerView.LensModified() {
		t.Fatal("the restore silently adopted the divergence it was holding")
	}
	if outlinerView.Filter.Priority != int(models.PriorityHigh) {
		t.Fatalf("restored priority facet = %d, want the diverged one", outlinerView.Filter.Priority)
	}
}

// The `0` reset clears the lens rather than returning to it, so a suspension is
// dropped whole: the key that empties every axis cannot leave one Esc away from
// a projection it exists to remove. It says so, too — a suspended lens is still
// a lens the reset has work to do about, and a reset that named only the query
// would report less than it did.
func TestTheLaunchViewResetDropsASuspendedLens(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", "needle"))
	outlinerView.ActivateSearch()
	typeIntoOutlinerSearch(t, outlinerView, "beta")
	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	// The second result is whether the reset owes a population reload, not whether
	// it found work: the axes it names are the claim under test.
	cleared, _ := outlinerView.ResetToLaunchView()
	if !containsString(cleared, "lens") {
		t.Fatalf("the reset named %v, want the suspended lens among them", cleared)
	}

	if outlinerView.HasActiveLens() || outlinerView.HasSuspendedLens() {
		t.Fatalf("the reset retained a lens: active=%v suspended=%v",
			outlinerView.HasActiveLens(), outlinerView.HasSuspendedLens())
	}
	if outlinerView.HasSearchProjection() || outlinerView.PrimaryMode != chrome.ModeAll {
		t.Fatalf("the reset retained the search axes: projection=%v mode=%q",
			outlinerView.HasSearchProjection(), outlinerView.PrimaryMode)
	}
}

func TestStartupAndGlobalResetRetainNoLensFrame(t *testing.T) {
	fresh := outliner.NewOutlinerModel(nil, nil, testConfigWithProjects())
	if fresh.HasActiveLens() || fresh.Lens.Pending != nil || fresh.Lens.Query != "" {
		t.Fatal("a fresh Outliner must open with no lens")
	}

	outlinerView := lensLifecycleOutliner()
	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", "needle"))
	outlinerView.ResetToGlobalAll()

	if outlinerView.HasActiveLens() || outlinerView.Lens.Pending != nil {
		t.Fatal("the global reset retained a lens frame")
	}
	if outlinerView.Lens.Query != "" {
		t.Fatalf("the global reset retained the narrowing query %q", outlinerView.Lens.Query)
	}
	if outlinerView.PrimaryMode != chrome.ModeAll || outlinerView.Filter.HasStateFilter() {
		t.Fatal("the global reset did not clear the lens-installed axes")
	}
}

// The frame holds the installed identity and fingerprint, never the live view:
// diverging an axis moves the capture and leaves the frame exactly where the
// application put it, which is the whole of how the modified marker is decided.
func TestTheActiveFrameKeepsTheInstalledIdentityAndFingerprint(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	saved := waitingLens(lensTestID(1), "Waiting", "needle")
	applyLensOrFatal(t, outlinerView, saved)

	installed := outlinerView.Lens.Active.SavedState.Fingerprint()
	if outlinerView.LensModified() {
		t.Fatal("a freshly applied lens reported itself modified")
	}

	// Diverging the live view must not move the installed fingerprint.
	outlinerView.Filter.Priority = int(models.PriorityHigh)
	adoptFilterCarriers(outlinerView)

	if outlinerView.Lens.Active.LensID != saved.ID || outlinerView.Lens.Active.Name != saved.Name {
		t.Fatalf("frame identity = %q/%q, want %q/%q",
			outlinerView.Lens.Active.LensID, outlinerView.Lens.Active.Name, saved.ID, saved.Name)
	}
	if outlinerView.Lens.Active.SavedState.Fingerprint() != installed {
		t.Fatal("the installed fingerprint absorbed a live edit")
	}
	if !outlinerView.LensModified() {
		t.Fatal("a diverged view did not report itself modified")
	}
}

// A relationship hop taken from inside a temporary search is the composition
// case: the hop puts the search down exactly as it puts a lens down, so the
// suspension has to travel inside the trail's own checkpoint rather than beside
// it. Three Escs come back out in the order they were entered — the trail, the
// search, and only then the lens the slash was pressed under.
func TestARelationHopFromATemporarySearchCarriesTheSuspension(t *testing.T) {
	env := newRelationLifecycleEnv(t)
	outlinerView := env.outliner()
	applyLensOrFatal(t, outlinerView, unfilteredLens(lensTestID(2), "Everything"))

	env.press(t, "/")
	env.press(t, "i", "m", "p", "o", "r", "t", "e", "r")
	env.press(t, "enter")
	if !outlinerView.HasSuspendedLens() || outlinerView.HasActiveLens() {
		t.Fatalf("the slash under the lens did not suspend it: suspended=%v active=%v",
			outlinerView.HasSuspendedLens(), outlinerView.HasActiveLens())
	}

	env.hop(t)
	if outlinerView.HasSuspendedLens() || outlinerView.HasActiveLens() {
		t.Fatalf("the hop kept lens state at the far end: suspended=%v active=%v",
			outlinerView.HasSuspendedLens(), outlinerView.HasActiveLens())
	}
	if !outlinerView.HasRelationTrail() {
		t.Fatal("the hop recorded no return trail")
	}

	env.press(t, "esc")
	if !outlinerView.HasSearchProjection() || !outlinerView.HasSuspendedLens() {
		t.Fatalf("the return did not hand back the suspended search: projection=%v suspended=%v",
			outlinerView.HasSearchProjection(), outlinerView.HasSuspendedLens())
	}
	if outlinerView.HasActiveLens() {
		t.Fatal("the return reinstated the lens a search was still standing in front of")
	}

	env.press(t, "esc")
	if !outlinerView.HasActiveLens() || outlinerView.HasSuspendedLens() || outlinerView.HasSearchProjection() {
		t.Fatalf("the last Esc did not restore the lens alone: active=%v suspended=%v projection=%v",
			outlinerView.HasActiveLens(), outlinerView.HasSuspendedLens(), outlinerView.HasSearchProjection())
	}
}

// unfilteredLens is a saved definition that narrows nothing: the suspension
// composition tests need the lens to own the projection without hiding the rows
// the search and the hop are driven over.
func unfilteredLens(id, name string) config.NamedLens {
	facets, err := models.ParseQuery("")
	if err != nil {
		panic(err)
	}
	state := outliner.LensState{
		Mode:   chrome.ModeAll,
		Facets: facets,
		Sort:   query.NewSortState(),
	}
	return state.NamedLens(id, name)
}

// typeIntoOutlinerSearch drives the open input one rune at a time through the
// ordinary key path, so the test exercises the same loop the user does.
func typeIntoOutlinerSearch(t *testing.T, outlinerView *outliner.OutlinerModel, query string) {
	t.Helper()
	for _, r := range query {
		outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}
