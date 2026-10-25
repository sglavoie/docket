package grammar

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
)

func TestCaptureCompletionReplacesCaretTokenAndPreservesBatch(t *testing.T) {
	for _, width := range []int{20, 80} {
		q := draftedCaptureInput(t, "Préparer 日本 #backend later !1\nOther #keep", quickCaptureTestProject(), nil, false)
		q.Input.SetWidth(width)
		q.Input.MoveToBegin()
		q.Input.SetCursorColumn(utf8.RuneCountInString("Préparer 日本 #ba"))
		q.TagCache = []string{"backend"}
		q.RefreshAutocomplete()
		if !q.HandleAutocompleteKey("tab") {
			t.Fatal("completion unavailable at caret")
		}
		if got := q.Input.Value(); got != "Préparer 日本 #backend later !1\nOther #keep" {
			t.Fatalf("width %d changed draft: %q", width, got)
		}
		if q.Input.Line() != 0 || q.Input.Column() != utf8.RuneCountInString("Préparer 日本 #backend ") {
			t.Fatalf("caret escaped inserted token: %d:%d", q.Input.Line(), q.Input.Column())
		}
	}
}

func TestCaptureCompletionUsesWholeCaretLineForRoutingAndExclusions(t *testing.T) {
	source, target := quickCaptureTestProject(), routedCaptureProject()
	q := draftedCaptureInput(t, "Previous %missing\nAdopt < %routed <2\nOther %missing", source, []*models.Project{source, target}, false)
	q.Input.MoveToBegin()
	q.Input.CursorEnd()
	q.Input.CursorDown()
	q.Input.SetCursorColumn(len("Adopt <"))
	q.RefreshAutocomplete()
	if len(q.AC.Items) != 1 || q.AC.Items[0].Value != "1" || !strings.Contains(q.AC.Items[0].Display, "Routed alpha") {
		t.Fatalf("wrong routed candidates: %+v", q.AC.Items)
	}
	q.HandleAutocompleteKey("tab")
	if q.Input.Value() != "Previous %missing\nAdopt <1 %routed <2\nOther %missing" || q.Input.Line() != 1 {
		t.Fatalf("completion changed another line or moved caret: %q", q.Input.Value())
	}
}

func TestCaptureCompletionSingletonDoesNotExcludeItself(t *testing.T) {
	q := draftedCaptureInput(t, "Task !2 later", quickCaptureTestProject(), nil, false)
	q.Input.MoveToBegin()
	q.Input.SetCursorColumn(len("Task !"))
	q.RefreshAutocomplete()
	if !q.AC.IsActive() || q.AC.Mode != acModePriority {
		t.Fatal("existing priority excluded itself")
	}
	q.Input.SetValue("Task !2 later !1")
	q.Input.MoveToBegin()
	q.Input.SetCursorColumn(len("Task !"))
	q.RefreshAutocomplete()
	if q.AC.IsActive() {
		t.Fatal("second priority was not excluded")
	}
}

func TestCaptureCompletionFollowsMovementButNotIdleUpdates(t *testing.T) {
	q := draftedCaptureInput(t, "Task #backend", quickCaptureTestProject(), nil, false)
	q.TagCache = []string{"backend"}
	q.RefreshAutocomplete()
	q.HandleAutocompleteKey("esc")
	q.UpdateInput(nil)
	if q.AC.IsActive() {
		t.Fatal("idle update reopened dismissed suggestions")
	}
	q.UpdateInput(tea.KeyPressMsg{Code: tea.KeyLeft})
	if !q.AC.IsActive() || q.AC.Mode != ACModeTag {
		t.Fatal("caret movement did not refresh suggestions")
	}
}

func TestCaptureSyntaxHelpFollowsCaretAndResizes(t *testing.T) {
	q := draftedCaptureInput(t, "Task ~1 \nOther @today", quickCaptureTestProject(), nil, false)
	q.Input.MoveToBegin()
	q.Input.CursorEnd()
	q.AC.Clear()
	column := q.Input.Column()
	q.ToggleHelp()
	group := q.syntaxAnchor
	if group == "" {
		t.Fatal("dependency at token boundary did not select a section")
	}
	for _, width := range []int{58, 98, 58} {
		q.Width = width
		q.SetSyntaxViewportWidth(width)
		q.SetSyntaxViewportHeight(8)
		if !strings.Contains(ansi.Strip(q.SyntaxVP.View()), group) {
			t.Fatalf("context lost at width %d: %s", width, q.SyntaxVP.View())
		}
	}
	q.SyntaxScrollLine(-1)
	offset := q.SyntaxVP.YOffset()
	q.SetSyntaxViewportHeight(8)
	if q.SyntaxVP.YOffset() != offset {
		t.Fatal("render sizing overrode manual scroll")
	}
	q.CloseHelp()
	if q.Input.Value() != "Task ~1 \nOther @today" || q.Input.Line() != 0 || q.Input.Column() != column {
		t.Fatal("help changed the draft or caret")
	}
	q.Input.SetCursorColumn(4)
	q.ToggleHelp()
	if q.SyntaxVP.YOffset() != 0 {
		t.Fatal("prose should open the guide at the beginning")
	}
}

func TestCaptureSyntaxHelpRespectsLongestSigilAndLinkedCapture(t *testing.T) {
	for _, draft := range []string{"Task @@tomorrow", "Task ##office", "Task @today"} {
		q := draftedCaptureInput(t, draft, quickCaptureTestProject(), nil, false)
		q.ToggleHelp()
		if q.syntaxAnchor == "" {
			t.Fatalf("no syntax group for %q", draft)
		}
	}
	q := draftedCaptureInput(t, "Task %routed", quickCaptureTestProject(), nil, true)
	q.ToggleHelp()
	content, _ := q.RenderSyntaxGuide()
	if q.syntaxAnchor != "" || strings.Contains(ansi.Strip(content), "Project Routing") {
		t.Fatal("linked capture offered disabled project syntax")
	}
}

func TestCaptureCompletionAtLimitPreservesWholeDraft(t *testing.T) {
	q := draftedCaptureInput(t, "Task #b\nKeep this", quickCaptureTestProject(), nil, false)
	q.TagCache = []string{"backend"}
	q.Input.CharLimit = utf8.RuneCountInString(q.Input.Value())
	q.Input.MoveToBegin()
	q.Input.CursorEnd()
	q.RefreshAutocomplete()
	q.HandleAutocompleteKey("tab")
	if q.Input.Value() != "Task #b\nKeep this" || q.Input.Line() != 0 || q.Input.Column() != len("Task #b") {
		t.Fatal("completion truncated the draft at its character limit")
	}
	q.UpdateInput(nil)
	if q.AC.IsActive() || !strings.Contains(q.ComposeBoundedPreviewBlock(2), "capture limit") {
		t.Fatal("completion limit was not explained after an idle update")
	}
	q.UpdateInput(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if q.err != nil {
		t.Fatal("editing retained the completion refusal")
	}
}
