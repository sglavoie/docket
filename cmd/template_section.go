package cmd

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// templateStateFromLegacySectionInput is the temporary CLI adapter retained
// until the public template flags cut over to --state. It validates the legacy
// section spelling and immediately returns a canonical workflow state.
func templateStateFromLegacySectionInput(section string, emptyState models.WorkflowState) (models.WorkflowState, error) {
	section = strings.TrimSpace(section)
	if section == "" {
		return emptyState, nil
	}
	if !models.IsGTDSection(section) {
		return "", fmt.Errorf("template section %q is not a built-in GTD section; use tags for flexible grouping", section)
	}
	return models.StateForSection(section), nil
}
