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

func TestOutlinerPerspectiveShortcutsUseRemappedIDsAcrossScopes(t *testing.T) {
	cases := []struct {
		key  string
		want int
	}{
		{key: "0", want: PerspectiveCompleted},
		{key: "8", want: PerspectiveReview},
		{key: "9", want: PerspectiveSnoozed},
	}
	for _, crossProject := range []bool{false, true} {
		for _, tc := range cases {
			m := minimalOutliner()
			scope := "cross-project"
			if !crossProject {
				scope = "project"
				m.project = &models.Project{ID: "project"}
			}
			t.Run(scope+"/P"+tc.key, func(t *testing.T) {
				_, _ = m.handlePerspectivePrefix(tc.key)
				if m.activePerspective != tc.want {
					t.Fatalf("active perspective = %d, want %d", m.activePerspective, tc.want)
				}
			})
		}
	}
}

func TestOutlinerPerspectiveHintsUseRemappedKeys(t *testing.T) {
	for _, tc := range []struct {
		id      int
		wantKey string
	}{
		{id: PerspectiveCompleted, wantKey: "0"},
		{id: PerspectiveReview, wantKey: "8"},
		{id: PerspectiveSnoozed, wantKey: "9"},
	} {
		m := minimalOutliner()
		m.activePerspective = tc.id
		hints := m.buildOutlinerPerspectiveHints("P")
		found := false
		for _, hint := range hints {
			if hint.Key == tc.wantKey && hint.Active {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("perspective %d did not activate shortcut hint %q", tc.id, tc.wantKey)
		}
	}
}

func TestOutlinerPerspectiveKeymapUsesRemappedDescriptions(t *testing.T) {
	entries := newOutlinerKeymapBuilder(minimalOutliner()).perspectiveEntries()
	want := map[string]string{
		"0": "Completed perspective",
		"8": "Review perspective",
		"9": "Snoozed perspective",
	}
	for _, entry := range entries {
		if entry.PrefixNode == nil || entry.PrefixNode.Key != "P" {
			continue
		}
		for _, child := range entry.PrefixNode.Children {
			if child.Binding == nil {
				continue
			}
			if description, ok := want[child.Binding.Key]; ok {
				if child.Binding.Description != description {
					t.Errorf("P%s description = %q, want %q", child.Binding.Key, child.Binding.Description, description)
				}
				delete(want, child.Binding.Key)
			}
		}
	}
	for key, description := range want {
		t.Errorf("missing P%s binding for %q", key, description)
	}
}
