package format

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// Matches comment start marker like "<!-- COMMENTS -->"
var commentsStartRegex = regexp.MustCompile(`^<!--\s*COMMENTS\s*-->$`)

// Matches comment end marker like "<!-- /COMMENTS -->"
var commentsEndRegex = regexp.MustCompile(`^<!--\s*/COMMENTS\s*-->$`)

// Matches individual comment header like "<!-- comment: 2025-12-27 10:30 -->"
var commentHeaderRegex = regexp.MustCompile(`^<!--\s*comment:\s*(.+?)\s*-->$`)

var blockHeadingRegex = regexp.MustCompile(`^(#{2,6})\s+(Epic|Story|Task|Bug|Idea|Note|Action):\s*(.+)$`)
var blockProjectRegex = regexp.MustCompile(`^#\s+Project:\s*(.+)$`)

// ExtractCommentsFromDescription parses the COMMENTS block from task description.
// It extracts comments and returns the cleaned description (without the comments block).
func ExtractCommentsFromDescription(task *models.Task) {
	if task.Description == "" {
		return
	}

	blockLines, cleanDesc := ExtractBlock(task.Description, commentsStartRegex, commentsEndRegex)
	task.Description = cleanDesc

	// Parse block lines into comments (header + accumulated text lines)
	var currentCommentText strings.Builder
	var currentCommentTime time.Time
	var hasCurrentComment bool

	flushComment := func() {
		if !hasCurrentComment {
			return
		}
		text := currentCommentText.String()
		if strings.TrimSpace(text) != "" {
			task.Comments = append(task.Comments, &models.Comment{
				Text:      unescapeBlockText(text),
				CreatedAt: currentCommentTime,
			})
		}
	}

	for _, bl := range blockLines {
		if matches := commentHeaderRegex.FindStringSubmatch(bl.Trimmed); matches != nil {
			flushComment()
			// Legacy headers may carry a retired ", resolved" suffix. Parse only
			// the timestamp so live records normalize to timestamped text.
			timeStr, _, _ := strings.Cut(matches[1], ",")
			timeStr = strings.TrimSpace(timeStr)
			if t, ok := ParseTimeField(timeStr); ok {
				currentCommentTime = t
			} else {
				currentCommentTime = time.Time{}
			}
			hasCurrentComment = true
			currentCommentText.Reset()
			continue
		}

		if hasCurrentComment {
			if currentCommentText.Len() > 0 {
				currentCommentText.WriteString("\n")
			}
			currentCommentText.WriteString(bl.Original)
		}
	}

	flushComment()
}

// WriteComments generates the comments block for a task.
// Comments whose text is empty (or whitespace-only) are skipped: the parser
// drops them on load, so emitting them would make write→parse→write lose the
// comment header on the second save.
func WriteComments(task *models.Task) string {
	if len(task.Comments) == 0 {
		return ""
	}

	var b strings.Builder
	headerWritten := false

	for _, comment := range task.Comments {
		if comment == nil {
			continue
		}
		text := comment.Text
		if strings.TrimSpace(text) == "" {
			continue
		}
		if !headerWritten {
			b.WriteString("<!-- COMMENTS -->\n")
			headerWritten = true
		}
		fmt.Fprintf(&b, "<!-- comment: %s -->\n", comment.CreatedAt.Format(util.DateTimeFormat))
		// Escape HTML comment markers and heading-like lines to prevent parser corruption.
		b.WriteString(escapeBlockText(text))
		b.WriteString("\n")
	}

	if !headerWritten {
		return ""
	}
	b.WriteString("<!-- /COMMENTS -->\n")
	return b.String()
}

// escapeCommentText escapes HTML comment markers in comment text
// to prevent parser corruption when the text contains <!-- or -->.
// Ampersands are escaped first so that pre-existing entity-like sequences
// (e.g., a literal "&lt;!--" in the text) survive a roundtrip without
// being falsely unescaped.
func escapeCommentText(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<!--", "&lt;!--")
	text = strings.ReplaceAll(text, "-->", "--&gt;")
	return text
}

// unescapeCommentText reverses the escaping done by escapeCommentText.
// Order is the reverse of escaping: comment markers first, then ampersands.
func unescapeCommentText(text string) string {
	text = strings.ReplaceAll(text, "&lt;!--", "<!--")
	text = strings.ReplaceAll(text, "--&gt;", "-->")
	text = strings.ReplaceAll(text, "&amp;", "&")
	return text
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
	if len(line) >= 2 && line[0] == '\\' {
		return needsBlockTextEscaping(line[1:])
	}
	return false
}
