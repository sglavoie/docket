package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/tt/internal/config"
)

// SettingsModel handles the settings view in the TUI.
type SettingsModel struct {
	config        *config.Config
	configManager *config.Manager

	editorInput    textinput.Model
	contextInput   textinput.Model
	undoDepthInput textinput.Model
	contexts       []string
	contextCursor  int // -1 for custom mode, 0+ for selecting from list

	focusIndex    int // 0=editor, 1=context, 2=undoDepth
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

	undoDepthInput := textinput.New()
	undoDepthInput.Placeholder = "50"
	undoDepthInput.CharLimit = 4
	undoDepthInput.Width = 10
	undoDepthValue := 50
	if cfg.UndoHistoryDepth != nil {
		undoDepthValue = *cfg.UndoHistoryDepth
	}
	undoDepthInput.SetValue(strconv.Itoa(undoDepthValue))

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
		config:         cfg,
		configManager:  manager,
		editorInput:    editorInput,
		contextInput:   contextInput,
		undoDepthInput: undoDepthInput,
		contexts:       contexts,
		contextCursor:  contextCursor,
		focusIndex:     0,
		width:          80,
		height:         24,
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
			switch m.focusIndex {
			case 0:
				m.editorInput.Blur()
			case 1:
				m.contextInput.Blur()
			case 2:
				m.undoDepthInput.Blur()
			}

			// Move focus
			if msg.String() == "tab" {
				m.focusIndex = (m.focusIndex + 1) % 3
			} else {
				m.focusIndex = (m.focusIndex + 2) % 3 // +2 is -1 mod 3
			}

			// Focus new field
			switch m.focusIndex {
			case 0:
				m.editorInput.Focus()
			case 1:
				m.contextInput.Focus()
			case 2:
				m.undoDepthInput.Focus()
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
			if err := m.validateUndoDepth(); err != nil {
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
	switch m.focusIndex {
	case 0:
		m.editorInput, cmd = m.editorInput.Update(msg)
	case 1:
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
	case 2:
		m.undoDepthInput, cmd = m.undoDepthInput.Update(msg)
	}

	// Validate on each keystroke
	switch m.focusIndex {
	case 0:
		if err := m.validateEditor(); err != nil {
			m.validationErr = err.Error()
		} else {
			m.validationErr = ""
		}
	case 1:
		if err := m.validateContext(); err != nil {
			m.validationErr = err.Error()
		} else {
			m.validationErr = ""
		}
	case 2:
		if err := m.validateUndoDepth(); err != nil {
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

func (m *SettingsModel) validateUndoDepth() error {
	value := strings.TrimSpace(m.undoDepthInput.Value())
	if value == "" {
		return fmt.Errorf("undo history depth is required")
	}

	depth, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("must be a number")
	}

	if depth < 0 {
		return fmt.Errorf("must be 0 or greater")
	}

	if depth > 1000 {
		return fmt.Errorf("maximum is 1000")
	}

	return nil
}

func (m *SettingsModel) saveSettings() tea.Cmd {
	return func() tea.Msg {
		editor := strings.TrimSpace(m.editorInput.Value())
		context := strings.TrimSpace(m.contextInput.Value())
		undoDepth, _ := strconv.Atoi(strings.TrimSpace(m.undoDepthInput.Value()))

		m.config.Editor = editor
		m.config.DefaultContext = context
		m.config.UndoHistoryDepth = &undoDepth

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save settings: %w", err)}
		}

		return SettingsSavedMsg{}
	}
}

func (m *SettingsModel) View() string {
	var b strings.Builder

	// Editor input
	editorLabel := "Editor: "
	if m.focusIndex == 0 {
		editorLabel = FieldLabelFocusedStyle.Render("Editor: ")
	} else {
		editorLabel = FieldLabelStyle.Render("Editor: ")
	}
	b.WriteString(editorLabel)
	b.WriteString(m.editorInput.View())
	b.WriteString("\n")

	// Validation error for editor
	if m.focusIndex == 0 && m.validationErr != "" {
		b.WriteString("        ")
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Default context input
	contextLabel := "Default Context: "
	if m.focusIndex == 1 {
		contextLabel = FieldLabelFocusedStyle.Render("Default Context: ")
	} else {
		contextLabel = FieldLabelStyle.Render("Default Context: ")
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
		b.WriteString("                 ")
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Undo history depth input
	undoLabel := "Undo History Depth: "
	if m.focusIndex == 2 {
		undoLabel = FieldLabelFocusedStyle.Render("Undo History Depth: ")
	} else {
		undoLabel = FieldLabelStyle.Render("Undo History Depth: ")
	}
	b.WriteString(undoLabel)
	b.WriteString(m.undoDepthInput.View())
	b.WriteString("\n")
	b.WriteString("                    ")
	b.WriteString(MutedStyle.Render("(0 to disable, max 1000)"))
	b.WriteString("\n")

	// Validation error for undo depth
	if m.focusIndex == 2 && m.validationErr != "" {
		b.WriteString("                    ")
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}

	// Help footer
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("Tab: fields · ←/→: context · Enter: save · Esc: cancel"))

	cardWidth := 70
	if m.width-4 < cardWidth {
		cardWidth = m.width - 4
	}
	config := OverlayCardConfig{
		Title:        "Settings",
		Width:        cardWidth,
		Centered:     true,
		NoTruncation: true,
	}
	return renderOverlayCard(b.String(), m.width, m.height, config)
}
