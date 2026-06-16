package tui

import (
	"os"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// minimalOutliner returns a bare OutlinerModel suitable for unit tests that
// call cross-project row builders directly without touching disk-backed state.
func minimalOutliner() *OutlinerModel {
	return &OutlinerModel{
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
	}
}

// makeTask creates a minimal task with the given title and section.
func makeTask(id, title, section string) *models.Task {
	return &models.Task{
		ID:      id,
		Title:   title,
		Section: section,
	}
}

func setOutlinerCustomSectionTabByName(t *testing.T, m *OutlinerModel, name string) {
	t.Helper()
	for i, tab := range m.customSectionTabs() {
		if tab == name {
			m.setCustomSectionTabIndex(i)
			return
		}
	}
	t.Fatalf("custom section tab %q not found in %v", name, m.customSectionTabs())
}

// readCrossTestFile reads a file's contents for integration test assertions.
func readCrossTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
