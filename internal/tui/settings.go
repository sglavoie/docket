package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/config"
)

// SettingsModel handles the settings view in the TUI.
type SettingsModel struct {
	config        *config.Config
	configManager *config.Manager

	editorInput   textinput.Model
	validationErr string

	width  int
	height int
}

// NewSettingsModel creates a new settings model.
func NewSettingsModel(cfg *config.Config, manager *config.Manager) *SettingsModel {
	editorInput := textinput.New()
	editorInput.Placeholder = "code"
	editorInput.Focus()
	editorInput.CharLimit = 256
	editorInput.Width = 40
	editorInput.SetValue(cfg.Editor)

	return &SettingsModel{
		config:        cfg,
		configManager: manager,
		editorInput:   editorInput,
		width:         80,
		height:        24,
	}
}

// SetSize sets the view dimensions.
func (m *SettingsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *SettingsModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "enter":
			if err := m.validateEditor(); err != nil {
				m.validationErr = err.Error()
				return m, nil
			}
			return m, m.saveSettings()
		}
	}

	m.editorInput, cmd = m.editorInput.Update(msg)

	// Validate on each keystroke
	if err := m.validateEditor(); err != nil {
		m.validationErr = err.Error()
	} else {
		m.validationErr = ""
	}

	return m, cmd
}

func (m *SettingsModel) validateEditor() error {
	editor := strings.TrimSpace(m.editorInput.Value())

	if editor == "" {
		return fmt.Errorf("editor is required")
	}

	// Check if it's an absolute path
	if strings.HasPrefix(editor, "/") {
		info, err := os.Stat(editor)
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", editor)
		}
		if err != nil {
			return fmt.Errorf("cannot access file: %s", editor)
		}
		if info.IsDir() {
			return fmt.Errorf("path is a directory, not an executable: %s", editor)
		}
		// Check if executable
		if info.Mode()&0111 == 0 {
			return fmt.Errorf("file is not executable: %s", editor)
		}
		return nil
	}

	// Otherwise, check if command exists in PATH
	_, err := exec.LookPath(editor)
	if err != nil {
		return fmt.Errorf("command not found in PATH: %s", editor)
	}

	return nil
}

func (m *SettingsModel) saveSettings() tea.Cmd {
	return func() tea.Msg {
		editor := strings.TrimSpace(m.editorInput.Value())
		m.config.Editor = editor

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save settings: %w", err)}
		}

		return SettingsSavedMsg{}
	}
}

func (m *SettingsModel) View() string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("Settings"))
	b.WriteString("\n\n")

	// Editor input
	editorLabel := BoldStyle.Foreground(ColorPrimary).Render("Editor: ")
	b.WriteString(editorLabel)
	b.WriteString(m.editorInput.View())
	b.WriteString("\n")

	// Validation error
	if m.validationErr != "" {
		errStyle := lipgloss.NewStyle().Foreground(ColorError)
		b.WriteString("        ")
		b.WriteString(errStyle.Render(m.validationErr))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Help
	help := lipgloss.JoinVertical(lipgloss.Left,
		BoldStyle.Render("Controls"),
		"  Enter      Save",
		"  Esc        Cancel",
	)
	b.WriteString(BoxStyle.Render(help))

	return b.String()
}
