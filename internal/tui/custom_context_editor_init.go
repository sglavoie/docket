package tui

import (
	"maps"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
)

// NewCustomContextEditorModel creates a new custom context editor.
// If editName is empty, creates a new context. Otherwise edits existing.
func NewCustomContextEditorModel(cfg *config.Config, editName string) *CustomContextEditorModel {
	factory := DefaultFormFieldFactory()

	nameInput := factory.NewTextInput("Custom context name")
	nameInput.Focus()
	nameInput.CharLimit = 50

	projects := cfg.GetActiveProjects()
	selectedIDs := make(map[string]bool)

	editMode := editName != ""
	if editMode {
		nameInput.SetValue(editName)
		// Pre-select projects that are in the custom context
		existingIDs := cfg.GetCustomContext(editName)
		for _, id := range existingIDs {
			selectedIDs[id] = true
		}
	}

	initialSelectedIDs := make(map[string]bool, len(selectedIDs))
	maps.Copy(initialSelectedIDs, selectedIDs)

	m := &CustomContextEditorModel{
		config:             cfg,
		editMode:           editMode,
		originalName:       editName,
		nameInput:          nameInput,
		projects:           projects,
		filteredProjects:   projects,
		selectedIDs:        selectedIDs,
		cursor:             0,
		initialSelectedIDs: initialSelectedIDs,
	}
	m.BaseSearchableView.SetSize(80, 24)
	m.InitSearchInput("Search projects...", 50, 80)
	return m
}

// SetSize sets the view dimensions.
func (m *CustomContextEditorModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
	formFactory.ResizeTextInput(&m.nameInput, width)
}

func (m *CustomContextEditorModel) Init() tea.Cmd {
	return textinput.Blink
}
