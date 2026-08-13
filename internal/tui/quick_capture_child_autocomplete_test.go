package tui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

// draftedCaptureInput builds a quickCaptureInput holding value and refreshes its
// autocomplete, so a test can assert what the composer would offer for a draft.
func draftedCaptureInput(t *testing.T, value string, project *models.Project, allProjects []*models.Project, skipProject bool) *quickCaptureInput {
	t.Helper()
	input := newQuickCaptureInput(project, allProjects, "", skipProject)
	q := &input
	q.width = 80
	q.input.SetValue(value)
	q.refreshAutocomplete()
	return q
}

func suggestionDisplays(items []ACSuggestion) []string {
	displays := make([]string, len(items))
	for i, item := range items {
		displays[i] = ansi.Strip(item.Display)
	}
	return displays
}

func routedCaptureProject() *models.Project {
	return &models.Project{
		ID: "routed-project", Name: "Routed", Slug: "routed",
		Tasks: []*models.Task{
			{ID: "routed-1", ShortID: 1, Title: "Routed alpha"},
			{ID: "routed-2", ShortID: 2, Title: "Routed beta"},
		},
	}
}

func TestCaptureAutocompleteChildSuggestionsRenderChildSigilInProjectOrder(t *testing.T) {
	q := draftedCaptureInput(t, "Adopt <", quickCaptureTestProject(), nil, false)

	if q.ac.Mode != acModeChild {
		t.Fatalf("autocomplete mode = %d, want acModeChild", q.ac.Mode)
	}
	want := []string{
		"<1 User Authentication",
		"<2 Login Flow",
		"<3 Implement button",
		"<4 Implement search",
	}
	got := suggestionDisplays(q.ac.Items)
	if len(got) != len(want) {
		t.Fatalf("child suggestions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("child suggestion %d = %q, want %q", i, got[i], want[i])
		}
	}
	if value := q.ac.Items[1].Value; value != "2" {
		t.Errorf("child suggestion value = %q, want the short ID %q", value, "2")
	}
}

func TestCaptureAutocompleteChildSuggestionsRepeatAndFilterAcceptedAliases(t *testing.T) {
	tests := []struct {
		name  string
		draft string
	}{
		{"short id", "Adopt <2 <"},
		{"title prefix alias", "Adopt <Login <"},
		{"both aliases for one task", "Adopt <2 <Login <"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := draftedCaptureInput(t, tt.draft, quickCaptureTestProject(), nil, false)
			if q.ac.Mode != acModeChild {
				t.Fatalf("autocomplete mode = %d, want acModeChild after an earlier child token", q.ac.Mode)
			}
			got := suggestionDisplays(q.ac.Items)
			if len(got) != 3 {
				t.Fatalf("child suggestions = %v, want 3 remaining candidates", got)
			}
			for _, display := range got {
				if strings.HasPrefix(display, "<2 ") {
					t.Errorf("already-selected task reappeared in %v", got)
				}
			}
		})
	}
}

func TestCaptureAutocompleteChildAndParentFollowEffectiveProject(t *testing.T) {
	source := quickCaptureTestProject()
	routed := routedCaptureProject()
	all := []*models.Project{source, routed}

	child := draftedCaptureInput(t, "Adopt %routed <", source, all, false)
	if got, want := suggestionDisplays(child.ac.Items), []string{"<1 Routed alpha", "<2 Routed beta"}; !slices.Equal(got, want) {
		t.Errorf("routed child suggestions = %v, want %v", got, want)
	}

	parent := draftedCaptureInput(t, "Adopt %routed >", source, all, false)
	if got, want := suggestionDisplays(parent.ac.Items), []string{"#1 Routed alpha", "#2 Routed beta"}; !slices.Equal(got, want) {
		t.Errorf("routed parent suggestions = %v, want %v", got, want)
	}

	linked := draftedCaptureInput(t, "Adopt %routed <", source, all, true)
	if got := suggestionDisplays(linked.ac.Items); len(got) != len(source.Tasks) {
		t.Errorf("project-disabled capture suggestions = %v, want the source project's %d tasks", got, len(source.Tasks))
	}
}

func TestCaptureAutocompleteParentStaysSingletonWhileChildRepeats(t *testing.T) {
	project := quickCaptureTestProject()

	parent := draftedCaptureInput(t, "Adopt >1 >", project, nil, false)
	if parent.ac.Mode != acModeNone || len(parent.ac.Items) != 0 {
		t.Fatalf("parent suggestions after >1 = %v (mode %d), want none", suggestionDisplays(parent.ac.Items), parent.ac.Mode)
	}

	child := draftedCaptureInput(t, "Adopt <1 <", project, nil, false)
	if child.ac.Mode != acModeChild || len(child.ac.Items) == 0 {
		t.Fatalf("child suggestions after <1 = %v (mode %d), want more candidates", suggestionDisplays(child.ac.Items), child.ac.Mode)
	}
}

func TestCaptureAutocompleteWithholdsCompletedTasksThatStayTypeable(t *testing.T) {
	project := quickCaptureTestProject()
	project.Tasks[2].WorkflowState = models.StateDone

	q := draftedCaptureInput(t, "Adopt <", project, nil, false)
	for _, display := range suggestionDisplays(q.ac.Items) {
		if strings.HasPrefix(display, "<3 ") {
			t.Fatalf("completed task was suggested: %v", suggestionDisplays(q.ac.Items))
		}
	}

	parsed, err := parseQuickCapture("Adopt <3", project, nil, false)
	if err != nil {
		t.Fatalf("manually typed completed-task reference: %v", err)
	}
	if got := parsed.childIDs(); len(got) != 1 || got[0] != "task-3" {
		t.Fatalf("child IDs = %v, want the completed task", got)
	}
}

func TestSyntaxGuideAdvertisesRepeatableChildPlacement(t *testing.T) {
	definition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldChild)
	if !ok {
		t.Fatal("capture child grammar is missing")
	}
	if !strings.Contains(strings.ToLower(definition.Description), "repeatable") {
		t.Fatalf("child description = %q, want a repeatability hint", definition.Description)
	}

	for _, skipProject := range []bool{false, true} {
		q := &quickCaptureInput{width: 120, skipProjectSyntax: skipProject}
		rendered := ansi.Strip(q.renderSyntaxGuide())
		if !strings.Contains(rendered, "<child") {
			t.Errorf("syntax guide (skipProject=%v) omits <child:\n%s", skipProject, rendered)
		}
		if !strings.Contains(rendered, definition.Description) {
			t.Errorf("syntax guide (skipProject=%v) omits the child description:\n%s", skipProject, rendered)
		}
		if got := strings.Contains(rendered, "%project"); got == skipProject {
			t.Errorf("syntax guide (skipProject=%v) project routing present = %v", skipProject, got)
		}
	}
}

func TestRenderPreview_ChildrenGroupFollowsRouteInReferenceOrder(t *testing.T) {
	q := makePreviewInput(t, &quickCaptureResult{
		title: "Ship it",
		children: []capturedChildRef{
			{id: "task-3", shortID: 3, title: "Implement button"},
			{id: "epic-1", shortID: 1, title: "User Authentication"},
		},
	}, "", 0)

	lines := strings.Split(ansi.Strip(q.renderPreview()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected Task, Route and Children lines, got %d:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	if !strings.HasPrefix(lines[2], "Children") {
		t.Fatalf("expected Children directly after Route, got %q", lines[2])
	}
	if idx3, idx1 := strings.Index(lines[2], "<3 Implement button"), strings.Index(lines[2], "<1 User Authentication"); idx3 < 0 || idx1 < 0 || idx3 > idx1 {
		t.Fatalf("Children line lost first-reference order: %q", lines[2])
	}

	without := makePreviewInput(t, &quickCaptureResult{title: "Ship it"}, "", 0)
	if strings.Contains(ansi.Strip(without.renderPreview()), "Children") {
		t.Fatal("Children group rendered without any child reference")
	}
}

func TestRenderPreview_ChildrenSurviveCompactFlattening(t *testing.T) {
	q := makePreviewInput(t, &quickCaptureResult{
		title:    "Ship it",
		children: []capturedChildRef{{id: "story-2", shortID: 2, title: "Login Flow"}},
	}, "", 120)

	compact := ansi.Strip(q.renderCompactPreview())
	if !strings.Contains(compact, "<2 Login Flow") {
		t.Fatalf("compact preview dropped the child selection: %q", compact)
	}
	if strings.Contains(compact, "Children") {
		t.Fatalf("compact preview should carry no group label: %q", compact)
	}
}

func TestRenderPreview_ChildrenNarrowSafeWithWideRunes(t *testing.T) {
	for _, width := range []int{28, 32, 100} {
		q := makePreviewInput(t, &quickCaptureResult{
			title: "修复收件箱卡片在窄宽度下的宽字符标题截断问题",
			children: []capturedChildRef{
				{id: "wide-1", shortID: 11, title: "登录流程与凭据刷新"},
				{id: "wide-2", shortID: 12, title: "Implement search across every configured workspace"},
			},
		}, "", width)

		for _, line := range strings.Split(q.renderPreview(), "\n") {
			stripped := ansi.Strip(line)
			if got := lipgloss.Width(stripped); got > width {
				t.Errorf("preview line exceeds width %d: %q (width=%d)", width, stripped, got)
			}
		}
	}
}
