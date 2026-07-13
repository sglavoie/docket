package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

var errClipboardUnsupported = errors.New("clipboard not supported on this OS")

type taskItemPanelModeHandlers struct {
	expanded       bool
	confirmActive  bool
	editMode       bool
	addMode        bool
	updateExpanded func(tea.Msg) (tea.Model, tea.Cmd)
	updateConfirm  func(tea.Msg) (tea.Model, tea.Cmd)
	updateEdit     func(tea.Msg) (tea.Model, tea.Cmd)
	updateAdd      func(tea.Msg) (tea.Model, tea.Cmd)
	updateNormal   func(tea.Msg) (tea.Model, tea.Cmd)
}

func updateTaskItemPanel(msg tea.Msg, handlers taskItemPanelModeHandlers) (tea.Model, tea.Cmd) {
	switch {
	case handlers.expanded:
		return handlers.updateExpanded(msg)
	case handlers.confirmActive:
		return handlers.updateConfirm(msg)
	case handlers.editMode:
		return handlers.updateEdit(msg)
	case handlers.addMode:
		return handlers.updateAdd(msg)
	default:
		return handlers.updateNormal(msg)
	}
}

func updateTaskItemPanelExpandedView(self tea.Model, msg tea.Msg, expanded *bool) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "enter":
			*expanded = false
		case "j", "down", "k", "up":
			return self, nil
		}
	}

	return self, nil
}

type taskItemPanelConfirmActions struct {
	executeDelete func() tea.Cmd
	cancelAdd     func()
	cancelEdit    func()
}

func updateTaskItemPanelConfirmMode(
	self tea.Model,
	msg tea.Msg,
	base *BaseView,
	state *InteractiveViewState,
	actions taskItemPanelConfirmActions,
) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		action := state.confirm.Action()
		if confirmed, handled := state.HandleConfirmKey(msg.String()); handled {
			if confirmed {
				switch action {
				case "delete":
					return self, actions.executeDelete()
				case "discard-add":
					actions.cancelAdd()
					return self, nil
				case "discard-edit":
					actions.cancelEdit()
					return self, nil
				}
			}
			base.ClearError()
			return self, nil
		}
	}

	return self, nil
}

func updateTaskItemPanelNormalMode(
	self tea.Model,
	msg tea.Msg,
	base *BaseView,
	state *InteractiveViewState,
	handleNormalKey func(string) tea.Cmd,
) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		state.cursorFlash = false
		return self, nil
	case tea.WindowSizeMsg:
		base.SetSize(msg.Width, msg.Height)
		return self, nil
	case tea.KeyPressMsg:
		if state.SkipModifierInPrefix(msg) {
			return self, nil
		}
		if state.pendingPrefix == "g" {
			state.pendingPrefix = ""
			_, cmd = handleGotoTopPrefix(msg.String(), &state.cursor, &state.cursorFlash)
			return self, cmd
		}

		return self, handleNormalKey(msg.String())
	}

	return self, nil
}

type taskItemPanelActions struct {
	model                    tea.Model
	itemName                 string
	inInputMode              func() bool
	moveCursorDown           func()
	moveCursorUp             func()
	moveCursorBottom         func()
	jumpToTop                func()
	beginAdd                 func() tea.Cmd
	hasSelection             func() bool
	startEdit                func() tea.Cmd
	editExternal             func() tea.Cmd
	showDeleteConfirm        func()
	showIndexedDeleteConfirm func()
	toggleResolved           func() tea.Cmd
	copyText                 func() tea.Cmd
	expandSelection          func()
	close                    func() tea.Msg
}

func (a taskItemPanelActions) handleNormalKey(state *InteractiveViewState, key string) tea.Cmd {
	switch key {
	case "esc":
		return func() tea.Msg { return a.close() }
	case "j", "down":
		a.moveCursorDown()
	case "k", "up":
		a.moveCursorUp()
	case "g":
		state.pendingPrefix = "g"
	case "G":
		a.moveCursorBottom()
	case "a", "n":
		return a.beginAdd()
	case "e":
		if a.hasSelection() {
			return a.startEdit()
		}
	case "E":
		if a.hasSelection() {
			return a.editExternal()
		}
	case "d", "x":
		if a.hasSelection() {
			a.showIndexedDeleteConfirm()
		}
	case "r":
		if a.hasSelection() {
			return a.toggleResolved()
		}
	case "y":
		if a.hasSelection() {
			return a.copyText()
		}
	case "enter":
		if a.hasSelection() {
			a.expandSelection()
		}
	}

	return nil
}

func buildTaskItemPanelKeyMap(viewName string, actions taskItemPanelActions, extraEntries ...keymap.Entry) *keymap.KeyMap {
	inInputMode := actions.inInputMode

	entries := []keymap.Entry{
		keymap.Bind("j", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			actions.moveCursorDown()
			return actions.model, nil
		}, keymap.AsHidden(), keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("down", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			actions.moveCursorDown()
			return actions.model, nil
		}, keymap.AsHidden(), keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("k", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			actions.moveCursorUp()
			return actions.model, nil
		}, keymap.AsHidden(), keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("up", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			actions.moveCursorUp()
			return actions.model, nil
		}, keymap.AsHidden(), keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("G", fmt.Sprintf("Jump to last %s", actions.itemName), "Navigation", func() (tea.Model, tea.Cmd) {
			actions.moveCursorBottom()
			return actions.model, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
	}

	entries = append(entries, keymap.GotoGroup(keymap.GotoDeps{
		JumpToTop: func() (tea.Model, tea.Cmd) {
			actions.jumpToTop()
			return actions.model, nil
		},
		GoToProject: nil,
	})...)

	entries = append(entries,
		keymap.Bind("a", fmt.Sprintf("Add %s", actions.itemName), "Actions", func() (tea.Model, tea.Cmd) {
			return actions.model, actions.beginAdd()
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("n", fmt.Sprintf("Add %s", actions.itemName), "Actions", func() (tea.Model, tea.Cmd) {
			return actions.model, actions.beginAdd()
		}, keymap.AsHidden(), keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("e", fmt.Sprintf("Edit %s", actions.itemName), "Actions", func() (tea.Model, tea.Cmd) {
			if actions.hasSelection() {
				return actions.model, actions.startEdit()
			}
			return actions.model, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("E", "Edit in external editor", "Actions", func() (tea.Model, tea.Cmd) {
			if actions.hasSelection() {
				return actions.model, actions.editExternal()
			}
			return actions.model, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("d", fmt.Sprintf("Delete %s", actions.itemName), "Actions", func() (tea.Model, tea.Cmd) {
			if actions.hasSelection() {
				actions.showDeleteConfirm()
			}
			return actions.model, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("x", fmt.Sprintf("Delete %s", actions.itemName), "Actions", func() (tea.Model, tea.Cmd) {
			if actions.hasSelection() {
				actions.showDeleteConfirm()
			}
			return actions.model, nil
		}, keymap.AsHidden(), keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("r", "Toggle resolved", "Actions", func() (tea.Model, tea.Cmd) {
			if actions.hasSelection() {
				return actions.model, actions.toggleResolved()
			}
			return actions.model, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("y", fmt.Sprintf("Copy %s text", actions.itemName), "Actions", func() (tea.Model, tea.Cmd) {
			if actions.hasSelection() {
				return actions.model, actions.copyText()
			}
			return actions.model, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
		keymap.Bind("enter", fmt.Sprintf("Expand %s", actions.itemName), "Actions", func() (tea.Model, tea.Cmd) {
			if actions.hasSelection() {
				actions.expandSelection()
			}
			return actions.model, nil
		}, keymap.WithGate(func() bool { return !inInputMode() })),
	)

	entries = append(entries, extraEntries...)

	return &keymap.KeyMap{
		ViewName: viewName,
		Entries:  entries,
	}
}

// commitTaskItemMutation commits an already-applied local panel mutation
// through its coordinator command and resolves to the panel's own success
// message. The panel mutates its model synchronously in Update() (avoiding
// data races with View()); the application layer then re-applies the same
// mutation against a fresh snapshot and commits one ChangeSet. The success
// message carries the freshest resolvable task so downstream handlers observe
// the committed object, not the panel's optimistic copy.
func commitTaskItemMutation(project *models.Project, task *models.Task, commit tea.Cmd, success func(*models.Task) tea.Msg) tea.Cmd {
	return adaptSubviewCommit(commit, func() tea.Msg {
		fresh := task
		if current := resolveTaskInProject(project, task.ID); current != nil {
			fresh = current
		}
		return success(fresh)
	})
}

func refreshTaskReference(project *models.Project, task **models.Task) {
	if current := resolveTaskInProject(project, (*task).ID); current != nil {
		*task = current
	}
}

type taskItemDeleteMutation struct {
	project      *models.Project
	task         **models.Task
	cursor       *int
	remove       func(*models.Task, int) bool
	count        func(*models.Task) int
	errorMessage string
	// commit builds the coordinator command that persists the removal of the
	// item at the given pre-decrement index.
	commit  func(taskID string, index int) tea.Cmd
	success func(*models.Task) tea.Msg
}

func executeTaskItemDeleteMutation(cfg taskItemDeleteMutation) tea.Cmd {
	refreshTaskReference(cfg.project, cfg.task)
	index := *cfg.cursor
	if !cfg.remove(*cfg.task, index) {
		return func() tea.Msg { return ErrorMsg{Err: errors.New(cfg.errorMessage)} }
	}
	if *cfg.cursor >= cfg.count(*cfg.task) && *cfg.cursor > 0 {
		*cfg.cursor--
	}

	return commitTaskItemMutation(cfg.project, *cfg.task, cfg.commit((*cfg.task).ID, index), cfg.success)
}

type taskItemToggleMutation struct {
	project      *models.Project
	task         **models.Task
	cursor       int
	toggle       func(*models.Task, int) bool
	errorMessage string
	// commit builds the coordinator command that persists the toggle of the
	// item at the given index.
	commit  func(taskID string, index int) tea.Cmd
	success func(*models.Task) tea.Msg
}

func executeTaskItemToggleMutation(cfg taskItemToggleMutation) tea.Cmd {
	refreshTaskReference(cfg.project, cfg.task)
	if !cfg.toggle(*cfg.task, cfg.cursor) {
		return func() tea.Msg { return ErrorMsg{Err: errors.New(cfg.errorMessage)} }
	}

	return commitTaskItemMutation(cfg.project, *cfg.task, cfg.commit((*cfg.task).ID, cfg.cursor), cfg.success)
}

func copyTextToClipboard(text string) error {
	var clipCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		clipCmd = exec.Command("pbcopy")
	case "linux":
		clipCmd = exec.Command("xclip", "-selection", "clipboard")
	case "windows":
		clipCmd = exec.Command("clip")
	default:
		return errClipboardUnsupported
	}

	clipCmd.Stdin = strings.NewReader(text)
	return clipCmd.Run()
}

func renderTaskItemSummary(b *strings.Builder, itemName string, total, unresolved int) {
	resolvedCount := total - unresolved
	countStr := fmt.Sprintf("%d %s(s)", total, itemName)
	if resolvedCount > 0 {
		countStr += fmt.Sprintf(" (%d resolved)", resolvedCount)
	}
	b.WriteString(MutedStyle.Render(countStr))
	b.WriteString("\n\n")
}

func renderTaskItemMessages(b *strings.Builder, err error) {
	if err == nil {
		return
	}
	b.WriteString(ValidationErrorStyle.Render(err.Error()))
	b.WriteString("\n\n")
}

func renderTaskItemStatusMessage(b *strings.Builder, status string) {
	if status == "" {
		return
	}
	b.WriteString(MutedStyle.Render(status))
	b.WriteString("\n")
}

func renderTaskItemPreview(entry *strings.Builder, text string, maxLen int, muted bool) {
	rendered := renderMarkdown(text, maxLen)
	renderedLines := strings.Split(rendered, "\n")
	for j, line := range renderedLines {
		if j > 1 {
			fmt.Fprintf(entry, "      ... (%d more lines, press Enter to expand)\n", len(renderedLines)-2)
			break
		}

		entry.WriteString("      ")
		line = truncateIntentional(line, maxLen, "")
		if muted {
			entry.WriteString(MutedStyle.Render(line))
		} else {
			entry.WriteString(line)
		}
		entry.WriteString("\n")
	}
}
