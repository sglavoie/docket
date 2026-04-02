package storage

import "strings"

// matchesHeadingOrProject returns true if the line matches a task heading
// or project title regex.
func matchesHeadingOrProject(line string) bool {
	return headingRegex.MatchString(line) || projectRegex.MatchString(line)
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
func escapeDescription(desc string) string {
	lines := strings.Split(desc, "\n")
	for i, line := range lines {
		if needsEscaping(line) {
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

// unescapeDescriptionLine reverses the escaping done by escapeDescription.
// If a line starts with `\` and the remainder is something that was escaped
// (a heading/project match, or another escape layer), the leading backslash
// is removed to restore the original text.
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
