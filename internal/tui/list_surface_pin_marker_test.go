package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// TestPinnedPickerItemMarksTheRowWithoutStylingItsText locks the contract
// written on ListItem.PrimaryText: query and ranking read it plain. The `%`
// picker used to render the pin by wrapping that text in ANSI, which
// listItemScore then fuzzy-matched raw — escape bytes and all. No rendered-output
// scenario can observe that, because the row looks right either way.
func TestPinnedPickerItemMarksTheRowWithoutStylingItsText(t *testing.T) {
	pinned := &models.Project{ID: "p-pinned", Name: "Alpha", Slug: "alpha", Pinned: true}
	plain := &models.Project{ID: "p-plain", Name: "Beta", Slug: "beta"}

	source := ListSourceFromPickerItems([]moveProjectPickerItem{
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
	if listItemScore(marked, pinned.Name) <= 0 {
		t.Errorf("pinned row does not match its own name %q", pinned.Name)
	}
}
