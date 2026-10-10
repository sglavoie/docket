package conflict

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// ResolveMsg freezes the inspected identity and choice until the host completes
// the commit. Version is the engine's index, not its displayed one-based number.
type ResolveMsg struct {
	Browser *ConflictBrowserModel
	ID      string
	Sync    bool
	Choice  string
	Version int
	Text    string
}

func (model *ConflictBrowserModel) resolutionKeys() string {
	if model.syncEntry != nil {
		if model.syncEntry.Replaceable {
			return "v keep version · c keep current · e edit"
		}
		return "v keep version · c keep current"
	}
	if model.bundle != nil && model.bundle.Record.OperationRetryRequired() {
		return "r keep current and clear"
	}
	return "l keep local · r keep current · e edit"
}

func (model *ConflictBrowserModel) rebuildDetail() {
	model.detailOffset = 0
	if model.syncEntry != nil {
		entry := model.syncEntry
		model.detail = syncConflictDetailLines(*entry)
		if len(entry.Versions) == 0 {
			return
		}
		version := entry.Versions[model.version]
		model.detail = append([]string{fmt.Sprintf("Selected version %d of %d · [/] change version", model.version+1, len(entry.Versions)), ""}, model.detail...)
		if model.diff {
			before, label, prefix := entry.CurrentText, "Current", "current/"
			if model.diffBase {
				before, label, prefix = entry.BaseText, "Base", "base/"
			}
			model.detail = []string{entry.Label, fmt.Sprintf("%s → version %d of %d · [/] change version", label, model.version+1, len(entry.Versions)), ""}
			if before == nil || version.Text == nil {
				model.detail = append(model.detail, "Text diff unavailable for binary, oversized, or missing content.")
				model.detail = append(model.detail, syncConflictBody(version.Content)...)
			} else {
				model.detail = append(model.detail, unifiedDiff(*before, *version.Text, prefix+entry.Path, fmt.Sprintf("version-%d/", model.version+1)+entry.Path)...)
				if model.diffBase && *before == "" {
					model.detail = append(model.detail, "", "The base is empty; use b to compare against the current file.")
				}
			}
		}
		return
	}
	if model.bundle == nil {
		return
	}
	b := model.bundle
	model.detail = conflictDetailLines(b.Record)
	if model.diff {
		if !model.diffBase {
			model.detail = []string{"Recorded remote → local", ""}
			model.detail = append(model.detail, unifiedDiff(b.RemoteText, b.LocalText, "remote/tasks.md", "local/tasks.md")...)
			return
		}
		model.detail = []string{"Base → local", ""}
		model.detail = append(model.detail, unifiedDiff(b.BaseText, b.LocalText, "base/tasks.md", "local/tasks.md")...)
		model.detail = append(model.detail, "", "Base → recorded remote", "")
		model.detail = append(model.detail, unifiedDiff(b.BaseText, b.RemoteText, "base/tasks.md", "remote/tasks.md")...)
	}
}

func (model *ConflictBrowserModel) chooseResolution(key string) {
	request := ResolveMsg{Browser: model}
	var summary []string
	if model.syncEntry != nil {
		entry := model.syncEntry
		request.ID, request.Sync = entry.ID, true
		switch key {
		case "v":
			if len(entry.Versions) == 0 {
				return
			}
			version := entry.Versions[model.version]
			request.Choice, request.Version = "version", version.Index
			summary = []string{fmt.Sprintf("Keep version %d of %s?", model.version+1, entry.Label)}
			for _, writer := range version.Writers {
				summary = append(summary, syncConflictWriterLine(writer))
			}
			if version.Blob == "" {
				summary = append(summary, "This version deletes the file.")
			}
			for _, label := range version.AlsoResolves {
				summary = append(summary, "Also resolves: "+label)
			}
		case "c":
			request.Choice = "current"
			summary = []string{"Keep the current workspace content of " + entry.Label + "?"}
			if entry.Result == "" {
				summary = append(summary, "The file is currently deleted.")
			}
		default:
			return
		}
	} else if model.bundle != nil {
		b := model.bundle
		request.ID = b.Record.ID
		switch key {
		case "l":
			if b.Record.OperationRetryRequired() {
				model.notice = "Keep current and retry the original operation; the full transaction cannot be replayed."
				return
			}
			if b.Local == nil || b.LocalUnreadable != "" {
				model.notice = "Stored local document cannot be applied: " + b.LocalUnreadable
				return
			}
			request.Choice = "local"
			summary = []string{"Keep local edits for " + b.Record.ProjectName + "?", "This replaces the recorded remote changes. Newer edits are checked before saving."}
		case "r":
			request.Choice = "remote"
			summary = []string{"Keep the current on-disk state of " + b.Record.ProjectName + "?", "The stored local edits will be discarded and this record cleared."}
			if b.Record.OperationRetryRequired() {
				summary = append(summary, "Then retry the original operation: "+b.Record.Action, b.Record.Context)
			}
		default:
			return
		}
	} else {
		return
	}
	model.confirmResolution(request, summary)
}

func (model *ConflictBrowserModel) confirmResolution(request ResolveMsg, lines []string) {
	model.notice = ""
	model.confirmation = &request
	model.detail = append(lines, "", "Enter confirms · Esc cancels")
	model.detailOffset = 0
}

func (model *ConflictBrowserModel) handleConfirmation(key string) (tea.Cmd, bool, tea.Msg) {
	switch key {
	case "esc":
		model.confirmation = nil
		model.rebuildDetail()
	case "enter":
		model.busy = true
		model.notice = "Resolving…"
		return nil, false, *model.confirmation
	case "up", "k":
		model.scrollDetail(-1)
	case "down", "j":
		model.scrollDetail(1)
	case "pgdown":
		model.scrollDetail(max(model.detailHeight-1, 1))
	case "pgup":
		model.scrollDetail(-max(model.detailHeight-1, 1))
	}
	return nil, false, nil
}

// ResolutionFailed leaves the inspected conflict and any draft intact for retry.
func (model *ConflictBrowserModel) ResolutionFailed(err error) {
	model.busy = false
	model.confirmation = nil
	model.notice = err.Error()
	model.rebuildDetail()
}

func (model *ConflictBrowserModel) openEditor() {
	if model.editor != nil {
		model.editor.active = true
		return
	}
	var text string
	if model.syncEntry != nil {
		entry := model.syncEntry
		if !entry.Replaceable {
			model.notice = "This file must be resolved by keeping a version or the current state."
			return
		}
		source := entry.CurrentText
		if len(entry.Versions) > 0 {
			source = entry.Versions[model.version].Text
		}
		if source == nil {
			model.notice = "This version is unavailable as editable text; choose another version."
			return
		}
		text = *source
	} else if model.bundle != nil {
		if model.bundle.Record.OperationRetryRequired() {
			model.notice = "Keep current and retry the original operation; the full transaction cannot be replayed."
			return
		}
		text = model.bundle.LocalText
	} else {
		return
	}
	model.notice = ""
	model.editor = newMergeEditor(text)
}

func (model *ConflictBrowserModel) handleEditorKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		// Keep the draft while inspecting diffs. e returns to exactly this draft.
		model.editor.active = false
		model.rebuildDetail()
	case "ctrl+s":
		request := ResolveMsg{Browser: model, Text: string(model.editor.text)}
		before := ""
		if model.syncEntry != nil {
			request.ID, request.Sync, request.Choice = model.syncEntry.ID, true, "replace"
			if model.syncEntry.CurrentText != nil {
				before = *model.syncEntry.CurrentText
			}
		} else {
			request.ID, request.Choice = model.bundle.Record.ID, "file"
			before = model.bundle.RemoteText
		}
		lines := []string{"Apply this merged result?", "Changes from the current/recorded remote content:", ""}
		lines = append(lines, unifiedDiff(before, request.Text, "current", "merged")...)
		model.confirmResolution(request, lines)
	default:
		model.editor.key(msg)
	}
	return nil, false, nil
}

// HandlePaste contains paste inside the overlay, even outside its editor.
func (model *ConflictBrowserModel) HandlePaste(msg tea.PasteMsg) tea.Cmd {
	if model.editor != nil && model.editor.active && model.confirmation == nil && !model.busy {
		model.editor.insert(msg.Content)
	}
	return nil
}

func (model *ConflictBrowserModel) editorActive() bool {
	return model.editor != nil && model.editor.active
}

// hasDraft is used by the browser's back action so leaving an edited conflict
// requires an explicit discard rather than losing it while reviewing diffs.
func (model *ConflictBrowserModel) hasDraft() bool {
	return model.editor != nil && string(model.editor.text) != model.editor.original
}

func (model *ConflictBrowserModel) scrollKeys() string {
	if model.syncEntry != nil {
		return "j/k scroll · [/] version"
	}
	return "j/k scroll"
}
