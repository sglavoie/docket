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
	want := theme.TagChipStyle("alpha").Render("#alpha") + theme.MutedStyle.Render(" (new)") + " " + theme.TagChipStyle("beta").Render("#beta") + theme.MutedStyle.Render(" (new)")
	if tags != want {
		t.Fatalf("tags = %q, want %q", tags, want)
	}
	if ansi.Strip(tags) != "#alpha (new) #beta (new)" {
		t.Fatalf("stripped tags = %q", ansi.Strip(tags))
	}
}

func TestCapturePreviewMarksOnlyNewVocabulary(t *testing.T) {
	home := &models.Project{ID: "home", Slug: "home", Name: "Home"}
	work := &models.Project{ID: "work", Slug: "work", Name: "Work", Tasks: []*models.Task{
		{Tags: []string{"Existing", "office"}, Context: "Office"},
	}}
	q := draftedCaptureInput(t, "Task #existing #fresh ##Office", home, []*models.Project{home, work}, false)
	q.Width = 100
	q.UpdatePreview()
	for _, preview := range []string{q.renderPreview(), q.renderCompactPreview()} {
		plain := ansi.Strip(preview)
		if !strings.Contains(plain, "#fresh (new)") || strings.Contains(plain, "#existing (new)") || strings.Contains(plain, "##office (new)") {
			t.Fatalf("incorrect vocabulary markers: %s", plain)
		}
	}
	q.Input.SetValue("Task ##existing")
	q.UpdatePreview()
	if plain := ansi.Strip(q.renderPreview()); !strings.Contains(plain, "##existing (new)") {
		t.Fatalf("a tag must not count as an existing context: %s", plain)
	}
	q.Input.SetValue("Task ##offcie")
	q.UpdatePreview()
	if plain := ansi.Strip(q.renderPreview()); !strings.Contains(plain, "##offcie (new)") {
		t.Fatalf("misspelled context was not marked: %s", plain)
	}
	if q.Preview.Context != "offcie" {
		t.Fatal("preview corrected the literal context")
	}
}

func TestDuplicateCaptureHintUsesParsedTitleAndResolvedProject(t *testing.T) {
	home := &models.Project{ID: "home", Slug: "home", Name: "Home"}
	web := &models.Project{ID: "web", Slug: "web", Name: "Web", Tasks: []*models.Task{
		{ID: "open", ShortID: 42, Title: "Fix login", WorkflowState: models.StateBacklog},
		{ID: "done", ShortID: 1, Title: "Finished", WorkflowState: models.StateDone},
		{ID: "canceled", ShortID: 2, Title: "Canceled", WorkflowState: models.StateCanceled},
	}}
	for _, tc := range []struct{ draft, want string }{
		{"Fix login %web !1 #auth", "Already exists: web#42"},
		{"Fix login", ""}, {"fix login %web", "Already exists: web#42"}, {"Fix login later %web", ""},
		{"Finished %web", ""}, {"Canceled %web", ""}, {"Fix login %missing", ""},
	} {
		t.Run(tc.draft, func(t *testing.T) {
			q := draftedCaptureInput(t, tc.draft, home, []*models.Project{home, web}, false)
			q.UpdatePreview()
			if got := q.duplicateCaptureHint(); got != tc.want {
				t.Fatalf("hint = %q, want %q", got, tc.want)
			}
			if tc.want != "" {
				if _, err := ParseQuickCapture(tc.draft, home, []*models.Project{home, web}, false); err != nil {
					t.Fatalf("duplicate blocked capture: %v", err)
				}
				if !strings.Contains(ansi.Strip(q.renderPreview()), tc.want) || !strings.Contains(ansi.Strip(q.renderCompactPreview()), tc.want) {
					t.Fatal("preview omitted duplicate hint")
				}
			}
		})
	}
}

func TestBatchDuplicateCaptureHintFollowsParsedTitleProjectAndCaret(t *testing.T) {
	home := &models.Project{ID: "home", Slug: "home", Name: "Home"}
	web := &models.Project{ID: "web", Slug: "web", Name: "Web"}
	for _, tc := range []struct{ name, draft, want string }{
		{"metadata", "Fix login !1 %web\nFix login #auth %web", "Same task on line 2"},
		{"blank lines", "Fix login\n\nFix login", "Same task on line 3"},
		{"project override", "Fix login %home\nFix login", "Same task on line 2"},
		{"different project", "Fix login %web\nFix login", ""},
		{"different case", "Fix login\nfix login", "Same task on line 2"},
		{"invalid sibling", "Fix login\nFix login !9", ""},
		{"invalid routing", "Fix login\nFix login %missing", ""},
		{"invalid current", "Fix login !9\nFix login", ""},
		{"blank current", "\nFix login\nFix login", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := draftedCaptureInput(t, tc.draft, home, []*models.Project{home, web}, false)
			q.Input.MoveToBegin()
			q.UpdatePreview()
			if q.batchDuplicateHint != tc.want {
				t.Fatalf("hint = %q, want %q", q.batchDuplicateHint, tc.want)
			}
			if tc.want != "" {
				for _, rendered := range []string{q.renderPreview(), q.renderCompactPreview()} {
					if !strings.Contains(ansi.Strip(rendered), tc.want) {
						t.Fatalf("notice missing: %s", ansi.Strip(rendered))
					}
				}
				for _, line := range strings.Split(tc.draft, "\n") {
					if strings.TrimSpace(line) == "" {
						continue
					}
					if _, err := ParseQuickCapture(line, home, []*models.Project{home, web}, false); err != nil {
						t.Fatalf("advisory blocked valid capture: %v", err)
					}
				}
			}
		})
	}
	q := draftedCaptureInput(t, "Fix login\nFix login", home, nil, false)
	q.Input.MoveToEnd()
	q.UpdatePreview()
	if q.batchDuplicateHint != "Same task on line 1" {
		t.Fatal(q.batchDuplicateHint)
	}
	q.Input.SetValue("Fix login\nDifferent task")
	q.UpdatePreview()
	if q.batchDuplicateHint != "" {
		t.Fatal("stale duplicate hint after editing")
	}
}
