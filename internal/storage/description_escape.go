package storage

import (
	"regexp"
	"strings"
)

// matchesHeadingOrProject returns true if the line matches a task heading
// or project title regex.
func matchesHeadingOrProject(line string) bool {
	return headingRegex.MatchString(line) || projectRegex.MatchString(line)
}

// blockMarkerRegex matches the 8 block-marker lines used by the storage
// format: DESCRIPTION/COMMENTS/ACTIVITY/DECISIONS start and end markers.
// Without escaping, such lines inside a description body cause the
// ExtractBlock parser to consume them and treat following text as
// belonging to that block, corrupting the round-trip.
var blockMarkerRegex = regexp.MustCompile(`^<!--\s*/?(DESCRIPTION|COMMENTS|ACTIVITY|DECISIONS)\s*-->$`)

// nonDescriptionBlockMarkerRegex matches start/end markers for blocks whose
// own extractors run their own unescape pass (COMMENTS/ACTIVITY/DECISIONS).
// Used by the parser to suppress per-line heading-unescape inside those
// blocks so the text isn't unescaped twice.
var nonDescriptionBlockMarkerRegex = regexp.MustCompile(`^<!--\s*/?(COMMENTS|ACTIVITY|DECISIONS)\s*-->$`)

// matchesBlockMarker returns true if the trimmed line is one of the 8
// block-marker lines (start or end of DESCRIPTION/COMMENTS/ACTIVITY/DECISIONS).
func matchesBlockMarker(line string) bool {
	return blockMarkerRegex.MatchString(strings.TrimSpace(line))
}

// matchesNonDescriptionBlockMarker returns true if the line is a start or end
// marker for COMMENTS/ACTIVITY/DECISIONS (but not DESCRIPTION).
func matchesNonDescriptionBlockMarker(line string) bool {
	return nonDescriptionBlockMarkerRegex.MatchString(strings.TrimSpace(line))
}

// escapeDescription escapes lines in a task description that would be
// mis-parsed as task headings (e.g., "## Task: something") or project
// titles ("# Project: something"). Without escaping, such lines cause
// the parser to split the description and create phantom tasks, leading
// to data corruption on round-trip.
//
// The escape prepends a backslash to the leading '#' characters. The
// parser reverses this via unescapeDescriptionLine.
//
// Lines that already start with one or more backslashes followed by a
// heading/project pattern (e.g., `\## Task: foo`) must also be escaped
// so that the unescape pass does not strip a user-written backslash.
//
// Block-marker lines (`<!-- COMMENTS -->`, etc.) are also escaped using a
// leading `\`. Their unescape is deferred via unescapeBlockMarkers so that
// the per-line unescape does not strip the backslash before ExtractBlock
// runs (which would re-introduce the data-loss bug).
func escapeDescription(desc string) string {
	lines := strings.Split(desc, "\n")
	for i, line := range lines {
		if needsEscaping(line) || needsBlockEscaping(line) {
			lines[i] = `\` + line
		}
	}
	return strings.Join(lines, "\n")
}

// needsEscaping returns true if the line would be modified by
// unescapeDescriptionLine (and therefore must be prepended with `\`).
func needsEscaping(line string) bool {
	// Direct heading/project match — must escape.
	if matchesHeadingOrProject(line) {
		return true
	}
	// Lines starting with `\` that unescapeDescriptionLine would strip:
	// `\## Task: ...` would be unescaped to `## Task: ...`
	// `\\## Task: ...` would be unescaped to `\## Task: ...` (which itself needs escaping)
	// In general, any line that unescape would change needs to be escaped.
	if len(line) >= 2 && line[0] == '\\' {
		stripped := line[1:]
		// unescapeDescriptionLine strips the backslash if the remainder
		// matches a heading/project OR if the remainder itself would be
		// unescaped (recursive). We mirror that check here.
		if wouldUnescape(stripped) {
			return true
		}
	}
	return false
}

// wouldUnescape returns true if unescapeDescriptionLine would modify this line.
func wouldUnescape(line string) bool {
	if len(line) < 2 || line[0] != '#' && line[0] != '\\' {
		return false
	}
	if matchesHeadingOrProject(line) {
		return true
	}
	// Recursive case: `\#...` or `\\#...` chains.
	if line[0] == '\\' {
		return wouldUnescape(line[1:])
	}
	return false
}

// needsBlockEscaping mirrors needsEscaping for block-marker lines. Used
// by the writer to decide whether to prepend `\`. The corresponding
// unescape runs after Extract* via unescapeBlockMarkers.
func needsBlockEscaping(line string) bool {
	if matchesBlockMarker(line) {
		return true
	}
	if len(line) >= 1 && line[0] == '\\' {
		if wouldBlockUnescape(line[1:]) {
			return true
		}
	}
	return false
}

// wouldBlockUnescape returns true if a line, after stripping one `\`,
// would be a block marker (possibly through nested backslash chains).
func wouldBlockUnescape(line string) bool {
	if matchesBlockMarker(line) {
		return true
	}
	if len(line) >= 1 && line[0] == '\\' {
		return wouldBlockUnescape(line[1:])
	}
	return false
}

// unescapeDescriptionLine reverses the escaping done by escapeDescription
// for heading/project lines. Block-marker lines are intentionally NOT
// unescaped here so that they survive ExtractBlock unmodified; their
// unescape happens later via unescapeBlockMarkers.
func unescapeDescriptionLine(line string) string {
	if len(line) < 2 || line[0] != '\\' {
		return line
	}
	stripped := line[1:]
	// The writer only prepends `\` when needsEscaping(original) is true.
	// So we strip one `\` when the remainder matches the same predicate.
	if needsEscaping(stripped) {
		return stripped
	}
	return line
}

// unescapeBlockMarkers strips the leading `\` from any block-marker
// lines in the description. Must run AFTER ExtractActivity/Comments/
// Decisions/Description so that escaped markers in the body survive
// extraction. Mirrors the recursive backslash-chain logic of
// unescapeDescriptionLine for headings.
func unescapeBlockMarkers(desc string) string {
	if desc == "" {
		return desc
	}
	lines := strings.Split(desc, "\n")
	for i, line := range lines {
		if len(line) < 1 || line[0] != '\\' {
			continue
		}
		stripped := line[1:]
		if needsBlockEscaping(stripped) {
			lines[i] = stripped
		}
	}
	return strings.Join(lines, "\n")
}
