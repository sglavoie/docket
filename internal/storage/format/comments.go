package format

import (
	"regexp"
	"strings"
)

var (
	commentsStartRegex = regexp.MustCompile(`^<!--\s*COMMENTS\s*-->$`)
	commentsEndRegex   = regexp.MustCompile(`^<!--\s*/COMMENTS\s*-->$`)
	commentHeaderRegex = regexp.MustCompile(`^<!--\s*comment:\s*(.+?)\s*-->$`)
	blockHeadingRegex  = regexp.MustCompile(`^(#{2,6})\s+(Epic|Story|Task|Bug|Idea|Note|Action):\s*(.+)$`)
	blockProjectRegex  = regexp.MustCompile(`^#\s+Project:\s*(.+)$`)
)

func escapeCommentText(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<!--", "&lt;!--")
	return strings.ReplaceAll(text, "-->", "--&gt;")
}

func unescapeCommentText(text string) string {
	text = strings.ReplaceAll(text, "&lt;!--", "<!--")
	text = strings.ReplaceAll(text, "--&gt;", "-->")
	return strings.ReplaceAll(text, "&amp;", "&")
}

func escapeBlockText(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		line = escapeCommentText(line)
		if needsBlockTextEscaping(line) {
			line = `\` + line
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

func unescapeBlockText(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if len(line) >= 2 && line[0] == '\\' && needsBlockTextEscaping(line[1:]) {
			line = line[1:]
		}
		lines[i] = unescapeCommentText(line)
	}
	return strings.Join(lines, "\n")
}

func needsBlockTextEscaping(line string) bool {
	if blockHeadingRegex.MatchString(line) || blockProjectRegex.MatchString(line) {
		return true
	}
	return len(line) >= 2 && line[0] == '\\' && needsBlockTextEscaping(line[1:])
}
