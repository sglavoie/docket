package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// parseTaskContent extracts the title and description from edited content.
func parseTaskContent(content string, task *models.Task) (title, description string) {
	lines := strings.Split(content, "\n")
	var descLines []string
	title = task.Title // Default to original title
	inDescription := false
	foundComment := false

	for _, line := range lines {
		// Already in description section
		if inDescription {
			descLines = append(descLines, line)
			continue
		}

		// Look for title line (first # line)
		if len(line) >= 2 && line[0] == '#' && line[1] == ' ' {
			title = strings.TrimSpace(line[2:])
			continue
		}

		// Look for the HTML comment marker
		if len(line) >= 4 && line[:4] == "<!--" {
			foundComment = true
			continue
		}

		// After comment, skip empty lines until description starts
		if foundComment && line == "" {
			continue
		}

		// Start of description content
		if foundComment {
			inDescription = true
			descLines = append(descLines, line)
		}
	}

	// Trim trailing empty lines
	for len(descLines) > 0 && descLines[len(descLines)-1] == "" {
		descLines = descLines[:len(descLines)-1]
	}

	// If title was cleared, keep the original
	if title == "" {
		title = task.Title
	}

	return title, strings.Join(descLines, "\n")
}
