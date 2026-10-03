package grammar

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestCaptureDuePreview(t *testing.T) {
	now := time.Date(2026, 12, 31, 12, 0, 0, 0, time.Local)
	for _, tc := range []struct {
		name string
		due  time.Time
		want string
	}{
		{"date only", time.Date(2026, 12, 31, 0, 0, 0, 0, time.Local), "Thu, Dec 31"},
		{"timed", time.Date(2026, 12, 31, 15, 30, 0, 0, time.Local), "Thu, Dec 31 15:30"},
		{"next year", time.Date(2027, 1, 1, 0, 0, 0, 0, time.Local), "Fri, Jan 1 2027"},
		{"previous year timed", time.Date(2025, 12, 31, 9, 5, 0, 0, time.Local), "Wed, Dec 31 2025 09:05"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := captureDuePreview(tc.due, now); got != tc.want {
				t.Fatalf("deadline = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCaptureTimedDeadlineSurvivesCompactPreview(t *testing.T) {
	due := time.Date(2027, 1, 1, 15, 30, 0, 0, time.Local)
	q := makePreviewInput(t, &QuickCaptureResult{Title: "Ship release", DueDate: &due}, "", 56)
	for _, preview := range []string{q.renderPreview(), q.renderCompactPreview()} {
		plain := ansi.Strip(preview)
		if !strings.Contains(plain, "@Fri, Jan 1") || !strings.Contains(plain, "15:30") {
			t.Fatalf("preview lost deadline details:\n%s", plain)
		}
	}
}

// TestCaptureDependencyPreviewNamesEveryReference is the preview half of the
// dependency capture contract: every resolved reference is named in the Depends
// group, in the order the line typed them. Its two siblings compare the typed
// route against the reference picker and so stay in the shell, which is the only
// package that can see the picker.
func TestCaptureDependencyPreviewNamesEveryReference(t *testing.T) {
	home, routed := dependencyWorkspace()
	q := draftedCaptureInput(t, "New work ~2 ~routed:1", home, []*models.Project{home, routed}, false)
	q.UpdatePreview()
	if q.PreviewErr != "" {
		t.Fatalf("preview error: %s", q.PreviewErr)
	}
	groups, ok := q.previewGroups()
	if !ok {
		t.Fatal("the composer painted no preview")
	}
	var depends *previewGroup
	for i, group := range groups {
		if group.label == "Depends" {
			depends = &groups[i]
		}
	}
	if depends == nil {
		t.Fatalf("preview groups = %+v, want a Depends group", groups)
	}
	want := []string{"~Home #2 Ship the API", "~Routed #1 Routed alpha"}
	if !slices.Equal(depends.chunks, want) {
		t.Fatalf("Depends chunks = %v, want %v", depends.chunks, want)
	}
}

func previewRouteAndTags(t *testing.T, value string) (route, tags string) {
	t.Helper()
	home := &models.Project{ID: "home", Name: "Home", Slug: "home"}
	q := draftedCaptureInput(t, value, home, []*models.Project{home}, false)
	q.UpdatePreview()
	groups, ok := q.previewGroups()
	if !ok {
		t.Fatalf("no preview for %q (%s)", value, q.PreviewErr)
	}
	for _, group := range groups {
		switch group.label {
		case "Route":
			route = strings.Join(group.chunks, " · ")
		case "Tags":
			tags = strings.Join(group.chunks, " · ")
		}
	}
	return route, tags
}

// TestCapturePreviewGlyphIsTheRowsStateBadge holds the preview's glyph to the
// badge the created row will wear: the destination's state and semantic accent.
func TestCapturePreviewGlyphIsTheRowsStateBadge(t *testing.T) {
	route, _ := previewRouteAndTags(t, "fix it #bug $doing")
	want := theme.GetTaskStateBadge(&models.Task{WorkflowState: models.StateDoing, Tags: []string{"bug"}})
	if !strings.HasPrefix(route, want+" ") {
		t.Fatalf("route %q does not lead with the Doing badge %q", route, want)
	}
	if want == theme.GetStateBadge(models.StateDoing) {
		t.Fatal("the #bug tag did not tint the state badge")
	}

	route, _ = previewRouteAndTags(t, "plain work $doing")
	if want := theme.GetStateBadge(models.StateDoing); !strings.HasPrefix(route, want+" ") {
		t.Fatalf("route %q does not lead with the plain Doing badge %q", route, want)
	}
	if !strings.Contains(route, CaptureStatusAccentStyle(models.StateDoing).Render("$doing")) {
		t.Fatalf("route %q does not accent the destination label", route)
	}
}

func TestCapturePreviewTagsWearTheirChips(t *testing.T) {
	_, tags := previewRouteAndTags(t, "Tagged #alpha #beta")
	want := theme.TagChipStyle("alpha").Render("#alpha") + " " + theme.TagChipStyle("beta").Render("#beta")
	if tags != want {
		t.Fatalf("tags = %q, want %q", tags, want)
	}
	if ansi.Strip(tags) != "#alpha #beta" {
		t.Fatalf("stripped tags = %q", ansi.Strip(tags))
	}
}
