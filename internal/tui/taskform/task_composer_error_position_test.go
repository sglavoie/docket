package taskform

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestCaptureRefusalFocusesOriginalToken(t *testing.T) {
	for _, token := range []string{"@tomorow", "@@nonsense", "%missing", ">missing", "<missing", "~missing", "-~1", "!5", "$unknown", "##0", "*unpin", "=unreview", "^bug"} {
		t.Run(token, func(t *testing.T) {
			project := &models.Project{ID: "project", Name: "Project", Slug: "project"}
			composer := NewTaskComposerModel(TaskComposerPreset{
				Project: project, AllProjects: []*models.Project{project}, AllowProjectSwitch: true,
			}, nil)
			composer.SetSize(60, 24)
			prefix := " \u2003 Réparer 日本 ;ignore @fake; #auth " + strings.Repeat("details ", 12)
			draft := "Valid task\n\n" + prefix + token
			composer.Input.SetValue(draft)
			if cmd := composer.CreateCompact(false); cmd != nil {
				t.Fatal("invalid input submitted")
			}
			if composer.Input.Line() != 2 || composer.Input.Column() != utf8.RuneCountInString(prefix) {
				t.Fatalf("cursor = %d:%d, errors = %+v", composer.Input.Line(), composer.Input.Column(), composer.Errors)
			}
			if composer.Input.Value() != draft || !strings.Contains(ansi.Strip(composer.Input.View()), token) {
				t.Fatalf("draft changed or wrapped token not visible: %s", composer.Input.View())
			}
		})
	}
}

func TestCaptureProblemNavigationUsesUpdatedTokenColumns(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Project: project}, nil)
	composer.SetSize(60, 24)
	composer.Input.SetValue("Été @bad\n\nOther @@bad")
	composer.CreateCompact(false)
	composer.Update(tea.PasteMsg{Content: "longer "})
	if composer.Input.Column() != 11 {
		t.Fatal("live validation moved the caret")
	}
	composer.focusNextCaptureError()
	if composer.Input.Line() != 2 || composer.Input.Column() != 6 {
		t.Fatal("next problem did not focus its token")
	}
	composer.focusNextCaptureError()
	if composer.Input.Line() != 0 || composer.Input.Column() != 11 {
		t.Fatal("wrapped navigation used a stale column")
	}
	composer.focusPreviousCaptureError()
	if composer.Input.Line() != 2 || composer.Input.Column() != 6 {
		t.Fatal("previous problem did not wrap to its token")
	}
}

func TestCaptureTokenAfterUnicodeIndent(t *testing.T) {
	project := &models.Project{ID: "project", Name: "Project"}
	composer := NewTaskComposerModel(TaskComposerPreset{Project: project}, nil)
	composer.SetSize(60, 24)
	composer.Input.SetValue("\u2003@bad Task")
	if cmd := composer.CreateCompact(false); cmd != nil || len(composer.Errors) != 1 {
		t.Fatal("leading Unicode whitespace hid the invalid date")
	}
	if composer.Input.Column() != 1 {
		t.Fatalf("column = %d, want the token after indentation", composer.Input.Column())
	}
}
