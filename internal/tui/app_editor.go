package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// parseTaskContent extracts the title and description from edited content.
// It extracts the title from the first "# " line and treats all remaining
// non-comment lines as description, regardless of their position relative
// to the comment marker.
func parseTaskContent(content string, task *models.Task) (title, description string) {
	lines := strings.Split(content, "\n")
	var descLines []string
	title = task.Title // Default to original title

	for _, line := range lines {
		// Extract title from the first "# " heading
		if len(line) >= 2 && line[0] == '#' && line[1] == ' ' {
			title = strings.TrimSpace(line[2:])
			continue
		}

		// Skip the instruction comment line
		if len(line) >= 4 && line[:4] == "<!--" {
			continue
		}

		descLines = append(descLines, line)
	}

	// Trim leading and trailing empty lines
	for len(descLines) > 0 && descLines[0] == "" {
		descLines = descLines[1:]
	}
	for len(descLines) > 0 && descLines[len(descLines)-1] == "" {
		descLines = descLines[:len(descLines)-1]
	}

	// If title was cleared, keep the original
	if title == "" {
		title = task.Title
	}

	return title, strings.Join(descLines, "\n")
}
