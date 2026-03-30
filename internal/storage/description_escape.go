package storage

import "strings"

// escapeDescription escapes lines in a task description that would be
// mis-parsed as task headings (e.g., "## Task: something") or project
// titles ("# Project: something"). Without escaping, such lines cause
// the parser to split the description and create phantom tasks, leading
// to data corruption on round-trip.
//
// The escape prepends a backslash to the leading '#' characters. The
// parser reverses this via unescapeDescriptionLine.
func escapeDescription(desc string) string {
	lines := strings.Split(desc, "\n")
	for i, line := range lines {
		if headingRegex.MatchString(line) || projectRegex.MatchString(line) {
			lines[i] = `\` + line
		}
	}
	return strings.Join(lines, "\n")
}

// unescapeDescriptionLine reverses the escaping done by escapeDescription.
// If a line starts with `\#` (backslash followed by hash), and stripping
// the backslash would match a heading or project regex, the backslash is
// removed to restore the original text.
func unescapeDescriptionLine(line string) string {
	if len(line) < 2 || line[0] != '\\' || line[1] != '#' {
		return line
	}
	stripped := line[1:]
	if headingRegex.MatchString(stripped) || projectRegex.MatchString(stripped) {
		return stripped
	}
	return line
}
