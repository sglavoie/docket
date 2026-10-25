package grammar

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/capturetext"
	"github.com/sglavoie/docket/internal/models"
)

func TestCaptureProjectTypoSuggestionsDoNotRoute(t *testing.T) {
	projects := []*models.Project{{ID: "web-id", Slug: "web", Name: "Website"}}
	for _, ref := range []string{"weeb", "we", "wet", "wbe", "WEB"} {
		t.Run(ref, func(t *testing.T) {
			input := "Fix login %" + ref
			result, err := capturetext.ParseQuickCapture(input, nil, projects, false)
			if err == nil || !strings.Contains(err.Error(), "did you mean %web?") || result != nil {
				t.Fatalf("capture = %+v, %v; want a suggestion and refusal", result, err)
			}
			preview, message := capturetext.ParseQuickCaptureForPreview(input, nil, projects, false)
			if !strings.Contains(message, "did you mean %web?") || preview.TargetProjectID != nil {
				t.Fatalf("preview = %+v, %q; must not route a typo", preview, message)
			}
		})
	}
	result, err := capturetext.ParseQuickCapture("Fix login %web", nil, projects, false)
	if err != nil || result.TargetProjectID == nil || *result.TargetProjectID != "web-id" {
		t.Fatalf("corrected capture = %+v, %v", result, err)
	}
	_, err = capturetext.ParseQuickCapture("Fix login %unrelated", nil, projects, false)
	if err == nil || strings.Contains(err.Error(), "did you mean") {
		t.Fatalf("unrelated reference = %v; want the ordinary refusal", err)
	}
}

func TestProjectSuggestionsAreBoundedAndOnlyNameAvailableSlugs(t *testing.T) {
	projects := []*models.Project{
		nil, {Slug: ""}, {Slug: "weeb", Archived: true},
		{Slug: "web"}, {Slug: "web"}, {Slug: "weed"}, {Slug: "weel"}, {Slug: "week"},
	}
	want := []string{"%web", "%weed", "%week"}
	if got := capturetext.SuggestProjectSlugs("weeb", projects); !slices.Equal(got, want) {
		t.Fatalf("suggestions = %v, want %v", got, want)
	}
	slices.Reverse(projects)
	if got := capturetext.SuggestProjectSlugs("weeb", projects); !slices.Equal(got, want) {
		t.Fatalf("reversed suggestions = %v, want %v", got, want)
	}
	if got := capturetext.SuggestProjectSlugs("cafe", []*models.Project{{Slug: "café"}}); !slices.Equal(got, []string{"%café"}) {
		t.Fatalf("Unicode suggestion = %v", got)
	}
	for _, ref := range []string{"", "w", "zzzz", "wxez"} {
		if got := capturetext.SuggestProjectSlugs(ref, []*models.Project{{Slug: "web"}}); len(got) != 0 {
			t.Errorf("%q suggested unrelated slugs: %v", ref, got)
		}
	}
}

func TestEditProjectTypoSuggestionsDoNotRoute(t *testing.T) {
	task := &models.Task{ID: "task", ShortID: 1, Title: "Fix login", WorkflowState: models.StateBacklog}
	source := &models.Project{ID: "source", Slug: "source", Tasks: []*models.Task{task}}
	web := &models.Project{ID: "web-id", Slug: "web", Name: "Website"}
	projects := []*models.Project{source, web, {ID: "archived", Slug: "old", Archived: true}}
	for _, ref := range []string{"weeb", "we", "wet", "wbe", "WEB", "sourc", "ol", "unrelated"} {
		t.Run(ref, func(t *testing.T) {
			input := "Fix login %" + ref
			_, captureErr := capturetext.ParseQuickCapture(input, source, projects, false)
			result, err := ParseQuickEdit(input, task, source, projects)
			if result != nil || err == nil || captureErr == nil {
				t.Fatalf("invalid project was accepted: %+v, %v, %v", result, err, captureErr)
			}
			// Capture and edit retain their error prefixes but explain recovery alike.
			want := captureErr.Error()[strings.Index(captureErr.Error(), "(use an exact"):]
			preview, message := parseQuickEditForPreview(input, task, source, projects)
			if preview.TargetProject != nil || !strings.Contains(message, want) || !strings.Contains(err.Error(), want) {
				t.Fatalf("edit/preview lost suggestion or routed a typo: %+v, %q, %v", preview, message, err)
			}
			for _, candidates := range []func(string, *models.Task, *models.Project, []*models.Project) ([]*models.Task, string){QuickEditParentCandidates, QuickEditChildCandidates} {
				items, reason := candidates(input, task, source, projects)
				if len(items) != 0 || !strings.Contains(reason, want) {
					t.Fatalf("completion = %v, %q; want refusal with %q", items, reason, want)
				}
			}
		})
	}
	result, err := ParseQuickEdit("Fix login %web", task, source, projects)
	if err != nil || result.TargetProject != web {
		t.Fatalf("corrected edit = %+v, %v", result, err)
	}
	if task.Title != "Fix login" || source.Tasks[0] != task || len(web.Tasks) != 0 {
		t.Fatal("parsing modified the source or destination")
	}
}

func TestCrossProjectCapture(t *testing.T) {
	currentProject := &models.Project{
		ID:   "current-project",
		Slug: "current-project",
		Name: "Current Project",
	}
	targetProject := &models.Project{
		ID:    "other-project",
		Slug:  "other-project",
		Name:  "Other Project",
		Tasks: []*models.Task{{ID: "other-parent", ShortID: 1, Title: "Other Parent"}},
	}
	allProjects := []*models.Project{currentProject, targetProject}

	tests := []struct {
		name           string
		input          string
		wantTitle      string
		wantTargetID   *string
		wantTargetName string
		wantParentID   *string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "no target project",
			input:          "Simple task",
			wantTitle:      "Simple task",
			wantTargetID:   nil,
			wantTargetName: "",
		},
		{
			name:           "target project by slug",
			input:          "Task for other %other-project",
			wantTitle:      "Task for other",
			wantTargetID:   strPtr("other-project"),
			wantTargetName: "Other Project",
		},
		{
			name:        "target project requires exact slug",
			input:       "%other Task in other project",
			wantErr:     true,
			errContains: "exact %project-slug",
		},
		{
			name:           "project resolves before earlier parent token",
			input:          "Routed child >1 %other-project",
			wantTitle:      "Routed child",
			wantTargetID:   strPtr("other-project"),
			wantTargetName: "Other Project",
			wantParentID:   strPtr("other-parent"),
		},
		{
			name:        "nonexistent project",
			input:       "%nonexistent Some task",
			wantErr:     true,
			errContains: "project not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := capturetext.ParseQuickCapture(tt.input, currentProject, allProjects, false)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", result.Title, tt.wantTitle)
			}

			if tt.wantTargetID == nil {
				if result.TargetProjectID != nil {
					t.Errorf("targetProjectID = %q, want nil", *result.TargetProjectID)
				}
			} else {
				if result.TargetProjectID == nil {
					t.Errorf("targetProjectID = nil, want %q", *tt.wantTargetID)
				} else if *result.TargetProjectID != *tt.wantTargetID {
					t.Errorf("targetProjectID = %q, want %q", *result.TargetProjectID, *tt.wantTargetID)
				}
			}

			if result.TargetProjectName != tt.wantTargetName {
				t.Errorf("TargetProjectName = %q, want %q", result.TargetProjectName, tt.wantTargetName)
			}
			if tt.wantParentID != nil && (result.ParentID == nil || *result.ParentID != *tt.wantParentID) {
				t.Errorf("parentID = %v, want %q", result.ParentID, *tt.wantParentID)
			}
		})
	}
}
