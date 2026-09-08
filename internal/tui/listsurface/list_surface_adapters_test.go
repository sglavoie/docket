package listsurface

import "testing"

// TestRetainKnownIDsIntersectsWithoutReordering holds the seed helper to the
// one thing it promises: it is an intersection with the rows, in the caller's
// order, and never a repair. The empty result is nil rather than an empty
// slice, which is what lets a picker distinguish "seeded nothing" from "seeded
// values this vocabulary no longer holds" only by what it passed in.
func TestRetainKnownIDsIntersectsWithoutReordering(t *testing.T) {
	source := ListSource{Items: []ListItem{
		{ID: "tag:backend"},
		{ID: "tag:platform-api"},
		{ID: "tag:urgent"},
	}}

	cases := []struct {
		name string
		ids  []ListItemID
		want []ListItemID
	}{
		{name: "no seed", ids: nil, want: nil},
		{
			name: "every seed known keeps the given order",
			ids:  []ListItemID{"tag:urgent", "tag:backend"},
			want: []ListItemID{"tag:urgent", "tag:backend"},
		},
		{
			// The rename case: the facet still spells the retired name, and the
			// picker has to open on the vocabulary it has.
			name: "a retired spelling is dropped",
			ids:  []ListItemID{"tag:security", "tag:backend"},
			want: []ListItemID{"tag:backend"},
		},
		{name: "nothing known", ids: []ListItemID{"tag:security"}, want: nil},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := source.RetainKnownIDs(testCase.ids)
			if len(got) != len(testCase.want) {
				t.Fatalf("RetainKnownIDs(%v) = %v, want %v", testCase.ids, got, testCase.want)
			}
			for index, id := range testCase.want {
				if got[index] != id {
					t.Fatalf("RetainKnownIDs(%v) = %v, want %v", testCase.ids, got, testCase.want)
				}
			}
		})
	}
}

// TestRetainKnownIDsFeedsSetSelectedIDs proves the pairing the filter pickers
// rely on: SetSelectedIDs stays strict, and the retained seed is exactly what
// makes it succeed where the raw one refuses.
func TestRetainKnownIDsFeedsSetSelectedIDs(t *testing.T) {
	items := []ListItem{{ID: "tag:backend"}, {ID: "tag:platform-api"}}
	surface := mustListSurface(t, items, nil, ListSurfaceConfig{SelectionMode: ListSelectionMulti})

	seed := []ListItemID{"tag:security", "tag:backend"}
	if err := surface.SetSelectedIDs(seed); err == nil {
		t.Fatal("SetSelectedIDs accepted an unknown ID; the strict path is what the seed guards")
	}
	source := ListSource{Items: items}
	if err := surface.SetSelectedIDs(source.RetainKnownIDs(seed)); err != nil {
		t.Fatalf("SetSelectedIDs(retained) error = %v", err)
	}
	selected := surface.SelectedIDs()
	if len(selected) != 1 || selected[0] != "tag:backend" {
		t.Fatalf("selected = %v, want [tag:backend]", selected)
	}
}
