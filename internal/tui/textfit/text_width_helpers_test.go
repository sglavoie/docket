package textfit

import (
	"io/fs"
	"os"
	"runtime"
	"strings"
	"testing"
)

// rawANSITruncateHelperFile is the single production file allowed to call
// ansi.Truncate directly. All other render code must route through the audited
// TruncateIntentional / truncateRequired helpers so truncation policy (dense
// data may clip silently; layout chrome records a render-audit issue) is applied
// consistently. See session 4 of the truncation-auditing plan.
const rawANSITruncateHelperFile = "text_width_helpers.go"

// TestNoRawANSITruncate fails when a non-test .go file anywhere in the
// internal/tui subtree other than the approved helper calls ansi.Truncate
// directly. New render truncation must go through TruncateIntentional (dense
// data) or FitStyledChromeToWidth (layout chrome/help). The audit lives beside
// the helper in textfit but still walks the whole internal/tui subtree, so every
// sibling package stays covered. The exemption is by filename so it follows the
// helper into whichever package owns it, which is only safe while that filename
// is unique — asserted below.
func TestNoRawANSITruncate(t *testing.T) {
	const needle = "ansi.Truncate("
	// Anchored on this file's own source path so the scan follows the subtree,
	// not the working directory, at any package depth.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate the raw ansi.Truncate audit")
	}

	var offenders []string
	helperFiles := 0
	err := walkTUIProductionSourcesFrom(t, thisFile, func(path, relative string, entry fs.DirEntry) error {
		if entry.Name() == rawANSITruncateHelperFile {
			helperFiles++
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(data), needle) {
			offenders = append(offenders, relative)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan production TUI sources: %v", err)
	}
	if helperFiles != 1 {
		t.Errorf("%s exists %d times in internal/tui, want exactly 1: a second copy would silently exempt itself",
			rawANSITruncateHelperFile, helperFiles)
	}

	if len(offenders) > 0 {
		t.Errorf("raw ansi.Truncate found outside %s in: %s\n"+
			"Use TruncateIntentional (dense data) or FitStyledChromeToWidth (layout chrome/help) instead.",
			rawANSITruncateHelperFile, strings.Join(offenders, ", "))
	}
}

// truncationRecord is one call the render-audit seam reported.
type truncationRecord struct {
	site, full, shown string
}

// withTruncationSink installs RequiredTextTruncationSink for the duration of
// render and returns what it recorded. It is the package-local counterpart to
// the shell's withRenderAudit, which installs the same seam and turns each
// record into a UITestIssue; that translation belongs to the harness, so the
// tests here observe the seam directly instead of reaching up for it.
func withTruncationSink(render func() string) (string, []truncationRecord) {
	var records []truncationRecord
	previous := RequiredTextTruncationSink
	RequiredTextTruncationSink = func(site, full, shown string) {
		records = append(records, truncationRecord{site: site, full: full, shown: shown})
	}
	defer func() { RequiredTextTruncationSink = previous }()
	return render(), records
}

func TestRequiredTruncationSinkIsSilentWithoutAnObserver(t *testing.T) {
	if RequiredTextTruncationSink != nil {
		t.Fatal("RequiredTextTruncationSink must be nil for ordinary rendering")
	}
	if got := truncateRequired("orphan site", "abcdefghij", 4); got != "abcd" {
		t.Errorf("truncateRequired without a sink = %q, want %q", got, "abcd")
	}
	if RequiredTextTruncationSink != nil {
		t.Error("truncateRequired must not install a sink")
	}
}

// TestTextWidthRequiredTruncationRecordsAudit verifies that truncateRequired
// reports a required-text-truncation record when it clips, and stays silent when
// the text already fits.
func TestTextWidthRequiredTruncationRecordsAudit(t *testing.T) {
	// Clip: should record against the site and return the shortened text.
	out, records := withTruncationSink(func() string {
		return truncateRequired("test chrome", "abcdefghij", 4)
	})
	if got := len(out); got == 0 || got >= len("abcdefghij") {
		t.Errorf("expected clipped output, got %q", out)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 required-truncation record, got %+v", records)
	}
	if records[0].site != "test chrome" || records[0].full != "abcdefghij" || records[0].shown != out {
		t.Errorf("record = %+v, want site %q, full %q and shown %q", records[0], "test chrome", "abcdefghij", out)
	}

	// Fits: no clip, no record.
	if _, records = withTruncationSink(func() string {
		return truncateRequired("test chrome", "abc", 40)
	}); len(records) != 0 {
		t.Errorf("expected no record when required text fits, got %+v", records)
	}
}

// TestFitStyledChromeToWidthRoutesThroughTheAudit verifies the one exported
// entry point onto truncateRequired keeps reporting, so a caller that fits its
// chrome the audited way is what the shell harness observes.
func TestFitStyledChromeToWidthRoutesThroughTheAudit(t *testing.T) {
	_, records := withTruncationSink(func() string {
		return FitStyledChromeToWidth("fullscreen title", "a title far wider than the pane", 8)
	})
	if len(records) != 1 || records[0].site != "fullscreen title" {
		t.Fatalf("expected one record naming the render site, got %+v", records)
	}

	if _, records = withTruncationSink(func() string {
		return FitStyledChromeToWidth("fullscreen title", "short", 40)
	}); len(records) != 0 {
		t.Errorf("expected no record when the chrome already fits, got %+v", records)
	}
}

// TestVisualWidthOverflowCompletesTheTruncatedHead verifies that the head
// TruncateToVisualWidth keeps, minus its ellipsis, concatenated with the tail
// VisualWidthOverflow returns, reconstructs the input exactly. The two must
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
			head := strings.TrimSuffix(TruncateToVisualWidth(tc.text, tc.width), "…")
			tail := VisualWidthOverflow(tc.text, tc.width)
			if tail == "" {
				t.Fatalf("VisualWidthOverflow(%q, %d) = \"\", want the clipped tail", tc.text, tc.width)
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
		if got := VisualWidthOverflow(tc.text, tc.width); got != "" {
			t.Fatalf("VisualWidthOverflow(%q, %d) = %q, want \"\"", tc.text, tc.width, got)
		}
	}
}

// TestTextWidthIntentionalTruncationNoAudit verifies that TruncateIntentional
// clips dense data without recording anything, even while a sink is installed.
func TestTextWidthIntentionalTruncationNoAudit(t *testing.T) {
	out, records := withTruncationSink(func() string {
		return TruncateIntentional("abcdefghij", 4, "")
	})
	if got := len(out); got == 0 || got >= len("abcdefghij") {
		t.Errorf("expected clipped output, got %q", out)
	}
	if len(records) != 0 {
		t.Errorf("intentional truncation must not record, got %+v", records)
	}
}
