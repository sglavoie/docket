package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// SectionManagerModel is the model for the section management view.
type SectionManagerModel struct {
	projectTree   *ProjectTreeModel
	project       *models.Project // Used directly when projectTree is nil
	config        *config.Config
	configManager *config.Manager
	sections      []SectionInfo
	cursor        int
	width         int
	height        int
	errorMsg      string
	colorPickMode bool
	colorCursor   int // Index in SectionColorPalette
}

// ShowSectionManagerMsg is sent to open the section manager view.
type ShowSectionManagerMsg struct {
	ProjectTree  *ProjectTreeModel
	Project      *models.Project // Used when ProjectTree is nil (e.g., from outliner view)
	StartAddMode bool            // Deprecated: section creation is no longer supported.
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

// GoToSectionMsg is sent to apply a section filter in the project tree.
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
func NewSectionManagerModel(pt *ProjectTreeModel, proj *models.Project, cfg *config.Config, mgr *config.Manager, _ bool) *SectionManagerModel {
	m := &SectionManagerModel{
		projectTree:   pt,
		project:       proj,
		config:        cfg,
		configManager: mgr,
		cursor:        0,
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

// buildSectionList builds the canonical GTD section list with task counts.
func (m *SectionManagerModel) buildSectionList() {
	proj := m.getProject()
	if proj == nil {
		m.sections = nil
		return
	}
	sectionCounts := make(map[string]int)
	for _, task := range proj.Tasks {
		section := taskSectionBucket(task)
		if section != "" && !models.IsGTDSection(section) {
			section = ""
		}
		sectionCounts[section]++
	}

	order := []string{
		"",
		models.SectionNext,
		models.SectionInbox,
		models.SectionWaiting,
		models.SectionReview,
		models.SectionStandby,
		models.SectionSomeday,
		models.SectionCompleted,
	}
	m.sections = make([]SectionInfo, 0, len(order))
	for _, name := range order {
		m.sections = append(m.sections, SectionInfo{
			Name:      name,
			TaskCount: sectionCounts[name],
			ColorName: proj.GetSectionColor(name),
		})
	}

	// Clamp cursor
	if m.cursor >= len(m.sections) {
		m.cursor = len(m.sections) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}
