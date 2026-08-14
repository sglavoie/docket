package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rawANSITruncateHelperFile is the single production file allowed to call
// ansi.Truncate directly. All other render code must route through the audited
// truncateIntentional / truncateRequired helpers so truncation policy (dense
// data may clip silently; layout chrome records a render-audit issue) is applied
// consistently. See session 4 of the truncation-auditing plan.
const rawANSITruncateHelperFile = "text_width_helpers.go"

// TestNoRawANSITruncate fails when a non-test .go file other than the approved
// helper calls ansi.Truncate directly. New render truncation must go through
// truncateIntentional (dense data) or truncateRequired (layout chrome/help).
func TestNoRawANSITruncate(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	const needle = "ansi.Truncate("
	var offenders []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") || name == rawANSITruncateHelperFile {
			continue
		}
		data, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(data), needle) {
			offenders = append(offenders, name)
		}
	}

	if len(offenders) > 0 {
		t.Errorf("raw ansi.Truncate found outside %s in: %s\n"+
			"Use truncateIntentional (dense data) or truncateRequired (layout chrome/help) instead.",
			rawANSITruncateHelperFile, strings.Join(offenders, ", "))
	}
}

// TestTextWidthRequiredTruncationRecordsAudit verifies that truncateRequired
// reports a required-text-truncation issue when it clips, and stays silent when
// the text already fits.
func TestTextWidthRequiredTruncationRecordsAudit(t *testing.T) {
	// Clip: should record an audit issue and return the shortened text.
	out, issues := withRenderAudit(func() string {
		return truncateRequired("test chrome", "abcdefghij", 4)
	})
	if got := len(out); got == 0 || got >= len("abcdefghij") {
		t.Errorf("expected clipped output, got %q", out)
	}
	if !hasIssueKind(issues, requiredTextTruncatedKind) {
		t.Errorf("expected %s issue for clipped required text, got %+v", requiredTextTruncatedKind, issues)
	}

	// Fits: no clip, no audit issue.
	_, issues = withRenderAudit(func() string {
		return truncateRequired("test chrome", "abc", 40)
	})
	if hasIssueKind(issues, requiredTextTruncatedKind) {
		t.Errorf("expected no audit issue when required text fits, got %+v", issues)
	}
}

// TestVisualWidthOverflowCompletesTheTruncatedHead verifies that the head
// truncateToVisualWidth keeps, minus its ellipsis, concatenated with the tail
// visualWidthOverflow returns, reconstructs the input exactly. The two must
// agree about the cut or Peek would duplicate or lose a character.
func TestVisualWidthOverflowCompletesTheTruncatedHead(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		width int
	}{
		{"ascii", "abcdefghijklmnopqrstuvwxyz", 10},
		{"wide runes on the boundary", strings.Repeat("界", 12), 9},
		{"mixed wide and narrow", "ab界cd界ef界gh", 7},
		{"width below the ellipsis reserve", "abcdef", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			head := strings.TrimSuffix(truncateToVisualWidth(tc.text, tc.width), "…")
			tail := visualWidthOverflow(tc.text, tc.width)
			if tail == "" {
				t.Fatalf("visualWidthOverflow(%q, %d) = \"\", want the clipped tail", tc.text, tc.width)
			}
			if got := head + tail; got != tc.text {
				t.Fatalf("head+tail = %q, want %q", got, tc.text)
			}
		})
	}
}

func TestVisualWidthOverflowIsEmptyWhenTextFits(t *testing.T) {
	for _, tc := range []struct {
		text  string
		width int
	}{
		{"short", 40},
		{"exact fit", len("exact fit")},
		{strings.Repeat("界", 4), 8},
		{"anything", 0},
	} {
		if got := visualWidthOverflow(tc.text, tc.width); got != "" {
			t.Fatalf("visualWidthOverflow(%q, %d) = %q, want \"\"", tc.text, tc.width, got)
		}
	}
}

// TestTextWidthIntentionalTruncationNoAudit verifies that truncateIntentional
// clips dense data without recording a render-audit issue, even inside an audit.
func TestTextWidthIntentionalTruncationNoAudit(t *testing.T) {
	out, issues := withRenderAudit(func() string {
		return truncateIntentional("abcdefghij", 4, "")
	})
	if got := len(out); got == 0 || got >= len("abcdefghij") {
		t.Errorf("expected clipped output, got %q", out)
	}
	if hasIssueKind(issues, requiredTextTruncatedKind) {
		t.Errorf("intentional truncation must not record an audit issue, got %+v", issues)
	}
}
