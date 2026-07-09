package tui

import (
	"strings"
	"testing"
)

// TestFullscreenTitleChromeTruncationRecordsAudit verifies that fullscreen title
// chrome routes through required-text auditing: when the title line cannot fit
// its width, the render audit records a required_text_truncated issue. Title
// chrome is not dense data, so a clip there should be surfaced as suspicious.
func TestFullscreenTitleChromeTruncationRecordsAudit(t *testing.T) {
	longPrefix := strings.Repeat("Fullscreen Title Prefix ", 6)

	// Clip: a wide title in a narrow terminal must record an audit issue.
	_, issues := withRenderAudit(func() string {
		return renderTaskScopedFullscreenTitle(longPrefix, nil, "", 20)
	})
	if !hasIssueKind(issues, requiredTextTruncatedKind) {
		t.Fatalf("expected %s issue when fullscreen title chrome is clipped, got %+v",
			requiredTextTruncatedKind, issues)
	}

	// Fits: a short title with ample width must not record any audit issue.
	_, issues = withRenderAudit(func() string {
		return renderTaskScopedFullscreenTitle("Tasks", nil, "", 120)
	})
	if hasIssueKind(issues, requiredTextTruncatedKind) {
		t.Fatalf("expected no audit issue when fullscreen title fits, got %+v", issues)
	}
}

// TestBasePickerDataRowTruncationNoAudit verifies that a long picker data row is
// clipped intentionally (via the dense-data path) without recording a
// required-text-truncation audit issue, even while the picker's title/footer
// chrome is audited. Picker rows are data, not chrome.
func TestBasePickerDataRowTruncationNoAudit(t *testing.T) {
	longRow := strings.Repeat("very-long-picker-row-content ", 8)
	picker := NewBasePicker([]testItem{{id: "1", text: longRow}}, PickerConfig{
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
