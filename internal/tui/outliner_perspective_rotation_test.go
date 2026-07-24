package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestOutlinerPerspectiveLabelsUseCanonicalOrderAcrossScopes(t *testing.T) {
	surfaces := []struct {
		name    string
		project *models.Project
	}{
		{name: "project", project: &models.Project{ID: "project"}},
		{name: "cross-project"},
	}
	cases := []struct {
		id   int
		want string
	}{
		{id: PerspectiveAll, want: "‹ Completed @ All Active ›"},
		{id: PerspectiveReview, want: "‹ Someday @ Review Snoozed ›"},
		{id: PerspectiveSnoozed, want: "‹ Review @ Snoozed Completed ›"},
		{id: PerspectiveCompleted, want: "‹ Snoozed @ Completed All ›"},
	}

	for _, surface := range surfaces {
		for _, tc := range cases {
			t.Run(surface.name+"/"+PerspectiveLensName(tc.id), func(t *testing.T) {
				m := minimalOutliner()
				m.project = surface.project
				m.activePerspective = tc.id
				plain := ansi.Strip(m.buildPerspectiveLabel(m.currentPerspective()))
				normalized := strings.Join(strings.Fields(plain), " ")
				if normalized != tc.want {
					t.Fatalf("perspective label = %q, want %q", normalized, tc.want)
				}
			})
		}
	}
}

func TestOutlinerPerspectiveCyclingUsesCanonicalOrderAcrossScopes(t *testing.T) {
	assertSequence := func(t *testing.T, m *OutlinerModel, cycle func()) {
		t.Helper()
		m.activePerspective = PerspectiveReview
		for _, want := range []int{PerspectiveSnoozed, PerspectiveCompleted, PerspectiveAll} {
			cycle()
			if m.activePerspective != want {
				t.Fatalf("cycled perspective = %d, want %d", m.activePerspective, want)
			}
		}
	}

	t.Run("project", func(t *testing.T) {
		m := minimalOutliner()
		m.project = &models.Project{ID: "project"}
		action := findTopLevelBinding(t, newOutlinerKeymapBuilder(m).perspectiveCycleEntries(), "l")
		assertSequence(t, m, func() { _, _ = action() })
	})

	t.Run("cross-project", func(t *testing.T) {
		m := minimalOutliner()
		assertSequence(t, m, func() { _, _ = m.crossHandlePerspectiveCycle("l") })
	})
}

// TestOutlinerModeKeymapBindsPickerAndCycle verifies the retired P0-P9 numeric
// prefix is replaced by a single P binding (opening the searchable mode picker)
// plus Tab/Shift+Tab for the Work cycle. No numeric-prefix children remain.
func TestOutlinerModeKeymapBindsPickerAndCycle(t *testing.T) {
	entries := newOutlinerKeymapBuilder(minimalOutliner()).modeEntries()
	got := map[string]string{}
	for _, entry := range entries {
		if entry.PrefixNode != nil {
			t.Fatalf("mode entries must not use a prefix subtree; found prefix %q", entry.PrefixNode.Key)
		}
		if entry.Binding != nil {
			got[entry.Binding.Key] = entry.Binding.Description
		}
	}
	want := map[string]string{
		"P":         "Switch mode",
		"tab":       "Next work mode",
		"shift+tab": "Previous work mode",
	}
	for key, description := range want {
		if got[key] != description {
			t.Errorf("mode binding %q = %q, want %q", key, got[key], description)
		}
	}
}
