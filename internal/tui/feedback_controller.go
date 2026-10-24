package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/tui/conflict"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/textfit"
)

// feedbackController is the shell's status and feedback cluster: the one line
// the footer speaks with, the error that outranks it, the undo callback and the
// transaction correlation a recovery toast carries, the divider flash a change
// to any of them arms, and the two ambient inputs the Outliner's status strip
// reads back — the outstanding-conflict count and whether advisories are shown
// at all.
//
// The field is named feedback, not status, because status is an imported
// package name inside the shell: the same shadowing reason the notes cluster is
// notes and the operation-wiring cluster is operations.
//
// Naming it as a cluster is also what keeps the shell's status line
// distinguishable from a view's. InteractiveViewState and BaseView declare
// statusMessage, err, width and height too, and merging the two namespaces
// would be a behavior change: a view owns a status line the shell never writes,
// and footerFlashProbe compares the two independently. Qualifying the shell's
// through this cluster is what makes every read site say which one it means,
// the same way the geometry pair now says viewNav.
//
// It owns the text and the deadlines, not the effects. Every setter stays an
// AppModel method, because a status change is where a committed transaction, a
// workspace-pulse refresh and a ledger read meet; nothing here touches storage,
// the runtime snapshot, or a view.
//
// Only warning and error status text arms the footer flash, and the arming
// itself stays in the one deferred closure in AppModel.Update. This type
// advances the sequence a setter records and holds the frames a tick steps; it
// decides nothing about when a flash starts, which severity is quiet, or which
// one supersedes another.
type feedbackController struct {
	// err is the shell's own failure line. It outranks status text in the
	// footer and, having no clock of its own, is compared by text rather than by
	// sequence when the flash probe runs.
	err error
	// The text field that received a draft failure. Typing in covering Help or
	// search must not dismiss an error belonging to the editor beneath it.
	errDraftKey any

	// message is the transient status line, info its neutral surface, and
	// updatedAt the stamp that guards it from a stale clear timer.
	history   []recalledFeedback // session-only recall, independent of expiry
	message   string
	info      bool
	updatedAt time.Time

	// undoCb is the callback the inline `u` invokes while the toast is live, and
	// expiresAt the deadline past which it is no longer offered.
	undoCb    func() tea.Cmd
	expiresAt time.Time

	// transactionID correlates the active recovery toast with a committed
	// workspace transaction, so delayed origin-history feedback cannot decorate
	// a newer status; recoveryFallback is the plain success text that correlation
	// degrades to, and recoveryWords the outcome phrasing — empty for every
	// mutation whose ledger entry already names its own result — that a re-render
	// of the same toast must keep saying.
	transactionID    string
	recoveryFallback string
	recoveryWords    recoveryWords

	// seq is bumped by every status setter and is what the footer flash reads a
	// change off; severity is the newest status text's severity and is what
	// decides whether that change animates at all. Clearing advances neither,
	// which is what makes cleared text arm nothing.
	seq      uint64
	severity footerFlashSeverity

	// flash is the live divider animation and flashSeq the generation that
	// rejects a tick from a superseded one.
	flash    footerFlashState
	flashSeq uint64

	// outstandingConflicts is how many bundles `.tt/conflicts/` holds, cached so
	// no render reads the directory. It is deliberately not part of
	// WorkspacePulse: that ladder is a closed ordinal with no seam for
	// filesystem-derived state.
	outstandingConflicts int

	// syncConflicts are an enrolled workspace's open sync conflicts, counted in
	// outstandingConflicts beside the bundles and listed by the browser after
	// them; syncAdvisory is the strip's sync line, drawn from syncStatus, the
	// status last read. All three are refreshed together and read by no render
	// from disk.
	syncConflicts []conflict.SyncConflictEntry
	syncAdvisory  string
	syncStatus    *engine.SyncStatus
	// syncRefreshOwed records that the last refresh failed to read the status
	// or the conflicts under it, which leaves all three empty rather than
	// current: the next exchange pass reads again whatever else it found.
	syncRefreshOwed bool

	// hideAdvisories suppresses every ambient advisory for this session: the
	// conflict count and the load-failure notices alike. Both the machine-local
	// setting and the process-level `--no-advisories` flag feed it; see
	// advisories_shell.go.
	hideAdvisories bool
}

// announce installs one status line and releases everything the previous one
// carried. A non-undoable status must not leave undoCb armed: a later `u` would
// silently invoke the previous destructive action's undo while unrelated text is
// on screen. Severity is a separate axis and is recorded by the caller, so this
// writes no sequence and therefore arms no flash on its own.
func (c *feedbackController) announce(message string, info bool, at time.Time) {
	c.message = message
	c.info = info
	c.updatedAt = at
	c.undoCb = nil
	c.expiresAt = time.Time{}
	c.transactionID = ""
	c.recoveryFallback = ""
	c.recoveryWords = recoveryWords{}
}

// recordSeverity advances the status generation the footer flash arms on. Only
// the setters call it, so a cleared status never flashes.
func (c *feedbackController) recordSeverity(severity footerFlashSeverity) {
	c.severity = severity
	c.seq++
}

// armUndo installs the inline undo callback and the deadline it is offered
// until. It runs after announce, which is what clears any previous callback
// first.
func (c *feedbackController) armUndo(undoCb func() tea.Cmd, expiresAt time.Time) {
	c.undoCb = undoCb
	c.expiresAt = expiresAt
}

// correlate ties the live toast to a committed transaction and records both the
// plain text a superseded correlation degrades to and the outcome phrase a
// re-render of this same toast must keep, so delayed origin feedback rewrites
// the decoration and never the answer.
func (c *feedbackController) correlate(transactionID, fallback string, words recoveryWords) {
	c.transactionID = transactionID
	c.recoveryFallback = fallback
	c.recoveryWords = words
}

// undoLive reports whether the inline undo is still offered at now.
func (c *feedbackController) undoLive(now time.Time) bool {
	return c.undoCb != nil && now.Before(c.expiresAt)
}

// undoLapsed reports an armed callback whose own window has closed. It is the
// exact complement of undoLive over an armed toast, and it is what lets a
// timeout tick clear the toast it was scheduled for without dismissing a newer
// one that has not reached its deadline yet.
func (c *feedbackController) undoLapsed(now time.Time) bool {
	return c.undoCb != nil && !now.Before(c.expiresAt)
}

// correlates reports whether a delayed origin-history result still belongs to
// the toast on screen: the transaction must match and the undo window must still
// be open.
func (c *feedbackController) correlates(transactionID string, now time.Time) bool {
	return transactionID != "" && transactionID == c.transactionID && c.undoLive(now)
}

// clear drops the status line, the error beside it, and the severity the flash
// would have read. It advances no sequence, so clearing arms nothing.
func (c *feedbackController) clear() {
	c.announce("", false, time.Time{})
	c.severity = flashNone
	c.err = nil
	c.errDraftKey = nil
}

// stale reports whether a clear scheduled at scheduledAt has been overtaken by
// newer status text. A zero deadline is an unconditional clear.
func (c *feedbackController) stale(scheduledAt time.Time) bool {
	return !scheduledAt.IsZero() && !c.updatedAt.Before(scheduledAt)
}

// appendDetail adds domain-specific consequence text while keeping the recovery
// controls in their canonical trailing order. It is a no-op without an armed
// callback, because unverified feedback must not resemble a correlated recovery
// toast.
func (c *feedbackController) appendDetail(detail string) {
	detail = strings.TrimSpace(detail)
	if detail == "" || c.undoCb == nil {
		return
	}
	head, _, origin := splitRecoverySuffix(c.message)
	c.message = head + " · " + detail + recoveryUndoSuffix
	if origin {
		c.message += recoveryOriginSuffix
	}
}

// appendNote adds an independent observation to the line already speaking,
// keeping the recovery controls in their canonical trailing order. It differs
// from appendDetail in what it may claim rather than in what it writes: a detail
// describes the transaction the toast names, so it is offered only beside an
// armed callback, while a note stands on committed evidence of its own and rides
// whichever line this Update published. It reports whether it found one, so a
// caller with nothing to ride can publish the note itself instead of dropping
// it silently.
func (c *feedbackController) appendNote(note string) bool {
	note = strings.TrimSpace(note)
	if note == "" || strings.TrimSpace(c.message) == "" {
		return false
	}
	head, undo, origin := splitRecoverySuffix(c.message)
	c.message = head + " · " + note
	if undo {
		c.message += recoveryUndoSuffix
	}
	if origin {
		c.message += recoveryOriginSuffix
	}
	return true
}

// The recovery grammar's two endings, in the order a line carries them. They
// are spelled here alone: every splice and every fit reads splitRecoverySuffix.
const (
	recoveryUndoSuffix   = " · u undo"
	recoveryOriginSuffix = " · Ctrl+O origin"
)

// splitRecoverySuffix peels the recovery controls off the end of a feedback
// line, reporting which of the two it carried.
func splitRecoverySuffix(message string) (head string, undo, origin bool) {
	head, origin = strings.CutSuffix(message, recoveryOriginSuffix)
	head, undo = strings.CutSuffix(head, recoveryUndoSuffix)
	return head, undo, origin
}

// fitRecoveryFeedback fits one feedback line to width. A line ending in the
// recovery controls gives up its head instead of its tail, since the keys are
// the half a reader can still act on and the head names something the row and
// Peek still show. A line carrying neither, or a row too narrow to hold them
// beside any head at all, is cut from the right as before.
func fitRecoveryFeedback(text string, width int) string {
	if lipgloss.Width(text) <= width {
		return text
	}
	head, undo, origin := splitRecoverySuffix(text)
	suffix := ""
	if undo {
		suffix += recoveryUndoSuffix
	}
	if origin {
		suffix += recoveryOriginSuffix
	}
	headWidth := width - lipgloss.Width(suffix)
	if suffix == "" || headWidth < 2 {
		return textfit.TruncateToVisualWidth(text, width)
	}
	return textfit.TruncateToVisualWidth(head, headWidth) + suffix
}

// armFlash starts a divider flash at the given severity and returns the
// generation that identifies it. The decision to arm is the caller's — it lives
// in the one deferred closure in AppModel.Update — so this never inspects the
// status text it is flashing for.
func (c *feedbackController) armFlash(severity footerFlashSeverity) uint64 {
	c.flashSeq++
	c.flash = footerFlashState{severity: severity, seq: c.flashSeq}
	return c.flashSeq
}

// retireFlash cancels a live divider animation and advances the generation, so
// every tick already queued for it is rejected when it arrives. It is what quiet
// feedback does to the animated warning it replaces: the moving glyph and the
// footer wash belong to that warning's text, and leaving them running under the
// line that superseded it would say the new line is the alert.
//
// Nothing to retire is not a supersession, so an idle cluster is left alone and
// the generation only counts animations that actually ran.
func (c *feedbackController) retireFlash() {
	if !c.flash.active() {
		return
	}
	c.flash = footerFlashState{}
	c.flashSeq++
}

// advanceFlash steps one frame and reports whether another is owed. A tick from
// a superseded flash is rejected by sequence, and the last frame retires the
// animation rather than scheduling past it.
func (c *feedbackController) advanceFlash(seq uint64) bool {
	if !c.flash.active() || c.flash.seq != seq {
		return false
	}
	c.flash.frame++
	if c.flash.frame >= footerFlashFrames {
		c.flash = footerFlashState{}
		return false
	}
	return true
}

// advisoriesHidden reports the shell's live setting. A nil cluster reads as
// showing, so nothing here can turn a missing shell into a silent one. It is the
// accessor injected into every Outliner; see wireOutlinerOperationExecutor.
func (c *feedbackController) advisoriesHidden() bool {
	if c == nil {
		return false
	}
	return c.hideAdvisories
}

// outstandingConflictCount is the other injected accessor, so the status strip
// reads one shell-owned count instead of touching the filesystem during a
// render.
// syncAdvisoryLine is the injected accessor for the strip's sync line.
func (c *feedbackController) syncAdvisoryLine() string {
	if c == nil {
		return ""
	}
	return c.syncAdvisory
}

func (c *feedbackController) outstandingConflictCount() int {
	if c == nil {
		return 0
	}
	return c.outstandingConflicts
}

const feedbackHistoryLimit = 20

type recalledFeedback struct {
	help.RecalledMessage
	source string
	seq    uint64
}

// rememberFeedback amends a generation already in the ring, or records a new
// event. Error text has no generation and must always be a separate event.
func (c *feedbackController) rememberFeedback(text, source string, seq uint64) {
	if text == "" {
		return
	}
	if source != "error" {
		for i := len(c.history) - 1; i >= 0; i-- {
			if c.history[i].source == source && c.history[i].seq == seq {
				c.history[i].Text = text
				return
			}
		}
	}
	kind := "Feedback"
	if source == "error" {
		kind = "Error"
	}
	c.history = append(c.history, recalledFeedback{RecalledMessage: help.RecalledMessage{Text: text, At: time.Now(), Kind: kind}, source: source, seq: seq})
	if len(c.history) > feedbackHistoryLimit {
		copy(c.history, c.history[1:])
		c.history = c.history[:feedbackHistoryLimit]
	}
}

// feedbackSnapshot detaches the viewer from subsequent feedback, newest first.
func (c *feedbackController) feedbackSnapshot() []help.RecalledMessage {
	messages := make([]help.RecalledMessage, len(c.history))
	for i, entry := range c.history {
		messages[len(c.history)-1-i] = entry.RecalledMessage
	}
	return messages
}
