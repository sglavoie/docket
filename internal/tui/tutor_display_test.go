package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// TestTutorTopicListByteSliceTruncation verifies that renderTopicList does not
// produce invalid UTF-8 when topic titles and category names contain multibyte
// (CJK) characters that exceed the list pane width. A byte-index slice on a
// UTF-8 string can split a multibyte character, producing garbage bytes.
func TestTutorTopicListByteSliceTruncation(t *testing.T) {
	// 17 CJK chars * 2 cells = 34 visual cells; listWidth for width=60 is 18,
	// so the truncation branch is taken. Each CJK char is 3 UTF-8 bytes, so
	// the byte-index slice at position 17 (listW-1) lands mid-character.
	longCJK := "日本語カテゴリー名前テスト追加文字"

	vp := viewport.New()
	vp.SetWidth(18)
	vp.SetHeight(40)

	m := &TutorModel{
		width:    60,
		height:   40,
		viewport: vp,
	}
	m.filteredTopics = []TutorTopic{
		{Title: longCJK, Category: longCJK, Content: "content"},
	}

	rendered := m.renderTopicList()
	stripped := ansi.Strip(rendered)

	if !utf8.ValidString(stripped) {
		t.Fatalf("renderTopicList produced invalid UTF-8 for CJK category/title: %q", stripped)
	}

	// After stripping ANSI, each visual line must fit within listWidth.
	listW := m.listWidth()
	for line := range strings.SplitSeq(stripped, "\n") {
		if w := lipgloss.Width(line); w > listW {
			t.Fatalf("renderTopicList line exceeds listWidth %d (got %d): %q", listW, w, line)
		}
	}
}

// TestTutorNormalWidthAlignedRowsRemainReadable drives the real TutorModel
// content-pane path at an 80-column terminal (the normal-width case from the
// review finding). At that size the content viewport is only ~53 columns, so
// aligned reference rows that used to be preserved verbatim at 57-80 columns
// were clipped by the viewport, hiding their right-hand content. The renderer
// now wraps over-wide aligned rows under a hanging indent, so this test asserts
// two things through the same width math the viewport uses:
//
//   - no rendered content line exceeds the viewport width (nothing is clipped);
//   - the representative rightmost token of each previously-clipped row survives
//     into the rendered output.
func TestTutorNormalWidthAlignedRowsRemainReadable(t *testing.T) {
	m := NewTutorModel()
	// A tall terminal keeps every topic's content within one viewport page so
	// the assertions are not confounded by vertical scrolling.
	m.SetSize(80, 400)

	vpWidth := m.viewport.Width()
	if vpWidth <= 0 {
		t.Fatalf("expected a positive content viewport width, got %d", vpWidth)
	}

	// Rightmost tokens sampled from aligned rows that overflowed the 53-column
	// viewport before the wrap fix (widths in comments are the source-row width).
	type sample struct {
		title string
		token string
	}
	samples := []sample{
		{"Quick Capture Grammar", ">Login"},              // >parent … (66 cols)
		{"Repeat Last Action", "+1m)"},                   // d1-9 … (68 cols)
		{"Attachments", "IDE)"},                          // tt attach … (65 cols)
		{"Help", "installed"},                            // provider's CLI … (54 cols)
		{"Command Palette", "inbox)"},                    // triage inbox … (61 cols)
		{"Recurrence", "2 weeks)"},                       // Nd, Nw … (63 cols)
		{"Cross-project signals at a glance", "triage)"}, // ctrl+i … (56 cols)
	}

	byTitle := map[string]int{}
	for i, topic := range m.filteredTopics {
		byTitle[topic.Title] = i
	}

	for _, s := range samples {
		idx, ok := byTitle[s.title]
		if !ok {
			t.Errorf("topic %q not found", s.title)
			continue
		}
		m.selectedIndex = idx
		// renderTopicContent is what SetSize feeds into the viewport, so it is
		// the exact string the viewport clips against.
		rendered := ansi.Strip(m.renderTopicContent())

		for _, line := range strings.Split(rendered, "\n") {
			if w := lipgloss.Width(line); w > vpWidth {
				t.Errorf("%s: rendered line exceeds viewport width %d (got %d), will be clipped: %q",
					s.title, vpWidth, w, line)
			}
		}

		if !strings.Contains(rendered, s.token) {
			t.Errorf("%s: rightmost token %q missing from rendered content (clipped or dropped):\n%s",
				s.title, s.token, rendered)
		}
	}
}
