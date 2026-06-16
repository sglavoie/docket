package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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

// setSectionColor sets the color for the current section globally and propagates to all projects.
func (m *SectionManagerModel) setSectionColor(sectionName, colorName string) tea.Cmd {
	proj := m.getProject()
	m.config.SetGlobalSectionColor(sectionName, colorName)
	cfg := m.config
	mgr := m.configManager
	projects := make([]*models.Project, len(cfg.Projects))
	copy(projects, cfg.Projects)

	return func() tea.Msg {
		if err := mgr.Save(cfg); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save config: %w", err)}
		}

		for _, p := range projects {
			if err := storage.LoadProject(p); err != nil {
				continue
			}

			if p.HasSection(sectionName) {
				p.SetSectionColor(sectionName, colorName)
				if err := storage.SaveProjectLocked(p); err != nil {
					continue
				}
			}
		}

		proj.SetSectionColor(sectionName, colorName)
		if err := saveProjectSync(proj); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return SectionColorChangedMsg{SectionName: sectionName, ColorName: colorName}
	}
}
