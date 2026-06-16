package cmd

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

func normalizeTemplateSectionInput(section string) (string, error) {
	section = strings.TrimSpace(section)
	if section == "" {
		return "", nil
	}
	if !models.IsGTDSection(section) {
		return "", fmt.Errorf("template section %q is not a built-in GTD section; use tags for flexible grouping", section)
	}
	return section, nil
}
