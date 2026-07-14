package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestFocusBoardFooterNeverShowsMoreVisibleItemsThanTotal(t *testing.T) {
	m := cockpitModel(nil, nil, nil, nil, OpenLoops{}, boardTabNext)
	m.width = 80
	m.filtered = []boardRow{{}}
	m.rows = nil

	footer := ansi.Strip(m.renderFocusBoardFooter(focusBoardTodayStats{}))
	if strings.Contains(footer, "1/0 items") {
		t.Fatalf("footer exposed impossible count: %q", footer)
	}
	if !strings.Contains(footer, "1/1 items") {
		t.Fatalf("footer count = %q, want 1/1 items", footer)
	}
}
