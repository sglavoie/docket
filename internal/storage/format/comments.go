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

// ExtractCommentsFromDescription parses the COMMENTS block from task description.
// It extracts comments and returns the cleaned description (without the comments block).
func ExtractCommentsFromDescription(task *models.Task) {
	if task.Description == "" {
		return
	}

	lines := strings.Split(task.Description, "\n")
	var cleanDesc strings.Builder
	var inCommentsBlock bool
	var currentCommentText strings.Builder
	var currentCommentTime time.Time
	var hasCurrentComment bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for comments block start
		if commentsStartRegex.MatchString(trimmed) {
			inCommentsBlock = true
			continue
		}

		// Check for comments block end
		if commentsEndRegex.MatchString(trimmed) {
			// Save any pending comment
			if hasCurrentComment {
				text := strings.TrimSpace(currentCommentText.String())
				if text != "" {
					task.Comments = append(task.Comments, &models.Comment{
						Text:      unescapeCommentText(text),
						CreatedAt: currentCommentTime,
					})
				}
			}
			inCommentsBlock = false
			hasCurrentComment = false
			currentCommentText.Reset()
			continue
		}

		if inCommentsBlock {
			// Check for new comment header
			if matches := commentHeaderRegex.FindStringSubmatch(trimmed); matches != nil {
				// Save previous comment if exists
				if hasCurrentComment {
					text := strings.TrimSpace(currentCommentText.String())
					if text != "" {
						task.Comments = append(task.Comments, &models.Comment{
							Text:      unescapeCommentText(text),
							CreatedAt: currentCommentTime,
						})
					}
				}
				// Start new comment
				if t, ok := ParseTimeField(matches[1]); ok {
					currentCommentTime = t
				} else {
					currentCommentTime = time.Time{}
				}
				hasCurrentComment = true
				currentCommentText.Reset()
				continue
			}

			// Accumulate comment text
			if hasCurrentComment {
				if currentCommentText.Len() > 0 {
					currentCommentText.WriteString("\n")
				}
				currentCommentText.WriteString(line)
			}
		} else {
			// Normal description content
			cleanDesc.WriteString(line)
			cleanDesc.WriteString("\n")
		}
	}

	// Update description with cleaned version (without comments block)
	task.Description = strings.TrimSpace(cleanDesc.String())
}

// WriteComments generates the comments block for a task.
func WriteComments(task *models.Task) string {
	if len(task.Comments) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("<!-- COMMENTS -->\n")

	for _, comment := range task.Comments {
		b.WriteString(fmt.Sprintf("<!-- comment: %s -->\n", comment.CreatedAt.Format(util.DateTimeFormat)))
		// Escape HTML comment markers to prevent parser corruption
		escapedText := escapeCommentText(comment.Text)
		b.WriteString(escapedText)
		b.WriteString("\n")
	}

	b.WriteString("<!-- /COMMENTS -->\n")
	return b.String()
}

// escapeCommentText escapes HTML comment markers in comment text
// to prevent parser corruption when the text contains <!-- or -->.
func escapeCommentText(text string) string {
	text = strings.ReplaceAll(text, "<!--", "&lt;!--")
	text = strings.ReplaceAll(text, "-->", "--&gt;")
	return text
}

// unescapeCommentText reverses the escaping done by escapeCommentText.
func unescapeCommentText(text string) string {
	text = strings.ReplaceAll(text, "&lt;!--", "<!--")
	text = strings.ReplaceAll(text, "--&gt;", "-->")
	return text
}
