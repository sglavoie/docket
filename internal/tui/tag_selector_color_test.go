package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// newTagSelectorForTest seeds the retained selector from a project whose tasks
// carry tags landing in different ramp slots, so a regression that paints every
// row one flat style cannot pass the distinctness guard below. `bug` is in the
// fixture on purpose: this surface names tags, so a semantic tag wears its chip
// here rather than the title color the `#` picker's SEMANTIC section previews.
func newTagSelectorForTest(t *testing.T, currentTags ...string) *listsurface.TagSelectorModel {
	t.Helper()
	project := &models.Project{
		ID:   "project-alpha",
		Name: "Alpha",
		Tasks: []*models.Task{
			{Title: "one", Tags: []string{"infra", "ux"}},
			{Title: "two", Tags: []string{"bug", "ux"}},
		},
	}
	selector := listsurface.NewTagSelectorModel(nil, project, currentTags)
	if got := len(selector.Surface.Items()); got != 3 {
		t.Fatalf("fixture built %d rows, want 3", got)
	}
	return selector
}

// assertTagSelectorRowWearsItsChip ties the row to the same authority every
// other tag surface reads rather than to a hex, so a ramp edit moves both.
func assertTagSelectorRowWearsItsChip(t *testing.T, item listsurface.ListItem) {
	t.Helper()
	if item.PrimaryStyle == nil {
		t.Fatalf("row %q has no color", item.ID)
	}
	want := theme.TagChipStyle(item.PrimaryText).GetForeground()
	if got := item.PrimaryStyle.GetForeground(); got != want {
		t.Fatalf("row %q foreground = %v, want %v", item.ID, got, want)
	}
}

func assertTagSelectorRowsAreChipColored(t *testing.T, items []listsurface.ListItem) {
	t.Helper()
	foregrounds := make(map[string]bool, len(items))
	for _, item := range items {
		assertTagSelectorRowWearsItsChip(t, item)
		foregrounds[fmt.Sprintf("%v", item.PrimaryStyle.GetForeground())] = true
	}
	if len(foregrounds) < 2 {
		t.Fatalf("%d rows span %d colors, want at least 2", len(items), len(foregrounds))
	}
}

// TestTagSelectorRowsWearTheirTagChipColor covers the selector as it opens: the
// task form reaches it through ShowTagSelectorMsg, and its rows are adapted by
// ListSourceFromPickerItems, which is the path that used to build them plain.
func TestTagSelectorRowsWearTheirTagChipColor(t *testing.T) {
	selector := newTagSelectorForTest(t)
	assertTagSelectorRowsAreChipColored(t, selector.Surface.Items())
}

// TestTagSelectorMintedRowKeepsItsChipColorAfterReplaceItems walks the inline
// add flow end to end, because that flow re-adapts every row through
// ReplaceItems: a fix applied once at construction would lose the color of the
// rows that were already there as well as ship the new one plain.
func TestTagSelectorMintedRowKeepsItsChipColorAfterReplaceItems(t *testing.T) {
	selector := newTagSelectorForTest(t)

	if _, cmd := selector.Update(tea.KeyPressMsg{Code: 'a', Text: "a"}); cmd == nil {
		t.Fatal("a did not enter add-tag mode")
	}
	if !selector.AddTagMode {
		t.Fatal("add-tag mode is not active")
	}
	selector.AddTagInput.SetValue("security")
	if _, _ = selector.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); selector.AddTagMode {
		t.Fatalf("add-tag mode survived the commit: %q", selector.AddTagError)
	}

	items := selector.Surface.Items()
	if len(items) != 4 {
		t.Fatalf("selector holds %d rows after minting, want 4", len(items))
	}
	assertTagSelectorRowsAreChipColored(t, items)

	minted := false
	for _, item := range items {
		if item.ID == "security" {
			minted = true
		}
	}
	if !minted {
		t.Fatal("the minted tag never reached the selector")
	}
	// Minting still selects and targets the new row; color is added beside that
	// behavior, not in place of it.
	if !selector.Surface.IsSelected("security") {
		t.Fatal("the minted tag is not selected")
	}
	if got := selector.SelectedItem().Name; got != "security" {
		t.Fatalf("cursor sits on %q, want the minted tag", got)
	}
}

// TestTagSelectorKeepsSelectionAndFilteringWithColoredRows pins the behavior the
// color rides along with: PrimaryStyle is presentation metadata, so preselection
// keeps working, filtering keeps ranking on the plain text, and Enter still
// reports the sorted selection.
func TestTagSelectorKeepsSelectionAndFilteringWithColoredRows(t *testing.T) {
	selector := newTagSelectorForTest(t, "ux")
	if !selector.Surface.IsSelected("ux") {
		t.Fatal("the current tag is not preselected")
	}

	selector.Filter("inf")
	matches := selector.FilteredItems()
	if len(matches) != 1 || matches[0].Name != "infra" {
		t.Fatalf("filtering for %q matched %+v", "inf", matches)
	}
	for _, item := range selector.Surface.Items() {
		if item.ID == "infra" {
			assertTagSelectorRowWearsItsChip(t, item)
		}
	}

	selector.SetCursorToID("infra")
	if handled, _ := selector.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeySpace}); !handled {
		t.Fatal("space did not toggle the filtered row")
	}
	selector.Filter("")

	_, cmd := selector.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter produced no message")
	}
	done, ok := cmd().(listsurface.TagSelectorDoneMsg)
	if !ok {
		t.Fatalf("enter emitted %T, want TagSelectorDoneMsg", cmd())
	}
	if done.Canceled || strings.Join(done.SelectedTags, ",") != "infra,ux" {
		t.Fatalf("selector reported %+v", done)
	}
}

// TestTagSelectorDropsRowColorUnderTheCursorOnly keeps the one exception intact:
// the cursor row is owned by SelectedLine, exactly as it is on every other list
// surface, so the chip is visible on the rows around it and not on that one.
func TestTagSelectorDropsRowColorUnderTheCursorOnly(t *testing.T) {
	selector := newTagSelectorForTest(t)
	selector.SetSize(80, 24)
	selector.SetCursorToID("bug")

	rendered := listsurface.RenderTagSelectorView(
		selector.ListSurfaceAdapter, &selector.TagAdder, selector.Config.Title, 80, 24,
	)
	if strings.Contains(rendered, theme.TagChipStyle("bug").Render("bug")) {
		t.Fatalf("the cursor row still wears its chip color:\n%s", rendered)
	}
	if !strings.Contains(ansi.Strip(rendered), "bug") {
		t.Fatalf("the cursor row lost its text:\n%s", rendered)
	}
	if !strings.Contains(rendered, theme.TagChipStyle("ux").Render("ux")) {
		t.Fatalf("a row away from the cursor lost its chip color:\n%s", rendered)
	}
}
