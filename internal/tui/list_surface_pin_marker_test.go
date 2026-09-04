package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
)

// TestPinnedPickerItemMarksTheRowWithoutStylingItsText locks the contract
// written on ListItem.PrimaryText: query and ranking read it plain. The `%`
// picker used to render the pin by wrapping that text in ANSI, which the list
// surface's scorer then fuzzy-matched raw — escape bytes and all. No
// rendered-output scenario can observe that, because the row looks right
// either way.
func TestPinnedPickerItemMarksTheRowWithoutStylingItsText(t *testing.T) {
	pinned := &models.Project{ID: "p-pinned", Name: "Alpha", Slug: "alpha", Pinned: true}
	plain := &models.Project{ID: "p-plain", Name: "Beta", Slug: "beta"}

	source := listsurface.ListSourceFromPickerItems([]moveProjectPickerItem{
		{project: pinned},
		{project: plain},
	}, false)
	if len(source.Items) != 2 {
		t.Fatalf("adapted %d rows, want 2", len(source.Items))
	}

	marked, unmarked := source.Items[0], source.Items[1]
	if marked.PrimaryMarker == "" {
		t.Error("pinned row carries no PrimaryMarker")
	}
	if !strings.Contains(ansi.Strip(marked.PrimaryMarker), "*") {
		t.Errorf("pinned row marker = %q, want the shared `*` glyph", ansi.Strip(marked.PrimaryMarker))
	}
	if unmarked.PrimaryMarker != "" {
		t.Errorf("unpinned row carries marker %q", unmarked.PrimaryMarker)
	}
	for _, item := range source.Items {
		if item.PrimaryText != ansi.Strip(item.PrimaryText) {
			t.Errorf("PrimaryText %q carries styling the matcher would score", item.PrimaryText)
		}
	}
	if marked.PrimaryText != pinned.Name {
		t.Errorf("pinned PrimaryText = %q, want %q", marked.PrimaryText, pinned.Name)
	}
	// Searching for the pinned project's own name is what proves the marker did
	// not leak into the matched text: the scorer reads PrimaryText, so a row
	// whose text carried the `*` glyph or its styling would drop out here. The
	// assertion runs the real filter rather than the scorer directly, because
	// the scorer is listsurface's own business and the search is the product's.
	surface := mustListSurface(t, source.Items, source.Categories, listsurface.ListSurfaceConfig{})
	surface.SetQuery(pinned.Name)
	matched := surface.Matches()
	if len(matched) != 1 || matched[0].Item.ID != marked.ID {
		t.Errorf("query %q matched %+v, want only the pinned row", pinned.Name, matched)
	}
}
