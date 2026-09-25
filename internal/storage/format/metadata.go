package format

import (
	"regexp"
	"strings"
)

var metadataRegex = regexp.MustCompile(`^<!--\s*(.+?)\s*-->\s*$`)

func MetadataRegex() *regexp.Regexp { return metadataRegex }

// reWhitespace is RE2's \s class, which is narrower than unicode.IsSpace.
const reWhitespace = "\t\n\f\r "

// MetadataBody returns what MetadataRegex captures from line, or false where it
// does not match. The lazy capture makes the regexp backtrack quadratically in
// the line's length, and every task's metadata line is parsed on every load, so
// the ordinary line — a comment with a nonblank single-line body — is answered
// with string operations and anything else is left to the regexp itself.
func MetadataBody(line string) (string, bool) {
	trimmed := strings.TrimRight(line, reWhitespace)
	if strings.HasPrefix(trimmed, "<!--") && strings.HasSuffix(trimmed, "-->") && len(trimmed) >= len("<!---->") {
		body := strings.Trim(trimmed[len("<!--"):len(trimmed)-len("-->")], reWhitespace)
		if body != "" && !strings.Contains(body, "\n") {
			return body, true
		}
	}
	match := metadataRegex.FindStringSubmatch(line)
	if match == nil {
		return "", false
	}
	return match[1], true
}
