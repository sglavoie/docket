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

	editorInput           textinput.Model
	contextInput          textinput.Model
	undoDepthInput        textinput.Model
	dueSoonInput          textinput.Model
	futureInput           textinput.Model
	oldDaysInput          textinput.Model
	completedHoursInput   textinput.Model
	recentActivityInput   textinput.Model
	linkedTaskJump        bool // Whether to jump to new task after linked creation
	recentActivityShowAll bool // Whether to show completed tasks in recent activity
	contexts              []string
	contextCursor         int // -1 for custom mode, 0+ for selecting from list

	focusIndex    int // 0=editor, 1=context, 2=undoDepth, 3=dueSoon, 4=future, 5=oldDays, 6=completedHours, 7=recentActivityLimit, 8=recentActivityShowAll, 9=linkedTaskJump
	validationErr string
	scrollOffset  int

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

	dueSoonInput := textinput.New()
	dueSoonInput.Placeholder = "7"
	dueSoonInput.CharLimit = 3
	dueSoonInput.Width = 6
	dueSoonInput.SetValue(strconv.Itoa(cfg.GetReviewDueSoonDays()))

	futureInput := textinput.New()
	futureInput.Placeholder = "30"
	futureInput.CharLimit = 3
	futureInput.Width = 6
	futureInput.SetValue(strconv.Itoa(cfg.GetReviewFutureDays()))

	oldDaysInput := textinput.New()
	oldDaysInput.Placeholder = "14"
	oldDaysInput.CharLimit = 3
	oldDaysInput.Width = 6
	oldDaysInput.SetValue(strconv.Itoa(cfg.GetReviewOldDays()))

	completedHoursInput := textinput.New()
	completedHoursInput.Placeholder = "24"
	completedHoursInput.CharLimit = 3
	completedHoursInput.Width = 6
	completedHoursInput.SetValue(strconv.Itoa(cfg.GetReviewCompletedHours()))

	recentActivityInput := textinput.New()
	recentActivityInput.Placeholder = "20"
	recentActivityInput.CharLimit = 3
	recentActivityInput.Width = 6
	recentActivityInput.SetValue(strconv.Itoa(cfg.GetRecentActivityLimit()))

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
		config:                cfg,
		configManager:         manager,
		editorInput:           editorInput,
		contextInput:          contextInput,
		undoDepthInput:        undoDepthInput,
		dueSoonInput:          dueSoonInput,
		futureInput:           futureInput,
		oldDaysInput:          oldDaysInput,
		completedHoursInput:   completedHoursInput,
		recentActivityInput:   recentActivityInput,
		linkedTaskJump:        cfg.GetLinkedTaskJumpToNew(),
		recentActivityShowAll: cfg.GetRecentActivityShowAll(),
		contexts:              contexts,
		contextCursor:         contextCursor,
		focusIndex:            0,
		width:                 80,
		height:                24,
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
			m.blurCurrentField()

			// Move focus (10 fields total)
			if msg.String() == "tab" {
				m.focusIndex = (m.focusIndex + 1) % 10
			} else {
				m.focusIndex = (m.focusIndex + 9) % 10 // +9 is -1 mod 10
			}

			// Focus new field
			m.focusCurrentField()
			return m, nil

		case " ":
			// Toggle fields
			if m.focusIndex == 8 {
				m.recentActivityShowAll = !m.recentActivityShowAll
			}
			if m.focusIndex == 9 {
				m.linkedTaskJump = !m.linkedTaskJump
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
			if err := m.validateReviewSettings(); err != nil {
				m.validationErr = err.Error()
				return m, nil
			}
			if err := m.validateRecentActivityLimit(); err != nil {
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
	case 3:
		m.dueSoonInput, cmd = m.dueSoonInput.Update(msg)
	case 4:
		m.futureInput, cmd = m.futureInput.Update(msg)
	case 5:
		m.oldDaysInput, cmd = m.oldDaysInput.Update(msg)
	case 6:
		m.completedHoursInput, cmd = m.completedHoursInput.Update(msg)
	case 7:
		m.recentActivityInput, cmd = m.recentActivityInput.Update(msg)
	}

	// Validate on each keystroke
	m.validationErr = ""
	switch m.focusIndex {
	case 0:
		if err := m.validateEditor(); err != nil {
			m.validationErr = err.Error()
		}
	case 1:
		if err := m.validateContext(); err != nil {
			m.validationErr = err.Error()
		}
	case 2:
		if err := m.validateUndoDepth(); err != nil {
			m.validationErr = err.Error()
		}
	case 3, 4, 5, 6:
		if err := m.validateReviewSettings(); err != nil {
			m.validationErr = err.Error()
		}
	case 7:
		if err := m.validateRecentActivityLimit(); err != nil {
			m.validationErr = err.Error()
		}
	}

	return m, cmd
}

// blurCurrentField removes focus from the current field.
func (m *SettingsModel) blurCurrentField() {
	switch m.focusIndex {
	case 0:
		m.editorInput.Blur()
	case 1:
		m.contextInput.Blur()
	case 2:
		m.undoDepthInput.Blur()
	case 3:
		m.dueSoonInput.Blur()
	case 4:
		m.futureInput.Blur()
	case 5:
		m.oldDaysInput.Blur()
	case 6:
		m.completedHoursInput.Blur()
	case 7:
		m.recentActivityInput.Blur()
	case 8, 9:
		// Toggle fields - no blur needed
	}
}

// focusCurrentField sets focus on the current field.
func (m *SettingsModel) focusCurrentField() {
	switch m.focusIndex {
	case 0:
		m.editorInput.Focus()
	case 1:
		m.contextInput.Focus()
	case 2:
		m.undoDepthInput.Focus()
	case 3:
		m.dueSoonInput.Focus()
	case 4:
		m.futureInput.Focus()
	case 5:
		m.oldDaysInput.Focus()
	case 6:
		m.completedHoursInput.Focus()
	case 7:
		m.recentActivityInput.Focus()
	case 8, 9:
		// Toggle fields - no focus needed
	}
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

func (m *SettingsModel) validateReviewSettings() error {
	// Validate Due Soon Days
	dueSoonStr := strings.TrimSpace(m.dueSoonInput.Value())
	if dueSoonStr == "" {
		return fmt.Errorf("due soon days is required")
	}
	dueSoon, err := strconv.Atoi(dueSoonStr)
	if err != nil {
		return fmt.Errorf("due soon days must be a number")
	}
	if dueSoon < 0 {
		return fmt.Errorf("due soon days must be 0 or greater")
	}

	// Validate Future Days
	futureStr := strings.TrimSpace(m.futureInput.Value())
	if futureStr == "" {
		return fmt.Errorf("future days is required")
	}
	futureDays, err := strconv.Atoi(futureStr)
	if err != nil {
		return fmt.Errorf("future days must be a number")
	}
	if futureDays < 0 {
		return fmt.Errorf("future days must be 0 or greater")
	}

	// Future must be > Due Soon when both are non-zero
	if dueSoon > 0 && futureDays > 0 && futureDays <= dueSoon {
		return fmt.Errorf("future must be greater than due soon")
	}

	// Validate Old Days
	oldDaysStr := strings.TrimSpace(m.oldDaysInput.Value())
	if oldDaysStr == "" {
		return fmt.Errorf("old days is required")
	}
	oldDays, err := strconv.Atoi(oldDaysStr)
	if err != nil {
		return fmt.Errorf("old days must be a number")
	}
	if oldDays < 0 {
		return fmt.Errorf("old days must be 0 or greater")
	}

	// Validate Completed Hours
	completedHoursStr := strings.TrimSpace(m.completedHoursInput.Value())
	if completedHoursStr == "" {
		return fmt.Errorf("completed hours is required")
	}
	completedHours, err := strconv.Atoi(completedHoursStr)
	if err != nil {
		return fmt.Errorf("completed hours must be a number")
	}
	if completedHours < 0 {
		return fmt.Errorf("completed hours must be 0 or greater")
	}

	return nil
}

func (m *SettingsModel) validateRecentActivityLimit() error {
	value := strings.TrimSpace(m.recentActivityInput.Value())
	if value == "" {
		return fmt.Errorf("recent activity limit is required")
	}
	limit, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("must be a number")
	}
	if limit < 0 {
		return fmt.Errorf("must be 0 or greater")
	}
	return nil
}

func (m *SettingsModel) saveSettings() tea.Cmd {
	return func() tea.Msg {
		editor := strings.TrimSpace(m.editorInput.Value())
		context := strings.TrimSpace(m.contextInput.Value())
		undoDepth, _ := strconv.Atoi(strings.TrimSpace(m.undoDepthInput.Value()))
		dueSoon, _ := strconv.Atoi(strings.TrimSpace(m.dueSoonInput.Value()))
		futureDays, _ := strconv.Atoi(strings.TrimSpace(m.futureInput.Value()))
		oldDays, _ := strconv.Atoi(strings.TrimSpace(m.oldDaysInput.Value()))
		completedHours, _ := strconv.Atoi(strings.TrimSpace(m.completedHoursInput.Value()))

		m.config.Editor = editor
		m.config.DefaultContext = context
		m.config.UndoHistoryDepth = &undoDepth
		m.config.ReviewDueSoonDays = &dueSoon
		m.config.ReviewFutureDays = &futureDays
		m.config.ReviewOldDays = &oldDays
		m.config.ReviewCompletedHours = &completedHours
		recentActivityLimit, _ := strconv.Atoi(strings.TrimSpace(m.recentActivityInput.Value()))
		m.config.RecentActivityLimit = &recentActivityLimit
		m.config.RecentActivityShowAll = &m.recentActivityShowAll
		m.config.LinkedTaskJumpToNew = &m.linkedTaskJump

		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save settings: %w", err)}
		}

		return SettingsSavedMsg{}
	}
}

func (m *SettingsModel) View() string {
	var b strings.Builder

	// Editor input
	var editorLabel string
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
	var contextLabel string
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
	var undoLabel string
	if m.focusIndex == 2 {
		undoLabel = FieldLabelFocusedStyle.Render("Undo History Depth: ")
	} else {
		undoLabel = FieldLabelStyle.Render("Undo History Depth: ")
	}
	b.WriteString(undoLabel)
	b.WriteString(m.undoDepthInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0=disable, max 1000)"))
	b.WriteString("\n")

	// Validation error for undo depth
	if m.focusIndex == 2 && m.validationErr != "" {
		b.WriteString("                    ")
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Review Settings section header
	b.WriteString(SubtitleStyle.Render("Review View Thresholds"))
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0 = hide section)"))
	b.WriteString("\n\n")

	// Due Soon Days
	var dueSoonLabel string
	if m.focusIndex == 3 {
		dueSoonLabel = FieldLabelFocusedStyle.Render("Due Soon (days): ")
	} else {
		dueSoonLabel = FieldLabelStyle.Render("Due Soon (days): ")
	}
	b.WriteString(dueSoonLabel)
	b.WriteString(m.dueSoonInput.View())
	b.WriteString("\n")

	// Future Days
	var futureLabel string
	if m.focusIndex == 4 {
		futureLabel = FieldLabelFocusedStyle.Render("Future (days): ")
	} else {
		futureLabel = FieldLabelStyle.Render("Future (days): ")
	}
	b.WriteString(futureLabel)
	b.WriteString(m.futureInput.View())
	b.WriteString("\n")

	// Old Days
	var oldDaysLabel string
	if m.focusIndex == 5 {
		oldDaysLabel = FieldLabelFocusedStyle.Render("Old (days): ")
	} else {
		oldDaysLabel = FieldLabelStyle.Render("Old (days): ")
	}
	b.WriteString(oldDaysLabel)
	b.WriteString(m.oldDaysInput.View())
	b.WriteString("       ")
	b.WriteString(MutedStyle.Render("(based on last update)"))
	b.WriteString("\n")

	// Completed Hours
	var completedLabel string
	if m.focusIndex == 6 {
		completedLabel = FieldLabelFocusedStyle.Render("Completed (hours): ")
	} else {
		completedLabel = FieldLabelStyle.Render("Completed (hours): ")
	}
	b.WriteString(completedLabel)
	b.WriteString(m.completedHoursInput.View())
	b.WriteString("\n")

	// Validation error for review settings
	if m.focusIndex >= 3 && m.focusIndex <= 6 && m.validationErr != "" {
		b.WriteString("                   ")
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Recent Activity section header
	b.WriteString(SubtitleStyle.Render("Recent Activity View"))
	b.WriteString("\n\n")

	// Recent Activity Limit
	var recentLimitLabel string
	if m.focusIndex == 7 {
		recentLimitLabel = FieldLabelFocusedStyle.Render("Task limit: ")
	} else {
		recentLimitLabel = FieldLabelStyle.Render("Task limit: ")
	}
	b.WriteString(recentLimitLabel)
	b.WriteString(m.recentActivityInput.View())
	b.WriteString("  ")
	b.WriteString(MutedStyle.Render("(0 = no limit)"))
	b.WriteString("\n")

	if m.focusIndex == 7 && m.validationErr != "" {
		b.WriteString("            ")
		b.WriteString(ValidationErrorStyle.Render(m.validationErr))
		b.WriteString("\n")
	}

	// Recent Activity Show All Toggle
	var recentShowLabel string
	if m.focusIndex == 8 {
		recentShowLabel = FieldLabelFocusedStyle.Render("Show completed: ")
	} else {
		recentShowLabel = FieldLabelStyle.Render("Show completed: ")
	}
	b.WriteString(recentShowLabel)
	if m.recentActivityShowAll {
		b.WriteString(BoldStyle.Foreground(ColorPrimary).Render("[All tasks]"))
		b.WriteString(MutedStyle.Render("  Active only"))
	} else {
		b.WriteString(MutedStyle.Render("All tasks  "))
		b.WriteString(BoldStyle.Foreground(ColorPrimary).Render("[Active only]"))
	}
	if m.focusIndex == 8 {
		b.WriteString(HelpStyle.Render("  (Space to toggle)"))
	}
	b.WriteString("\n")

	b.WriteString("\n")

	// Linked Task section header
	b.WriteString(SubtitleStyle.Render("Linked Task Creation"))
	b.WriteString("\n\n")

	// Linked Task Jump Toggle
	var linkedLabel string
	if m.focusIndex == 9 {
		linkedLabel = FieldLabelFocusedStyle.Render("After creation: ")
	} else {
		linkedLabel = FieldLabelStyle.Render("After creation: ")
	}
	b.WriteString(linkedLabel)
	if m.linkedTaskJump {
		b.WriteString(BoldStyle.Foreground(ColorPrimary).Render("[Jump to new]"))
		b.WriteString(MutedStyle.Render("  Stay on source"))
	} else {
		b.WriteString(MutedStyle.Render("Jump to new  "))
		b.WriteString(BoldStyle.Foreground(ColorPrimary).Render("[Stay on source]"))
	}
	if m.focusIndex == 9 {
		b.WriteString(HelpStyle.Render("  (Space to toggle)"))
	}
	b.WriteString("\n")

	// Help footer
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("Tab: fields · ←/→: context · Space: toggle · Enter: save · Esc: cancel"))

	cardWidth := 70
	if m.width-4 < cardWidth {
		cardWidth = m.width - 4
	}

	// Apply scrolling when content exceeds available height.
	// Card border (2) + title (1) + top/bottom padding (2) + centering overhead ~ 8 lines reserved.
	maxVisible := m.height - 8
	if maxVisible < 5 {
		maxVisible = 5
	}

	allLines := strings.Split(b.String(), "\n")
	if len(allLines) > maxVisible {
		// Compute line ranges per focus index so we can scroll to keep focused field visible.
		// Each field occupies roughly: label line + optional validation + blank separator.
		// Map focusIndex to approximate first line of that field group.
		fieldStartLine := []int{
			0,  // 0: editor
			3,  // 1: context
			8,  // 2: undo depth
			12, // 3: due soon (after section header)
			14, // 4: future
			16, // 5: old days
			18, // 6: completed hours
			23, // 7: recent activity limit (after section header)
			26, // 8: recent activity show all
			30, // 9: linked task jump (after section header)
		}
		targetLine := 0
		if m.focusIndex >= 0 && m.focusIndex < len(fieldStartLine) {
			targetLine = fieldStartLine[m.focusIndex]
		}

		// Adjust scroll offset to keep target visible
		if targetLine < m.scrollOffset {
			m.scrollOffset = targetLine
		}
		if targetLine >= m.scrollOffset+maxVisible-2 {
			m.scrollOffset = targetLine - maxVisible + 3
		}
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
		if m.scrollOffset > len(allLines)-maxVisible {
			m.scrollOffset = len(allLines) - maxVisible
		}

		visible := allLines[m.scrollOffset:]
		if len(visible) > maxVisible {
			visible = visible[:maxVisible]
		}

		// Add scroll indicators
		var out strings.Builder
		if m.scrollOffset > 0 {
			out.WriteString(MutedStyle.Render("↑ more above"))
			out.WriteString("\n")
		}
		out.WriteString(strings.Join(visible, "\n"))
		if m.scrollOffset+maxVisible < len(allLines) {
			out.WriteString("\n")
			out.WriteString(MutedStyle.Render("↓ more below"))
		}

		cfg := OverlayCardConfig{
			Title:        "Settings",
			Width:        cardWidth,
			Centered:     true,
			NoTruncation: true,
		}
		return renderOverlayCard(out.String(), m.width, m.height, cfg)
	}

	cfg := OverlayCardConfig{
		Title:        "Settings",
		Width:        cardWidth,
		Centered:     true,
		NoTruncation: true,
	}
	return renderOverlayCard(b.String(), m.width, m.height, cfg)
}
