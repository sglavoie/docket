package listsurface

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/tui/route"
)

// pointerTestSurface is two categories of four items each, so a compact render
// short enough to scroll draws both a category header and continuation rows.
func pointerTestSurface(t *testing.T, mode ListSelectionMode) *ListSurfaceModel {
	t.Helper()
	categories := []ListCategory{{ID: "first", Label: "First"}, {ID: "second", Label: "Second"}}
	var items []ListItem
	for index := range 8 {
		category := ListCategoryID("first")
		if index >= 4 {
			category = "second"
		}
		items = append(items, ListItem{
			ID:          ListItemID(fmt.Sprintf("item-%d", index)),
			CategoryID:  category,
			PrimaryText: fmt.Sprintf("Item %d", index),
		})
	}
	return mustListSurface(t, items, categories, ListSurfaceConfig{SelectionMode: mode})
}

func click(x, y int) route.PointerEvent {
	return route.PointerEvent{X: x, Y: y, Button: route.PointerButtonLeft, Clicks: 1}
}

// screenRowOf is the pane row the plain render draws text on.
func screenRowOf(t *testing.T, rendered, text string) int {
	t.Helper()
	for row, line := range strings.Split(ansi.Strip(rendered), "\n") {
		if strings.Contains(line, text) {
			return row
		}
	}
	t.Fatalf("render does not draw %q:\n%s", text, ansi.Strip(rendered))
	return -1
}

// TestListPointerSeatsTheRowItWasDrawnOn holds a click to the frame the last
// render drew: whatever row a scrolled list put an item on, a click there seats
// that item, and the header and continuation rows around it seat nothing.
func TestListPointerSeatsTheRowItWasDrawnOn(t *testing.T) {
	model := pointerTestSurface(t, ListSelectionSingle)
	model.SetCursorToID("item-5")
	rendered := RenderCompactListSurface(model, 30, 5)

	row := screenRowOf(t, rendered, "Item 4")
	if key := model.AnswerPointer(click(2, row), true); key != "" {
		t.Fatalf("a single click pressed %q", key)
	}
	assertCurrentListItem(t, model, "item-4")

	rendered = RenderCompactListSurface(model, 30, 5)
	for _, text := range []string{"↑ ", "SECOND", "↓ "} {
		model.AnswerPointer(click(2, screenRowOf(t, rendered, text)), true)
		assertCurrentListItem(t, model, "item-4")
	}

	model.AnswerPointer(click(30, screenRowOf(t, rendered, "Item 5")), true)
	assertCurrentListItem(t, model, "item-4")
	model.AnswerPointer(click(29, screenRowOf(t, rendered, "Item 5")), true)
	assertCurrentListItem(t, model, "item-5")
}

// TestListPointerKeyIsTheKeyThePointerStandsFor pins the whole answer: a
// double-click is Enter, a right-click is Space only where Space is the
// surface's menu, the wheel is an arrow wherever it lands, and nothing presses
// a key for a click that seated nothing.
func TestListPointerKeyIsTheKeyThePointerStandsFor(t *testing.T) {
	double := route.PointerEvent{Button: route.PointerButtonLeft, Clicks: 2}
	right := route.PointerEvent{Button: route.PointerButtonRight, Clicks: 1}
	cases := []struct {
		name   string
		event  route.PointerEvent
		seated bool
		menu   bool
		want   string
	}{
		{"click", click(0, 0), true, true, ""},
		{"double-click", double, true, false, "enter"},
		{"double-click on nothing", double, false, true, ""},
		{"right-click on a menu surface", right, true, true, "space"},
		{"right-click without a menu", right, true, false, ""},
		{"wheel down", route.PointerEvent{WheelDelta: 1}, false, false, "down"},
		{"wheel up", route.PointerEvent{WheelDelta: -1}, false, false, "up"},
		{"middle", route.PointerEvent{Button: route.PointerButtonMiddle, Clicks: 1}, true, true, ""},
	}
	for _, tc := range cases {
		if got := ListPointerKey(tc.event, tc.seated, tc.menu); got != tc.want {
			t.Errorf("%s pressed %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestListPointerReadsNothingBeforeAFrame keeps a list that has never been
// drawn from answering a click, and an empty list from answering one at all.
func TestListPointerReadsNothingBeforeAFrame(t *testing.T) {
	model := pointerTestSurface(t, ListSelectionMulti)
	model.AnswerPointer(click(2, 2), true)
	assertCurrentListItem(t, model, "item-0")

	model.SetQuery("no such item")
	RenderCompactListSurface(model, 30, 5)
	if key := model.AnswerPointer(route.PointerEvent{X: 2, Button: route.PointerButtonLeft, Clicks: 2}, true); key != "" {
		t.Fatalf("a double-click on an empty list pressed %q", key)
	}
}

// TestListPointerKnowsThePreviewItDrew answers the Hub's question — is this
// column the preview's — from the layout the renderer chose, not from a second
// copy of the split.
func TestListPointerKnowsThePreviewItDrew(t *testing.T) {
	model := pointerTestSurface(t, ListSelectionSingle)
	items := model.Items()
	for index := range items {
		items[index].Preview = ListPreview{Title: items[index].PrimaryText}
	}
	if err := model.SetItems(items); err != nil {
		t.Fatalf("SetItems() error = %v", err)
	}
	config := ListSurfaceRenderConfig{Width: 100, Height: 10, Layout: ListSurfaceLayoutAuto, ShowPreview: true, HideHelp: true}
	NewListSurfaceRenderer(config).Render(model)
	listWidth, _ := ListSurfacePreviewGeometry(config)
	if model.PointerOverPreview(listWidth) || !model.PointerOverPreview(ListSurfacePreviewLeft(listWidth)) {
		t.Fatalf("preview left = %d, want %d", model.previewLeft, ListSurfacePreviewLeft(listWidth))
	}

	config.Width = 40
	NewListSurfaceRenderer(config).Render(model)
	if model.PointerOverPreview(39) {
		t.Fatal("a compact render still reports a preview pane")
	}
}
