package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// addSection adds a new section to the project.
func (m *SectionManagerModel) addSection(name string) tea.Cmd {
	proj := m.getProject()
	cfg := m.config
	return func() tea.Msg {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil
		}

		if err := models.ValidateSectionName(name); err != nil {
			return ErrorMsg{Err: err}
		}

		if models.IsGTDSection(name) {
			return ErrorMsg{Err: fmt.Errorf("'%s' is a built-in GTD section", name)}
		}

		if proj.HasSection(name) {
			return ErrorMsg{Err: fmt.Errorf("section '%s' already exists", name)}
		}

		proj.EnsureSection(name)
		if globalColor := cfg.GetGlobalSectionColor(name); globalColor != "gray" {
			proj.SetSectionColor(name, globalColor)
		}

		if err := saveProjectSync(proj); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return SectionAddedMsg{Name: name}
	}
}

// renameSection renames a section.
func (m *SectionManagerModel) renameSection(oldName, newName string) tea.Cmd {
	proj := m.getProject()
	cfg := m.config
	return func() tea.Msg {
		newName = strings.TrimSpace(newName)
		if newName == "" || newName == oldName {
			return nil
		}

		if err := models.ValidateSectionName(newName); err != nil {
			return ErrorMsg{Err: err}
		}

		if models.IsGTDSection(oldName) {
			return ErrorMsg{Err: fmt.Errorf("cannot rename GTD section '%s'", oldName)}
		}

		if models.IsGTDSection(newName) {
			return ErrorMsg{Err: fmt.Errorf("'%s' is a built-in GTD section", newName)}
		}

		if proj.HasSection(newName) {
			return ErrorMsg{Err: fmt.Errorf("section '%s' already exists", newName)}
		}

		if err := proj.RenameSection(oldName, newName); err != nil {
			return ErrorMsg{Err: err}
		}

		if globalColor := cfg.GetGlobalSectionColor(newName); globalColor != "gray" {
			proj.SetSectionColor(newName, globalColor)
		}

		if err := saveProjectSync(proj); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return SectionRenamedMsg{OldName: oldName, NewName: newName}
	}
}

// removeSection removes a section from the project.
func (m *SectionManagerModel) removeSection(name string) tea.Cmd {
	proj := m.getProject()
	return func() tea.Msg {
		if models.IsGTDSection(name) {
			return ErrorMsg{Err: fmt.Errorf("cannot remove GTD section '%s'", name)}
		}

		if !proj.RemoveSection(name) {
			return ErrorMsg{Err: fmt.Errorf("failed to remove section")}
		}

		if err := saveProjectSync(proj); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return SectionRemovedMsg{Name: name}
	}
}

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
