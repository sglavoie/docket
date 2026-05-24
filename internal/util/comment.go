package util

import "strings"

// ParseEditedCommentContent extracts the comment text from edited content.
// Used when editing comments via external editor - strips the header template
// and extracts only the user's comment text.
//
// The expected format is:
//
//	# Comment N on #ID: Title
//
//	<!-- Edit the comment text below. The timestamp will be preserved. -->
//
//	Comment text here...
func ParseEditedCommentContent(content string) string {
	lines := strings.Split(content, "\n")
	var textLines []string
	foundComment := false

	for _, line := range lines {
		// Already collecting text
		if foundComment {
			textLines = append(textLines, line)
			continue
		}

		// Skip title line
		if len(line) >= 2 && line[0] == '#' && line[1] == ' ' {
			continue
		}

		// Look for the HTML comment marker
		if len(line) >= 4 && line[:4] == "<!--" {
			foundComment = true
			continue
		}
	}

	// Trim leading and trailing empty lines
	for len(textLines) > 0 && textLines[0] == "" {
		textLines = textLines[1:]
	}
	for len(textLines) > 0 && textLines[len(textLines)-1] == "" {
		textLines = textLines[:len(textLines)-1]
	}

	result := strings.Join(textLines, "\n")
	if strings.TrimSpace(result) == "" {
		return ""
	}
	return result
}
