package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/syncexchange"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/util"
	"github.com/sglavoie/tt/internal/workspace"
)

// Automatic and manual exchange share one worker for enrolled desktop sessions.

const automaticSyncInterval = 5 * time.Second

// Only enrolled sessions own this timer. Folder I/O runs on an independent
// engine snapshot; all scheduling and view adoption stay on the UI goroutine.
type automaticSyncState struct {
	exchange func() (*syncexchange.FolderReport, error)
	// readStatus reads the sync status a finished pass leaves, on the pass's
	// own goroutine, so that read never runs on the UI goroutine. Without one
	// the finished pass reads it there.
	readStatus    func() syncStatusRead
	running       bool
	pending       bool
	localSequence func() (uint64, error)
	sequence      uint64
	// stamps is the workspace as the session last adopted it. Commits made in
	// this process advance it without a reload; any other change owes one.
	stamps workspace.Stamps
	// ownWrites is what this session's commits wrote, its sync worker's
	// included: the changes stamps advances over without a reload.
	ownWrites *storage.OwnWrites
	manual    bool // An explicit request wants completion feedback, even for a running pass.
	err       error
	// outcome is what the last finished pass left for the sync line to say; a
	// pass repeating it has nothing new for the status to report.
	outcome string
}

type automaticSyncTickMsg struct{}
type automaticSyncFinishedMsg struct {
	report  *syncexchange.FolderReport
	changed bool
	err     error
	// status is the sync status the pass read after exchanging, when the rule
	// syncStatusReadOwed applies said it would be needed.
	status *syncStatusRead
}

func (m *AppModel) automaticSyncTickCmd() tea.Cmd {
	if m.automaticSync.exchange == nil || m.lifecycle.quitting {
		return nil
	}
	return status.TickCmd(automaticSyncInterval, func(time.Time) tea.Msg {
		return automaticSyncTickMsg{}
	})
}

func (m *AppModel) startAutomaticSync() tea.Cmd {
	sync := &m.automaticSync
	if sync.exchange == nil || sync.running || m.lifecycle.quitting {
		return nil
	}
	sync.running = true
	exchange, readStatus := sync.exchange, sync.readStatus
	// What the finished handler will compare against is read here: one pass
	// runs at a time, and only its own finish moves the outcome.
	manual, owed, previous := sync.manual, m.feedback.syncRefreshOwed, sync.outcome
	return func() tea.Msg {
		report, err := exchange()
		changed := report != nil && (report.Imported > 0 ||
			(report.External != nil && report.External.TransactionID != ""))
		msg := automaticSyncFinishedMsg{report: report, changed: changed, err: err}
		if readStatus != nil && syncStatusReadOwed(msg, manual, owed, previous) {
			read := readStatus()
			msg.status = &read
		}
		return msg
	}
}

// syncStatusReadOwed is whether a finished pass must re-read the sync status.
// Reading it derives the open conflicts from the whole history, so only a pass
// that could have moved it pays for one: it changed history, published (a
// resolution among what it may have published), was asked for, or left the
// sync line something new to say, or the last read failed and is still owed.
func syncStatusReadOwed(msg automaticSyncFinishedMsg, manual, owed bool, previous string) bool {
	published := msg.report != nil && len(msg.report.Published) > 0
	return msg.changed || published || manual || owed ||
		automaticSyncOutcome(msg.report, msg.err) != previous
}

// backgroundSyncStatusReader reads the sync status from a runtime of its own
// on each call, which is what lets a pass read it off the UI goroutine.
func backgroundSyncStatusReader(load func() (*engine.Runtime, error)) func() syncStatusRead {
	if load == nil {
		return nil
	}
	return func() syncStatusRead {
		rt, err := load()
		if err != nil {
			return syncStatusRead{statusErr: err}
		}
		return readSyncStatus(rt.SyncStatus, rt.SyncConflicts)
	}
}

func (m *AppModel) handleAutomaticSyncMessage(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case automaticSyncTickMsg:
		m.pollExternalWorkspaceChanges()
		return true, tea.Batch(m.automaticSyncTickCmd(), m.adoptExternalWorkspaceChange(), m.startAutomaticSync())
	case automaticSyncFinishedMsg:
		m.automaticSync.running = false
		m.automaticSync.err = msg.err
		if msg.err != nil {
			logging.Warn("automatic sync: %v", msg.err)
		}
		// A manual request also picks up changes a separate CLI exchange already
		// applied, even when this pass has nothing left to import.
		if msg.changed || m.automaticSync.manual {
			m.externalReload.owed = true
		}
		m.pollExternalWorkspaceChanges()
		// Any pass syncStatusReadOwed spares only keeps the sync line's age
		// current. One it does not spare applies what it read off the UI
		// goroutine, and reads here only when it carried none: a manual
		// request that joined it, or a failed read owed since it started.
		outcome := automaticSyncOutcome(msg.report, msg.err)
		if syncStatusReadOwed(msg, m.automaticSync.manual, m.feedback.syncRefreshOwed, m.automaticSync.outcome) {
			if msg.status != nil {
				m.applySyncStatus(*msg.status)
				m.recountOutstandingConflicts()
			} else {
				m.refreshOutstandingConflicts()
			}
		} else {
			if status := m.feedback.syncStatus; status != nil && msg.report != nil && msg.report.Succeeded {
				status.LastSuccess = msg.report.At
			}
			m.renderSyncAdvisory()
		}
		m.automaticSync.outcome = outcome
		cmd := m.adoptExternalWorkspaceChange()
		if m.automaticSync.pending {
			m.automaticSync.pending = false
			cmd = tea.Batch(cmd, m.startAutomaticSync())
		}
		if m.automaticSync.manual {
			m.automaticSync.manual = false
			return true, tea.Batch(cmd, m.reportManualSync(msg.report, msg.err))
		}
		return true, cmd
	default:
		return false, nil
	}
}

// automaticSyncOutcome spells everything of a finished pass the sync line
// reads apart from its age: whether it failed, what is still waiting either
// way, and the updates its holds and peers call for.
func automaticSyncOutcome(report *syncexchange.FolderReport, err error) string {
	if report == nil {
		return fmt.Sprintf("failed=%t", err != nil)
	}
	upgrades := make([]string, 0, len(report.Held))
	for _, held := range report.Held {
		upgrades = append(upgrades, held.Upgrade)
	}
	schemas := make([]string, 0, len(report.Peers))
	for _, peer := range report.Peers {
		schemas = append(schemas, fmt.Sprintf("%s=%v", peer.DeviceID, peer.Schemas))
	}
	return fmt.Sprintf("failed=%t succeeded=%t up=%d down=%d unavailable=%d errors=%d held=%q peers=%q",
		err != nil, report.Succeeded, report.OutstandingUp, report.OutstandingDown,
		len(report.Unavailable), len(report.Errors), upgrades, schemas)
}

// startManualSync joins an in-flight pass rather than starting overlapping I/O.
// Scheduling and feedback belong to Update; the worker only returns a report.
func (m *AppModel) startManualSync() tea.Cmd {
	if m.lifecycle.quitting {
		return nil
	}
	if m.automaticSync.exchange == nil {
		m.setWarningStatus("This workspace is not enrolled in sync; use tt sync enroll or tt sync join")
		return m.statusReadingTimeout()
	}
	m.automaticSync.manual = true
	if m.automaticSync.running {
		m.setInfoStatus("Sync already in progress…")
	} else {
		m.setInfoStatus("Syncing…")
	}
	return tea.Batch(m.startAutomaticSync(), m.statusReadingTimeout())
}

func (m *AppModel) reportManualSync(report *syncexchange.FolderReport, err error) tea.Cmd {
	if err != nil {
		m.setWarningStatus("Sync failed: " + err.Error() + " (tt sync doctor)")
		return m.statusReadingTimeout()
	}
	if report == nil {
		m.setWarningStatus("Sync returned no result; check tt sync status")
		return m.statusReadingTimeout()
	}
	parts := []string{fmt.Sprintf("Sync: %d published, %d imported", len(report.Published), report.Imported)}
	if report.OutstandingUp > 0 {
		parts = append(parts, fmt.Sprintf("%d waiting to publish", report.OutstandingUp))
	}
	if report.OutstandingDown > 0 {
		parts = append(parts, fmt.Sprintf("%d waiting to apply", report.OutstandingDown))
	}
	if len(report.Unavailable) > 0 {
		parts = append(parts, "files not downloaded yet")
	}
	if len(report.Held) > 0 {
		parts = append(parts, "update tt to apply held changes (tt sync status)")
	}
	if report.External != nil && len(report.External.Held) > 0 {
		parts = append(parts, "local edits held (tt sync status)")
	}
	if len(report.Errors) > 0 {
		parts = append(parts, "exchange errors (tt sync doctor)")
	}
	if count := len(m.feedback.syncConflicts); count > 0 {
		parts = append(parts, manualSyncConflictPart(count))
	}
	if m.externalReload.owed {
		parts = append(parts, "view refresh pending")
	}
	message := strings.Join(parts, " · ")
	if len(report.Errors) > 0 || len(report.Held) > 0 || (report.External != nil && len(report.External.Held) > 0) {
		m.setWarningStatus(message)
	} else {
		m.setInfoStatus(message)
	}
	return m.statusReadingTimeout()
}

// requestAutomaticSync coalesces explicit events into one follow-up pass. The
// periodic timer uses startAutomaticSync directly, so a slow provider does not
// create a permanent stream of queued retries merely by missing a timer tick.
func (m *AppModel) requestAutomaticSync() tea.Cmd {
	if m.automaticSync.exchange == nil || m.lifecycle.quitting {
		return nil
	}
	if m.automaticSync.running {
		m.automaticSync.pending = true
		return nil
	}
	return m.startAutomaticSync()
}

func (m *AppModel) syncAfterLocalCommit() tea.Cmd {
	sync := &m.automaticSync
	if sync.localSequence == nil || m.lifecycle.quitting {
		return nil
	}
	sequence, err := sync.localSequence()
	if err != nil || sequence == sync.sequence {
		return nil
	}
	sync.sequence = sequence
	return m.requestAutomaticSync()
}

// manualSyncConflictPart names the sync conflicts a manual pass left behind in
// the same words the status strip's conflict line uses, in agreement with the
// count.
func manualSyncConflictPart(count int) string {
	return util.Count(count, "unresolved sync conflict") + " (gc to review)"
}
