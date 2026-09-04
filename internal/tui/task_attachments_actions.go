package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// addAttachment routes a pasted value to one of the three attach paths. The
// classification itself lives in app.ResolveAttachmentInput so every surface that
// accepts a pasted attachment agrees on it; the panel keeps only the optimistic
// add-mode teardown and its own error state.
func (m *TaskAttachmentsModel) addAttachment() tea.Cmd {
	resolved, err := app.ResolveAttachmentInput(m.textInput.Value())
	if err != nil {
		m.Err = err
		return nil
	}

	m.resolveTask()

	switch resolved.Kind {
	case app.AttachmentInputURL:
		return m.addURLAttachment(resolved.URL)
	case app.AttachmentInputCodeReference:
		return m.addCodeReference(resolved)
	default:
		return m.addFileAttachment(resolved.SourcePath)
	}
}

func (m *TaskAttachmentsModel) addURLAttachment(input string) tea.Cmd {
	m.finishOptimisticAdd()
	return m.coordinator.AttachURL(taskref.For(m.task, m.project), input)
}

func (m *TaskAttachmentsModel) addCodeReference(resolved app.ResolvedAttachmentInput) tea.Cmd {
	if resolved.SourceMissing {
		m.SetStatus("Code reference added (file may not exist)")
	}

	m.finishOptimisticAdd()
	return m.coordinator.AttachCodeReference(taskref.For(m.task, m.project), resolved.CodePath, resolved.CodeLine)
}

func (m *TaskAttachmentsModel) addFileAttachment(absPath string) tea.Cmd {
	m.finishOptimisticAdd()
	return m.coordinator.AttachFile(taskref.For(m.task, m.project), absPath)
}

// finishOptimisticAdd exits add mode before the transaction commits; the new
// attachment lands at the end of the list, so the cursor targets that slot and
// refreshAfterReload clamps it if the commit is rejected.
func (m *TaskAttachmentsModel) finishOptimisticAdd() {
	m.addMode = false
	m.textInput.Reset()
	m.Err = nil
	m.Cursor = len(m.task.Attachments)
}

func (m *TaskAttachmentsModel) executeDelete() tea.Cmd {
	m.resolveTask()
	if m.Cursor < 0 || m.Cursor >= len(m.task.Attachments) {
		return func() tea.Msg { return status.ErrorMsg{Err: fmt.Errorf("attachment no longer exists")} }
	}
	// URL attachments have no bytes on disk; the file-backed ones delete their
	// copied bytes in the same transaction (matching the old RemoveAttachmentFile).
	deleteFile := !m.task.Attachments[m.Cursor].IsURL
	return m.coordinator.RemoveAttachment(taskref.For(m.task, m.project), m.Cursor, deleteFile)
}

func (m *TaskAttachmentsModel) executeKeep() tea.Cmd {
	m.resolveTask()
	if m.Cursor < 0 || m.Cursor >= len(m.task.Attachments) {
		return func() tea.Msg { return status.ErrorMsg{Err: fmt.Errorf("attachment no longer exists")} }
	}
	return m.coordinator.RemoveAttachment(taskref.For(m.task, m.project), m.Cursor, false)
}

func (m *TaskAttachmentsModel) reorderAttachment(from, to int) tea.Cmd {
	// Swap locally for immediate render; the coordinator commit re-applies the
	// swap against a fresh snapshot and refreshAfterReload re-resolves the view.
	m.resolveTask()
	m.task.Attachments[from], m.task.Attachments[to] = m.task.Attachments[to], m.task.Attachments[from]
	m.Cursor = to

	return m.coordinator.Reorder(taskref.For(m.task, m.project), from, to)
}

func (m *TaskAttachmentsModel) openAttachmentDefault() tea.Cmd {
	att := m.task.Attachments[m.Cursor]

	if att.IsCodeRef {
		return m.openCodeReference(att)
	}

	target := m.attachmentOpenTarget(att)
	if target == "" {
		return nil
	}

	openCmd, err := buildOpenCommand(target)
	if err != nil {
		m.Err = err
		return nil
	}

	return tea.ExecProcess(openCmd, func(err error) tea.Msg {
		if err != nil {
			return status.ErrorMsg{Err: fmt.Errorf("failed to open: %w", err)}
		}
		return nil
	})
}

func (m *TaskAttachmentsModel) openAttachmentInEditor() tea.Cmd {
	att := m.task.Attachments[m.Cursor]

	if att.IsCodeRef {
		return m.openCodeReference(att)
	}
	if att.IsURL {
		m.Err = fmt.Errorf("cannot edit a URL in an editor")
		return nil
	}

	target, err := storage.ResolveAttachmentPath(m.project, m.task, att.Filename)
	if err != nil {
		m.Err = fmt.Errorf("invalid attachment path: %w", err)
		return nil
	}
	if !storage.AttachmentExists(m.project, m.task, att.Filename) {
		m.Err = fmt.Errorf("file not found: %s", att.Filename)
		return nil
	}

	openCmd, err := buildOpenCommand(target)
	if err != nil {
		return status.ErrorCmd("%s", err)
	}
	return tea.ExecProcess(openCmd, func(err error) tea.Msg {
		if err != nil {
			return status.ErrorMsg{Err: fmt.Errorf("failed to open: %w", err)}
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
		m.Err = err
		return nil
	}

	return tea.ExecProcess(openCmd, func(err error) tea.Msg {
		if err != nil {
			return status.ErrorMsg{Err: fmt.Errorf("failed to open directory: %w", err)}
		}
		return nil
	})
}

func (m *TaskAttachmentsModel) copyPath() tea.Cmd {
	path := m.attachmentClipboardPath(m.task.Attachments[m.Cursor], false)
	if path == "" {
		return nil
	}
	if err := copyToClipboard(path); err != nil {
		m.Err = err
		return nil
	}

	m.SetStatus("Path copied to clipboard")
	return nil
}

func (m *TaskAttachmentsModel) copyPathWithLine() tea.Cmd {
	att := m.task.Attachments[m.Cursor]
	path := m.attachmentClipboardPath(att, true)
	if path == "" {
		return nil
	}
	if err := copyToClipboard(path); err != nil {
		m.Err = err
		return nil
	}

	if att.IsCodeRef && att.LineNumber > 0 {
		m.SetStatus("Path:line copied to clipboard")
	} else {
		m.SetStatus("Path copied to clipboard")
	}
	return nil
}

func (m *TaskAttachmentsModel) openOriginalPath() tea.Cmd {
	att := m.task.Attachments[m.Cursor]

	if att.IsURL {
		if err := copyToClipboard(att.OriginalPath); err != nil {
			m.Err = fmt.Errorf("failed to copy URL to clipboard: %w", err)
			return nil
		}
		m.SetStatus("URL copied to clipboard")
		return nil
	}

	dir := filepath.Dir(att.OriginalPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		m.Err = fmt.Errorf("original directory not found: %s", dir)
		return nil
	}

	openCmd, err := buildDirectoryOpenCommand(dir)
	if err != nil {
		m.Err = err
		return nil
	}

	return tea.ExecProcess(openCmd, func(err error) tea.Msg {
		if err != nil {
			return status.ErrorMsg{Err: fmt.Errorf("failed to open directory: %w", err)}
		}
		return nil
	})
}

func (m *TaskAttachmentsModel) revealOriginalFile() tea.Cmd {
	att := m.task.Attachments[m.Cursor]

	if att.IsURL {
		m.Err = fmt.Errorf("cannot reveal URL attachments in file manager")
		return nil
	}

	path := att.OriginalPath
	if _, err := os.Stat(path); os.IsNotExist(err) {
		m.Err = fmt.Errorf("original file not found: %s", path)
		return nil
	}

	openCmd, err := buildRevealCommand(path)
	if err != nil {
		m.Err = err
		return nil
	}

	return tea.ExecProcess(openCmd, func(err error) tea.Msg {
		if err != nil {
			return status.ErrorMsg{Err: fmt.Errorf("failed to reveal file: %w", err)}
		}
		return nil
	})
}

func (m *TaskAttachmentsModel) openCodeReference(att models.Attachment) tea.Cmd {
	if !storage.CodeReferenceFileExists(att.OriginalPath) {
		m.Err = fmt.Errorf("code reference file not found: %s", att.OriginalPath)
		return nil
	}
	if err := external.OpenCodeReference("", att.OriginalPath, att.LineNumber); err != nil {
		m.Err = fmt.Errorf("failed to open code reference: %w", err)
		return nil
	}
	m.SetStatus(fmt.Sprintf("Opened %s:%d", filepath.Base(att.OriginalPath), att.LineNumber))
	return nil
}

func (m *TaskAttachmentsModel) attachmentOpenTarget(att models.Attachment) string {
	if att.IsURL {
		return att.Filename
	}

	target, err := storage.ResolveAttachmentPath(m.project, m.task, att.Filename)
	if err != nil {
		m.Err = fmt.Errorf("invalid attachment path: %w", err)
		return ""
	}
	if !storage.AttachmentExists(m.project, m.task, att.Filename) {
		m.Err = fmt.Errorf("file not found: %s", att.Filename)
		return ""
	}
	return target
}

func (m *TaskAttachmentsModel) selectedAttachmentDir() string {
	if m.hasSelection() {
		att := m.task.Attachments[m.Cursor]
		if att.IsCodeRef {
			dir := filepath.Dir(att.OriginalPath)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				m.Err = fmt.Errorf("directory not found: %s", dir)
				return ""
			}
			return dir
		}
	}

	if err := storage.EnsureTaskAttachmentsDir(m.project, m.task); err != nil {
		m.Err = fmt.Errorf("failed to create directory: %w", err)
		return ""
	}
	dir, err := storage.ResolveTaskAttachmentsDir(m.project, m.task)
	if err != nil {
		m.Err = fmt.Errorf("invalid attachments directory: %w", err)
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
	path, err := storage.ResolveAttachmentPath(m.project, m.task, att.Filename)
	if err != nil {
		m.Err = fmt.Errorf("invalid attachment path: %w", err)
		return ""
	}
	return path
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
