package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/control"
	"github.com/sglavoie/docket/internal/icons"
	"github.com/sglavoie/docket/internal/logging"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/pulse"
)

// The shell half of the advisory opt-out. advisories.go owns what an advisory
// is and the two helpers every ambient line routes through; this file owns the
// session value those helpers read, which is where the machine-local setting and
// the process-level `--no-advisories` flag meet.
//
// The suppression is session state on the shell rather than a read of the
// machine file, because two inputs decide it: the machine-local flag the menu
// toggle writes, and the process-level `--no-advisories` flag, which forces
// advisories hidden for one run and writes nothing. The toggle commits, so
// toggling back after a flagged launch persists "shown" — the state a user can
// see is the state that was saved.

// The session value itself lives on the feedback cluster beside the conflict
// count, because both are ambient status-strip inputs and both travel into an
// Outliner as the same kind of injected accessor; see feedback_controller.go.

// WithHiddenAdvisories starts the session with advisories suppressed. RunApp
// translates the `--no-advisories` flag into this option; the machine-local
// setting is read in NewAppModel, and either one hiding is enough.
func WithHiddenAdvisories() AppOption {
	return func(m *AppModel) { m.feedback.hideAdvisories = true }
}

// coreToggleAdvisories flips the machine-local setting and the session value
// together. It mirrors coreToggleMenuBar: chrome follows the terminal you are
// sitting at, so the durable home is machine.json rather than the workspace.
func (m *AppModel) coreToggleAdvisories() error {
	if m.domain.runtime == nil || m.domain.runtime.Machine() == nil {
		return fmt.Errorf("machine chrome state is unavailable")
	}
	machine := *m.domain.runtime.Machine()
	machine.AdvisoriesHidden = !m.feedback.hideAdvisories
	if err := m.domain.runtime.CommitMachine(&machine); err != nil {
		return fmt.Errorf("save advisory visibility: %w", err)
	}
	m.feedback.hideAdvisories = machine.AdvisoriesHidden
	if m.feedback.hideAdvisories {
		m.setInfoStatus("Advisories hidden")
	} else {
		m.setInfoStatus("Advisories shown")
	}
	return nil
}

// The shell half of the outstanding-conflict indicator. conflict_indicator.go
// owns the line the Outliner's status strip draws; this file owns the count it
// draws, which is cached here so no render reads the filesystem.
//
// The store is read straight from the storage layer rather than through an
// internal/app port, following the direct storage.LoadProject reads the TUI
// already makes: nothing here validates, mutates, or commits, so there is no
// application-layer policy for a port to hold.

// countOutstandingConflicts reports how many conflict bundles the workspace has
// recorded. An absent directory is zero, not an error, and a read failure is
// also zero: an indicator that cannot be counted must not be asserted.
func countOutstandingConflicts(dataDir string) int {
	if dataDir == "" {
		return 0
	}
	records, err := storage.NewConflictStore(config.ConflictsDir(dataDir)).List()
	if err != nil {
		logging.Warn("counting outstanding conflicts: %v", err)
		return 0
	}
	return len(records)
}

// refreshOutstandingConflicts recounts the workspace's conflict bundles and
// re-reads the sync status beside them. It runs at launch, when a reload adopts
// a change, when `gc` opens or resolves, and after an exchange pass that could
// have moved either; it arms no timer of its own.
func (m *AppModel) refreshOutstandingConflicts() {
	if m == nil || m.domain.runtime == nil {
		return
	}
	// What another device delivered or left in conflict is as invisible to a
	// local mutation as a bundle another process wrote.
	m.refreshSyncStatus()
	m.recountOutstandingConflicts()
}

// recountOutstandingConflicts recounts the conflict bundles against the sync
// conflicts last read and redraws the sync line from the status last read. It
// is the 60s tick's whole refresh in a session exchanging on its own timer:
// deriving sync conflicts costs a walk of the history, and there every pass
// that could have changed them has already read them again.
func (m *AppModel) recountOutstandingConflicts() {
	if m == nil || m.domain.runtime == nil {
		return
	}
	m.renderSyncAdvisory()
	m.feedback.outstandingConflicts = countOutstandingConflicts(m.domain.runtime.DataDir()) + len(m.feedback.syncConflicts)
}

// The shell half of the mouse opt-out, on the advisory precedent exactly (see
// the top of this file). Two inputs decide it: the machine-local flag the View
// menu's Mouse row writes, and the process-level `--no-mouse` flag, which turns
// reporting off for one run and writes nothing. The toggle commits, so toggling
// back on after a flagged launch persists "on" — the state a user can see is the
// state that was saved.
//
// The session value is read by View on every frame, so the toggle takes effect
// on the next paint: Bubble Tea diffs the view's mouse mode and writes the
// terminal mode change itself.

// WithDisabledMouse starts the session with mouse reporting off. RunApp
// translates the `--no-mouse` flag into this option; the machine-local setting
// is read in NewAppModel, and either one turning it off is enough.
func WithDisabledMouse() AppOption {
	return func(m *AppModel) { m.pointer.disabled = true }
}

// mouseDisabled reports the shell's live setting. It is the accessor injected
// into every Outliner so the View menu's Mouse row can show it.
func (m *AppModel) mouseDisabled() bool {
	return m != nil && m.pointer.disabled
}

// viewMouseMode is the mouse mode View declares: cell motion, which reports
// clicks, releases and wheel steps but no bare motion, unless the session turned
// the mouse off. A headless session has no renderer, so nothing it declares here
// ever reaches a terminal.
func (m *AppModel) viewMouseMode() tea.MouseMode {
	if m.mouseDisabled() {
		return tea.MouseModeNone
	}
	return tea.MouseModeCellMotion
}

// coreToggleMouse flips the machine-local setting and the session value
// together. Mouse reporting is chrome that follows the terminal you are sitting
// at, so its durable home is machine.json rather than the workspace.
func (m *AppModel) coreToggleMouse() error {
	if m.domain.runtime == nil || m.domain.runtime.Machine() == nil {
		return fmt.Errorf("machine chrome state is unavailable")
	}
	machine := *m.domain.runtime.Machine()
	machine.MouseDisabled = !m.pointer.disabled
	if err := m.domain.runtime.CommitMachine(&machine); err != nil {
		return fmt.Errorf("save mouse setting: %w", err)
	}
	m.pointer.disabled = machine.MouseDisabled
	// A click remembered from before the switch is not half of a double-click
	// made after it.
	m.pointer.lastClickAt = time.Time{}
	if m.pointer.disabled {
		m.setInfoStatus("Mouse off")
	} else {
		m.setInfoStatus("Mouse on")
	}
	return nil
}

// Two machine-local presentation preferences on the mouse precedent (see
// above): Motion and the Nerd Font glyph set. Each is a keyless,
// target-free View menu row and a durable machine.json key, and each has a
// session-only override a host or the process environment supplies.

// glyphsEnv names the process-level glyph override. Docket.app sets it when it
// spawns docket inside its bundled terminal, whose font carries the Nerd set.
const glyphsEnv = "DOCKET_GLYPHS"

// glyphPreferences is the shell's session value for the glyph set. nerd is what
// icons.SetNerdGlyphs was last told; forced records that DOCKET_GLYPHS decided
// it at launch, which makes the View toggle session-only: persisting a choice
// the environment overrides on every launch would save a state nobody sees.
type glyphPreferences struct {
	nerd   bool
	forced bool
}

// glyphOverrideFromEnv reads DOCKET_GLYPHS: "nerd" forces the Nerd set,
// "unicode" forces the Unicode set, and anything else leaves the preference.
func glyphOverrideFromEnv(value string) (nerd, ok bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "nerd":
		return true, true
	case "unicode":
		return false, true
	}
	return false, false
}

// WithGlyphOverride forces the glyph set for this session and writes nothing.
// RunApp translates DOCKET_GLYPHS into it.
func WithGlyphOverride(nerd bool) AppOption {
	return func(m *AppModel) { m.glyphs = glyphPreferences{nerd: nerd, forced: true} }
}

// applyGlyphs publishes the session value to the one icon registry, so every
// render after it draws from that set. Nothing is cached per glyph, so the next
// frame is the whole switch.
func (m *AppModel) applyGlyphs() {
	icons.SetNerdGlyphs(m.glyphs.nerd)
}

// nerdGlyphs reports the session's glyph set; it ticks the View menu's row.
func (m *AppModel) nerdGlyphs() bool { return m != nil && m.glyphs.nerd }

// coreToggleNerdGlyphs flips the glyph set. Without an environment override it
// commits machine.json first, as the mouse toggle does; under one it changes the
// session alone and says so.
func (m *AppModel) coreToggleNerdGlyphs() error {
	next := !m.glyphs.nerd
	if !m.glyphs.forced {
		if m.domain.runtime == nil || m.domain.runtime.Machine() == nil {
			return fmt.Errorf("machine chrome state is unavailable")
		}
		machine := *m.domain.runtime.Machine()
		machine.NerdGlyphs = next
		if err := m.domain.runtime.CommitMachine(&machine); err != nil {
			return fmt.Errorf("save glyph setting: %w", err)
		}
	}
	m.glyphs.nerd = next
	m.applyGlyphs()
	label := "Nerd Font glyphs off"
	if next {
		label = "Nerd Font glyphs on"
	}
	if m.glyphs.forced {
		label += " for this session (" + glyphsEnv + " decides at launch)"
	}
	m.setInfoStatus(label)
	return nil
}

// applyMotion derives the session's motion from its three inputs and hands it
// to the Outliner. Turning it off drops whatever is animating at once: the
// Outliner's ghost and arrival, the undo countdown, and a lit pulse.
func (m *AppModel) applyMotion() {
	c := &m.lifecycle
	c.motion = c.motionArmed && !c.motionOff && !c.hostReducesMotion
	if !c.motion {
		m.feedback.countdown.live = false
		m.pulseMotion.seq++
		m.pulseMotion.change = pulse.PulseChange{}
	}
	if m.crossOutliner != nil {
		m.crossOutliner.SetMotion(c.motion)
	}
}

// motionPreferred reports the preference, which is what the View menu's Motion
// row ticks: a launch the harness never arms, and a host's Reduce Motion, both
// hold motion off without unticking what the user chose.
func (m *AppModel) motionPreferred() bool { return m != nil && !m.lifecycle.motionOff }

// coreToggleMotion flips the machine-local preference and the session together.
func (m *AppModel) coreToggleMotion() error {
	if m.domain.runtime == nil || m.domain.runtime.Machine() == nil {
		return fmt.Errorf("machine chrome state is unavailable")
	}
	machine := *m.domain.runtime.Machine()
	machine.MotionDisabled = !m.lifecycle.motionOff
	if err := m.domain.runtime.CommitMachine(&machine); err != nil {
		return fmt.Errorf("save motion setting: %w", err)
	}
	m.lifecycle.motionOff = machine.MotionDisabled
	m.applyMotion()
	switch {
	case m.lifecycle.motionOff:
		m.setInfoStatus("Motion off")
	case m.lifecycle.hostReducesMotion:
		m.setInfoStatus("Motion on (held off while the system reduces motion)")
	default:
		m.setInfoStatus("Motion on")
	}
	return nil
}

// controlReduceMotion answers control.OpReduceMotion: "on" holds motion off for
// the session whatever the preference says, "off" returns it to the preference.
// It writes no machine state.
func (m *AppModel) controlReduceMotion(text string, reply chan<- control.Reply) {
	switch text {
	case "on":
		m.lifecycle.hostReducesMotion = true
	case "off":
		m.lifecycle.hostReducesMotion = false
	default:
		answer(reply, false, fmt.Sprintf("unknown reduce motion %q: want on or off", text))
		return
	}
	m.applyMotion()
	answer(reply, true, "Reduce motion: "+text)
}
