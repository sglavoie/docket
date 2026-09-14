package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// normalizeScreen strips ANSI escape sequences from raw rendered content and
// validates the result against the terminal dimensions. It returns the plain
// (ANSI-free) screen plus any layout issues found:
//
//   - invalid_utf8: the plain screen is not valid UTF-8
//   - width_overflow: a line's visual width exceeds the terminal width
//   - height_overflow: the screen has more rows than the terminal height
//
// width and height of zero (or negative) disable the corresponding check.
func normalizeScreen(rawContent string, width, height int) (plain string, issues []UITestIssue) {
	// Validate UTF-8 on the raw content first: ansi.Strip sanitizes invalid
	// bytes, so checking the stripped result would never surface a real problem.
	if !utf8.ValidString(rawContent) {
		issues = append(issues, UITestIssue{
			Kind:    "invalid_utf8",
			Message: "screen contains invalid UTF-8",
		})
	}

	plain = ansi.Strip(rawContent)

	lines := strings.Split(plain, "\n")

	if height > 0 && len(lines) > height {
		issues = append(issues, UITestIssue{
			Kind:    "height_overflow",
			Message: fmt.Sprintf("rendered %d lines exceeds terminal height %d", len(lines), height),
		})
	}

	if width > 0 {
		for i, line := range lines {
			if w := lipgloss.Width(line); w > width {
				issues = append(issues, UITestIssue{
					Kind:    "width_overflow",
					Line:    i + 1,
					Message: fmt.Sprintf("line %d visual width %d exceeds terminal width %d", i+1, w, width),
				})
			}
		}
	}

	return plain, issues
}

// truncationEllipsis is the single character every width-aware renderer appends
// when it drops content: textfit.TruncateToVisualWidth writes it, and the row,
// header, chip, and guidance fitters all reach the terminal through that one
// call. Recognizing it is the whole of clipping detection, which is why nothing
// here pattern-matches a particular surface.
const truncationEllipsis = "…"

// screenClippedLines names every rendered line the fitters had to truncate.
//
// It exists because normalizeScreen is otherwise a perfect way to hide exactly
// the defect this work is about: a line clipped to the terminal width passes the
// width check, carries no layout issue, and leaves a `contains` assertion on the
// full text failing for a reason that reads like a missing row rather than a lost
// one. The list is an observation and never a UITestIssue: clipping is legal
// today on rows, chips, and guidance, so promoting it to a layout issue would
// make `no_layout_issues` mean something else on every committed scenario.
//
// It is the complement of the render audit, not a second copy of it: the audit
// hears only from the call sites that declared their text required, while this
// reads the finished screen and so sees every clip, required or not.
func screenClippedLines(plain string) []UITestClippedLine {
	var clipped []UITestClippedLine
	for i, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, truncationEllipsis) {
			clipped = append(clipped, UITestClippedLine{
				Line: i + 1,
				Text: strings.TrimRight(line, " "),
			})
		}
	}
	return clipped
}

// clippedPrefixOf reports the longest leading run of want that the screen shows
// immediately before a truncation ellipsis, which is how a "this text is not on
// screen" failure can say whether the text was absent or merely cut short.
//
// It only ever shapes the failure message: the verdict `not_clipped` returns is
// a plain containment check, so a short coincidental match costs a wording
// choice rather than a wrong result. minimumClippedPrefixRunes keeps that wording
// honest by refusing to call a one-character coincidence a clipped rendering.
const minimumClippedPrefixRunes = 4

func clippedPrefixOf(plain, want string) (string, bool) {
	runes := []rune(want)
	if len(runes) <= minimumClippedPrefixRunes {
		return "", false
	}
	longest := ""
	for offset := 0; ; {
		index := strings.Index(plain[offset:], truncationEllipsis)
		if index < 0 {
			break
		}
		head := plain[:offset+index]
		offset += index + len(truncationEllipsis)
		// The longest prefix of want that this clipped line ends with. Walking
		// down from the full string stops at the first hit, which is the longest.
		for size := len(runes) - 1; size >= minimumClippedPrefixRunes; size-- {
			candidate := string(runes[:size])
			if strings.HasSuffix(head, candidate) {
				if len(candidate) > len(longest) {
					longest = candidate
				}
				break
			}
		}
	}
	return longest, longest != ""
}
