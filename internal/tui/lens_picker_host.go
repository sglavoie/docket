package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// The shell side of the named-lens picker. The picker itself knows only the
// executor and its own rows; everything that reads live state or
// mounts another overlay lives here, so the picker stays a projection and the
// AppModel stays the one place overlays are created and torn down.

// namedLensPickerSnapshot reads the population and the live projection the
// picker draws. The current-view summary is rendered from a throwaway
// definition under no identity, which is exactly the record Save would write.
func (m *AppModel) namedLensPickerSnapshot() NamedLensPickerSnapshot {
	snapshot := NamedLensPickerSnapshot{}
	if m == nil {
		return snapshot
	}
	if m.runtime != nil {
		if machine := m.runtime.Machine(); machine != nil {
			snapshot.Lenses = machine.NamedLenses.Clone()
		}
	}
	if out := m.crossOutliner; out != nil {
		snapshot.CurrentSummary = namedLensSummary(out.captureLensState().namedLens("", ""))
		snapshot.Modified = out.lensModified()
		if out.activeLens != nil {
			snapshot.ActiveID = out.activeLens.lensID
		}
	}
	return snapshot
}

// openNamedLensPicker mounts the picker over the current route. It is the only
// constructor: the key that reaches it is bound with the rest of the release
// integration, and until then this is what tests and that binding both call.
func (m *AppModel) openNamedLensPicker() tea.Cmd {
	if m == nil {
		return nil
	}
	picker, err := NewNamedLensPickerModel(
		m.operationExecutor,
		m.currentView(),
		m.namedLensPickerSnapshot(),
	)
	if err != nil {
		m.err = err
		return statusTimeout(5 * time.Second)
	}
	m.lensPicker = picker
	return nil
}

// refreshNamedLensPicker re-reads the population after a mutation. The surface
// retains its cursor by row ID, so this alone satisfies the whole retention
// contract: rename and replace keep the lens selected through any alphabetical
// reordering, and delete drops the anchor and lands on the nearest survivor.
func (m *AppModel) refreshNamedLensPicker() {
	if m == nil || m.lensPicker == nil {
		return
	}
	if err := m.lensPicker.Refresh(m.namedLensPickerSnapshot()); err != nil {
		m.err = err
	}
}

// openNamedLensActions mounts the management sheet above the picker. The picker
// stays mounted underneath, which is what makes every cancel path inside the
// sheet land back on the list rather than on the Outliner.
func (m *AppModel) openNamedLensActions(msg NamedLensActionsRequestMsg) tea.Cmd {
	if m == nil || m.lensPicker == nil {
		return nil
	}
	sheet, err := NewContextActionSheetModel(
		m.operationRegistry,
		m.operationExecutor,
		m.lensPicker.commandContext(msg.Entity),
		namedLensActionSheetOperations,
		ContextActionSheetConfig{
			Host:         ContextActionHostLensPicker,
			Title:        "Lens actions",
			Subtitle:     msg.Name,
			ItemIDPrefix: "lens-action:",
			SecondaryFor: operationActionSecondaryText,
			Targets:      1,
		},
	)
	if err != nil {
		m.err = err
		return statusTimeout(5 * time.Second)
	}
	m.contextActionSheet = sheet
	return nil
}

// applyNamedLensPickerResult applies one execution the picker raised itself.
// A failure is already retained on the picker, a pending value or confirmation
// opens above it, and a completion refreshes the list — except for Apply and
// Create, whose whole point is the view behind the overlays.
func (m *AppModel) applyNamedLensPickerResult(msg NamedLensPickerResultMsg) tea.Cmd {
	switch {
	case msg.Result.Err != nil:
		return nil
	case msg.Result.ValueRequired != nil:
		return m.openOperationValueStep(*msg.Result.ValueRequired)
	case msg.Result.ConfirmationRequired != nil:
		m.showOperationConfirmation(*msg.Result.ConfirmationRequired)
		return nil
	}
	m.refreshNamedLensPicker()
	return operationResultCmd(msg.Result)
}

// closeNamedLensOverlays tears the whole lens stack down at once. Create is the
// one action that finishes on the view rather than in the list, so it leaves
// nothing above the newly active lens.
func (m *AppModel) closeNamedLensOverlays() {
	if m == nil {
		return
	}
	m.lensPicker = nil
	m.contextActionSheet = nil
	m.clearOperationValueStep()
	m.operationConfirm.Cancel()
	m.pendingOperationRequest = OperationExecutionRequest{}
}
