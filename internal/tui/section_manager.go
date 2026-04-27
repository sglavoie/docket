package tui

import (
	"charm.land/bubbles/v2/textinput"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// SectionManagerModel is the model for the section management view.
type SectionManagerModel struct {
	projectTree    *ProjectTreeModel
	project        *models.Project // Used directly when projectTree is nil
	config         *config.Config
	configManager  *config.Manager
	sections       []SectionInfo
	cursor         int
	width          int
	height         int
	renameMode     bool
	renameInput    textinput.Model
	addMode        bool
	addInput       textinput.Model
	errorMsg       string
	colorPickMode  bool
	colorCursor    int // Index in SectionColorPalette
	confirmAction string
	discardConfirmState
}

// ShowSectionManagerMsg is sent to open the section manager view.
type ShowSectionManagerMsg struct {
	ProjectTree  *ProjectTreeModel
	Project      *models.Project // Used when ProjectTree is nil (e.g., from outliner view)
	StartAddMode bool            // If true, open directly in add mode
}

// CloseSectionManagerMsg is sent when the section manager should close.
type CloseSectionManagerMsg struct{}

// SectionRenamedMsg is sent after a section has been renamed.
type SectionRenamedMsg struct {
	OldName string
	NewName string
}

// SectionAddedMsg is sent after a section has been added.
type SectionAddedMsg struct {
	Name string
}

// SectionRemovedMsg is sent after a section has been removed.
type SectionRemovedMsg struct {
	Name string
}

// GoToSectionMsg is sent to navigate to a section tab in the project tree.
type GoToSectionMsg struct {
	SectionName string
}

// SectionColorChangedMsg is sent after a section's color has been changed.
type SectionColorChangedMsg struct {
	SectionName string
	ColorName   string
}

// NewSectionManagerModel creates a new section manager model.
// Either pt or proj must be provided. If pt is provided, proj is ignored.
func NewSectionManagerModel(pt *ProjectTreeModel, proj *models.Project, cfg *config.Config, mgr *config.Manager, startAddMode bool) *SectionManagerModel {
	renameInput := textinput.New()
	renameInput.Placeholder = "new section name"
	renameInput.CharLimit = 50
	renameInput.SetWidth(30)
	applyThemedTextInput(&renameInput)

	addInput := textinput.New()
	addInput.Placeholder = "section name"
	addInput.CharLimit = 50
	addInput.SetWidth(30)
	applyThemedTextInput(&addInput)

	m := &SectionManagerModel{
		projectTree:   pt,
		project:       proj,
		config:        cfg,
		configManager: mgr,
		cursor:        0,
		renameInput:   renameInput,
		addInput:      addInput,
		addMode:       startAddMode,
	}
	if startAddMode {
		m.addInput.Focus()
	}
	m.buildSectionList()
	return m
}

// getProject returns the project from projectTree if available, otherwise returns the direct project.
func (m *SectionManagerModel) getProject() *models.Project {
	if m.projectTree != nil {
		return m.projectTree.project
	}
	return m.project
}

// SetSize sets the view dimensions.
func (m *SectionManagerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// buildSectionList builds a sorted list of custom sections with counts.
// GTD sections (Next, Inbox, Waiting, Someday) are excluded as they are
// managed via view modes, not the section manager.
func (m *SectionManagerModel) buildSectionList() {
	proj := m.getProject()
	sectionCounts := proj.CountTasksBySection()

	// Get sections in project order, excluding GTD sections
	projectSections := proj.GetSections()

	m.sections = make([]SectionInfo, 0, len(projectSections))
	for _, name := range projectSections {
		if models.IsGTDSection(name) {
			continue // Skip GTD sections
		}
		count := sectionCounts[name]
		m.sections = append(m.sections, SectionInfo{Name: name, TaskCount: count})
	}

	// Clamp cursor
	if m.cursor >= len(m.sections) {
		m.cursor = len(m.sections) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}
