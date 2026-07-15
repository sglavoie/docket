package actionplan

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// DisplayTextMode selects the terminal-safety policy for provider-authored
// display text. Single-line text rejects every Unicode control (Cc) and format
// (Cf) character. Multiline text permits LF and tab for Markdown layout while
// rejecting every other control or format character, including carriage
// return, terminal escape payloads, and bidirectional formatting controls.
type DisplayTextMode uint8

const (
	// DisplayTextSingleLine is used for summaries, titles, identifiers, tags,
	// dates, and every other provider-authored field that must remain one line.
	DisplayTextSingleLine DisplayTextMode = iota
	// DisplayTextMultiline is used only for descriptions whose Markdown may
	// legitimately contain LF and tab characters.
	DisplayTextMultiline
)

func validateDisplayText(field, value string, mode DisplayTextMode) error {
	for _, r := range value {
		if displayRuneAllowed(r, mode) {
			continue
		}
		kind := "control"
		if unicode.Is(unicode.Cf, r) {
			kind = "format"
		}
		return fmt.Errorf("%s contains unsafe terminal %s character U+%04X", field, kind, r)
	}
	return nil
}

func displayRuneAllowed(r rune, mode DisplayTextMode) bool {
	if mode == DisplayTextMultiline && (r == '\n' || r == '\t') {
		return true
	}
	return !unicode.IsControl(r) && !unicode.Is(unicode.Cf, r)
}

// TerminalSafeDisplayText defensively renders display text under the same
// policy used at the provider boundary. Unsafe control and format characters
// become visible Go-style escapes, so a caller remains safe even if
// unvalidated data reaches preview rendering.
func TerminalSafeDisplayText(value string, mode DisplayTextMode) string {
	var safe strings.Builder
	for _, r := range value {
		if displayRuneAllowed(r, mode) {
			safe.WriteRune(r)
			continue
		}
		quoted := strconv.QuoteRune(r)
		safe.WriteString(quoted[1 : len(quoted)-1])
	}
	return safe.String()
}
