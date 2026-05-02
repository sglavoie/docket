package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
)

// GlobalTagManagerModel is the model for the global tag management view.
type GlobalTagManagerModel struct {
	config *config.Config
	tags   []GlobalTagInfo
	cursor int
	width  int
	height int

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

// NewGlobalTagManagerModel creates a new global tag manager model.
func NewGlobalTagManagerModel(cfg *config.Config) *GlobalTagManagerModel {
	renameInput := textinput.New()
	renameInput.Placeholder = "new tag name"
	renameInput.CharLimit = 50
	renameInput.SetWidth(30)
	applyThemedTextInput(&renameInput)

	m := &GlobalTagManagerModel{
		config:      cfg,
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
