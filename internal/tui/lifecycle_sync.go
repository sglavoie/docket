package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/syncexchange"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/util"
)

// Automatic and manual exchange share one worker for enrolled desktop sessions.

const automaticSyncInterval = 5 * time.Second

// Only enrolled sessions own this timer. Folder I/O runs on an independent
// engine snapshot; all scheduling and view adoption stay on the UI goroutine.
type automaticSyncState struct {
	exchange      func() (*syncexchange.FolderReport, error)
	running       bool
	pending       bool
	localSequence func() (uint64, error)
	sequence      uint64
	fingerprint   string
	manual        bool // An explicit request wants completion feedback, even for a running pass.
	err           error
	// outcome is what the last finished pass left for the sync line to say; a
	// pass repeating it has nothing new for the status to report.
	outcome string
}

type automaticSyncTickMsg struct{}
type automaticSyncFinishedMsg struct {
	report  *syncexchange.FolderReport
	changed bool
	err     error
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
	exchange := sync.exchange
	return func() tea.Msg {
		report, err := exchange()
		changed := report != nil && (report.Imported > 0 ||
			(report.External != nil && report.External.TransactionID != ""))
		return automaticSyncFinishedMsg{report: report, changed: changed, err: err}
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
		// Reading the sync status derives the open conflicts from the whole
		// history, so only a pass that could have moved it pays for one: it
		// changed history, published (a resolution among what it may have
		// published), was asked for, or left the sync line something new to
		// say, or the last read failed and is still owed. Any other pass only
		// keeps that line's age current.
		outcome := automaticSyncOutcome(msg.report, msg.err)
		published := msg.report != nil && len(msg.report.Published) > 0
		if msg.changed || published || m.automaticSync.manual || outcome != m.automaticSync.outcome || m.feedback.syncRefreshOwed {
			m.refreshOutstandingConflicts()
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
