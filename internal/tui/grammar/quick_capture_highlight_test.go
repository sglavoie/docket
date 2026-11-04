package grammar

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/theme"
)

func highlightInput(t *testing.T, width int, value string) *QuickCaptureInput {
	t.Helper()
	project := &models.Project{ID: "p", Slug: "web", Name: "Web"}
	q := NewQuickCaptureInput(project, []*models.Project{project}, "", false)
	q.Input.SetWidth(width)
	q.Input.SetHeight(4)
	q.Input.SetValue(value)
	return &q
}

// Highlighting changes color alone: the text, every row's width, and the row
// count are exactly what the textarea drew.
func TestHighlightedDraftKeepsTheTextareaText(t *testing.T) {
	q := highlightInput(t, 24, "Ship the login page #ux @fri !1 %web and then tidy #later")
	plain, highlighted := q.Input.View(), q.RenderHighlightedInput()
	if ansi.Strip(highlighted) != ansi.Strip(plain) {
		t.Fatalf("highlighting changed the text:\n%q\nwant\n%q", ansi.Strip(highlighted), ansi.Strip(plain))
	}
	plainRows, rows := strings.Split(plain, "\n"), strings.Split(highlighted, "\n")
	if len(rows) != len(plainRows) {
		t.Fatalf("rows = %d, want %d", len(rows), len(plainRows))
	}
	for i := range rows {
		if ansi.StringWidth(rows[i]) != ansi.StringWidth(plainRows[i]) {
			t.Fatalf("row %d width = %d, want %d", i, ansi.StringWidth(rows[i]), ansi.StringWidth(plainRows[i]))
		}
	}
	if highlighted == plain {
		t.Fatal("no token was colored")
	}
}

// Each token takes the color it wears once committed, on whichever wrapped row
// the textarea put it.
func TestHighlightedDraftColorsEachTokenAsWhatItBecomes(t *testing.T) {
	q := highlightInput(t, 24, "Ship the login page #ux @fri !1 %web and then tidy #later")
	q.Input.Blur()
	view := q.RenderHighlightedInput()
	fg := func(style lipgloss.Style) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(style.GetForeground())
	}
	for token, style := range map[string]lipgloss.Style{
		"#ux":    fg(theme.TagChipStyle("ux")),
		"#later": fg(theme.TagChipStyle("later")),
		"!1":     fg(theme.GetPriorityStyle(1)),
		"@fri":   lipgloss.NewStyle().Foreground(theme.ColorSecondary),
	} {
		if !strings.Contains(view, style.Render(token)) {
			t.Errorf("%s is not drawn in its color:\n%q", token, view)
		}
	}
	if !strings.Contains(ansi.Strip(strings.Split(view, "\n")[0]), "Ship") || strings.Contains(ansi.Strip(strings.Split(view, "\n")[0]), "#later") {
		t.Fatal("the fixture no longer wraps #later onto a later row")
	}
}

// The token capture refuses is underlined where it sits, located by the
// refusal's own column, and the tokens beside it keep their colors.
func TestHighlightedDraftUnderlinesTheRefusedToken(t *testing.T) {
	q := highlightInput(t, 60, "Ship it %nope #ux")
	q.Input.Blur()
	view := q.RenderHighlightedInput()
	refused := lipgloss.NewStyle().Foreground(theme.ColorError).Underline(true).Render("%nope")
	if !strings.Contains(view, refused) {
		t.Fatalf("the unknown project is not underlined:\n%q", view)
	}
	if strings.Contains(view, lipgloss.NewStyle().Foreground(theme.ColorError).Underline(true).Render("#ux")) {
		t.Fatal("a valid token beside the refusal was underlined")
	}
}

// A plain draft, an empty one, and a placeholder draw exactly what the
// textarea drew.
func TestUnhighlightedDraftIsTheTextareaView(t *testing.T) {
	for _, value := range []string{"", "Just a title", "R&D and Fix a; then b"} {
		q := highlightInput(t, 40, value)
		if got := q.RenderHighlightedInput(); got != q.Input.View() {
			t.Errorf("%q changed with no token:\n%q", value, got)
		}
	}
}

// A multiline draft taller than the input scrolls, and the rows on screen are
// still matched to their own lines rather than to the draft's first ones.
func TestHighlightedDraftFollowsTheScrolledWindow(t *testing.T) {
	q := highlightInput(t, 30, "one\ntwo\nthree\nfour\nfive\nsix #last")
	q.Input.SetHeight(2)
	_ = q.Input.View() // the viewport learns its content from a frame, as it does live
	q.Input.MoveToEnd()
	q.Input.Blur()
	if q.Input.ScrollYOffset() == 0 {
		t.Fatal("the fixture did not scroll the input")
	}
	view := q.RenderHighlightedInput()
	if !strings.Contains(view, lipgloss.NewStyle().Foreground(theme.TagChipStyle("last").GetForeground()).Render("#last")) {
		t.Fatalf("the scrolled row's token is not colored:\n%q", view)
	}
	if ansi.Strip(view) != ansi.Strip(q.Input.View()) {
		t.Fatal("highlighting a scrolled draft changed its text")
	}
}
