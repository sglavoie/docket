package format

import (
	"regexp"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// Matches description start marker "<!-- DESCRIPTION -->"
var descriptionStartRegex = regexp.MustCompile(`^<!--\s*DESCRIPTION\s*-->$`)

// Matches description end marker "<!-- /DESCRIPTION -->"
var descriptionEndRegex = regexp.MustCompile(`^<!--\s*/DESCRIPTION\s*-->$`)

// ExtractDescriptionBlock parses the DESCRIPTION block from task description.
// If no DESCRIPTION marker is present the description is left unchanged (backward
// compatibility with old-format files that have no markers). When the marker is
// present, the content inside the block replaces task.Description and any content
// found outside the block is reported as a warning and appended to prevent data loss.
func ExtractDescriptionBlock(task *models.Task) []ParseWarning {
	if task.Description == "" {
		return nil
	}

	// Backward compatibility: if no start marker present, leave description as-is.
	hasMarker := false
	for _, line := range strings.Split(task.Description, "\n") {
		if descriptionStartRegex.MatchString(strings.TrimSpace(line)) {
			hasMarker = true
			break
		}
	}
	if !hasMarker {
		return nil
	}

	blockLines, cleanDesc := ExtractBlock(task.Description, descriptionStartRegex, descriptionEndRegex)

	var origLines []string
	for _, bl := range blockLines {
		origLines = append(origLines, bl.Original)
	}
	task.Description = strings.TrimSpace(strings.Join(origLines, "\n"))

	var warnings []ParseWarning
	if strings.TrimSpace(cleanDesc) != "" {
		warnings = append(warnings, ParseWarning{
			Type:    WarningValidation,
			TaskID:  task.ID,
			Field:   "description",
			Message: "orphaned content found outside DESCRIPTION block: " + cleanDesc,
		})
		// Append orphaned content to description to prevent data loss.
		if task.Description != "" {
			task.Description += "\n" + cleanDesc
		} else {
			task.Description = cleanDesc
		}
	}

	return warnings
}
