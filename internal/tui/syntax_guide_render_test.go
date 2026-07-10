package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// syntaxGuideGroupNames are the grouped headings the shared reference must show
// (Project Routing only when %project capture is enabled).
var syntaxGuideGroupNames = []string{
	"Basics",
	"Priority & Tags",
	"Schedule",
	"Placement",
	"Status",
}

// TestQuickCaptureSyntaxGuideGroupedRespectsVisualWidth confirms the toggled
// Quick Capture guide opens at the top showing its title and first group, never
// overflows at wide or narrow widths, and — because it now lives in a bounded
// scroll viewport — makes every grouped heading (including the last Project
// Routing entry) reachable through scrolling. Full-content validation of the raw
// guide lives in the width-independent TestSyntaxGuideRawRender* tests.
func TestQuickCaptureSyntaxGuideGroupedRespectsVisualWidth(t *testing.T) {
	project := testutil.NewTestProject("syntax guide")

	for _, width := range []int{120, 48} {
		m := NewQuickCaptureModel(project, nil, false)
		m.SetSize(width, 24)

		updated, _ := m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
		m = updated.(*QuickCaptureModel)
		if !m.helpExpanded {
			t.Fatalf("expected helpExpanded=true after '?' toggle @%d", width)
		}

		// The guide opens at the top: title and first group are visible.
		top := ansi.Strip(m.View().Content)
		if !strings.Contains(top, syntaxGuideTitle) {
			t.Fatalf("expected syntax guide header at top @%d, got:\n%s", width, top)
		}
		if !strings.Contains(top, "Basics") {
			t.Fatalf("expected guide to open at the first group @%d, got:\n%s", width, top)
		}

		// Page through the guide, accumulating what each frame shows. Every group
		// heading, including the last (Project Routing), must become reachable.
		seen := top
		for i := 0; i < 12; i++ {
			frame := m.View().Content
			assertRenderedLinesWithinVisualWidth(t, fmt.Sprintf("quick capture syntax guide @%d frame %d", width, i), frame, width)
			plain := ansi.Strip(frame)
			seen += "\n" + plain
			// `note` is not a valid task type and must never be advertised.
			if strings.Contains(plain, "^note") || strings.Contains(plain, "type:note") {
				t.Fatalf("guide must not advertise the invalid `note` type @%d, got:\n%s", width, plain)
			}
			updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
			m = updated.(*QuickCaptureModel)
		}

		for _, name := range append(append([]string{}, syntaxGuideGroupNames...), "Project Routing") {
			if !strings.Contains(seen, name) {
				t.Fatalf("expected group %q reachable through scrolling @%d, accumulated:\n%s", name, width, seen)
			}
		}
	}
}

// TestNarrowLayoutQuickCaptureSyntaxGuideRespectsVisualWidth exercises the full
// Quick Capture view with the syntax guide expanded at the canonical narrow and
// wide widths, guarding the grouped guide against reintroducing overflow.
func TestNarrowLayoutQuickCaptureSyntaxGuideRespectsVisualWidth(t *testing.T) {
	project := testutil.NewTestProject("narrow syntax guide")

	for _, width := range []int{120, 48} {
		m := NewQuickCaptureModel(project, nil, false)
		m.SetSize(width, 24)

		updated, _ := m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
		m = updated.(*QuickCaptureModel)

		rendered := m.View().Content
		assertRenderedLinesWithinVisualWidth(t, fmt.Sprintf("quick capture guide view @%d", width), rendered, width)
	}
}

// syntaxGuideRawWidths are the widths at which the raw shared grammar guide is
// exercised directly (bypassing View()) so viewport clipping in later sessions
// cannot weaken full-content validation.
var syntaxGuideRawWidths = []int{48, 60, 80, 100, 140}

// newSyntaxGuideInput builds a quickCaptureInput plus the fixtures needed to
// parse the guide's stored validation examples. The allProjects fixture
// includes a non-archived project whose slug matches syntaxGuideProjectSlug so
// the Project Routing examples resolve.
func newSyntaxGuideInput(t *testing.T, skipProject bool) (quickCaptureInput, *models.Project, []*models.Project) {
	t.Helper()
	project := quickCaptureTestProject()
	target := &models.Project{ID: "target-project", Name: "My Project", Slug: syntaxGuideProjectSlug}
	allProjects := []*models.Project{project, target}
	qci := newQuickCaptureInput(project, allProjects, "", skipProject)
	return qci, project, allProjects
}

// findGuideLine returns the first plain (ANSI-stripped) rendered line that
// contains needle, failing the test if none does.
func findGuideLine(t *testing.T, rendered, needle string) string {
	t.Helper()
	for _, line := range strings.Split(ansi.Strip(rendered), "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	t.Fatalf("no rendered line contains %q in:\n%s", needle, ansi.Strip(rendered))
	return ""
}

// TestSyntaxGuideRawRenderRespectsVisualWidth renders the raw shared guide
// directly at every supported width for both capture hosts, asserting the
// title, all group headings, no visual overflow, absence of the invalid `note`
// type, and correct conditional Project Routing visibility.
func TestSyntaxGuideRawRenderRespectsVisualWidth(t *testing.T) {
	for _, skip := range []bool{false, true} {
		qci, _, _ := newSyntaxGuideInput(t, skip)
		for _, width := range syntaxGuideRawWidths {
			qci.width = width
			rendered := qci.renderSyntaxGuide()
			label := fmt.Sprintf("raw syntax guide skipProject=%v @%d", skip, width)
			assertRenderedLinesWithinVisualWidth(t, label, rendered, width)

			plain := ansi.Strip(rendered)
			if !strings.Contains(plain, syntaxGuideTitle) {
				t.Fatalf("%s: missing title %q, got:\n%s", label, syntaxGuideTitle, plain)
			}
			for _, name := range syntaxGuideGroupNames {
				if !strings.Contains(plain, name) {
					t.Fatalf("%s: missing group %q, got:\n%s", label, name, plain)
				}
			}
			if strings.Contains(plain, "^note") || strings.Contains(plain, "type:note") {
				t.Fatalf("%s: guide advertises invalid `note` type, got:\n%s", label, plain)
			}
			hasRouting := strings.Contains(plain, "Project Routing")
			switch {
			case skip && hasRouting:
				t.Fatalf("%s: linked guide must not advertise Project Routing, got:\n%s", label, plain)
			case !skip && !hasRouting:
				t.Fatalf("%s: quick capture guide must advertise Project Routing, got:\n%s", label, plain)
			}
		}
	}
}

// TestSyntaxGuideRawRenderAlignsWideAndWrapsNarrow confirms the measured layout:
// wide widths collapse an entry into a single aligned row (forms and examples
// share a line) while narrow widths wrap the examples underneath the forms.
func TestSyntaxGuideRawRenderAlignsWideAndWrapsNarrow(t *testing.T) {
	qci, _, _ := newSyntaxGuideInput(t, false)
	const forms = "^type · type:"
	const example = "^epic"

	for _, width := range []int{80, 100, 140} {
		qci.width = width
		line := findGuideLine(t, qci.renderSyntaxGuide(), forms)
		if !strings.Contains(line, example) {
			t.Fatalf("@%d expected aligned forms+examples row, got line %q", width, line)
		}
	}

	for _, width := range []int{48, 60} {
		qci.width = width
		line := findGuideLine(t, qci.renderSyntaxGuide(), forms)
		if strings.Contains(line, example) {
			t.Fatalf("@%d expected examples wrapped below forms, got line %q", width, line)
		}
	}
}

// TestSyntaxGuideRawRenderStacksAtVeryNarrowWidth drives the guide below the
// aligned/wrapped thresholds so every field stacks, guarding the stacked mode
// against visual-width overflow.
func TestSyntaxGuideRawRenderStacksAtVeryNarrowWidth(t *testing.T) {
	qci, _, _ := newSyntaxGuideInput(t, false)
	qci.width = 28
	rendered := qci.renderSyntaxGuide()
	assertRenderedLinesWithinVisualWidth(t, "raw syntax guide @28 (stacked)", rendered, 28)
}

// TestSyntaxGuideValidationExamplesParse parses every stored validation example
// in its supported context, confirming the documented grammar is authoritative
// (actually accepted by the parser) rather than aspirational prose.
func TestSyntaxGuideValidationExamplesParse(t *testing.T) {
	for _, skip := range []bool{false, true} {
		qci, project, allProjects := newSyntaxGuideInput(t, skip)
		for _, g := range qci.syntaxGuideGroups() {
			for _, e := range g.entries {
				for _, input := range e.validation {
					result, err := parseQuickCapture(input, project, allProjects, skip)
					if err != nil {
						t.Errorf("skipProject=%v group %q: %q failed to parse: %v", skip, g.name, input, err)
						continue
					}
					if result.title == "" {
						t.Errorf("skipProject=%v group %q: %q parsed to an empty title", skip, g.name, input)
					}
				}
			}
		}
	}
}

// TestSyntaxGuideTypeSuggestionsExcludeNote confirms the task-type autocomplete
// is derived from the model's valid task types and never offers `note`.
func TestSyntaxGuideTypeSuggestionsExcludeNote(t *testing.T) {
	qci, _, _ := newSyntaxGuideInput(t, false)
	suggestions := qci.getTypeSuggestions("")
	if len(suggestions) != len(models.AllTaskTypes()) {
		t.Fatalf("expected %d type suggestions, got %d", len(models.AllTaskTypes()), len(suggestions))
	}
	for _, s := range suggestions {
		if s.Value == "note" {
			t.Fatalf("type autocomplete must not offer invalid `note` type: %+v", suggestions)
		}
	}
}

// TestLinkedTaskCreatorSyntaxGuideRespectsVisualWidth confirms the Linked Task
// Creator shares the grouped guide inside a bounded scroll viewport: it opens at
// the top, never overflows at wide or narrow widths, makes every group heading
// reachable by paging, and — because project capture is disabled here — never
// advertises the Project Routing group in any frame.
func TestLinkedTaskCreatorSyntaxGuideRespectsVisualWidth(t *testing.T) {
	project, err := models.NewProject("proj-1", "creator", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	sourceTask, err := models.NewTask("task-1", 1, models.TaskTypeTask, "source")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	for _, width := range []int{120, 48} {
		m := NewLinkedTaskCreatorModel(sourceTask, project, LinkTypeBlocker)
		m.SetSize(width, 24)

		updated, _ := m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
		m = updated.(*LinkedTaskCreatorModel)
		if !m.helpExpanded {
			t.Fatalf("expected helpExpanded=true after '?' toggle @%d", width)
		}

		// The guide opens at the top: title and first group are visible.
		top := ansi.Strip(m.View().Content)
		if !strings.Contains(top, syntaxGuideTitle) {
			t.Fatalf("expected syntax guide header at top @%d, got:\n%s", width, top)
		}
		if !strings.Contains(top, "Basics") {
			t.Fatalf("expected guide to open at the first group @%d, got:\n%s", width, top)
		}

		// Page through the guide, accumulating what each frame shows. Every group
		// heading must become reachable, and Project Routing must never appear.
		seen := top
		for i := 0; i < 16; i++ {
			frame := m.View().Content
			assertRenderedLinesWithinVisualWidth(t, fmt.Sprintf("linked task creator syntax guide @%d frame %d", width, i), frame, width)
			plain := ansi.Strip(frame)
			if strings.Contains(plain, "Project Routing") || strings.Contains(plain, "%project") {
				t.Fatalf("linked task creator must not advertise project routing @%d, got:\n%s", width, plain)
			}
			if strings.Contains(plain, "^note") || strings.Contains(plain, "type:note") {
				t.Fatalf("guide must not advertise the invalid `note` type @%d, got:\n%s", width, plain)
			}
			seen += "\n" + plain
			updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
			m = updated.(*LinkedTaskCreatorModel)
		}

		for _, name := range syntaxGuideGroupNames {
			if !strings.Contains(seen, name) {
				t.Fatalf("expected group %q reachable through scrolling @%d, accumulated:\n%s", name, width, seen)
			}
		}
	}
}
