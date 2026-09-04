package projecthub

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// projectHubPreviewPane returns the right pane of the Hub's rendered frame, so a
// test reads what the preview shows rather than the whole two-pane composition.
func projectHubPreviewPane(t *testing.T, hub *ProjectHubModel) []string {
	t.Helper()
	var pane []string
	for _, line := range strings.Split(hub.View().Content, "\n") {
		stripped := ansi.Strip(line)
		index := strings.Index(stripped, " │ ")
		if index < 0 {
			continue
		}
		pane = append(pane, strings.TrimRight(stripped[index+len(" │ "):], " "))
	}
	if len(pane) == 0 {
		t.Fatalf("Hub frame carries no preview pane:\n%s", hub.View().Content)
	}
	return pane
}

func projectHubPreviewText(t *testing.T, hub *ProjectHubModel) string {
	t.Helper()
	return strings.Join(projectHubPreviewPane(t, hub), "\n")
}

// TestProjectHubPreviewPagesWithCtrlFAndResetsOnAnotherProject holds the pane's
// own scroll arithmetic: the page step, the tail clamp the render owns, and the
// reset a new cursor project forces. What each row reads is the Outliner
// painter's, and is asserted beside it.
func TestProjectHubPreviewPagesWithCtrlFAndResetsOnAnotherProject(t *testing.T) {
	fixture := newProjectHubFixture(t, scopeTestProject("alpha", "Alpha", "1", "2"))
	// Several panes' worth, so a page down lands mid-list rather than straight on
	// the tail clamp.
	titles := make([]string, 0, 120)
	for i := 1; i <= 120; i++ {
		titles = append(titles, fmt.Sprintf("Task %d", i))
	}
	fixture.add(scopeTestProject("long", "Long", titles...))

	hub := fixture.open()
	focusProjectHubItem(t, hub, "project-hub:project:long")
	pane := projectHubPreviewText(t, hub)
	if !strings.Contains(pane, "↓ ") {
		t.Fatalf("a 120-task project fits the pane, so paging cannot be exercised:\n%s", pane)
	}
	page := hub.preview.pageSize
	if page <= 0 {
		t.Fatalf("render settled on page size %d, want the pane's row capacity", page)
	}

	hub.Update(syntheticCtrlKey('f'))
	pane = projectHubPreviewText(t, hub)
	if hub.preview.offset != page {
		t.Fatalf("Ctrl+F moved to offset %d, want a full page of %d", hub.preview.offset, page)
	}
	if !strings.Contains(pane, "↑ ") {
		t.Fatalf("paged pane carries no upper continuation:\n%s", pane)
	}

	hub.Update(syntheticCtrlKey('b'))
	projectHubPreviewText(t, hub)
	if hub.preview.offset != 0 {
		t.Fatalf("Ctrl+B left offset %d, want the top", hub.preview.offset)
	}

	// The tail clamps: pressing past the end stops on the last full pane rather
	// than scrolling into blank rows.
	for range 20 {
		hub.Update(syntheticCtrlKey('f'))
		projectHubPreviewText(t, hub)
	}
	pane = projectHubPreviewText(t, hub)
	if strings.Contains(pane, "↓ ") {
		t.Fatalf("paging past the tail left a lower continuation:\n%s", pane)
	}
	if !strings.Contains(pane, "Task 120") {
		t.Fatalf("tail of the pane omits the last task:\n%s", pane)
	}

	// Another project starts at its own top rather than inheriting the offset.
	focusProjectHubItem(t, hub, "project-hub:project:alpha")
	projectHubPreviewText(t, hub)
	if hub.preview.offset != 0 {
		t.Fatalf("moving to another project kept offset %d, want 0", hub.preview.offset)
	}
}
