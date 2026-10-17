package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

func TestOutlinerGuidanceProfilesFollowInteractionState(t *testing.T) {
	app := newFilterChordTestApp(t)
	out := app.crossOutliner

	out.PeekMode = true
	spec, ok := out.ContextualGuidance()
	if !ok || spec.Rows != 2 {
		t.Fatalf("Peek guidance = (%+v, %v), want two-row profile", spec, ok)
	}
	// Two keys, one act, one entry: Peek's own key and the cascade's first rung
	// both put the pane down, so the footer says so once.
	if got := guidanceEntryText(spec); !strings.Contains(got, "b/Esc close Peek") {
		t.Fatalf("Peek guidance = %q", got)
	}

	out.PeekMode = false
	out.MultiSelectMode = true
	out.ReplaceSelection(out.VisibleTaskIdentities()[:1])
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
	for index, row := range out.Rows {
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
	for _, taskOnly := range []string{"$ workflow", "! priority", "b peek"} {
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
		configure   func(*outliner.OutlinerModel)
		wantMessage string
		wantHint    string
	}{
		{
			name: "no projects",
			configure: func(out *outliner.OutlinerModel) {
				out.Project = nil
				out.Population.Projects = nil
				out.Rows = nil
			},
			wantMessage: "No projects are available. Press P to open Projects.",
			wantHint:    "P projects",
		},
		{
			name: "empty mode",
			configure: func(out *outliner.OutlinerModel) {
				out.Project = project
				out.Population.Projects = []*models.Project{project}
				out.PrimaryMode = chrome.ModeDue
				out.Rows = nil
			},
			wantMessage: "No upcoming or overdue deadlines. Press gt to open Today.",
			wantHint:    "gt to open Today",
		},
		{
			name: "empty Today",
			configure: func(out *outliner.OutlinerModel) {
				out.Project = project
				out.Population.Projects = []*models.Project{project}
				out.PrimaryMode = chrome.ModeToday
				out.Rows = nil
			},
			wantMessage: "Nothing planned for today. Press gn to choose from Next.",
			wantHint:    "gn to choose from Next",
		},
		{
			name: "empty Next",
			configure: func(out *outliner.OutlinerModel) {
				out.Project = project
				out.Population.Projects = []*models.Project{project}
				out.PrimaryMode = chrome.ModeNext
				out.Rows = nil
			},
			wantMessage: "Nothing planned for Next. Press gb to choose from Backlog.",
			wantHint:    "gb to choose from Backlog",
		},
		{
			name: "filtered empty",
			configure: func(out *outliner.OutlinerModel) {
				out.Project = project
				out.Population.Projects = []*models.Project{project}
				out.PrimaryMode = chrome.ModeAll
				out.Filter.Priority = models.PriorityHigh
				adoptFilterCarriers(out)
				out.Rows = nil
			},
			wantMessage: listsurface.EmptyStateFiltered,
			wantHint:    "fx clear filters",
		},
		{
			name: "search empty",
			configure: func(out *outliner.OutlinerModel) {
				out.Project = project
				out.Population.Projects = []*models.Project{project}
				out.SetSearchQuery("missing")
				out.Rows = nil
			},
			wantMessage: `No tasks match search "missing". Press Esc to clear.`,
			wantHint:    "Esc clear search",
		},
		{
			name: "Focus empty",
			configure: func(out *outliner.OutlinerModel) {
				out.Project = project
				out.Population.Projects = []*models.Project{project}
				out.Focus.Stack = []outliner.FocusEntry{{TaskID: "missing"}}
				out.Rows = nil
			},
			wantMessage: "No tasks are visible in this Focus. Press Esc to leave Focus.",
			wantHint:    "Esc leave Focus",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newUITestApp()
			out := app.crossOutliner
			test.configure(out)
			if got := out.OutlinerEmptyMessage(); got != test.wantMessage {
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

func guidanceEntryText(spec chrome.GuidanceSpec) string {
	parts := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		parts = append(parts, chrome.GuidanceHintText(hint))
	}
	return strings.Join(parts, " · ")
}

// TestOutlinerGuidanceAdvertisesOnlyApplicableControlsOnEveryRowType is the
// other half of the ranked ladder. Rank decides the order the acts a row can
// take are offered in; live applicability decides which acts those are, and the
// two are only correct together — a row that led with `a capture` and then
// offered `w workflow` on a checklist item would be ordered perfectly and still
// wrong.
//
// The two rows here are the ones the ladder does not describe, because both shed
// most of it: a checklist item is not a task and a project header is not work.
// The section header, the empty states, Peek, and a visual selection are each
// covered by their own contract above.
func TestOutlinerGuidanceAdvertisesOnlyApplicableControlsOnEveryRowType(t *testing.T) {
	t.Run("checklist item", func(t *testing.T) {
		driver, ws, err := NewFixtureDriver(FixtureChecklist, 100, 35)
		if err != nil {
			t.Fatalf("NewFixtureDriver(checklist) error: %v", err)
		}
		t.Cleanup(func() { _ = ws.Cleanup() })
		out := driver.App().crossOutliner
		focusChecklistOwner(t, driver, checklistFixtureFoldTitle)
		if !out.ExpandAtCursor() {
			t.Fatal("the checklist did not expand, so no item row exists to answer for")
		}
		seatCursorOnRow(t, out, "checklist item", func(row outliner.OutlinerRow) bool { return row.IsChecklistItem })

		got := guidanceEntryText(mustContextualGuidance(t, out))
		// The item's own edit leads, which is the whole of what rank buys a row
		// that sheds the first two rungs: `a capture` is refused here because `c`
		// is the owner task's append, and `w workflow` has no state to move.
		if !strings.HasPrefix(got, "e item · ") {
			t.Fatalf("checklist guidance does not lead with the item's own edit: %q", got)
		}
		if !strings.Contains(got, "b peek") {
			t.Fatalf("checklist guidance omits its item preview: %q", got)
		}
		for _, taskOnly := range []string{"a capture", "w workflow", "@ date", "! priority"} {
			if strings.Contains(got, taskOnly) {
				t.Errorf("checklist guidance exposes task-only hint %q: %q", taskOnly, got)
			}
		}
	})

	t.Run("project header", func(t *testing.T) {
		driver, ws, err := NewFixtureDriver(FixtureMultiProject, 100, 35)
		if err != nil {
			t.Fatalf("NewFixtureDriver(multi-project) error: %v", err)
		}
		t.Cleanup(func() { _ = ws.Cleanup() })
		out := driver.App().crossOutliner
		// The fixture's work sits in the Backlog lane a launch folds, so the lane
		// is opened before `p` folds its project groups.
		openLaunchFoldedLanes(out)
		if err := driver.SendKey("p"); err != nil {
			t.Fatalf("fold project groups: %v", err)
		}
		seatCursorOnRow(t, out, "project header", func(row outliner.OutlinerRow) bool { return row.IsProjectHeader })

		got := guidanceEntryText(mustContextualGuidance(t, out))
		// The shared keys answer for the project here, and the footer says which
		// half it means rather than reusing the task label.
		for _, projectAct := range []string{"e project", "# project tags", "* project pin", "Space actions", "b peek"} {
			if !strings.Contains(got, projectAct) {
				t.Errorf("project header guidance omits %q: %q", projectAct, got)
			}
		}
		for _, taskOnly := range []string{"w workflow", "@ date", "! priority", "e edit", "# tags/context"} {
			if strings.Contains(got, taskOnly) {
				t.Errorf("project header guidance exposes task-only hint %q: %q", taskOnly, got)
			}
		}
	})
}

func mustContextualGuidance(t *testing.T, out *outliner.OutlinerModel) chrome.GuidanceSpec {
	t.Helper()
	spec, ok := out.ContextualGuidance()
	if !ok {
		t.Fatal("the Outliner published no guidance for this row")
	}
	return spec
}
