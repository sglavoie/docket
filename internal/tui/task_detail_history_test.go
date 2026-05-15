package tui

import (
	"strings"
	"testing"
)

func TestHistoryCountsSummary(t *testing.T) {
	cases := []struct {
		name                                string
		commentsN, decisionsN, attachmentsN int
		want                                string
	}{
		{"all zero", 0, 0, 0, ""},
		{"singular comment", 1, 0, 0, "1 comment"},
		{"plural comments", 3, 0, 0, "3 comments"},
		{"singular decision only", 0, 1, 0, "1 decision"},
		{"singular attachment only", 0, 0, 1, "1 attachment"},
		{"mixed singular/plural", 3, 1, 2, "3 comments · 1 decision · 2 attachments"},
		{"skip zero entries", 0, 2, 0, "2 decisions"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := historyCountsSummary(c.commentsN, c.decisionsN, c.attachmentsN)
			if got != c.want {
				t.Errorf("historyCountsSummary(%d,%d,%d) = %q, want %q",
					c.commentsN, c.decisionsN, c.attachmentsN, got, c.want)
			}
		})
	}
}

func TestRenderHistoryClusterDivider(t *testing.T) {
	collapsed := renderHistoryClusterDivider(3, 1, 2, false, 100)
	if !strings.Contains(collapsed, "History") {
		t.Errorf("collapsed divider missing label: %q", collapsed)
	}
	if !strings.Contains(collapsed, "3 comments · 1 decision · 2 attachments") {
		t.Errorf("collapsed divider missing summary: %q", collapsed)
	}
	if !strings.Contains(collapsed, "press H to expand") {
		t.Errorf("collapsed divider missing expand hint: %q", collapsed)
	}

	expanded := renderHistoryClusterDivider(3, 1, 2, true, 100)
	if !strings.Contains(expanded, "press H to collapse") {
		t.Errorf("expanded divider missing collapse hint: %q", expanded)
	}
}

func TestTaskDetailHistoryToggleDefault(t *testing.T) {
	m := &TaskDetailModel{}
	if m.historyExpanded {
		t.Errorf("historyExpanded should default to false")
	}
}
