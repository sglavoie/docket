package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// SectionManagerModel is the model for the section management view.
type SectionManagerModel struct {
	project *models.Project
	// runtime backs the domain-native global section-color commit (Settings).
	runtime       *config.RuntimeState
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
	Project      *models.Project
	StartAddMode bool // Deprecated: section creation is no longer supported.
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

// NewSectionManagerModel creates a new section manager model for a project.
func NewSectionManagerModel(proj *models.Project, runtime *config.RuntimeState, _ bool) *SectionManagerModel {
	m := &SectionManagerModel{
		project: proj,
		runtime: runtime,
		cursor:  0,
	}
	m.buildSectionList()
	return m
}

// getProject returns the section manager's backing project.
func (m *SectionManagerModel) getProject() *models.Project {
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
