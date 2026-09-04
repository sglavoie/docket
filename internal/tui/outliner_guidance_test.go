package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
)

func guidanceEntryText(spec chrome.GuidanceSpec) string {
	parts := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		parts = append(parts, chrome.GuidanceHintText(hint))
	}
	return strings.Join(parts, " · ")
}

func TestOutlinerGuidanceProfilesFollowInteractionState(t *testing.T) {
	app := newFilterChordTestApp(t)
	out := app.crossOutliner

	out.peekMode = true
	spec, ok := out.ContextualGuidance()
	if !ok || spec.Rows != 2 {
		t.Fatalf("Peek guidance = (%+v, %v), want two-row profile", spec, ok)
	}
	if got := guidanceEntryText(spec); !strings.Contains(got, "Ctrl+Enter close Peek") || !strings.Contains(got, "Esc close Peek") {
		t.Fatalf("Peek guidance = %q", got)
	}

	out.peekMode = false
	out.MultiSelectMode = true
	out.ReplaceSelection(out.visibleTaskIdentities()[:1])
	spec, ok = out.ContextualGuidance()
	if !ok || spec.Rows != 1 {
		t.Fatalf("selection guidance = (%+v, %v), want one-row browse profile", spec, ok)
	}
	if got := guidanceEntryText(spec); !strings.Contains(got, "Space actions") || !strings.Contains(got, "Esc clear selection") {
		t.Fatalf("selection guidance = %q", got)
	}

	out.MultiSelectMode = false
	out.ClearSelection()
	out.SetSearchQuery("critical")
	out.SetSearchActive(true)
	spec, ok = out.ContextualGuidance()
	if !ok || spec.Rows != 2 {
		t.Fatalf("search-edit guidance = (%+v, %v), want two-row profile", spec, ok)
	}
	if got := guidanceEntryText(spec); !strings.Contains(got, "Enter apply search") || !strings.Contains(got, "Esc clear search") {
		t.Fatalf("search-edit guidance = %q", got)
	}
}

func TestOutlinerHeaderGuidanceDoesNotAdvertiseTaskWorkflow(t *testing.T) {
	app := newFilterChordTestApp(t)
	out := app.crossOutliner
	header := -1
	for index, row := range out.rows {
		if row.IsSectionHeader || row.IsProjectHeader {
			header = index
			break
		}
	}
	if header < 0 {
		t.Fatal("fixture exposed no header row")
	}
	out.Cursor = header
	spec, ok := out.ContextualGuidance()
	if !ok || spec.Rows != 1 {
		t.Fatalf("header guidance = (%+v, %v), want one-row profile", spec, ok)
	}
	got := guidanceEntryText(spec)
	if !strings.Contains(got, "collapse") && !strings.Contains(got, "expand") {
		t.Fatalf("header guidance omits its live fold action: %q", got)
	}
	for _, taskOnly := range []string{"$ workflow", "! priority", "Ctrl+Enter peek"} {
		if strings.Contains(got, taskOnly) {
			t.Errorf("header guidance exposes task-only hint %q: %q", taskOnly, got)
		}
	}
}

func TestOutlinerEmptyStatesAdvertiseCausalRecovery(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project", Slug: "project"}
	project.BuildTaskTree()

	tests := []struct {
		name        string
		configure   func(*OutlinerModel)
		wantMessage string
		wantHint    string
	}{
		{
			name: "no projects",
			configure: func(out *OutlinerModel) {
				out.project = nil
				out.loadedProjects = nil
				out.rows = nil
			},
			wantMessage: "No projects are available. Press P to open Projects.",
			wantHint:    "P projects",
		},
		{
			name: "empty mode",
			configure: func(out *OutlinerModel) {
				out.project = project
				out.loadedProjects = []*models.Project{project}
				out.primaryMode = chrome.ModeDue
				out.rows = nil
			},
			wantMessage: "Due is empty. Press Tab or Shift+Tab to change modes.",
			wantHint:    "Tab next mode",
		},
		{
			name: "filtered empty",
			configure: func(out *OutlinerModel) {
				out.project = project
				out.loadedProjects = []*models.Project{project}
				out.primaryMode = chrome.ModeAll
				out.filter.Priority = models.PriorityHigh
				out.adoptFilterCarriers()
				out.rows = nil
			},
			wantMessage: listsurface.EmptyStateFiltered,
			wantHint:    "fx clear filters",
		},
		{
			name: "search empty",
			configure: func(out *OutlinerModel) {
				out.project = project
				out.loadedProjects = []*models.Project{project}
				out.SetSearchQuery("missing")
				out.rows = nil
			},
			wantMessage: `No tasks match search "missing". Press Esc to clear.`,
			wantHint:    "Esc clear search",
		},
		{
			name: "Focus empty",
			configure: func(out *OutlinerModel) {
				out.project = project
				out.loadedProjects = []*models.Project{project}
				out.focusStack = []FocusEntry{{TaskID: "missing"}}
				out.rows = nil
			},
			wantMessage: "No tasks are visible in this Focus. Press Esc to return.",
			wantHint:    "Esc return",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newUITestApp()
			out := app.crossOutliner
			test.configure(out)
			if got := out.outlinerEmptyMessage(); got != test.wantMessage {
				t.Fatalf("empty message = %q, want %q", got, test.wantMessage)
			}
			spec, ok := out.ContextualGuidance()
			if !ok || spec.Rows != 1 {
				t.Fatalf("empty guidance = (%+v, %v), want one-row profile", spec, ok)
			}
			if got := guidanceEntryText(spec); !strings.Contains(got, test.wantHint) {
				t.Fatalf("empty guidance = %q, want %q", got, test.wantHint)
			}
			if len(spec.Hints) == 0 || !spec.Hints[0].Required {
				t.Fatalf("causal recovery is not required: %+v", spec.Hints)
			}
		})
	}
}
