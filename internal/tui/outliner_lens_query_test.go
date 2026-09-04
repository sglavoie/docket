package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

func TestLensQueryEditingNarrowsWithoutASearchProjection(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", ""))

	outlinerView.ActivateSearch()
	if !outlinerView.EditingLensQuery() {
		t.Fatal("slash under a lens must edit the narrowing query")
	}
	if outlinerView.HasSearchProjection() {
		t.Fatal("lens query editing must not create a search projection")
	}

	typeLensQuery(t, outlinerView, "beta")
	if outlinerView.Lens.Query != "beta" {
		t.Fatalf("lens query = %q, want beta", outlinerView.Lens.Query)
	}
	if outlinerView.SearchQuery() != "" {
		t.Fatalf("the ordinary search query must stay empty, got %q", outlinerView.SearchQuery())
	}
	if got := searchVisibleTaskIDs(outlinerView); !slices.Equal(got, []string{"beta-waiting"}) {
		t.Fatalf("narrowed rows = %v, want [beta-waiting]", got)
	}

	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	if outlinerView.EditingLensQuery() || outlinerView.IsSearchMode() {
		t.Fatal("Enter did not leave the query editor")
	}
	if outlinerView.Lens.Query != "beta" {
		t.Fatalf("Enter did not accept the query: %q", outlinerView.Lens.Query)
	}
	if !outlinerView.LensModified() {
		t.Fatal("an accepted narrowing query must mark the lens modified")
	}
}

func TestLensQueryEditingEscRestoresTheQueryItOpenedOn(t *testing.T) {
	outlinerView := lensLifecycleOutliner()
	applyLensOrFatal(t, outlinerView, waitingLens(lensTestID(1), "Waiting", "needle"))

	outlinerView.ActivateSearch()
	typeLensQuery(t, outlinerView, "z")
	outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape})

	if outlinerView.EditingLensQuery() || outlinerView.IsSearchMode() {
		t.Fatal("Esc did not leave the query editor")
	}
	if outlinerView.Lens.Query != "needle" {
		t.Fatalf("Esc left the query at %q, want the original needle", outlinerView.Lens.Query)
	}
	if !outlinerView.HasActiveLens() {
		t.Fatal("abandoning the query edit must not dismiss the lens")
	}
	if outlinerView.LensModified() {
		t.Fatal("an abandoned query edit must leave the lens unmodified")
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

	installed := outlinerView.Lens.Active.SavedFingerprint
	if outlinerView.LensModified() {
		t.Fatal("a freshly applied lens reported itself modified")
	}

	// Diverging the live view must not move the installed fingerprint.
	outlinerView.Filter.Priority = int(models.PriorityHigh)
	outlinerView.AdoptFilterCarriers()

	if outlinerView.Lens.Active.LensID != saved.ID || outlinerView.Lens.Active.Name != saved.Name {
		t.Fatalf("frame identity = %q/%q, want %q/%q",
			outlinerView.Lens.Active.LensID, outlinerView.Lens.Active.Name, saved.ID, saved.Name)
	}
	if outlinerView.Lens.Active.SavedFingerprint != installed {
		t.Fatal("the installed fingerprint absorbed a live edit")
	}
	if !outlinerView.LensModified() {
		t.Fatal("a diverged view did not report itself modified")
	}
}

// typeLensQuery drives the lens query editor one rune at a time through the
// ordinary key path, so the test exercises the same input loop the user does.
func typeLensQuery(t *testing.T, outlinerView *outliner.OutlinerModel, query string) {
	t.Helper()
	for _, r := range query {
		outlinerView.HandleKeyMsg(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func loadedProjectIDs(outlinerView *outliner.OutlinerModel) []string {
	ids := make([]string, 0, len(outlinerView.Population.Projects))
	for _, project := range outlinerView.Population.Projects {
		if project != nil {
			ids = append(ids, project.ID)
		}
	}
	return ids
}
