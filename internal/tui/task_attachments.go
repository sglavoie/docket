package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TaskAttachmentsModel displays and manages attachments for a task.
type TaskAttachmentsModel struct {
	BaseView
	InteractiveViewState
	task    *models.Task
	project *models.Project
	config  *config.Config

	// Add mode
	addMode   bool
	textInput textinput.Model
}

// NewTaskAttachmentsModel creates a new task attachments viewer.
func NewTaskAttachmentsModel(task *models.Task, project *models.Project, cfg *config.Config) *TaskAttachmentsModel {
	ti := textinput.New()
	ti.Placeholder = "Enter file path, URL, or code ref (file:line)..."
	ti.CharLimit = 500
	ti.SetWidth(60)

	return &TaskAttachmentsModel{
		task:      task,
		project:   project,
		config:    cfg,
		textInput: ti,
	}
}

// resolveTask ensures m.task points to the task currently in m.project.Tasks.
func (m *TaskAttachmentsModel) resolveTask() {
	if t := m.project.GetTaskByID(m.task.ID); t != nil {
		m.task = t
	}
}

func (m *TaskAttachmentsModel) Init() tea.Cmd {
	return nil
}

// SetSize updates the view dimensions.
func (m *TaskAttachmentsModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
	m.textInput.SetWidth(width - 4)
}

func (m *TaskAttachmentsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle confirmation mode
	if m.confirm.IsActive() {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			action := m.confirm.Action()
			if confirmed, handled := m.HandleConfirmKey(msg.String()); handled {
				if confirmed {
					switch action {
					case "delete":
						return m, m.executeDelete()
					case "unlink":
						return m, m.executeKeep()
					case "discard-add":
						m.addMode = false
						m.textInput.Reset()
						return m, nil
					}
				}
				m.ClearError()
				return m, nil
			}
		}
		return m, nil
	}

	// Handle add mode
	if m.addMode {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch msg.String() {
			case "esc":
				if strings.TrimSpace(m.textInput.Value()) != "" {
					m.confirm.Show("discard-add", "Discard attachment?")
					return m, nil
				}
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
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.WindowSizeMsg:
		m.BaseView.SetSize(msg.Width, msg.Height)

	case tea.KeyPressMsg:
		if m.pendingPrefix == "g" {
			m.pendingPrefix = ""
			_, cmd := handleGotoTopPrefix(msg.String(), &m.cursor, &m.cursorFlash)
			return m, cmd
		}

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
			m.pendingPrefix = "g"
			return m, nil

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
			// Open selected attachment with system default
			if len(m.task.Attachments) > 0 && m.cursor < len(m.task.Attachments) {
				return m, m.openAttachmentDefault()
			}

		case "e":
			// Open selected attachment in editor
			if len(m.task.Attachments) > 0 && m.cursor < len(m.task.Attachments) {
				return m, m.openAttachmentInEditor()
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

		case "Y":
			// Copy path with line number to clipboard
			if len(m.task.Attachments) > 0 && m.cursor < len(m.task.Attachments) {
				return m, m.copyPathWithLine()
			}

		case "ctrl+e":
			// Open original path reference
			if len(m.task.Attachments) > 0 && m.cursor < len(m.task.Attachments) {
				return m, m.openOriginalPath()
			}

		case "ctrl+o":
			// Reveal original file in file manager
			if len(m.task.Attachments) > 0 && m.cursor < len(m.task.Attachments) {
				return m, m.revealOriginalFile()
			}

		case "O":
			// Open attachments directory in file manager
			return m, m.openAttachmentsDir()

		case "?":
			return m, helpCmd(ViewTaskAttachments, "Attachments", m.GetHelpItems())
		}
	}

	return m, nil
}

func (m *TaskAttachmentsModel) addAttachment() tea.Cmd {
	return func() tea.Msg {
		input := strings.TrimSpace(m.textInput.Value())
		// Strip surrounding single quotes (e.g. from drag-and-drop paths with spaces)
		if len(input) >= 2 && input[0] == '\'' && input[len(input)-1] == '\'' {
			input = input[1 : len(input)-1]
		}
		if input == "" {
			m.err = fmt.Errorf("path/URL cannot be empty")
			return nil
		}

		m.resolveTask()

		// Detect if it's a URL
		isURL := strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://")

		if isURL {
			// Add URL attachment
			if err := m.task.AddAttachment(input, true, input); err != nil {
				m.err = err
				return nil
			}
		} else {
			// Check if it's a code reference (file:line format)
			codePath, lineNum, isCodeRef := storage.ParseCodeReference(input)
			if isCodeRef {
				// Store code reference without copying
				if err := m.task.AddCodeReference(codePath, lineNum); err != nil {
					m.err = err
					return nil
				}

				// Check if file exists and show info message if not
				if !storage.CodeReferenceFileExists(codePath) {
					m.setStatus("Code reference added (file may not exist)")
				}
			} else {
				// Handle regular file path
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

				if err := m.task.AddAttachment(filename, false, absPath); err != nil {
					m.err = err
					return nil
				}
			}
		}

		if err := saveProjectNoReload(m.project); err != nil {
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
		m.resolveTask()
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
				m.setStatus(fmt.Sprintf("Warning: file not deleted: %v", err))
			}
		}

		if err := saveProjectNoReload(m.project); err != nil {
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
		m.resolveTask()
		_, ok := m.task.RemoveAttachment(m.cursor)
		if !ok {
			return ErrorMsg{Err: fmt.Errorf("failed to unlink attachment")}
		}

		if err := saveProjectNoReload(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		// Adjust cursor if needed
		if m.cursor >= len(m.task.Attachments) && m.cursor > 0 {
			m.cursor--
		}

		return AttachmentDeletedMsg{Task: m.task}
	}
}

func (m *TaskAttachmentsModel) openAttachmentDefault() tea.Cmd {
	att := m.task.Attachments[m.cursor]

	// Handle code references — open in code editor since that's the sensible default
	if att.IsCodeRef {
		if !storage.CodeReferenceFileExists(att.OriginalPath) {
			m.err = fmt.Errorf("code reference file not found: %s", att.OriginalPath)
			return nil
		}
		codeEditor := m.config.GetCodeEditor()
		if err := external.OpenCodeReference(codeEditor, att.OriginalPath, att.LineNumber); err != nil {
			m.err = fmt.Errorf("failed to open code reference: %w", err)
			return nil
		}
		m.setStatus(fmt.Sprintf("Opened %s:%d", filepath.Base(att.OriginalPath), att.LineNumber))
		return nil
	}

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

	// Open with OS default
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

func (m *TaskAttachmentsModel) openAttachmentInEditor() tea.Cmd {
	att := m.task.Attachments[m.cursor]

	// Handle code references — open in code editor (same as default for code refs)
	if att.IsCodeRef {
		if !storage.CodeReferenceFileExists(att.OriginalPath) {
			m.err = fmt.Errorf("code reference file not found: %s", att.OriginalPath)
			return nil
		}
		codeEditor := m.config.GetCodeEditor()
		if err := external.OpenCodeReference(codeEditor, att.OriginalPath, att.LineNumber); err != nil {
			m.err = fmt.Errorf("failed to open code reference: %w", err)
			return nil
		}
		m.setStatus(fmt.Sprintf("Opened %s:%d", filepath.Base(att.OriginalPath), att.LineNumber))
		return nil
	}

	// URLs can't be edited
	if att.IsURL {
		m.err = fmt.Errorf("cannot edit a URL in an editor")
		return nil
	}

	target := storage.GetAttachmentPath(m.project, m.task, att.Filename)
	if !storage.AttachmentExists(m.project, m.task, att.Filename) {
		m.err = fmt.Errorf("file not found: %s", att.Filename)
		return nil
	}

	// Use configured attachment editor, fall back to $EDITOR, then default
	editor, editorErr := external.ResolveEditor(m.config.GetAttachmentEditor())
	if editorErr != nil {
		return errorCmd("%s", editorErr)
	}

	openCmd := exec.Command(editor, target)
	return tea.ExecProcess(openCmd, func(err error) tea.Msg {
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to open in editor: %w", err)}
		}
		return nil
	})
}

func (m *TaskAttachmentsModel) openAttachmentsDir() tea.Cmd {
	var dir string

	// For code references, open the directory containing the referenced file
	if len(m.task.Attachments) > 0 && m.cursor < len(m.task.Attachments) {
		att := m.task.Attachments[m.cursor]
		if att.IsCodeRef {
			dir = filepath.Dir(att.OriginalPath)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				m.err = fmt.Errorf("directory not found: %s", dir)
				return nil
			}
		}
	}

	// For regular attachments, open the internal .attachments directory
	if dir == "" {
		dir = storage.GetTaskAttachmentsDir(m.project, m.task)
		// Create directory if it doesn't exist
		if err := storage.EnsureTaskAttachmentsDir(m.project, m.task); err != nil {
			m.err = fmt.Errorf("failed to create directory: %w", err)
			return nil
		}
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
	} else if att.IsCodeRef {
		path = att.OriginalPath
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
		m.setStatus("Clipboard not supported on this OS")
		return nil
	}

	clipCmd.Stdin = strings.NewReader(path)
	if err := clipCmd.Run(); err != nil {
		m.err = fmt.Errorf("failed to copy to clipboard: %w", err)
		return nil
	}

	m.setStatus("Path copied to clipboard")
	return nil
}

func (m *TaskAttachmentsModel) copyPathWithLine() tea.Cmd {
	att := m.task.Attachments[m.cursor]

	var path string
	if att.IsCodeRef && att.LineNumber > 0 {
		path = fmt.Sprintf("%s:%d", att.OriginalPath, att.LineNumber)
	} else if att.IsURL {
		path = att.Filename
	} else if att.IsCodeRef {
		path = att.OriginalPath
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
		m.setStatus("Clipboard not supported on this OS")
		return nil
	}

	clipCmd.Stdin = strings.NewReader(path)
	if err := clipCmd.Run(); err != nil {
		m.err = fmt.Errorf("failed to copy to clipboard: %w", err)
		return nil
	}

	if att.IsCodeRef && att.LineNumber > 0 {
		m.setStatus("Path:line copied to clipboard")
	} else {
		m.setStatus("Path copied to clipboard")
	}
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
			m.setStatus("Clipboard not supported on this OS")
			return nil
		}

		clipCmd.Stdin = strings.NewReader(att.OriginalPath)
		if err := clipCmd.Run(); err != nil {
			m.err = fmt.Errorf("failed to copy URL to clipboard: %w", err)
			return nil
		}

		m.setStatus("URL copied to clipboard")
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

func (m *TaskAttachmentsModel) revealOriginalFile() tea.Cmd {
	att := m.task.Attachments[m.cursor]

	if att.IsURL {
		m.err = fmt.Errorf("cannot reveal URL attachments in file manager")
		return nil
	}

	path := att.OriginalPath

	// Check if the original file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		m.err = fmt.Errorf("original file not found: %s", path)
		return nil
	}

	var openCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		openCmd = exec.Command("open", "-R", path)
	case "linux":
		openCmd = exec.Command("xdg-open", filepath.Dir(path))
	case "windows":
		openCmd = exec.Command("explorer", "/select,", path)
	default:
		m.err = fmt.Errorf("unsupported OS: %s", runtime.GOOS)
		return nil
	}

	return tea.ExecProcess(openCmd, func(err error) tea.Msg {
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to reveal file: %w", err)}
		}
		return nil
	})
}

func (m *TaskAttachmentsModel) View() tea.View {
	if m.task == nil {
		return tea.NewView("No task selected")
	}

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
		b.WriteString(HelpStyle.Render("Enter: add · Esc: cancel"))
		b.WriteString("\n")
		return tea.NewView(b.String())
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
		availableHeight := m.Height() - headerLines
		maxVisible := max(
			// ~3 lines per attachment
			availableHeight/3, 3)

		scrollOffset := calcScrollOffset(m.cursor, maxVisible)

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
			} else if att.IsCodeRef {
				typeStr = "CODE"
			}

			// Style based on selection
			if i == m.cursor {
				entry.WriteString(BoldStyle.Render(fmt.Sprintf("[%d] ", i)))
				entry.WriteString(FieldLabelFocusedStyle.Render(typeStr))
			} else {
				entry.WriteString(MutedStyle.Render(fmt.Sprintf("[%d] ", i)))
				entry.WriteString(MutedStyle.Render(typeStr))
			}

			// Show warning icon for missing code reference files
			if att.IsCodeRef && !storage.CodeReferenceFileExists(att.OriginalPath) {
				entry.WriteString(" ")
				entry.WriteString(WarningIconStyle.Render("!"))
			}
			entry.WriteString("\n")

			// Filename/URL (indented) - for code refs, show path:line format
			var name string
			if att.IsCodeRef {
				name = fmt.Sprintf("%s:%d", att.OriginalPath, att.LineNumber)
			} else {
				name = att.Filename
			}
			maxLen := max(m.Width()-8, 40)
			if len(name) > maxLen {
				name = name[:maxLen-3] + "..."
			}
			entry.WriteString("      ")
			entry.WriteString(name)
			entry.WriteString("\n")

			// Original path (only for regular files, not URLs or code refs)
			if !att.IsURL && !att.IsCodeRef {
				origPath := att.OriginalPath
				if len(origPath) > maxLen {
					origPath = origPath[:maxLen-3] + "..."
				}
				entry.WriteString("      ")
				entry.WriteString(MutedStyle.Render("from: " + origPath))
				entry.WriteString("\n")
			}

			// For code refs, show warning if file doesn't exist
			if att.IsCodeRef && !storage.CodeReferenceFileExists(att.OriginalPath) {
				entry.WriteString("      ")
				entry.WriteString(WarningIconStyle.Render("file may not exist"))
				entry.WriteString("\n")
			}

			// Size and date (for regular files only, not URLs or code refs)
			if !att.IsURL && !att.IsCodeRef {
				size := storage.GetAttachmentSize(m.project, m.task, &att)
				sizeStr := "(missing)"
				if size > 0 {
					sizeStr = storage.FormatFileSize(size)
				}
				entry.WriteString("      ")
				entry.WriteString(MutedStyle.Render(fmt.Sprintf("%s | %s", sizeStr, storage.FormatRelativeTime(att.AddedAt))))
				entry.WriteString("\n")
			} else if att.IsURL || att.IsCodeRef {
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

	// Build help text dynamically based on selected attachment type
	isCodeRef := len(m.task.Attachments) > 0 && m.cursor < len(m.task.Attachments) && m.task.Attachments[m.cursor].IsCodeRef

	// Build help segments
	var helpParts []string
	helpParts = append(helpParts, "a: add", "o: open", "e: edit", "O: open dir")
	if !isCodeRef {
		helpParts = append(helpParts, "Ctrl+O: original")
	}
	helpParts = append(helpParts, "d: del", "u: unlink", "y: copy")
	if isCodeRef {
		helpParts = append(helpParts, "Y: copy w/line")
	}
	helpParts = append(helpParts, "?: help", "Esc: close")

	// Join with separators and check if wrapping is needed
	helpText := strings.Join(helpParts, " · ")
	if lipgloss.Width(helpText) > m.Width()-2 {
		// Split into two lines at a reasonable point
		mid := len(helpParts) / 2
		line1 := strings.Join(helpParts[:mid], " · ")
		line2 := strings.Join(helpParts[mid:], " · ")
		b.WriteString(renderStatusBarFooter(HelpStyle.Render(line1), "", m.Width()))
		b.WriteString("\n")
		b.WriteString(HelpStyle.Render(line2))
	} else {
		b.WriteString(renderStatusBarFooter(HelpStyle.Render(helpText), "", m.Width()))
	}

	return tea.NewView(b.String())
}

// HasTextInput returns true when add mode is active.
func (m *TaskAttachmentsModel) HasTextInput() bool {
	return m.addMode
}

// ShouldCaptureEsc returns true when in add, delete, or unlink confirmation modes, or pending prefix.
func (m *TaskAttachmentsModel) ShouldCaptureEsc() bool {
	return m.DefaultShouldCaptureEsc() || m.addMode
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *TaskAttachmentsModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints: PrefixHints{
			"g": NavigationPrefixHints,
		},
	}
}

// GetHelpItems returns help items for the attachments view.
func (m *TaskAttachmentsModel) GetHelpItems() []HelpItem {
	return GetKeybindingRegistry().GetForView(ViewTaskAttachments)
}

func (m *TaskAttachmentsModel) StatusBarLabel() string { return "Attachments" }
