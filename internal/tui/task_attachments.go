package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TaskAttachmentsModel displays and manages attachments for a task.
type TaskAttachmentsModel struct {
	task    *models.Task
	project *models.Project

	// Navigation state
	cursor int // Selected attachment index

	// Add mode
	addMode   bool
	textInput textinput.Model

	// Confirmation state (actions: "delete", "unlink")
	confirm ConfirmDialog

	// Dimensions
	width  int
	height int

	// Error/status display
	err           error
	statusMessage string
}

// NewTaskAttachmentsModel creates a new task attachments viewer.
func NewTaskAttachmentsModel(task *models.Task, project *models.Project) *TaskAttachmentsModel {
	ti := textinput.New()
	ti.Placeholder = "Enter file path or URL..."
	ti.CharLimit = 500
	ti.Width = 60

	return &TaskAttachmentsModel{
		task:      task,
		project:   project,
		cursor:    0,
		addMode:   false,
		textInput: ti,
	}
}

func (m *TaskAttachmentsModel) Init() tea.Cmd {
	return nil
}

// SetSize updates the view dimensions.
func (m *TaskAttachmentsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.textInput.Width = width - 4
}

func (m *TaskAttachmentsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle confirmation mode
	if m.confirm.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			action := m.confirm.Action()
			if confirmed, handled := m.confirm.HandleKey(msg.String()); handled {
				if confirmed {
					switch action {
					case "delete":
						return m, m.executeDelete()
					case "unlink":
						return m, m.executeKeep()
					}
				}
				m.err = nil
				return m, nil
			}
		}
		return m, nil
	}

	// Handle add mode
	if m.addMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.addMode = false
				m.textInput.Reset()
				return m, nil
			case "enter":
				return m, m.addAttachment()
			}
		}
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	// Normal mode
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return CloseTaskAttachmentsMsg{} }

		case "j", "down":
			if len(m.task.Attachments) > 0 && m.cursor < len(m.task.Attachments)-1 {
				m.cursor++
			}

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}

		case "g":
			m.cursor = 0

		case "G":
			if len(m.task.Attachments) > 0 {
				m.cursor = len(m.task.Attachments) - 1
			}

		case "a":
			// Enter add mode
			m.addMode = true
			m.textInput.Focus()
			return m, textinput.Blink

		case "o", "enter":
			// Open selected attachment
			if len(m.task.Attachments) > 0 && m.cursor < len(m.task.Attachments) {
				return m, m.openAttachment()
			}

		case "d", "x":
			// Request delete confirmation
			if len(m.task.Attachments) > 0 && m.cursor < len(m.task.Attachments) {
				m.confirm.Show("delete", fmt.Sprintf("Delete attachment [%d]?", m.cursor))
				return m, nil
			}

		case "u":
			// Request unlink (keep file) confirmation
			if len(m.task.Attachments) > 0 && m.cursor < len(m.task.Attachments) {
				m.confirm.Show("unlink", fmt.Sprintf("Unlink attachment [%d]?", m.cursor))
				return m, nil
			}

		case "y":
			// Copy path to clipboard
			if len(m.task.Attachments) > 0 && m.cursor < len(m.task.Attachments) {
				return m, m.copyPath()
			}

		case "ctrl+o":
			// Open original path reference
			if len(m.task.Attachments) > 0 && m.cursor < len(m.task.Attachments) {
				return m, m.openOriginalPath()
			}

		case "O":
			// Open attachments directory in file manager
			return m, m.openAttachmentsDir()

		case "?":
			return m, func() tea.Msg {
				return ShowHelpMsg{
					SourceView: ViewTaskAttachments,
					ViewName:   "Attachments",
					Items:      m.GetHelpItems(),
				}
			}
		}
	}

	return m, nil
}

func (m *TaskAttachmentsModel) addAttachment() tea.Cmd {
	return func() tea.Msg {
		input := strings.TrimSpace(m.textInput.Value())
		if input == "" {
			m.err = fmt.Errorf("path/URL cannot be empty")
			return nil
		}

		// Detect if it's a URL
		isURL := strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://")

		if isURL {
			// Add URL attachment
			m.task.AddAttachment(input, true, input)
		} else {
			// Handle file path
			path := input

			// Expand ~ to home directory
			if strings.HasPrefix(path, "~/") {
				home, err := os.UserHomeDir()
				if err != nil {
					m.err = fmt.Errorf("failed to expand home directory: %w", err)
					return nil
				}
				path = filepath.Join(home, path[2:])
			}

			// Convert to absolute path
			absPath, err := filepath.Abs(path)
			if err != nil {
				m.err = fmt.Errorf("failed to resolve path: %w", err)
				return nil
			}

			// Check if file exists
			if _, statErr := os.Stat(absPath); os.IsNotExist(statErr) {
				m.err = fmt.Errorf("file not found: %s", absPath)
				return nil
			}

			// Copy file to attachments directory
			filename, err := storage.CopyAttachment(m.project, m.task, absPath)
			if err != nil {
				m.err = fmt.Errorf("failed to copy: %w", err)
				return nil
			}

			m.task.AddAttachment(filename, false, absPath)
		}

		// Create backup before saving
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Reset add mode
		m.addMode = false
		m.textInput.Reset()
		m.err = nil

		// Move cursor to the new attachment (last in list)
		m.cursor = len(m.task.Attachments) - 1

		return AttachmentAddedMsg{Task: m.task}
	}
}

func (m *TaskAttachmentsModel) executeDelete() tea.Cmd {
	return func() tea.Msg {
		att := m.task.Attachments[m.cursor]

		// Remove from task metadata
		removed, ok := m.task.RemoveAttachment(m.cursor)
		if !ok {
			return ErrorMsg{Err: fmt.Errorf("failed to remove attachment")}
		}

		// Delete file from disk if not a URL
		if !att.IsURL {
			if err := storage.RemoveAttachmentFile(m.project, m.task, removed.Filename); err != nil {
				// Log warning but don't fail
				m.statusMessage = fmt.Sprintf("Warning: file not deleted: %v", err)
			}
		}

		// Create backup before saving
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Adjust cursor if needed
		if m.cursor >= len(m.task.Attachments) && m.cursor > 0 {
			m.cursor--
		}

		return AttachmentDeletedMsg{Task: m.task}
	}
}

func (m *TaskAttachmentsModel) executeKeep() tea.Cmd {
	return func() tea.Msg {
		// Remove from task metadata only (keep file on disk)
		_, ok := m.task.RemoveAttachment(m.cursor)
		if !ok {
			return ErrorMsg{Err: fmt.Errorf("failed to unlink attachment")}
		}

		// Create backup before saving
		if err := storage.BackupProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}

		if err := storage.SaveProject(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Adjust cursor if needed
		if m.cursor >= len(m.task.Attachments) && m.cursor > 0 {
			m.cursor--
		}

		return AttachmentDeletedMsg{Task: m.task}
	}
}

func (m *TaskAttachmentsModel) openAttachment() tea.Cmd {
	att := m.task.Attachments[m.cursor]

	var target string
	if att.IsURL {
		target = att.Filename
	} else {
		target = storage.GetAttachmentPath(m.project, m.task, att.Filename)
		if !storage.AttachmentExists(m.project, m.task, att.Filename) {
			m.err = fmt.Errorf("file not found: %s", att.Filename)
			return nil
		}
	}

	// Build the open command
	var openCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		openCmd = exec.Command("open", target)
	case "linux":
		openCmd = exec.Command("xdg-open", target)
	case "windows":
		openCmd = exec.Command("cmd", "/c", "start", "", target)
	default:
		m.err = fmt.Errorf("unsupported OS: %s", runtime.GOOS)
		return nil
	}

	return tea.ExecProcess(openCmd, func(err error) tea.Msg {
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to open: %w", err)}
		}
		return nil
	})
}

func (m *TaskAttachmentsModel) openAttachmentsDir() tea.Cmd {
	dir := storage.GetTaskAttachmentsDir(m.project, m.task)

	// Create directory if it doesn't exist
	if err := storage.EnsureTaskAttachmentsDir(m.project, m.task); err != nil {
		m.err = fmt.Errorf("failed to create directory: %w", err)
		return nil
	}

	var openCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		openCmd = exec.Command("open", dir)
	case "linux":
		openCmd = exec.Command("xdg-open", dir)
	case "windows":
		openCmd = exec.Command("explorer", dir)
	default:
		m.err = fmt.Errorf("unsupported OS: %s", runtime.GOOS)
		return nil
	}

	return tea.ExecProcess(openCmd, func(err error) tea.Msg {
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to open directory: %w", err)}
		}
		return nil
	})
}

func (m *TaskAttachmentsModel) copyPath() tea.Cmd {
	att := m.task.Attachments[m.cursor]

	var path string
	if att.IsURL {
		path = att.Filename
	} else {
		path = storage.GetAttachmentPath(m.project, m.task, att.Filename)
	}

	// Build clipboard command based on OS
	var clipCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		clipCmd = exec.Command("pbcopy")
	case "linux":
		clipCmd = exec.Command("xclip", "-selection", "clipboard")
	case "windows":
		clipCmd = exec.Command("clip")
	default:
		m.statusMessage = "Clipboard not supported on this OS"
		return nil
	}

	clipCmd.Stdin = strings.NewReader(path)
	if err := clipCmd.Run(); err != nil {
		m.err = fmt.Errorf("failed to copy to clipboard: %w", err)
		return nil
	}

	m.statusMessage = "Path copied to clipboard"
	return nil
}

func (m *TaskAttachmentsModel) openOriginalPath() tea.Cmd {
	att := m.task.Attachments[m.cursor]

	// For URLs, copy to clipboard
	if att.IsURL {
		var clipCmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			clipCmd = exec.Command("pbcopy")
		case "linux":
			clipCmd = exec.Command("xclip", "-selection", "clipboard")
		case "windows":
			clipCmd = exec.Command("clip")
		default:
			m.statusMessage = "Clipboard not supported on this OS"
			return nil
		}

		clipCmd.Stdin = strings.NewReader(att.OriginalPath)
		if err := clipCmd.Run(); err != nil {
			m.err = fmt.Errorf("failed to copy URL to clipboard: %w", err)
			return nil
		}

		m.statusMessage = "URL copied to clipboard"
		return nil
	}

	// For files, open the directory containing the original file
	dir := filepath.Dir(att.OriginalPath)

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		m.err = fmt.Errorf("original directory not found: %s", dir)
		return nil
	}

	var openCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		openCmd = exec.Command("open", dir)
	case "linux":
		openCmd = exec.Command("xdg-open", dir)
	case "windows":
		openCmd = exec.Command("explorer", dir)
	default:
		m.err = fmt.Errorf("unsupported OS: %s", runtime.GOOS)
		return nil
	}

	return tea.ExecProcess(openCmd, func(err error) tea.Msg {
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to open directory: %w", err)}
		}
		return nil
	})
}

func (m *TaskAttachmentsModel) View() string {
	var b strings.Builder

	// Title
	b.WriteString(renderFullscreenTitle(fmt.Sprintf("Attachments for #%d: %s", m.task.ShortID, m.task.Title), fmt.Sprintf("%d attachment(s)", len(m.task.Attachments))))
	b.WriteString("\n\n")

	// Add mode input
	if m.addMode {
		b.WriteString(FieldLabelFocusedStyle.Render("Add Attachment:"))
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("Enter a file path or URL"))
		b.WriteString("\n\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Enter: add | Esc: cancel"))
		b.WriteString("\n")
		return b.String()
	}

	// Delete confirmation
	if m.confirm.Action() == "delete" && len(m.task.Attachments) > 0 && m.cursor < len(m.task.Attachments) {
		att := m.task.Attachments[m.cursor]
		name := att.Filename
		if len(name) > 50 {
			name = name[:47] + "..."
		}

		warningStyle := lipgloss.NewStyle().Foreground(ColorError).Bold(true)
		b.WriteString(warningStyle.Render(fmt.Sprintf("Delete attachment [%d]?", m.cursor)))
		b.WriteString("\n\n")
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  \"%s\"", name)))
		if !att.IsURL {
			b.WriteString("\n")
			b.WriteString(MutedStyle.Render("  (copy will be deleted from .attachments/)"))
		}
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("y: confirm delete | n/Esc: cancel"))
		b.WriteString("\n")
		return b.String()
	}

	// Keep-file (unlink) confirmation
	if m.confirm.Action() == "unlink" && len(m.task.Attachments) > 0 && m.cursor < len(m.task.Attachments) {
		att := m.task.Attachments[m.cursor]
		name := att.Filename
		if len(name) > 50 {
			name = name[:47] + "..."
		}

		warningStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
		b.WriteString(warningStyle.Render(fmt.Sprintf("Unlink attachment [%d]?", m.cursor)))
		b.WriteString("\n\n")
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  \"%s\"", name)))
		if !att.IsURL {
			b.WriteString("\n")
			b.WriteString(MutedStyle.Render("  (copy stays in .attachments/)"))
		}
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("y: confirm unlink | n/Esc: cancel"))
		b.WriteString("\n")
		return b.String()
	}

	// Error/status display
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	}
	if m.statusMessage != "" {
		b.WriteString(MutedStyle.Render(m.statusMessage))
		b.WriteString("\n\n")
	}

	// Attachments list
	if len(m.task.Attachments) == 0 {
		b.WriteString(MutedStyle.Render("No attachments yet. Press 'a' to add one."))
		b.WriteString("\n")
	} else {
		// Calculate visible attachments based on height
		headerLines := 5 // title+subtitle, spacing, help
		availableHeight := m.height - headerLines
		maxVisible := availableHeight / 3 // ~3 lines per attachment
		if maxVisible < 3 {
			maxVisible = 3
		}

		// Calculate scroll offset
		scrollOffset := 0
		if m.cursor >= maxVisible {
			scrollOffset = m.cursor - maxVisible + 1
		}

		for i := scrollOffset; i < len(m.task.Attachments) && i < scrollOffset+maxVisible; i++ {
			att := m.task.Attachments[i]

			// Cursor indicator
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			// Build attachment entry
			var entry strings.Builder
			entry.WriteString(cursor)

			// Type indicator
			typeStr := "FILE"
			if att.IsURL {
				typeStr = "URL "
			}

			// Style based on selection
			if i == m.cursor {
				entry.WriteString(BoldStyle.Render(fmt.Sprintf("[%d] ", i)))
				entry.WriteString(FieldLabelFocusedStyle.Render(typeStr))
			} else {
				entry.WriteString(MutedStyle.Render(fmt.Sprintf("[%d] ", i)))
				entry.WriteString(MutedStyle.Render(typeStr))
			}
			entry.WriteString("\n")

			// Filename/URL (indented)
			name := att.Filename
			maxLen := m.width - 8
			if maxLen < 40 {
				maxLen = 40
			}
			if len(name) > maxLen {
				name = name[:maxLen-3] + "..."
			}
			entry.WriteString("      ")
			entry.WriteString(name)
			entry.WriteString("\n")

			// Original path (only for files, not URLs)
			if !att.IsURL {
				origPath := att.OriginalPath
				if len(origPath) > maxLen {
					origPath = origPath[:maxLen-3] + "..."
				}
				entry.WriteString("      ")
				entry.WriteString(MutedStyle.Render("from: " + origPath))
				entry.WriteString("\n")
			}

			// Size and date (for files)
			if !att.IsURL {
				size := storage.GetAttachmentSize(m.project, m.task, &att)
				sizeStr := "(missing)"
				if size > 0 {
					sizeStr = storage.FormatFileSize(size)
				}
				entry.WriteString("      ")
				entry.WriteString(MutedStyle.Render(fmt.Sprintf("%s | %s", sizeStr, storage.FormatRelativeTime(att.AddedAt))))
				entry.WriteString("\n")
			} else {
				entry.WriteString("      ")
				entry.WriteString(MutedStyle.Render(storage.FormatRelativeTime(att.AddedAt)))
				entry.WriteString("\n")
			}

			b.WriteString(entry.String())
		}

		// Scroll indicator
		if len(m.task.Attachments) > maxVisible {
			remaining := len(m.task.Attachments) - scrollOffset - maxVisible
			if remaining > 0 {
				b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more (use j/k to scroll)", remaining)))
				b.WriteString("\n")
			}
		}
	}

	b.WriteString(renderStatusBarFooter(HelpStyle.Render("a: add | o: open | O: open dir | Ctrl+O: original | d: del | u: unlink | y: copy | ?: help | Esc: close"), "", m.width))

	return b.String()
}

// HasTextInput returns true when add mode is active.
func (m *TaskAttachmentsModel) HasTextInput() bool {
	return m.addMode
}

// ShouldCaptureEsc returns true when in add, delete, or unlink confirmation modes.
func (m *TaskAttachmentsModel) ShouldCaptureEsc() bool {
	return m.confirm.IsActive() || m.addMode
}

// GetHelpItems returns help items for the attachments view.
func (m *TaskAttachmentsModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewTaskAttachments)
}
