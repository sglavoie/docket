package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *TaskAttachmentsModel) addAttachment() tea.Cmd {
	input := strings.TrimSpace(m.textInput.Value())
	if len(input) >= 2 && input[0] == '\'' && input[len(input)-1] == '\'' {
		input = input[1 : len(input)-1]
	}
	if input == "" {
		m.err = fmt.Errorf("path/URL cannot be empty")
		return nil
	}

	m.resolveTask()

	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return m.addURLAttachment(input)
	}

	codePath, lineNum, isCodeRef := storage.ParseCodeReference(input)
	if isCodeRef {
		return m.addCodeReference(codePath, lineNum)
	}

	return m.addFileAttachment(input)
}

func (m *TaskAttachmentsModel) addURLAttachment(input string) tea.Cmd {
	if err := m.task.AddAttachment(input, true, input); err != nil {
		m.err = err
		return nil
	}

	m.finishSynchronousAdd()
	task := m.task
	project := m.project
	return func() tea.Msg {
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return AttachmentAddedMsg{Task: task}
	}
}

func (m *TaskAttachmentsModel) addCodeReference(codePath string, lineNum int) tea.Cmd {
	if err := m.task.AddCodeReference(codePath, lineNum); err != nil {
		m.err = err
		return nil
	}
	if !storage.CodeReferenceFileExists(codePath) {
		m.setStatus("Code reference added (file may not exist)")
	}

	m.finishSynchronousAdd()
	task := m.task
	project := m.project
	return func() tea.Msg {
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return AttachmentAddedMsg{Task: task}
	}
}

func (m *TaskAttachmentsModel) addFileAttachment(input string) tea.Cmd {
	path := input
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			m.err = fmt.Errorf("failed to expand home directory: %w", err)
			return nil
		}
		path = filepath.Join(home, path[2:])
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		m.err = fmt.Errorf("failed to resolve path: %w", err)
		return nil
	}
	if _, statErr := os.Stat(absPath); os.IsNotExist(statErr) {
		m.err = fmt.Errorf("file not found: %s", absPath)
		return nil
	}

	m.addMode = false
	m.textInput.Reset()
	m.err = nil

	task := m.task
	project := m.project
	return func() tea.Msg {
		filename, copyErr := storage.CopyAttachment(project, task, absPath)
		if copyErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to copy: %w", copyErr)}
		}
		if err := task.AddAttachment(filename, false, absPath); err != nil {
			return ErrorMsg{Err: err}
		}
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return AttachmentAddedMsg{Task: task}
	}
}

func (m *TaskAttachmentsModel) finishSynchronousAdd() {
	m.addMode = false
	m.textInput.Reset()
	m.err = nil
	m.cursor = len(m.task.Attachments) - 1
}

func (m *TaskAttachmentsModel) executeDelete() tea.Cmd {
	m.resolveTask()
	if m.cursor < 0 || m.cursor >= len(m.task.Attachments) {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("attachment no longer exists")} }
	}
	isURL := m.task.Attachments[m.cursor].IsURL

	removed, ok := m.task.RemoveAttachment(m.cursor)
	if !ok {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("failed to remove attachment")} }
	}
	if m.cursor >= len(m.task.Attachments) && m.cursor > 0 {
		m.cursor--
	}

	task := m.task
	project := m.project
	return func() tea.Msg {
		if !isURL {
			_ = storage.RemoveAttachmentFile(project, task, removed.Filename)
		}
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return AttachmentDeletedMsg{Task: task}
	}
}

func (m *TaskAttachmentsModel) executeKeep() tea.Cmd {
	m.resolveTask()
	_, ok := m.task.RemoveAttachment(m.cursor)
	if !ok {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("failed to unlink attachment")} }
	}
	if m.cursor >= len(m.task.Attachments) && m.cursor > 0 {
		m.cursor--
	}

	task := m.task
	project := m.project
	return func() tea.Msg {
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return AttachmentDeletedMsg{Task: task}
	}
}

func (m *TaskAttachmentsModel) reorderAttachment(from, to int) tea.Cmd {
	m.resolveTask()
	m.task.Attachments[from], m.task.Attachments[to] = m.task.Attachments[to], m.task.Attachments[from]
	m.cursor = to

	task := m.task
	project := m.project
	return func() tea.Msg {
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return AttachmentReorderedMsg{Task: task}
	}
}

func (m *TaskAttachmentsModel) openAttachmentDefault() tea.Cmd {
	att := m.task.Attachments[m.cursor]

	if att.IsCodeRef {
		return m.openCodeReference(att)
	}

	target := m.attachmentOpenTarget(att)
	if target == "" {
		return nil
	}

	openCmd, err := buildOpenCommand(target)
	if err != nil {
		m.err = err
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

	if att.IsCodeRef {
		return m.openCodeReference(att)
	}
	if att.IsURL {
		m.err = fmt.Errorf("cannot edit a URL in an editor")
		return nil
	}

	target := storage.GetAttachmentPath(m.project, m.task, att.Filename)
	if !storage.AttachmentExists(m.project, m.task, att.Filename) {
		m.err = fmt.Errorf("file not found: %s", att.Filename)
		return nil
	}

	// Empty attachment_editor means "use OS default app", not a text editor.
	if m.config.GetAttachmentEditor() == "" {
		openCmd, err := buildOpenCommand(target)
		if err != nil {
			return errorCmd("%s", err)
		}
		return tea.ExecProcess(openCmd, func(err error) tea.Msg {
			if err != nil {
				return ErrorMsg{Err: fmt.Errorf("failed to open: %w", err)}
			}
			return nil
		})
	}

	editor, editorErr := external.ResolveEditor(m.config.GetAttachmentEditor())
	if editorErr != nil {
		return errorCmd("%s", editorErr)
	}

	openCmd := external.EditorCommand(editor, target)
	return tea.ExecProcess(openCmd, func(err error) tea.Msg {
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to open in editor: %w", err)}
		}
		return nil
	})
}

func (m *TaskAttachmentsModel) openAttachmentsDir() tea.Cmd {
	dir := m.selectedAttachmentDir()
	if dir == "" {
		return nil
	}

	openCmd, err := buildDirectoryOpenCommand(dir)
	if err != nil {
		m.err = err
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
	path := m.attachmentClipboardPath(m.task.Attachments[m.cursor], false)
	if err := copyToClipboard(path); err != nil {
		m.err = err
		return nil
	}

	m.setStatus("Path copied to clipboard")
	return nil
}

func (m *TaskAttachmentsModel) copyPathWithLine() tea.Cmd {
	att := m.task.Attachments[m.cursor]
	path := m.attachmentClipboardPath(att, true)
	if err := copyToClipboard(path); err != nil {
		m.err = err
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

	if att.IsURL {
		if err := copyToClipboard(att.OriginalPath); err != nil {
			m.err = fmt.Errorf("failed to copy URL to clipboard: %w", err)
			return nil
		}
		m.setStatus("URL copied to clipboard")
		return nil
	}

	dir := filepath.Dir(att.OriginalPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		m.err = fmt.Errorf("original directory not found: %s", dir)
		return nil
	}

	openCmd, err := buildDirectoryOpenCommand(dir)
	if err != nil {
		m.err = err
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
	if _, err := os.Stat(path); os.IsNotExist(err) {
		m.err = fmt.Errorf("original file not found: %s", path)
		return nil
	}

	openCmd, err := buildRevealCommand(path)
	if err != nil {
		m.err = err
		return nil
	}

	return tea.ExecProcess(openCmd, func(err error) tea.Msg {
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to reveal file: %w", err)}
		}
		return nil
	})
}

func (m *TaskAttachmentsModel) openCodeReference(att models.Attachment) tea.Cmd {
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

func (m *TaskAttachmentsModel) attachmentOpenTarget(att models.Attachment) string {
	if att.IsURL {
		return att.Filename
	}

	target := storage.GetAttachmentPath(m.project, m.task, att.Filename)
	if !storage.AttachmentExists(m.project, m.task, att.Filename) {
		m.err = fmt.Errorf("file not found: %s", att.Filename)
		return ""
	}
	return target
}

func (m *TaskAttachmentsModel) selectedAttachmentDir() string {
	if m.hasSelection() {
		att := m.task.Attachments[m.cursor]
		if att.IsCodeRef {
			dir := filepath.Dir(att.OriginalPath)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				m.err = fmt.Errorf("directory not found: %s", dir)
				return ""
			}
			return dir
		}
	}

	dir := storage.GetTaskAttachmentsDir(m.project, m.task)
	if err := storage.EnsureTaskAttachmentsDir(m.project, m.task); err != nil {
		m.err = fmt.Errorf("failed to create directory: %w", err)
		return ""
	}
	return dir
}

func (m *TaskAttachmentsModel) attachmentClipboardPath(att models.Attachment, includeLine bool) string {
	if att.IsCodeRef {
		if includeLine && att.LineNumber > 0 {
			return fmt.Sprintf("%s:%d", att.OriginalPath, att.LineNumber)
		}
		return att.OriginalPath
	}
	if att.IsURL {
		return att.Filename
	}
	return storage.GetAttachmentPath(m.project, m.task, att.Filename)
}

func copyToClipboard(value string) error {
	clipCmd, err := buildClipboardCommand()
	if err != nil {
		return err
	}
	clipCmd.Stdin = strings.NewReader(value)
	if err := clipCmd.Run(); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}
	return nil
}

func buildClipboardCommand() (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("pbcopy"), nil
	case "linux":
		return exec.Command("xclip", "-selection", "clipboard"), nil
	case "windows":
		return exec.Command("clip"), nil
	default:
		return nil, fmt.Errorf("clipboard not supported on this OS")
	}
}

func buildOpenCommand(target string) (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", target), nil
	case "linux":
		return exec.Command("xdg-open", target), nil
	case "windows":
		return exec.Command("cmd", "/c", "start", "", target), nil
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func buildDirectoryOpenCommand(dir string) (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", dir), nil
	case "linux":
		return exec.Command("xdg-open", dir), nil
	case "windows":
		return exec.Command("explorer", dir), nil
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func buildRevealCommand(path string) (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", path), nil
	case "linux":
		return exec.Command("xdg-open", filepath.Dir(path)), nil
	case "windows":
		return exec.Command("explorer", "/select,", path), nil
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}
