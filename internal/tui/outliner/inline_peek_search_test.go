package outliner

import (
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestPeekSearchExcerptTracksQueryAndCommentChanges(t *testing.T) {
	task := tierTask()
	task.Comments = []*models.Comment{{Text: strings.Repeat("old context ", 30) + "zeppelin hangar\nneeds repairs " + strings.Repeat("later context ", 30)}}
	m := pagedPeekOutliner(t, task, 60, 20)
	searchRankedList(m, "Zeppelin") // No case-sensitive hit: use the projection's relaxed case.
	doc := cursorPeekDocument(t, m)
	body := ansi.Strip(strings.Join(doc.Body, "\n"))
	if !strings.Contains(body, "Match: comment · …") || !strings.Contains(body, "zeppelin hangar") {
		t.Fatalf("Peek did not bring the buried comment match into view:\n%s", body)
	}
	if !strings.Contains(body, "Comments: 1") {
		t.Fatalf("excerpt replaced the ordinary comment count:\n%s", body)
	}
	for _, line := range doc.Lines(0) {
		if lipgloss.Width(line) > 60 {
			t.Fatalf("narrow Peek overflow: %q", line)
		}
	}
	if !m.scrollPeek(m.peekDockGeometry().HalfPageRows()) || peekOffset(t, m) == 0 {
		t.Fatal("fixture did not scroll")
	}
	searchRankedList(m, "hangar")
	if cursorPeekDocument(t, m).Fingerprint == doc.Fingerprint || peekOffset(t, m) != 0 {
		t.Fatal("changing query did not reset the excerpt's window")
	}
	task.Comments[0].Text = "hangar repaired"
	m.RebuildRows()
	if got := ansi.Strip(strings.Join(cursorPeekDocument(t, m).Body, "\n")); !strings.Contains(got, "hangar repaired") || strings.Contains(got, "needs repairs") {
		t.Fatalf("comment change left a stale excerpt: %s", got)
	}
	m.ClearSearch()
	m.RebuildRows()
	if got := ansi.Strip(strings.Join(cursorPeekDocument(t, m).Body, "\n")); strings.Contains(got, "Match:") || strings.Contains(got, "hangar repaired") {
		t.Fatalf("ordinary Peek retained the search-only comment excerpt: %s", got)
	}
}

func TestPeekSearchExcerptUsesOnlyFreeTextAndHandlesDetails(t *testing.T) {
	task := tierTask()
	task.Description = "zeppelin hangar"
	task.Comments = []*models.Comment{{Text: "quartz invoice"}}
	task.Tags = []string{"release"}
	m := pagedPeekOutliner(t, task, 80, 24)
	for _, tc := range []struct{ query, want string }{
		{"#release zeppelin", "description · zeppelin hangar"},
		{"zeppelin quartz", "details ·"},
		{"#release", ""},
		{"untitled", ""},
	} {
		searchRankedList(m, tc.query)
		got := ansi.Strip(m.peekSearchExcerpt(m.Rows[0]))
		if tc.want == "" && got != "" || tc.want != "" && !strings.Contains(got, tc.want) {
			t.Errorf("query %q excerpt = %q, want %q", tc.query, got, tc.want)
		}
		if tc.query == "zeppelin quartz" && (!strings.Contains(got, "zeppelin") || !strings.Contains(got, "quartz")) {
			t.Errorf("combined-field match did not explain both terms: %q", got)
		}
	}
}

func TestPeekSearchExcerptWrapsUnicodeAndBoundsLongText(t *testing.T) {
	task := tierTask()
	task.Description = strings.Repeat("前置き ", 100) + "請求書を確認" + strings.Repeat(" 続き", 100)
	m := pagedPeekOutliner(t, task, 60, 20)
	searchRankedList(m, "請求書")
	excerpt := ansi.Strip(m.peekSearchExcerpt(m.Rows[0]))
	if !utf8.ValidString(excerpt) || !strings.Contains(excerpt, "請求書") || utf8.RuneCountInString(excerpt) > 180 {
		t.Fatalf("invalid or unbounded Unicode excerpt: %q", excerpt)
	}
	for _, width := range []int{60, 80, 120} {
		m.SetSize(width, 20)
		doc := cursorPeekDocument(t, m)
		for _, line := range doc.Body {
			if lipgloss.Width(line) > width {
				t.Fatalf("width %d: overflowing line %q", width, line)
			}
		}
	}
}

func TestPeekSearchExcerptExplainsDistantTerms(t *testing.T) {
	task := tierTask()
	task.Description = "zeppelin" + strings.Repeat(" ordinary context", 40)
	task.Comments = []*models.Comment{{Text: "quartz invoice"}}
	m := pagedPeekOutliner(t, task, 60, 20)
	searchRankedList(m, "zeppelin quartz")
	excerpt := ansi.Strip(m.peekSearchExcerpt(m.Rows[0]))
	if !strings.Contains(excerpt, "zeppelin") || !strings.Contains(excerpt, "quartz") || !strings.Contains(excerpt, "…") {
		t.Fatalf("distant terms not explained: %q", excerpt)
	}
	if utf8.RuneCountInString(excerpt) > 340 {
		t.Fatalf("unbounded excerpt: %q", excerpt)
	}
	for _, line := range cursorPeekDocument(t, m).Body {
		if lipgloss.Width(line) > 60 {
			t.Fatalf("overflow: %q", line)
		}
	}
}

func TestSearchExcerptHighlightPositionsSurviveWhitespaceAndUnicode(t *testing.T) {
	text := []rune("  café\n\t 請求書  ")
	got := highlightedExcerptWindow(text, []int{5, 9, 10}, 0, len(text))
	want := textfit.HighlightText("café 請求書", []int{3, 5, 6}, theme.MutedStyle)
	if got != want {
		t.Fatalf("highlight positions changed: %q, want %q", got, want)
	}
	if got := searchMatchExcerpt("short", nil); got != "" {
		t.Fatalf("empty match: %q", got)
	}
}
