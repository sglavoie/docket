package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// TestFullscreenTitleChromeFittingStaysWithinWidth verifies that the semantic
// title renderer keeps full-screen chrome on one bounded row at both narrow and
// wide terminal sizes.
func TestFullscreenTitleChromeFittingStaysWithinWidth(t *testing.T) {
	longPrefix := strings.Repeat("Fullscreen Title Prefix ", 6)
	for _, tc := range []struct {
		name  string
		title string
		width int
		pulse WorkspacePulse
	}{
		{name: "narrow", title: longPrefix, width: 20, pulse: WorkspacePulse{}},
		{name: "wide", title: "Tasks", width: 120, pulse: WorkspacePulse{Inbox: 3, Today: 2, Overdue: 1, Done: 4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rendered := renderTaskScopedFullscreenTitleWithPulse(tc.title, nil, "", tc.pulse, tc.width)
			if strings.Contains(rendered, "\n") {
				t.Fatalf("fullscreen title wrapped onto multiple rows: %q", rendered)
			}
			if got := lipgloss.Width(rendered); got > tc.width {
				t.Fatalf("fullscreen title width = %d, want <= %d: %q", got, tc.width, rendered)
			}
			if tc.name == "wide" && !strings.Contains(rendered, "done 4") {
				t.Fatalf("wide fullscreen title omitted Done progress: %q", rendered)
			}
		})
	}
}

// TestListSurfaceAdapterDataRowTruncationNoAudit verifies that a long picker data row is
// clipped intentionally (via the dense-data path) without recording a
// required-text-truncation audit issue, even while the picker's title/footer
// chrome is audited. Picker rows are data, not chrome.
func TestListSurfaceAdapterDataRowTruncationNoAudit(t *testing.T) {
	longRow := strings.Repeat("very-long-picker-row-content ", 8)
	picker := NewListSurfaceAdapter([]testItem{{id: "1", text: longRow}}, PickerConfig{
		EmptyMessage:   "No items",
		NoMatchMessage: "No matches",
	})
	// Width is wide enough for the short title chrome but far narrower than the
	// row, so only the data row clips.
	picker.SetSize(40, 20)

	_, issues := withRenderAudit(func() string {
		return picker.RenderInBox("Pick", "", 5)
	})
	if hasIssueKind(issues, requiredTextTruncatedKind) {
		t.Fatalf("picker data row truncation must not record a %s audit issue, got %+v",
			requiredTextTruncatedKind, issues)
	}
}
