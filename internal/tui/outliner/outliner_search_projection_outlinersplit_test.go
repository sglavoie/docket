package outliner

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// The ranked list is the one presentation that leaves the mode's lanes and the
// user's folds behind. Scope is not a clause it can leave: the search runs
// inside the frame it was typed in. Facets still gate it, and the deferred and
// terminal populations open only when the query says so.
func TestTheRankedListLeavesModeAndFoldsButHonorsScopeAndFacets(t *testing.T) {
	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	parent := primaryRowsTask("parent", models.StateBacklog)
	parent.Title = "Ordinary parent"
	child := primaryRowsTask("child", models.StateWaiting)
	child.Title = "Child title"
	child.Description = "contains Needle"
	child.ParentID = &parent.ID
	deferred := primaryRowsTask("deferred", models.StateSomeday)
	deferred.Title = "Needle deferred"
	deferred.DeferredUntil = &tomorrow
	inbox := primaryRowsTask("inbox", models.StateInbox)
	inbox.Title = "Needle inbox"
	inbox.Priority = models.PriorityLow
	done := primaryRowsTask("done", models.StateDone)
	done.Title = "Needle done"
	canceled := primaryRowsTask("canceled", models.StateCanceled)
	canceled.Title = "Needle canceled"
	alpha := primaryRowsProject("alpha", parent, child, deferred, inbox, done, canceled)

	other := primaryRowsTask("other", models.StateBacklog)
	other.Title = "Needle other project"
	beta := primaryRowsProject("beta", other)

	outliner := scopeTestOutliner(alpha, beta)
	outliner.PushProjectScope(alpha.ID)
	outliner.SelectOutlinerMode(chrome.ModeBacklog)
	outliner.Folds.ExpandedNodes[taskref.TaskIdentity{ProjectID: alpha.ID, TaskID: parent.ID}] = false
	searchRankedList(outliner, "needle")

	got := qualifiedVisibleTaskIDs(outliner)
	want := []taskref.TaskIdentity{
		{ProjectID: alpha.ID, TaskID: inbox.ID},
		{ProjectID: alpha.ID, TaskID: child.ID},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ranked list rows = %v, want the scoped open rows %v", got, want)
	}
	// The scoped projection leads with its focused-project header, so the child is
	// found by identity rather than by a row number that header would shift.
	assertSoleFocusedProjectHeader(t, outliner, alpha.ID, "ranked")
	childRow, found := rowForTaskID(outliner, child.ID)
	if !found {
		t.Fatal("ranked list dropped the child row")
	}
	if childRow.Breadcrumb != "Ordinary parent ›" {
		t.Fatalf("child breadcrumb = %q, want preserved ancestor context", childRow.Breadcrumb)
	}

	// The deferred task is reachable from the same list, but only because the
	// query asked for it. Admission has one rule and the query is it.
	searchRankedList(outliner, "needle @@any")
	if got := qualifiedVisibleTaskIDs(outliner); !slices.Equal(got,
		[]taskref.TaskIdentity{{ProjectID: alpha.ID, TaskID: deferred.ID}}) {
		t.Fatalf("deferred query = %v, want the deferred hit alone", got)
	}
	searchRankedList(outliner, "needle")

	// A facet is an AND gate applied before scoring, so it narrows the same ranked
	// list without disturbing its order. Only the low-priority task clears the
	// threshold: an unset priority is effectively below Low.
	outliner.Filter.Priority = models.PriorityLow
	adoptFilterCarriers(outliner)
	outliner.Population.Invalidate()
	outliner.CrossProjectRebuildRows()
	if got := qualifiedVisibleTaskIDs(outliner); !slices.Equal(got,
		[]taskref.TaskIdentity{{ProjectID: alpha.ID, TaskID: inbox.ID}}) {
		t.Fatalf("search under the priority facet = %v, want only the low-priority hit", got)
	}
}

func TestScopeWideSearchRanksFieldsFuzzyScoreAndStableSourceOrder(t *testing.T) {
	titleStrong := primaryRowsTask("title-strong", models.StateBacklog)
	titleStrong.Title = "Needle"
	titleWeak := primaryRowsTask("title-weak", models.StateBacklog)
	titleWeak.Title = "Need a little extra"
	metadataFirst := primaryRowsTask("metadata-first", models.StateBacklog)
	metadataFirst.Description = "Needle"
	metadataTie := primaryRowsTask("metadata-tie", models.StateBacklog)
	metadataTie.Tags = []string{"needle"}
	project := primaryRowsProject("alpha", titleWeak, metadataFirst, metadataTie, titleStrong)
	project.Slug = "alpha"

	matches := searchProjectionMatchesWithin([]*models.Project{project}, "needle", nil, fuzzy.Matcher{}, nil)
	got := searchMatchTaskIDs(matches)
	want := []string{titleStrong.ID, titleWeak.ID, metadataFirst.ID, metadataTie.ID}
	if !slices.Equal(got, want) {
		t.Fatalf("ranked search = %v, want title then metadata score/source order %v", got, want)
	}

	exact := primaryRowsTask("exact", models.StateBacklog)
	exact.ShortID = 7
	exact.Title = "No shared words"
	project.Tasks = append(project.Tasks, exact)
	if got := searchMatchTaskIDs(searchProjectionMatchesWithin([]*models.Project{project}, "alpha#7", nil, fuzzy.Matcher{}, nil)); !slices.Equal(got, []string{exact.ID}) {
		t.Fatalf("exact alpha#7 lookup = %v, want %s", got, exact.ID)
	}
	if got := searchProjectionMatchesWithin([]*models.Project{project}, "alpha#999", nil, fuzzy.Matcher{}, nil); len(got) != 0 {
		t.Fatalf("missing exact qualifier returned %v", searchMatchTaskIDs(got))
	}
}

func TestScopeWideSearchRefreshesAfterMutationsAndShowsNoMatches(t *testing.T) {
	matching := primaryRowsTask("matching", models.StateBacklog)
	matching.Title = "Needle before edit"
	appearing := primaryRowsTask("appearing", models.StateBacklog)
	appearing.Title = "Initially absent"
	project := primaryRowsProject("alpha", matching, appearing)
	outliner := scopeTestOutliner(project)
	searchRankedList(outliner, "needle")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{matching.ID}) {
		t.Fatalf("initial search = %v", got)
	}

	matching.Title = "No longer matches"
	appearing.Description = "Needle added by mutation"
	outliner.Population.Invalidate()
	outliner.CrossProjectRebuildRows()
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{appearing.ID}) {
		t.Fatalf("refreshed search = %v, want appearing task", got)
	}

	appearing.Description = "Removed again"
	outliner.Population.Invalidate()
	outliner.CrossProjectRebuildRows()
	if len(outliner.Rows) != 0 {
		t.Fatalf("no-match projection retained rows: %v", searchVisibleTaskIDs(outliner))
	}
	if got := outliner.OutlinerEmptyMessage(); got != `No tasks match search "needle". Press Esc to clear.` {
		t.Fatalf("no-match message = %q", got)
	}
}

func TestScopeWideSearchAppliesSmartCaseAndFallsBackWhenEmpty(t *testing.T) {
	upper := primaryRowsTask("upper", models.StateBacklog)
	upper.Title = "Ship API client"
	lower := primaryRowsTask("lower", models.StateBacklog)
	lower.Title = "Rework api docs"
	exact := primaryRowsTask("exact", models.StateBacklog)
	exact.ShortID = 42
	exact.Title = "Nothing in common"
	project := primaryRowsProject("alpha", upper, lower, exact)
	outliner := scopeTestOutliner(project)

	searchRankedList(outliner, "api")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{upper.ID, lower.ID}) {
		t.Fatalf("lowercase search = %v, want both casings", got)
	}
	if outliner.Search.caseRelaxed || outliner.searchCaseSensitive() {
		t.Fatal("lowercase query was treated as case-sensitive")
	}

	searchRankedList(outliner, "API")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{upper.ID}) {
		t.Fatalf("uppercase search = %v, want only the exact-case title", got)
	}
	if outliner.Search.caseRelaxed || !outliner.searchCaseSensitive() {
		t.Fatalf("narrowing search relaxed=%v sensitive=%v, want a case-sensitive pass",
			outliner.Search.caseRelaxed, outliner.searchCaseSensitive())
	}

	searchRankedList(outliner, "Docs")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{lower.ID}) {
		t.Fatalf("fallback search = %v, want the case-insensitive retry to match", got)
	}
	if !outliner.Search.caseRelaxed || outliner.searchCaseSensitive() {
		t.Fatalf("fallback search relaxed=%v sensitive=%v, want a relaxed pass",
			outliner.Search.caseRelaxed, outliner.searchCaseSensitive())
	}

	searchRankedList(outliner, "Zebra")
	if len(outliner.Rows) != 0 {
		t.Fatalf("unmatchable query kept rows %v", searchVisibleTaskIDs(outliner))
	}
	if outliner.Search.caseRelaxed {
		t.Fatal("an empty insensitive retry still claimed the case was ignored")
	}

	searchRankedList(outliner, "Alpha#42")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{exact.ID}) {
		t.Fatalf("capitalized project#id shorthand = %v, want %s", got, exact.ID)
	}

	outliner.ClearSearch()
	if outliner.Search.caseRelaxed {
		t.Fatal("ClearSearch left the relaxed flag set")
	}
}

// Slash is the user's own entry path, so it opens the flat ranked list: a typed
// query is a question about the workspace rather than about the lane the cursor
// was standing in. The programmatic seam keeps the other presentation, because a
// caller installing a query without a keystroke is asking the mode it left
// standing to answer it.
func TestSlashOpensTheRankedListWhileTheProgrammaticSeamNarrowsTheMode(t *testing.T) {
	doing := primaryRowsTask("doing", models.StateDoing)
	doing.Title = "Needle in doing"
	backlog := primaryRowsTask("backlog", models.StateBacklog)
	backlog.Title = "Needle in backlog"
	outliner := scopeTestOutliner(primaryRowsProject("alpha", doing, backlog))
	outliner.SelectOutlinerMode(chrome.ModeAll)

	outliner.ActivateSearch()
	if !outliner.HasSearchProjection() || !outliner.ShowsRankedList() {
		t.Fatalf("slash opened projection=%v ranked=%v, want the flat ranked list",
			outliner.HasSearchProjection(), outliner.ShowsRankedList())
	}
	outliner.SetSearchQuery("needle")
	if !outliner.ShowsRankedList() {
		t.Fatal("typing into the open input restated the presentation slash chose")
	}
	if names := sectionHeaderNames(outliner); len(names) != 0 {
		t.Fatalf("the ranked list drew sections %v, want the mode's lanes left behind", names)
	}
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{doing.ID, backlog.ID}) &&
		!slices.Equal(got, []string{backlog.ID, doing.ID}) {
		t.Fatalf("ranked rows = %v, want both hits across the modes' lanes", got)
	}

	outliner.ClearSearch()
	outliner.SetSearchQuery("needle")
	if !outliner.HasSearchProjection() {
		t.Fatal("the programmatic seam opened no projection")
	}
	if outliner.ShowsRankedList() {
		t.Fatal("the programmatic seam opened the ranked list, which no keystroke asked for")
	}
	if names := sectionHeaderNames(outliner); len(names) == 0 {
		t.Fatal("the narrowed query dropped the mode's own lanes")
	}
}

// One slash press captures one return frame and declares one presentation. A
// second press edits the query inside that frame: it must not replace the
// snapshot Esc spends, and it must not silently undo a narrowing the user asked
// for with the ranked toggle.
func TestRepeatedSlashKeepsOneReturnFrameAndTheDeliberatePresentation(t *testing.T) {
	doing := primaryRowsTask("doing", models.StateDoing)
	doing.Title = "Needle in doing"
	backlog := primaryRowsTask("backlog", models.StateBacklog)
	backlog.Title = "Needle in backlog"
	outliner := scopeTestOutliner(primaryRowsProject("alpha", doing, backlog))
	outliner.SelectOutlinerMode(chrome.ModeAll)

	doingLabel := models.StateGroupDoing.Label()
	if !outliner.EnterSectionZoom(outliner.Rows[sectionHeaderIndex(t, outliner, doingLabel)]) {
		t.Fatal("the Doing header refused the zoom")
	}

	outliner.ActivateSearch()
	outliner.SetSearchQuery("needle")
	if outliner.Focus.Zoom.Active() {
		t.Fatal("the ranked open left the section zoom standing, though the flat list runs none of its emitters")
	}

	if !outliner.ToggleRankedList() {
		t.Fatal("the held query refused the ranked toggle")
	}
	outliner.ActivateSearch()
	if outliner.ShowsRankedList() {
		t.Fatal("a second slash undid the narrowing the toggle deliberately asked for")
	}

	outliner.ClearSearch()
	if !outliner.Focus.Zoom.Active() || outliner.Focus.Zoom.Name != doingLabel {
		t.Fatalf("restored zoom = %q active=%v, want the %q zoom the first open suspended",
			outliner.Focus.Zoom.Name, outliner.Focus.Zoom.Active(), doingLabel)
	}
	if outliner.HasSearchProjection() {
		t.Fatal("ClearSearch left a projection behind")
	}
}
