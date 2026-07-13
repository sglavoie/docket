package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
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
// markdown save is performed here. Open in-memory projects are updated directly
// so views reflect the change immediately without a reload.
func (m *SectionManagerModel) setSectionColor(sectionName, colorName string) tea.Cmd {
	m.config.SetGlobalSectionColor(sectionName, colorName)
	for _, p := range m.config.Projects {
		p.SetSectionColor(sectionName, colorName)
	}
	if proj := m.getProject(); proj != nil {
		proj.SetSectionColor(sectionName, colorName)
	}
	cfg := m.config
	mgr := m.configManager

	return func() tea.Msg {
		if err := mgr.Save(cfg); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save config: %w", err)}
		}
		return SectionColorChangedMsg{SectionName: sectionName, ColorName: colorName}
	}
}
