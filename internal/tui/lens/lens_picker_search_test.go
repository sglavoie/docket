package lens

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/theme"
)

func TestLensSearchHighlightsNameAndDefinitionWithoutChangingRows(t *testing.T) {
	previous := theme.HighlightStyle
	theme.HighlightStyle = lipgloss.NewStyle().Underline(true)
	t.Cleanup(func() { theme.HighlightStyle = previous })
	m, err := NewNamedLensPickerModel(operation.NewOperationExecutor(operation.NewOperationRegistry()), presentation.ViewCrossProjectOutliner,
		NamedLensPickerSnapshot{Lenses: config.NamedLensList{
			{ID: "release", Name: "界 Release", Mode: config.LensModeAll, Scope: "web", Query: "$waiting"},
			{ID: "other", Name: "Other", Mode: config.LensModeAll},
		}, ProjectSlugs: map[string]string{"web": "café"}})
	if err != nil {
		t.Fatal(err)
	}
	m.setQuery("界 café")
	items := m.Surface.Items()
	if len(items) != 3 || !slices.Equal(m.Ordinals, []string{"release"}) {
		t.Fatalf("matching changed the action rows or ordinals: %+v", items)
	}
	item := items[2]
	if item.PrimaryText != "界 Release" || !slices.Equal(item.PrimaryMatchPositions, []int{0}) || len(item.SecondaryMatchPositions) != 4 {
		t.Fatalf("incorrect field positions: %+v", item)
	}
	for _, cursor := range []bool{false, true} {
		view := listsurface.RenderListSurfaceRow(listsurface.ListSurfaceRow{Cursor: cursor, Match: listsurface.ListMatch{Item: item}}, listsurface.ListSelectionSingle, 80)
		style := theme.HighlightStyle
		if cursor {
			style = theme.SelectedStyle.Underline(true)
		}
		for _, char := range []string{"界", "é"} {
			if !strings.Contains(view, style.Render(char)) {
				t.Fatalf("row lost highlight %q: %q", char, view)
			}
		}
	}
	m.SelectLens("release")
	for _, size := range [][2]int{{60, 18}, {100, 35}, {60, 18}} {
		view := m.RenderWithin(size[0], size[1])
		if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
			t.Fatalf("highlighted picker exceeds %v", size)
		}
		if !strings.Contains(view, theme.HighlightStyle.Render("é")) {
			t.Fatalf("details lost highlight: %q", view)
		}
	}
	m.setQuery("")
	for _, item := range m.Surface.Items() {
		if len(item.PrimaryMatchPositions)+len(item.SecondaryMatchPositions) > 0 || item.PrimaryText != ansi.Strip(item.PrimaryText) {
			t.Fatal("clearing search retained highlights or styled data")
		}
	}
}

func TestLensScopeAvailabilityIsVisibleWithoutDisablingManagement(t *testing.T) {
	for _, tc := range []struct{ scope, reason, want string }{
		{"web", "project is parked", "Unavailable: project is parked"},
		{"web", "project is archived", "Unavailable: project is archived"},
		{"missing", "", "Unavailable: project no longer exists"},
		{"web", "", ""},
		{"", "", ""},
	} {
		t.Run(tc.scope+tc.reason, func(t *testing.T) {
			snapshot := NamedLensPickerSnapshot{Lenses: config.NamedLensList{{ID: "one", Name: "Release", Mode: config.LensModeAll, Scope: tc.scope}},
				ProjectSlugs: map[string]string{"web": "website"}, ProjectUnavailableReasons: map[string]string{"web": tc.reason}}
			m, err := NewNamedLensPickerModel(operation.NewOperationExecutor(operation.NewOperationRegistry()), presentation.ViewCrossProjectOutliner, snapshot)
			if err != nil {
				t.Fatal(err)
			}
			m.SelectLens("one")
			for _, size := range [][2]int{{60, 18}, {100, 35}} {
				view := ansi.Strip(m.RenderWithin(size[0], size[1]))
				if tc.want != "" && !strings.Contains(view, tc.want) {
					t.Fatalf("missing reason: %s", view)
				}
				if tc.want == "" && strings.Contains(view, "Unavailable:") {
					t.Fatalf("available lens carries a warning: %s", view)
				}
				if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
					t.Fatalf("reason exceeds %v", size)
				}
			}
			if _, ok := m.openActionsForCursor().(NamedLensActionsRequestMsg); !ok {
				t.Fatal("unavailable lens cannot be managed")
			}
			snapshot.ProjectSlugs[tc.scope] = "restored"
			delete(snapshot.ProjectUnavailableReasons, "web")
			if err := m.Refresh(snapshot); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(ansi.Strip(m.RenderWithin(60, 18)), "Unavailable:") {
				t.Fatal("refresh kept an obsolete reason")
			}
		})
	}
}
