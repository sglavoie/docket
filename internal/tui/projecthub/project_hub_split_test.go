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
	drawn := start * 100 / hub.Width()

	// The fixture's one short row fits the list under the 25% bound, so `<` has
	// nowhere to go and must not clamp the divider rightward against the press.
	hub.Update(syntheticKey('<'))
	if got := hub.previewSplit().ListPercent; got != 0 {
		t.Fatalf("< from a list already under the bound moved the split to %d%%", got)
	}
	// The first move starts from the divider the content-fitted default drew,
	// not from the default share it was narrowed from.
	hub.Update(syntheticKey('>'))
	if got, want := hub.previewSplit().ListPercent, max(drawn+listsurface.ListSurfaceListWidthPercentStep, listsurface.ListSurfaceMinListWidthPercent); got != want {
		t.Fatalf("> left the split at %d%%, want %d%% from the drawn %d%%", got, want, drawn)
	}
	if moved := projectHubDividerColumn(t, hub); moved <= start {
		t.Fatalf("> drew the divider at column %d, want right of %d", moved, start)
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
	moved := shared.ListPercent
	if moved <= 0 {
		t.Fatal("> left the shared split at the default")
	}

	second := fixture.open()
	second.SetPreviewSplit(shared)
	if got := second.projectHubRenderConfig(0).ListWidthPercent; got != moved {
		t.Fatalf("a reopened Hub drew the split at %d%%, want %d%%", got, moved)
	}
}

func TestProjectHubDefaultSplitFitsTheListToItsWidestRow(t *testing.T) {
	hub := newProjectHubFixture(t, scopeTestProject("alpha", "Alpha", "1")).open()
	config := hub.projectHubRenderConfig(0)
	listWidth, previewWidth := listsurface.ListSurfacePreviewGeometry(config)
	if want := max(hub.surface.NaturalRowWidth(), listsurface.ListSurfaceDefaultListMinWidth); listWidth != want {
		t.Fatalf("default list pane = %d columns, want the widest row's %d", listWidth, want)
	}
	if previewWidth != hub.Width()-listWidth-3 {
		t.Fatalf("preview pane = %d columns, want the rest of %d", previewWidth, hub.Width())
	}
	if column := projectHubDividerColumn(t, hub); column != listWidth {
		t.Fatalf("divider drawn at column %d, want %d", column, listWidth)
	}
}
