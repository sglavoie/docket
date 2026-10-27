package outliner

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
)

// The bar is chrome with a budget: it must be exactly as wide as every other
// Outliner line, it must cost exactly the one row contentHeight subtracts, and
// it must drop whole menus rather than truncate a label. These are the
// assertions that hold it to all three.

func fakeMenus(titles ...string) []ResolvedMenu {
	menus := make([]ResolvedMenu, 0, len(titles))
	for _, title := range titles {
		menus = append(menus, ResolvedMenu{
			ID:      strings.ToLower(title),
			Title:   title,
			Entries: []ResolvedMenuEntry{{OperationID: "test.menu.entry", Label: title}},
		})
	}
	return menus
}

func menuTitles(boxes []menuLabelBox) []string {
	titles := make([]string, 0, len(boxes))
	for _, box := range boxes {
		titles = append(titles, box.Title)
	}
	return titles
}

func TestMenuBarLayoutPlacesEveryLabelEndToEnd(t *testing.T) {
	menus := fakeMenus("Task", "View", "Go", "Find", "Help")
	boxes := menuBarLayout(menus, 100)
	if len(boxes) != len(menus) {
		t.Fatalf("laid out %d labels at a width that fits them all, want %d", len(boxes), len(menus))
	}

	style := menuBarMeasureStyle()
	col := 0
	for index, box := range boxes {
		if box.Title != menus[index].Title {
			t.Errorf("box %d is %q, want the menu order's %q", index, box.Title, menus[index].Title)
		}
		if box.StartCol != col {
			t.Errorf("box %q starts at column %d, want %d", box.Title, box.StartCol, col)
		}
		if want := lipgloss.Width(style.Render(box.Title)); box.Width != want {
			t.Errorf("box %q is %d wide, want the padded cell's %d", box.Title, box.Width, want)
		}
		col += box.Width
	}
}

// A dropped menu is still reachable by the accelerators its entries carry; a
// truncated label is a lie about what the bar offers. So the ladder sheds Help,
// then Find, then Go, and never cuts a word in half.
func TestMenuBarLayoutDropsWholeLabelsFromTheRight(t *testing.T) {
	menus := fakeMenus("Task", "View", "Go", "Find", "Help")
	full := menuBarLayout(menus, 100)
	total := 0
	for _, box := range full {
		total += box.Width
	}

	cases := []struct {
		width int
		want  []string
	}{
		{width: total, want: []string{"Task", "View", "Go", "Find", "Help"}},
		{width: total - 1, want: []string{"Task", "View", "Go", "Find"}},
		{width: full[0].Width + full[1].Width + full[2].Width, want: []string{"Task", "View", "Go"}},
		{width: full[0].Width + full[1].Width, want: []string{"Task", "View"}},
		{width: full[0].Width, want: []string{"Task"}},
		// Narrower than one label, the first menu still stands: the full-width fit
		// every Outliner line goes through is what clips it, not a half-word.
		{width: 2, want: []string{"Task"}},
	}
	for _, tc := range cases {
		got := menuTitles(menuBarLayout(menus, tc.width))
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("width %d laid out %v, want %v", tc.width, got, tc.want)
		}
	}

	if boxes := menuBarLayout(menus, 0); boxes != nil {
		t.Errorf("a zero width laid out %v, want nothing", menuTitles(boxes))
	}
	if boxes := menuBarLayout(nil, 100); boxes != nil {
		t.Errorf("an empty bar laid out %v, want nothing", menuTitles(boxes))
	}
}

func TestRenderMenuBarRowIsExactlyOneLineOfTheGivenWidth(t *testing.T) {
	menus := fakeMenus("Task", "View", "Go", "Find", "Help")
	for _, width := range []int{100, 60, 30, 12} {
		row := renderMenuBarRow(menus, width, -1)
		if strings.Contains(row, "\n") {
			t.Errorf("width %d produced more than one row", width)
		}
		if got := lipgloss.Width(row); got != width {
			t.Errorf("width %d rendered %d cells", width, got)
		}
	}
}

// The open label is the only thing on this row that changes appearance, and it
// has to line up with the column the layout assigned it, so the highlight is
// decided beside the placement rather than by a second walk of the menus.
func TestRenderMenuBarRowMarksOnlyTheOpenMenu(t *testing.T) {
	menus := fakeMenus("Task", "View", "Go")
	closed := renderMenuBarRow(menus, 60, -1)
	for openIdx := 0; openIdx < len(menus); openIdx++ {
		row := renderMenuBarRow(menus, 60, openIdx)
		if row == closed {
			t.Errorf("opening menu %d rendered the same row as an inert bar", openIdx)
		}
		if got := lipgloss.Width(row); got != 60 {
			t.Errorf("opening menu %d rendered %d cells, want 60", openIdx, got)
		}
		if plain := ansi.Strip(row); !strings.Contains(plain, menus[openIdx].Title) {
			t.Errorf("opening menu %d lost its label: %q", openIdx, plain)
		}
	}
}

// The View menu's own row is the one thing a user who found the bar by accident
// cannot read off the bar itself, so it has to describe live visibility rather
// than a constant.
func TestViewMenuBarRowCarriesLiveVisibility(t *testing.T) {
	for _, visible := range []bool{false, true} {
		var found bool
		for _, entry := range viewMenuEntries(chrome.ModeAll, visible, true, true) {
			if entry.OperationID != operation.OperationCoreToggleMenuBar {
				continue
			}
			found = true
			if entry.Label != "Menu Bar" {
				t.Errorf("the bar's own row is labeled %q", entry.Label)
			}
			if entry.Checked == nil {
				t.Fatal("the bar's own row carries no checkbox state")
			}
			if *entry.Checked != visible {
				t.Errorf("visible=%v resolved checked=%v", visible, *entry.Checked)
			}
		}
		if !found {
			t.Fatalf("visible=%v: the View menu does not offer the bar's own toggle", visible)
		}
	}
}
