package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

func TestLensQueryEditingNarrowsWithoutASearchProjection(t *testing.T) {
	outliner := lensLifecycleOutliner()
	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", ""))

	outliner.ActivateSearch()
	if !outliner.editingLensQuery() {
		t.Fatal("slash under a lens must edit the narrowing query")
	}
	if outliner.hasSearchProjection() {
		t.Fatal("lens query editing must not create a search projection")
	}

	typeLensQuery(t, outliner, "beta")
	if outliner.lens.query != "beta" {
		t.Fatalf("lens query = %q, want beta", outliner.lens.query)
	}
	if outliner.SearchQuery() != "" {
		t.Fatalf("the ordinary search query must stay empty, got %q", outliner.SearchQuery())
	}
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"beta-waiting"}) {
		t.Fatalf("narrowed rows = %v, want [beta-waiting]", got)
	}

	outliner.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	if outliner.editingLensQuery() || outliner.IsSearchMode() {
		t.Fatal("Enter did not leave the query editor")
	}
	if outliner.lens.query != "beta" {
		t.Fatalf("Enter did not accept the query: %q", outliner.lens.query)
	}
	if !outliner.lensModified() {
		t.Fatal("an accepted narrowing query must mark the lens modified")
	}
}

func TestLensQueryEditingEscRestoresTheQueryItOpenedOn(t *testing.T) {
	outliner := lensLifecycleOutliner()
	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", "needle"))

	outliner.ActivateSearch()
	typeLensQuery(t, outliner, "z")
	outliner.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape})

	if outliner.editingLensQuery() || outliner.IsSearchMode() {
		t.Fatal("Esc did not leave the query editor")
	}
	if outliner.lens.query != "needle" {
		t.Fatalf("Esc left the query at %q, want the original needle", outliner.lens.query)
	}
	if !outliner.hasActiveLens() {
		t.Fatal("abandoning the query edit must not dismiss the lens")
	}
	if outliner.lensModified() {
		t.Fatal("an abandoned query edit must leave the lens unmodified")
	}
}

func TestStartupAndGlobalResetRetainNoLensFrame(t *testing.T) {
	fresh := NewOutlinerModel(nil, nil, testConfigWithProjects())
	if fresh.hasActiveLens() || fresh.lens.pending != nil || fresh.lens.query != "" {
		t.Fatal("a fresh Outliner must open with no lens")
	}

	outliner := lensLifecycleOutliner()
	applyLensOrFatal(t, outliner, waitingLens(lensTestID(1), "Waiting", "needle"))
	outliner.resetToGlobalAll()

	if outliner.hasActiveLens() || outliner.lens.pending != nil {
		t.Fatal("the global reset retained a lens frame")
	}
	if outliner.lens.query != "" {
		t.Fatalf("the global reset retained the narrowing query %q", outliner.lens.query)
	}
	if outliner.primaryMode != chrome.ModeAll || outliner.filter.HasStateFilter() {
		t.Fatal("the global reset did not clear the lens-installed axes")
	}
}

// The frame holds the installed identity and fingerprint, never the live view:
// diverging an axis moves the capture and leaves the frame exactly where the
// application put it, which is the whole of how the modified marker is decided.
func TestTheActiveFrameKeepsTheInstalledIdentityAndFingerprint(t *testing.T) {
	outliner := lensLifecycleOutliner()
	saved := waitingLens(lensTestID(1), "Waiting", "needle")
	applyLensOrFatal(t, outliner, saved)

	installed := outliner.lens.active.savedFingerprint
	if outliner.lensModified() {
		t.Fatal("a freshly applied lens reported itself modified")
	}

	// Diverging the live view must not move the installed fingerprint.
	outliner.filter.Priority = int(models.PriorityHigh)
	outliner.adoptFilterCarriers()

	if outliner.lens.active.lensID != saved.ID || outliner.lens.active.name != saved.Name {
		t.Fatalf("frame identity = %q/%q, want %q/%q",
			outliner.lens.active.lensID, outliner.lens.active.name, saved.ID, saved.Name)
	}
	if outliner.lens.active.savedFingerprint != installed {
		t.Fatal("the installed fingerprint absorbed a live edit")
	}
	if !outliner.lensModified() {
		t.Fatal("a diverged view did not report itself modified")
	}
}

// typeLensQuery drives the lens query editor one rune at a time through the
// ordinary key path, so the test exercises the same input loop the user does.
func typeLensQuery(t *testing.T, outliner *OutlinerModel, query string) {
	t.Helper()
	for _, r := range query {
		outliner.handleKeyMsg(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func loadedProjectIDs(outliner *OutlinerModel) []string {
	ids := make([]string, 0, len(outliner.population.projects))
	for _, project := range outliner.population.projects {
		if project != nil {
			ids = append(ids, project.ID)
		}
	}
	return ids
}
