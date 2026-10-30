package outliner

import (
	"reflect"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
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

func TestFullscreenPeekUsesTheFrameAndRestoresTheDock(t *testing.T) {
	for _, size := range [][2]int{{60, 20}, {80, 24}, {120, 40}} {
		m := dockFixtureOutliner(t, size[0], size[1])
		cursors, _ := peekableRowIndexes(t, m)
		m.Cursor = cursors[len(cursors)-1]
		m.TogglePeek()
		m.HalfPagePeek(1)
		m.View()
		compact, cursor, offset, reading := m.PeekDockRows(), m.Cursor, m.ViewScrollOffset, m.peek
		m.ToggleFullscreenPeek()
		for _, frame := range [][2]int{size, {60, 20}, size} {
			m.SetSize(frame[0], frame[1])
			view := m.View().Content
			if !m.PeekFullscreen || m.PeekDockRows() <= compact || lipgloss.Height(view) != m.outlinerHostFrameHeight(frame[0]) {
				t.Fatalf("%v fullscreen did not fill its frame: %d rows", frame, lipgloss.Height(view))
			}
			for _, line := range strings.Split(view, "\n") {
				if lipgloss.Width(line) > frame[0] {
					t.Fatalf("overflow: %q", ansi.Strip(line))
				}
			}
		}
		m.ToggleFullscreenPeek()
		if m.PeekFullscreen || !m.PeekMode || m.Cursor != cursor || !reflect.DeepEqual(m.peek, reading) {
			t.Fatal("fullscreen did not restore the compact reader")
		}
		// Resize deliberately resets the outline viewport; opening and closing the
		// reader itself must preserve it when the terminal stays the same size.
		m.ViewScrollOffset = offset
		m.ToggleFullscreenPeek()
		m.View()
		m.ToggleFullscreenPeek()
		if m.ViewScrollOffset != offset {
			t.Fatal("fullscreen moved the outline window")
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
