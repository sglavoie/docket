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
// Quick Capture guide renders the grouped headings, advertises the Project
// Routing group (project capture enabled), and never overflows at wide or
// narrow widths.
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

		rendered := m.View().Content
		plain := ansi.Strip(rendered)
		if !strings.Contains(plain, "Quick Capture Syntax") {
			t.Fatalf("expected syntax guide header @%d, got:\n%s", width, plain)
		}
		for _, name := range syntaxGuideGroupNames {
			if !strings.Contains(plain, name) {
				t.Fatalf("expected group %q in guide @%d, got:\n%s", name, width, plain)
			}
		}
		if !strings.Contains(plain, "Project Routing") {
			t.Fatalf("expected Project Routing group when project capture enabled @%d, got:\n%s", width, plain)
		}
		// `note` is not a valid task type and must never be advertised.
		if strings.Contains(plain, "^note") || strings.Contains(plain, "type:note") {
			t.Fatalf("guide must not advertise the invalid `note` type @%d, got:\n%s", width, plain)
		}
		assertRenderedLinesWithinVisualWidth(t, fmt.Sprintf("quick capture syntax guide @%d", width), rendered, width)
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

// TestLinkedTaskCreatorSyntaxGuideRespectsVisualWidth confirms the Linked Task
// Creator shares the grouped guide, hides the Project Routing group (project
// capture disabled there), and never overflows at wide or narrow widths.
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

		rendered := m.View().Content
		plain := ansi.Strip(rendered)
		for _, name := range syntaxGuideGroupNames {
			if !strings.Contains(plain, name) {
				t.Fatalf("expected group %q in linked guide @%d, got:\n%s", name, width, plain)
			}
		}
		if strings.Contains(plain, "Project Routing") || strings.Contains(plain, "%project") {
			t.Fatalf("linked task creator must not advertise project routing @%d, got:\n%s", width, plain)
		}
		assertRenderedLinesWithinVisualWidth(t, fmt.Sprintf("linked task creator syntax guide @%d", width), rendered, width)
	}
}
