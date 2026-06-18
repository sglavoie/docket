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
