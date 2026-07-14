package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// GlobalTagManagerModel is the model for the global tag management view.
type GlobalTagManagerModel struct {
	projects []*models.Project
	// runtime backs the domain-native global-pinned-tag read/commit (Settings).
	runtime     *config.RuntimeState
	coordinator TagCoordinator
	tags        []GlobalTagInfo
	cursor      int
	width       int
	height      int

	// Rename state
	renameMode   bool
	renameInput  textinput.Model
	renameTarget string
	errorMsg     string

	discardConfirmState

	// Project picker state
	showProjectPicker bool
	projectCursor     int
	selectedTag       string
}

// NewGlobalTagManagerModel creates a new global tag manager model. The
// coordinator backs the workspace-wide tag-rename use case; task and catalog
// mutations still route through the shell.
func NewGlobalTagManagerModel(projects []*models.Project, runtime *config.RuntimeState, coordinator TagCoordinator) *GlobalTagManagerModel {
	renameInput := textinput.New()
	renameInput.Placeholder = "new tag name"
	renameInput.CharLimit = 50
	renameInput.SetWidth(30)
	applyThemedTextInput(&renameInput)

	m := &GlobalTagManagerModel{
		projects:    projects,
		runtime:     runtime,
		coordinator: coordinator,
		cursor:      0,
		renameInput: renameInput,
	}
	m.buildGlobalTagList()
	return m
}

// SetSize sets the view dimensions.
func (m *GlobalTagManagerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Init initializes the model.
func (m *GlobalTagManagerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *GlobalTagManagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case GlobalTagRenamedMsg:
		m.renameMode = false
		m.renameTarget = ""
		m.renameInput.SetValue("")
		return m, func() tea.Msg {
			return CloseGlobalTagManagerMsg{}
		}

	case GlobalTagPinnedMsg:
		m.buildGlobalTagList()
		m.selectTag(msg.Name)
		return m, nil

	case GlobalTagManagerErrorMsg:
		if msg.Err != nil {
			m.errorMsg = msg.Err.Error()
		}
		return m, nil

	case tea.PasteMsg:
		if m.renameMode {
			var cmd tea.Cmd
			m.renameInput, cmd = m.renameInput.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyPressMsg:
		// Handle discard confirmation
		if m.confirmDiscard {
			switch msg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				m.renameMode = false
				m.renameTarget = ""
				m.renameInput.SetValue("")
				return m, nil
			case "n", "N", "esc":
				m.confirmDiscard = false
				return m, nil
			}
			return m, nil
		}

		m.errorMsg = ""

		if m.renameMode {
			return m.handleRenameMode(msg)
		}

		if m.showProjectPicker {
			return m.handleProjectPickerMode(msg)
		}

		return m.handleNormalMode(msg)
	}

	return m, nil
}

func (m *GlobalTagManagerModel) HasTextInput() bool     { return m.renameMode }
func (m *GlobalTagManagerModel) ShouldCaptureEsc() bool { return m.renameMode || m.confirmDiscard }
func (m *GlobalTagManagerModel) IsFullHeight() bool     { return false }

func (m *GlobalTagManagerModel) StatusBarLabel() string { return "Global Tags" }
