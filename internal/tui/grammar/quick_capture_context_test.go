package grammar

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestCaptureContextSuggestionsAcrossProjects(t *testing.T) {
	unfiled := &models.Project{ID: "unfiled", Slug: "unfiled", Name: "Unfiled"}
	web := &models.Project{ID: "web", Slug: "web", Name: "Web", Tasks: []*models.Task{
		{Context: "Office"}, {Context: "home"}, nil, {},
	}}
	other := &models.Project{ID: "other", Tasks: []*models.Task{{Context: "office"}}}
	archived := &models.Project{ID: "archived", Archived: true, Tasks: []*models.Task{{Context: "retired"}}}
	projects := []*models.Project{unfiled, web, other, archived}
	for _, draft := range []string{"Work ##", "Work %web ##", "First ##home\nSecond %web ##"} {
		t.Run(draft, func(t *testing.T) {
			q := draftedCaptureInput(t, draft, unfiled, projects, false)
			if got, want := suggestionDisplays(q.AC.Items), []string{"##home", "##office"}; !slices.Equal(got, want) {
				t.Fatalf("suggestions = %v, want %v", got, want)
			}
			q.Input.SetValue(draft + "off")
			q.RefreshAutocomplete()
			if got := suggestionDisplays(q.AC.Items); !slices.Equal(got, []string{"##office"}) {
				t.Fatalf("filtered suggestions = %v, want ##office", got)
			}
		})
	}
	q := draftedCaptureInput(t, "Work ##", web, nil, false)
	if got := suggestionDisplays(q.AC.Items); !slices.Equal(got, []string{"##home", "##office"}) {
		t.Fatalf("source-only suggestions = %v", got)
	}
}

func TestCaptureContextPreviewFollowsDraftLine(t *testing.T) {
	q := draftedCaptureInput(t, "First ##Office\nSecond ##Home", nil, nil, false)
	q.Width = 56 // The preview's content width at the 60-column terminal floor.
	for _, tc := range []struct {
		line int
		want string
		omit string
	}{{0, "##office", "##home"}, {1, "##home", "##office"}} {
		q.Input.MoveToBegin()
		for range tc.line {
			q.Input.CursorDown()
		}
		q.UpdatePreview()
		if q.PreviewErr != "" {
			t.Fatal(q.PreviewErr)
		}
		for _, preview := range []string{q.renderPreview(), q.ComposeBoundedPreviewBlock(2)} {
			plain := ansi.Strip(preview)
			if !strings.Contains(plain, tc.want) || strings.Contains(plain, tc.omit) {
				t.Fatalf("line %d preview = %q, want %s only", tc.line, plain, tc.want)
			}
			for _, line := range strings.Split(preview, "\n") {
				if ansi.StringWidth(line) > q.Width {
					t.Fatalf("preview exceeds %d columns: %q", q.Width, line)
				}
			}
		}
	}
}
