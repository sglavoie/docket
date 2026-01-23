package storage

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

const scratchpadFileName = "scratchpad.md"

// DefaultScratchpadTemplate is the initial content for new scratchpad files.
const DefaultScratchpadTemplate = "# Scratchpad\n\nDump your ideas and notes here.\n\n"

// LoadScratchpad reads the scratchpad content for a project.
// Returns empty string if the scratchpad file doesn't exist.
func LoadScratchpad(project *models.Project) (string, error) {
	scratchpadPath := filepath.Join(project.Path, scratchpadFileName)

	content, err := os.ReadFile(scratchpadPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// SaveScratchpad writes the scratchpad content for a project.
func SaveScratchpad(project *models.Project, content string) error {
	if err := os.MkdirAll(project.Path, 0755); err != nil {
		return err
	}

	scratchpadPath := filepath.Join(project.Path, scratchpadFileName)
	return os.WriteFile(scratchpadPath, []byte(content), 0644)
}

// ScratchpadExists checks if a project has a scratchpad with user content.
// Returns false if the scratchpad doesn't exist or contains only the default template.
func ScratchpadExists(project *models.Project) bool {
	content, err := LoadScratchpad(project)
	if err != nil || content == "" {
		return false
	}
	return strings.TrimSpace(content) != strings.TrimSpace(DefaultScratchpadTemplate)
}

// EnsureScratchpadFile creates the scratchpad file with default template if it doesn't exist.
// Returns the path to the scratchpad file.
func EnsureScratchpadFile(project *models.Project) (string, error) {
	scratchpadPath := ScratchpadPath(project)
	if _, err := os.Stat(scratchpadPath); os.IsNotExist(err) {
		if err := os.MkdirAll(project.Path, 0755); err != nil {
			return "", err
		}
		if err := os.WriteFile(scratchpadPath, []byte(DefaultScratchpadTemplate), 0644); err != nil {
			return "", err
		}
	}
	return scratchpadPath, nil
}

// ScratchpadPath returns the path to the scratchpad file for a project.
func ScratchpadPath(project *models.Project) string {
	return filepath.Join(project.Path, scratchpadFileName)
}
