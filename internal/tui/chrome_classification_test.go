package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/tui/listsurface"
)

// TestListSurfaceAdapterDataRowTruncationNoAudit verifies that a long picker data row is
// clipped intentionally (via the dense-data path) without recording a
// required-text-truncation audit issue, even while the picker's title/footer
// chrome is audited. Picker rows are data, not chrome.
func TestListSurfaceAdapterDataRowTruncationNoAudit(t *testing.T) {
	longRow := strings.Repeat("very-long-picker-row-content ", 8)
	picker := listsurface.NewListSurfaceAdapter([]testItem{{id: "1", text: longRow}}, listsurface.PickerConfig{
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
