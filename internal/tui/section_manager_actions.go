package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
)

// filterBySection sets the project tree filter and closes the section manager.
// When opened from outliner view (no projectTree), it just closes without filtering.
func (m *SectionManagerModel) filterBySection(sectionName string) tea.Cmd {
	if m.projectTree != nil {
		m.projectTree.filter.Sections = map[string]bool{sectionName: true}
		m.projectTree.updateVisibility()
		m.projectTree.clampCursor()
	}
	return func() tea.Msg {
		return CloseSectionManagerMsg{}
	}
}

// setSectionColor records a section's color in the global config, which is the
// sole owner of section colors. Projects re-derive their colors from the global
// registry on load (config.ApplyGlobalColorsToProject), so no per-project
// markdown save is performed here. Open in-memory projects are updated by the
// AppModel only after this command returns SectionColorChangedMsg, keeping a
// failed asynchronous commit from leaking tentative colors into live caches.
func (m *SectionManagerModel) setSectionColor(sectionName, colorName string) tea.Cmd {
	// Global section colors live in the Settings domain; commit only that domain
	// under the runtime's checked commit rather than saving the whole config.
	runtime := m.runtime

	return func() tea.Msg {
		if err := commitSettingsToRuntime(runtime, func(s *config.Settings) {
			s.SetGlobalSectionColor(sectionName, colorName)
		}); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save config: %w", err)}
		}
		return SectionColorChangedMsg{SectionName: sectionName, ColorName: colorName}
	}
}
