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
	contextInput  textinput.Model
	contexts      []string
	contextCursor int // -1 for custom mode, 0+ for selecting from list

	focusIndex    int // 0=editor, 1=context
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

	contextInput := textinput.New()
	contextInput.Placeholder = "personal"
	contextInput.CharLimit = 50
	contextInput.Width = 30
	contextInput.SetValue(cfg.DefaultContext)

	// Get all existing contexts
	contexts := cfg.GetAllContexts()

	// Find current context in list (or -1 for custom)
	contextCursor := -1
	for i, ctx := range contexts {
		if ctx == cfg.DefaultContext {
			contextCursor = i
			break
		}
	}

	return &SettingsModel{
		config:        cfg,
		configManager: manager,
		editorInput:   editorInput,
		contextInput:  contextInput,
		contexts:      contexts,
		contextCursor: contextCursor,
		focusIndex:    0,
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

		case "tab", "shift+tab":
			// Blur current field
			if m.focusIndex == 0 {
				m.editorInput.Blur()
			} else {
				m.contextInput.Blur()
			}

			// Move focus
			if msg.String() == "tab" {
				m.focusIndex = (m.focusIndex + 1) % 2
			} else {
				m.focusIndex = (m.focusIndex + 1) % 2
			}

			// Focus new field
			if m.focusIndex == 0 {
				m.editorInput.Focus()
			} else {
				m.contextInput.Focus()
			}
			return m, nil

		case "enter":
			if err := m.validateEditor(); err != nil {
				m.validationErr = err.Error()
				return m, nil
			}
			if err := m.validateContext(); err != nil {
				m.validationErr = err.Error()
				return m, nil
			}
			return m, m.saveSettings()

		case "left", "h":
			// Cycle through contexts when on context field
			if m.focusIndex == 1 && len(m.contexts) > 0 {
				if m.contextCursor > 0 {
					m.contextCursor--
					m.contextInput.SetValue(m.contexts[m.contextCursor])
				} else if m.contextCursor == -1 {
					// From custom mode, go to last context
					m.contextCursor = len(m.contexts) - 1
					m.contextInput.SetValue(m.contexts[m.contextCursor])
				}
			}

		case "right", "l":
			// Cycle through contexts when on context field
			if m.focusIndex == 1 && len(m.contexts) > 0 {
				if m.contextCursor >= 0 && m.contextCursor < len(m.contexts)-1 {
					m.contextCursor++
					m.contextInput.SetValue(m.contexts[m.contextCursor])
				}
			}
		}
	}

	// Update the focused input
	if m.focusIndex == 0 {
		m.editorInput, cmd = m.editorInput.Update(msg)
	} else {
		oldValue := m.contextInput.Value()
		m.contextInput, cmd = m.contextInput.Update(msg)
		// If user typed something, switch to custom mode
		if m.contextInput.Value() != oldValue {
			m.contextCursor = -1
			// Check if typed value matches an existing context
			for i, ctx := range m.contexts {
				if ctx == m.contextInput.Value() {
					m.contextCursor = i
					break
				}
			}
		}
	}

	// Validate on each keystroke
	if m.focusIndex == 0 {
		if err := m.validateEditor(); err != nil {
			m.validationErr = err.Error()
		} else {
			m.validationErr = ""
		}
	} else {
		if err := m.validateContext(); err != nil {
			m.validationErr = err.Error()
		} else {
			m.validationErr = ""
		}
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

func (m *SettingsModel) validateContext() error {
	context := strings.TrimSpace(m.contextInput.Value())
	if context == "" {
		return fmt.Errorf("default context is required")
	}
	return nil
}

func (m *SettingsModel) saveSettings() tea.Cmd {
	return func() tea.Msg {
		editor := strings.TrimSpace(m.editorInput.Value())
		context := strings.TrimSpace(m.contextInput.Value())

		m.config.Editor = editor
		m.config.DefaultContext = context

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
	editorLabel := "Editor: "
	if m.focusIndex == 0 {
		editorLabel = BoldStyle.Foreground(ColorPrimary).Render("Editor: ")
	} else {
		editorLabel = MutedStyle.Render("Editor: ")
	}
	b.WriteString(editorLabel)
	b.WriteString(m.editorInput.View())
	b.WriteString("\n")

	// Validation error for editor
	if m.focusIndex == 0 && m.validationErr != "" {
		errStyle := lipgloss.NewStyle().Foreground(ColorError)
		b.WriteString("        ")
		b.WriteString(errStyle.Render(m.validationErr))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Default context input
	contextLabel := "Default Context: "
	if m.focusIndex == 1 {
		contextLabel = BoldStyle.Foreground(ColorPrimary).Render("Default Context: ")
	} else {
		contextLabel = MutedStyle.Render("Default Context: ")
	}
	b.WriteString(contextLabel)
	b.WriteString(m.contextInput.View())
	b.WriteString("\n")

	// Show available contexts
	if len(m.contexts) > 0 {
		b.WriteString("                 ")
		b.WriteString(MutedStyle.Render("Available: "))
		for i, ctx := range m.contexts {
			if i == m.contextCursor {
				b.WriteString(BoldStyle.Foreground(ColorPrimary).Render(fmt.Sprintf("[%s]", ctx)))
			} else {
				b.WriteString(MutedStyle.Render(ctx))
			}
			if i < len(m.contexts)-1 {
				b.WriteString(MutedStyle.Render(", "))
			}
		}
		if m.focusIndex == 1 {
			b.WriteString(HelpStyle.Render("  (←/→ to select, or type custom)"))
		}
	}
	b.WriteString("\n")

	// Validation error for context
	if m.focusIndex == 1 && m.validationErr != "" {
		errStyle := lipgloss.NewStyle().Foreground(ColorError)
		b.WriteString("                 ")
		b.WriteString(errStyle.Render(m.validationErr))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Help
	help := lipgloss.JoinVertical(lipgloss.Left,
		BoldStyle.Render("Controls"),
		"  Tab        Switch field",
		"  ←/→        Select context",
		"  Enter      Save",
		"  Esc        Cancel",
	)
	b.WriteString(BoxStyle.Render(help))

	return b.String()
}
