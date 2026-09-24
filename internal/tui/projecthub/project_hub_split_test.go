package projecthub

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/listsurface"
)

// projectHubDividerColumn is where the frame's pane divider sits, read off the
// rendered rows rather than the geometry the Hub computed it from.
func projectHubDividerColumn(t *testing.T, hub *ProjectHubModel) int {
	t.Helper()
	for _, line := range strings.Split(hub.View().Content, "\n") {
		if index := strings.Index(ansi.Strip(line), " │ "); index >= 0 {
			return index
		}
	}
	t.Fatalf("Hub frame carries no pane divider:\n%s", hub.View().Content)
	return -1
}

func TestProjectHubSplitKeysMoveTheDividerWithinBounds(t *testing.T) {
	hub := newProjectHubFixture(t, scopeTestProject("alpha", "Alpha", "1")).open()
	start := projectHubDividerColumn(t, hub)

	hub.Update(syntheticKey('<'))
	if got := hub.previewSplit().ListPercent; got != listsurface.ListSurfaceDefaultListWidthPercent-listsurface.ListSurfaceListWidthPercentStep {
		t.Fatalf("< left the split at %d%%", got)
	}
	if left := projectHubDividerColumn(t, hub); left >= start {
		t.Fatalf("< drew the divider at column %d, want left of %d", left, start)
	}

	for range 20 {
		hub.Update(syntheticKey('<'))
	}
	if got := hub.previewSplit().ListPercent; got != listsurface.ListSurfaceMinListWidthPercent {
		t.Fatalf("repeated < settled at %d%%, want the %d%% bound", got, listsurface.ListSurfaceMinListWidthPercent)
	}
	for range 20 {
		hub.Update(syntheticKey('>'))
	}
	if got := hub.previewSplit().ListPercent; got != listsurface.ListSurfaceMaxListWidthPercent {
		t.Fatalf("repeated > settled at %d%%, want the %d%% bound", got, listsurface.ListSurfaceMaxListWidthPercent)
	}
	if right := projectHubDividerColumn(t, hub); right <= start {
		t.Fatalf("> drew the divider at column %d, want right of %d", right, start)
	}
}

func TestProjectHubSplitIsInertWithoutAPreviewPane(t *testing.T) {
	hub := newProjectHubFixture(t, scopeTestProject("alpha", "Alpha", "1")).open()
	hub.SetSize(listsurface.ListSurfaceDefaultWideMinWidth-1, 30)
	hub.Update(syntheticKey('<'))
	if got := hub.previewSplit().ListPercent; got != 0 {
		t.Fatalf("< below the two-pane width moved the split to %d%%", got)
	}
	if hub.previewPaneVisible() {
		t.Fatal("a compact Hub reports a preview pane")
	}
}

func TestProjectHubSplitKeysTypeIntoTheQuery(t *testing.T) {
	hub := newProjectHubFixture(t, scopeTestProject("alpha", "Alpha", "1")).open()
	hub.Update(syntheticKey('/'))
	hub.Update(syntheticKey('<'))
	hub.Update(syntheticKey('>'))
	if hub.SearchQuery() != "<>" {
		t.Fatalf("query=%q, want the literal characters", hub.SearchQuery())
	}
	if got := hub.previewSplit().ListPercent; got != 0 {
		t.Fatalf("typing into the query moved the split to %d%%", got)
	}
}

func TestProjectHubSplitOutlivesTheHubItWasMovedIn(t *testing.T) {
	shared := &ProjectHubPreviewSplit{}
	fixture := newProjectHubFixture(t, scopeTestProject("alpha", "Alpha", "1"))
	first := fixture.open()
	first.SetPreviewSplit(shared)
	first.Update(syntheticKey('>'))

	second := fixture.open()
	second.SetPreviewSplit(shared)
	if got := second.projectHubRenderConfig(0).ListWidthPercent; got != listsurface.ListSurfaceDefaultListWidthPercent+listsurface.ListSurfaceListWidthPercentStep {
		t.Fatalf("a reopened Hub drew the split at %d%%", got)
	}
}
