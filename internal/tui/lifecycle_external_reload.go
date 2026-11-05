package tui

import (
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/handoff"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/util"
	"github.com/sglavoie/docket/internal/workspace"
)

// =============================================================================
// Attention while in the background: focus reload and --notify
// =============================================================================

// externalReloadState is the shell's memory of what the workspace looked like
// on disk when the terminal last lost focus. A long-lived session — Docket.app hides
// its window rather than ending Docket — otherwise never re-reads a workspace that
// a synced machine changed underneath it, and learns of that change only when
// its own next commit records a conflict.
//
// It needs a terminal that reports focus (mode 1004). Without one neither
// message arrives and nothing here runs.
type externalReloadState struct {
	// blurFingerprint is the workspace fingerprint taken at the last blur; empty
	// while focused and before the first blur, which is what keeps a launch from
	// reloading. A control-socket commit made while blurred advances it, since
	// that change is Docket's own.
	blurFingerprint string
	// owed records a detected change the shell could not yet adopt because the
	// user was mid-interaction. The periodic tick and the next focus retry it.
	owed  bool
	quiet bool // Polling must not replace a save/undo message with a reload notice.
}

// workspaceFingerprint is workspace.Fingerprint over the runtime's data dir
// and every catalog project, parked and archived included, since a reload
// rereads them all.
func (m *AppModel) workspaceFingerprint() string {
	return m.workspaceStamps().Fingerprint()
}

// workspaceStamps is workspace.StampWorkspace over the same files.
func (m *AppModel) workspaceStamps() workspace.Stamps {
	rs := m.domain.runtime
	if rs == nil {
		return nil
	}
	return workspace.StampWorkspace(rs.DataDir(), rs.AllProjects())
}

// raiseWindowMsg is a later launch's nudge, which internal/handoff sends this
// terminal session as SIGUSR1 when the workspace's lock refused it.
type raiseWindowMsg struct{}

// sessionNudges names the session RunApp's lock records and, for a terminal
// session, subscribes to the nudge before that lock advertises it, since
// SIGUSR1 left unhandled ends the process. A headless session has no terminal
// to raise, so it subscribes to nothing.
func sessionNudges(headless bool) (string, <-chan os.Signal, func()) {
	if headless {
		return util.SessionHeadless, nil, func() {}
	}
	nudges, stop := handoff.ListenForNudges()
	return util.SessionTerminal, nudges, stop
}

// forwardNudges hands each nudge to the program until done closes. A nil
// channel never delivers, so a platform with no nudge waits for done alone.
func forwardNudges(p *tea.Program, nudges <-chan os.Signal, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case <-nudges:
			p.Send(raiseWindowMsg{})
		}
	}
}

// handleFocusMessage answers the terminal's focus reports. A blur records the
// workspace as it stands; a focus compares against it and adopts whatever
// changed on disk meanwhile. A nudge writes handoff.RaiseSequence, asking the
// terminal to raise this window; one that does not implement it ignores it, and
// the focus report that follows a raise is answered as any other.
func (m *AppModel) handleFocusMessage(msg tea.Msg) (bool, tea.Cmd) {
	switch msg.(type) {
	case raiseWindowMsg:
		return true, tea.Raw(handoff.RaiseSequence)
	case tea.BlurMsg:
		m.externalReload.blurFingerprint = m.workspaceFingerprint()
		m.noteTerminalFocus(false)
		return true, nil
	case tea.FocusMsg:
		m.noteTerminalFocus(true)
		before := m.externalReload.blurFingerprint
		m.externalReload.blurFingerprint = ""
		if before != "" && before != m.workspaceFingerprint() {
			m.externalReload.owed = true
		}
		return true, tea.Batch(m.adoptExternalWorkspaceChange(), m.requestAutomaticSync())
	default:
		return false, nil
	}
}

// noteTerminalFocus hands a focus report to the two things that show it: the
// Outliner's cursor rests dimmed while the window is in the background, and
// the pulse holds the counts it would light until the user is back to see them.
func (m *AppModel) noteTerminalFocus(focused bool) {
	if m.crossOutliner != nil {
		m.crossOutliner.SetTerminalFocused(focused)
	}
	m.notePulseFocus(focused)
}

// adoptBlurredWorkspaceChange is the focus check for a session acting while
// blurred, as a control-socket request makes it: a change made elsewhere since
// the blur is adopted before acting, so the act commits against a fresh read.
func (m *AppModel) adoptBlurredWorkspaceChange() tea.Cmd {
	before := m.externalReload.blurFingerprint
	if before == "" {
		return nil
	}
	if current := m.workspaceFingerprint(); before != current {
		m.externalReload.owed = true
		m.externalReload.blurFingerprint = current
	}
	return m.adoptExternalWorkspaceChange()
}

// recordBlurredWorkspace advances a blurred session's fingerprint over a commit
// of its own.
func (m *AppModel) recordBlurredWorkspace() {
	if m.externalReload.blurFingerprint != "" {
		m.externalReload.blurFingerprint = m.workspaceFingerprint()
	}
}

// safeToAdoptExternalChange is the time-driven invalidation's own guard plus
// the Outliner being the live route: a reload rebinds every project pointer,
// so it waits for the user to put down a picker, an editor, a selection, or a
// child view rather than renumber what they are acting on.
func (m *AppModel) safeToAdoptExternalChange() bool {
	return m.currentView() == presentation.ViewCrossProjectOutliner &&
		m.activeOutlinerRoute() &&
		m.safeToInvalidateTimeDrivenRows()
}

// adoptExternalWorkspaceChange reloads the workspace from disk when a change is
// owed and the shell can take it. It is the ledger restore's reload without the
// restore: catalog and planning first, so a project another machine created is
// present, then every project's tasks, then the planning collections resolved
// against them. It writes nothing; reconciling planning is a commit's job.
func (m *AppModel) adoptExternalWorkspaceChange() tea.Cmd {
	if !m.externalReload.owed || !m.safeToAdoptExternalChange() {
		return nil
	}
	stamps := m.workspaceStamps()
	if !m.refreshRuntime() && m.domain.runtime != nil && m.domain.runtime.DataDir() != "" {
		return nil // Keep the reload owed; a transient read failure is retried.
	}
	m.externalReload.owed = false
	for id := range m.domain.projects {
		if !m.reloadProjectByID(id) {
			m.externalReload.owed = true
		}
	}
	m.rebuildPlanningFromRuntime()
	m.refreshLoadedTaskWorkspace()
	m.refreshOutstandingConflicts()
	cmds := m.refreshTaskViews(true, true)
	cmds = append(cmds, m.requestNoteSearchIndexLoad())
	if !m.externalReload.owed {
		m.automaticSync.stamps = stamps
	}
	// A refresh is ambient; it must not replace the save, undo, or refusal
	// the interaction that just closed is still reporting.
	if !m.externalReload.quiet && m.footerFeedbackLine() == "" {
		m.setInfoStatus("Reloaded: the workspace changed on disk")
		cmds = append(cmds, m.statusReadingTimeout())
	}
	m.externalReload.quiet = false
	return tea.Batch(cmds...)
}

func (m *AppModel) pollExternalWorkspaceChanges() {
	if m.automaticSync.exchange == nil || m.automaticSync.running {
		return
	}
	stamps := m.workspaceStamps()
	if !m.automaticSync.stamps.ChangedElsewhere(stamps, m.automaticSync.ownWrites) {
		// Whatever moved, this process's own commits moved it, and the session
		// already holds what they wrote: advance rather than reread it.
		m.automaticSync.stamps = stamps
		return
	}
	m.externalReload.owed = true
	m.externalReload.quiet = true
}
