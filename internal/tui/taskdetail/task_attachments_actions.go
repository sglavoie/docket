package taskdetail

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/external"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// addAttachment routes a pasted value to one of the three attach paths. The
// classification itself lives in app.ResolveAttachmentInput so every surface that
// accepts a pasted attachment agrees on it; the panel keeps only the optimistic
// add-mode teardown and its own error state.
func (m *TaskAttachmentsModel) addAttachment() tea.Cmd {
	resolved, err := app.ResolveAttachmentInput(m.TextInput.Value())
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
	return m.Coordinator.AttachURL(taskref.For(m.Task, m.project), input)
}

func (m *TaskAttachmentsModel) addCodeReference(resolved app.ResolvedAttachmentInput) tea.Cmd {
	if resolved.SourceMissing {
		m.SetStatus("Code reference added (file may not exist)")
	}

	m.finishOptimisticAdd()
	return m.Coordinator.AttachCodeReference(taskref.For(m.Task, m.project), resolved.CodePath, resolved.CodeLine)
}

func (m *TaskAttachmentsModel) addFileAttachment(absPath string) tea.Cmd {
	m.finishOptimisticAdd()
	return m.Coordinator.AttachFile(taskref.For(m.Task, m.project), absPath)
}

// finishOptimisticAdd exits add mode before the transaction commits; the new
// attachment lands at the end of the list, so the cursor targets that slot and
// RefreshAfterReload clamps it if the commit is rejected.
func (m *TaskAttachmentsModel) finishOptimisticAdd() {
	m.AddMode = false
	m.TextInput.Reset()
	m.Err = nil
	m.Cursor = len(m.Task.Attachments)
}

func (m *TaskAttachmentsModel) executeDelete(pending *pendingAttachmentRemoval) tea.Cmd {
	return m.executeRemoval(pending, true)
}

func (m *TaskAttachmentsModel) executeKeep(pending *pendingAttachmentRemoval) tea.Cmd {
	return m.executeRemoval(pending, false)
}

// executeRemoval removes the attachment the prompt asked about, never the one
// under the cursor now. The panel refuses at once when its own fresh read no
// longer holds that attachment where the prompt found it, and the application
// layer checks the same identity again against the snapshot it commits on.
func (m *TaskAttachmentsModel) executeRemoval(pending *pendingAttachmentRemoval, deleteFile bool) tea.Cmd {
	m.resolveTask()
	if pending == nil || pending.index < 0 || pending.index >= len(m.Task.Attachments) {
		return func() tea.Msg { return status.ErrorMsg{Err: fmt.Errorf("attachment no longer exists")} }
	}
	if !app.SameAttachment(m.Task.Attachments[pending.index], pending.attachment) {
		return func() tea.Msg {
			return status.ErrorMsg{Err: fmt.Errorf("attachments changed since you asked; nothing was removed")}
		}
	}
	// URL attachments have no bytes on disk; the file-backed ones delete their
	// copied bytes in the same transaction (matching the old RemoveAttachmentFile).
	deleteFile = deleteFile && !pending.attachment.IsURL
	return m.Coordinator.RemoveAttachment(taskref.For(m.Task, m.project), pending.index, pending.attachment, deleteFile)
}

func (m *TaskAttachmentsModel) reorderAttachment(from, to int) tea.Cmd {
	// Swap locally for immediate render; the coordinator commit re-applies the
	// swap against a fresh snapshot and RefreshAfterReload re-resolves the view.
	m.resolveTask()
	m.Task.Attachments[from], m.Task.Attachments[to] = m.Task.Attachments[to], m.Task.Attachments[from]
	m.Cursor = to

	return m.Coordinator.Reorder(taskref.For(m.Task, m.project), from, to)
}

func (m *TaskAttachmentsModel) openAttachmentDefault() tea.Cmd {
	att := m.Task.Attachments[m.Cursor]

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
			return status.ErrorMsg{Err: fmt.Errorf("could not open %s: %w", m.attachmentDisplayName(att), err)}
		}
		return nil
	})
}

func (m *TaskAttachmentsModel) openAttachmentInEditor() tea.Cmd {
	att := m.Task.Attachments[m.Cursor]

	if att.IsCodeRef {
		return m.openCodeReference(att)
	}
	if att.IsURL {
		m.Err = fmt.Errorf("cannot edit a URL in an editor")
		return nil
	}

	target, err := storage.ResolveAttachmentPath(m.project, m.Task, att.Filename)
	if err != nil {
		m.Err = fmt.Errorf("invalid attachment path: %w", err)
		return nil
	}
	if !storage.AttachmentExists(m.project, m.Task, att.Filename) {
		m.Err = fmt.Errorf("file not found: %s", att.Filename)
		return nil
	}

	openCmd, err := buildOpenCommand(target)
	if err != nil {
		return status.ErrorCmd("%s", err)
	}
	return tea.ExecProcess(openCmd, func(err error) tea.Msg {
		if err != nil {
			return status.ErrorMsg{Err: fmt.Errorf("could not open %s: %w", m.attachmentDisplayName(att), err)}
		}
		return nil
	})
}

func (m *TaskAttachmentsModel) OpenAttachmentsDir() tea.Cmd {
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
			return status.ErrorMsg{Err: fmt.Errorf("could not open attachments folder: %w", err)}
		}
		return nil
	})
}

func (m *TaskAttachmentsModel) copyPath() tea.Cmd {
	path := m.attachmentClipboardPath(m.Task.Attachments[m.Cursor], false)
	if path == "" {
		return nil
	}
	if err := copyToClipboard(path); err != nil {
		m.Err = fmt.Errorf("couldn't copy to clipboard: %w", err)
		return nil
	}

	m.SetStatus("Path copied to clipboard")
	return nil
}

func (m *TaskAttachmentsModel) copyPathWithLine() tea.Cmd {
	att := m.Task.Attachments[m.Cursor]
	path := m.attachmentClipboardPath(att, true)
	if path == "" {
		return nil
	}
	if err := copyToClipboard(path); err != nil {
		m.Err = fmt.Errorf("couldn't copy to clipboard: %w", err)
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
	att := m.Task.Attachments[m.Cursor]

	if att.IsURL {
		if err := copyToClipboard(att.OriginalPath); err != nil {
			m.Err = fmt.Errorf("couldn't copy to clipboard: %w", err)
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
			return status.ErrorMsg{Err: fmt.Errorf("could not open folder for %s: %w", m.attachmentDisplayName(att), err)}
		}
		return nil
	})
}

func (m *TaskAttachmentsModel) revealOriginalFile() tea.Cmd {
	att := m.Task.Attachments[m.Cursor]

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
			return status.ErrorMsg{Err: fmt.Errorf("could not reveal %s: %w", m.attachmentDisplayName(att), err)}
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
		m.Err = fmt.Errorf("could not open code reference: %w", err)
		return nil
	}
	m.SetStatus(fmt.Sprintf("Opened %s:%d", filepath.Base(att.OriginalPath), att.LineNumber))
	return nil
}

func (m *TaskAttachmentsModel) attachmentOpenTarget(att models.Attachment) string {
	if att.IsURL {
		return att.Filename
	}

	target, err := storage.ResolveAttachmentPath(m.project, m.Task, att.Filename)
	if err != nil {
		m.Err = fmt.Errorf("invalid attachment path: %w", err)
		return ""
	}
	if !storage.AttachmentExists(m.project, m.Task, att.Filename) {
		m.Err = fmt.Errorf("file not found: %s", att.Filename)
		return ""
	}
	return target
}

func (m *TaskAttachmentsModel) selectedAttachmentDir() string {
	if m.HasSelection() {
		att := m.Task.Attachments[m.Cursor]
		if att.IsCodeRef {
			dir := filepath.Dir(att.OriginalPath)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				m.Err = fmt.Errorf("directory not found: %s", dir)
				return ""
			}
			return dir
		}
	}

	if err := storage.EnsureTaskAttachmentsDir(m.project, m.Task); err != nil {
		m.Err = fmt.Errorf("failed to create directory: %w", err)
		return ""
	}
	dir, err := storage.ResolveTaskAttachmentsDir(m.project, m.Task)
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
	path, err := storage.ResolveAttachmentPath(m.project, m.Task, att.Filename)
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
		return err
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
