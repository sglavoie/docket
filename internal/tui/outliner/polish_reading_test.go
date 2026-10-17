package outliner

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestProjectLabelCacheTracksPopulationAndRenames(t *testing.T) {
	alpha := &models.Project{ID: "a", Slug: "platform-services-api"}
	beta := &models.Project{ID: "b", Slug: "platform-services-web"}
	m := NewOutlinerModel(nil, nil, nil)
	m.Population.Projects = []*models.Project{alpha, beta}
	check := func(width int) {
		t.Helper()
		names := []string{alpha.Slug, beta.Slug}
		want := projectAbbreviations(names, width)
		for _, name := range names {
			if got := m.abbreviatedProjectSlug(name, width); got != want[name] {
				t.Fatalf("%q => %q want %q", name, got, want[name])
			}
		}
	}
	check(12)
	check(24)
	beta.Slug = "platform-services-infra"
	check(12)
	m.Population.Projects = []*models.Project{beta, alpha}
	check(12)
	m.Population.Projects = []*models.Project{alpha}
	if got := m.abbreviatedProjectSlug(alpha.Slug, 12); got != projectAbbreviations([]string{alpha.Slug}, 12)[alpha.Slug] {
		t.Fatal("removed project left stale collision")
	}
	if got := m.abbreviatedProjectSlug("standalone-preview", 12); got == "" || lipgloss.Width(got) > 12 {
		t.Fatal("missing standalone preview")
	}
	standalone := "platform-services-standalone"
	m.abbreviatedProjectSlug(standalone, 12)
	if allocations := testing.AllocsPerRun(20, func() { m.abbreviatedProjectSlug(standalone, 12) }); allocations != 0 {
		t.Fatalf("unchanged standalone preview allocated %g times", allocations)
	}
	if got := m.abbreviatedProjectSlug(alpha.Slug, 12); got != projectAbbreviations([]string{alpha.Slug}, 12)[alpha.Slug] {
		t.Fatal("standalone preview changed the loaded project's label")
	}
}

func TestProjectAbbreviationsDistinguishSharedPrefixes(t *testing.T) {
	names := []string{"platform-services-api", "platform-services-web", "platform-services-infra", "short", "国際化プロジェクト-one", "国際化プロジェクト-two", "same-prefix-a-same-suffix", "same-prefix-b-same-suffix"}
	for _, width := range []int{12, 18, 24} {
		labels := projectAbbreviations(names, width)
		seen := map[string]bool{}
		for _, name := range names {
			label := labels[name]
			if label == "" || seen[label] || lipgloss.Width(label) > width {
				t.Fatalf("width %d: %q => %q (labels %v)", width, name, label, labels)
			}
			seen[label] = true
		}
		if labels["short"] != "short" {
			t.Fatal("short names should be unchanged")
		}
	}
	labels := projectAbbreviations(names, 12)
	if !strings.HasSuffix(labels[names[0]], "api") || !strings.HasSuffix(labels[names[1]], "web") {
		t.Fatalf("lost distinguishing suffix: %v", labels)
	}
}

func TestExpandedPeekIsStableAndRemembersItsSize(t *testing.T) {
	for _, size := range [][2]int{{60, 20}, {80, 24}, {120, 40}} {
		m := dockFixtureOutliner(t, size[0], size[1])
		m.PeekMode = true
		compact := m.PeekDockRows()
		m.PeekExpanded = true
		expanded := m.PeekDockRows()
		if expanded <= compact {
			t.Fatalf("%v expanded %d <= compact %d", size, expanded, compact)
		}
		for i := range m.Rows {
			m.Cursor = i
			m.View()
			if m.PeekDockRows() != expanded {
				t.Fatal("cursor moved the dock")
			}
			if m.LastViewport.VisibleRows < 1 {
				t.Fatal("dock consumed the whole list")
			}
			for _, line := range m.PeekDockLines() {
				if lipgloss.Width(line) > size[0] {
					t.Fatalf("overflow: %q", ansi.Strip(line))
				}
			}
		}
		m.TogglePeek()
		m.PeekMode = true
		if m.PeekDockRows() != expanded {
			t.Fatal("closing lost the size preference")
		}
		m.SetSize(60, 20)
		m.SetSize(size[0], size[1])
		if m.PeekDockRows() != expanded {
			t.Fatal("resize lost expanded size")
		}
	}
}

func TestPeekNamesTheFullProject(t *testing.T) {
	m := dockFixtureOutliner(t, 80, 24)
	m.Population.Projects[0].Slug = "platform-services-api"
	m.PeekMode = true
	cursors, _ := peekableRowIndexes(t, m)
	for _, cursor := range cursors {
		if !m.Rows[cursor].IsTaskRow() {
			continue
		}
		m.Cursor = cursor
		doc := cursorPeekDocument(t, m)
		if !strings.Contains(ansi.Strip(strings.Join(doc.Body, "\n")), "Project: platform-services-api") {
			t.Fatal("Peek lost project identity")
		}
	}
}
