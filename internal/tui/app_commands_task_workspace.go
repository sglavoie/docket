package tui

import tea "charm.land/bubbletea/v2"

// taskWorkspaceCommands exposes the capability-scoped Task Workspace grammar to
// the command palette. The catalog is view-agnostic (Views is nil): typed
// capabilities gate availability, so these commands surface only when the live
// Task Workspace route is active. Most actions — including Delete — route back
// through the same handleAction dispatcher the direct keys use, so palette
// execution and keyboard execution emit the identical request message. The three
// shell-owned actions AppModel intercepts before delegation (Peek, Undo, Redo)
// route through shellOwnedPaletteAction instead, so the palette matches the
// direct Shift+Enter / u / Ctrl+R keys rather than the dispatcher's stand-ins.
func (m *AppModel) taskWorkspaceCommands() []Command {
	return TaskWorkspaceCommands(func(action TaskWorkspaceAction) tea.Msg {
		view, _, ok := m.taskWorkspacePrimary()
		if !ok || view == nil {
			return nil
		}
		task, project := view.getTaskRef()
		if msg, shellOwned := m.shellOwnedPaletteAction(action, task, project); shellOwned {
			return msg
		}
		cmd := view.handleAction(action)
		if cmd == nil {
			return nil
		}
		return PaletteCommandResultMsg{Cmd: cmd}
	})
}
