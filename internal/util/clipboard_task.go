package util

import (
	"regexp"
	"strings"
)

const maxTitleLen = 700

// PastedTask holds the parsed result of clipboard text.
type PastedTask struct {
	Title       string
	URL         string // empty if none detected
	Description string // multi-line remainder, empty for single-line input
}

var (
	reMDLink  = regexp.MustCompile(`^\[(.+)\]\((https?://\S+|file://\S+)\)$`)
	reBareURL = regexp.MustCompile(`^(https?://\S+|file://\S+)$`)
)

// ParseClipboard parses clipboard text into a PastedTask.
//
// Rules:
//   - Empty/whitespace-only → zero value.
//   - Single non-empty line: markdown link → Title+URL; bare URL → Title=URL; otherwise Title=text.
//   - Multi-line: first non-empty line treated as the single-line case for title/URL;
//     remaining non-leading-empty lines become Description.
//
// Titles are capped at maxTitleLen (700) characters.
func ParseClipboard(text string) PastedTask {
	// Normalise CRLF/CR to LF so multi-line descriptions don't carry
	// stray \r bytes that corrupt downstream rendering.
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimRight(text, "\n")
	if strings.TrimSpace(text) == "" {
		return PastedTask{}
	}

	lines := strings.Split(text, "\n")

	// Find first non-empty line for the title.
	titleLineIdx := -1
	for i, l := range lines {
		if strings.TrimSpace(l) != "" {
			titleLineIdx = i
			break
		}
	}
	if titleLineIdx == -1 {
		return PastedTask{}
	}

	titleLine := strings.TrimSpace(lines[titleLineIdx])
	pt := parseSingleLine(titleLine)

	// Collect remainder as description (lines after the title line, skip leading blanks).
	rest := lines[titleLineIdx+1:]
	descLines := append([]string{}, rest...)
	pt.Description = strings.TrimSpace(strings.Join(descLines, "\n"))

	return pt
}

// parseSingleLine handles the title/URL extraction from one trimmed line.
func parseSingleLine(line string) PastedTask {
	// Markdown link: [title](url)
	if m := reMDLink.FindStringSubmatch(line); m != nil {
		return PastedTask{
			Title: capTitle(m[1]),
			URL:   m[2],
		}
	}
	// Bare URL
	if reBareURL.MatchString(line) {
		return PastedTask{
			Title: capTitle(line),
			URL:   line,
		}
	}
	return PastedTask{Title: capTitle(line)}
}

func capTitle(s string) string {
	runes := []rune(s)
	if len(runes) > maxTitleLen {
		return string(runes[:maxTitleLen])
	}
	return s
}
