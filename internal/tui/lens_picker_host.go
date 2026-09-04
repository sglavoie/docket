package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
)

// The shell side of the named-lens picker. The picker itself knows only the
// executor and its own rows; everything that reads live state or
// mounts another overlay lives here, so the picker stays a projection and the
// AppModel stays the one place overlays are created and torn down.

// namedLensPickerSnapshot reads the population and the live projection the
// picker draws. The current-view summary is rendered from a throwaway
// definition under no identity, which is exactly the record Save would write.
func (m *AppModel) namedLensPickerSnapshot() lens.NamedLensPickerSnapshot {
	snapshot := lens.NamedLensPickerSnapshot{}
	if m == nil {
		return snapshot
	}
	if m.runtime != nil {
		if machine := m.runtime.Machine(); machine != nil {
			snapshot.Lenses = machine.NamedLenses.Clone()
		}
	}
	if out := m.crossOutliner; out != nil {
		snapshot.CurrentSummary = lens.NamedLensSummary(out.captureLensState().namedLens("", ""))
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
	picker, err := lens.NewNamedLensPickerModel(
		m.operationExecutor,
		m.currentView(),
		m.namedLensPickerSnapshot(),
	)
	if err != nil {
		m.err = err
		return status.StatusTimeout(5 * time.Second)
	}
	m.lensPicker = picker
	return nil
}

// saveViewAsLens runs Create over the live view without the picker in front of
// it. The query readout raises it, and the whole of the difference from the
// picker's own Save row is where the request comes from: the same descriptor,
// the same executor, and the same name prompt answer it.
//
// CapabilityLensPicker is granted for this one request. The picker is still the
// only surface that grants it for a cursor row, so the four lens operations that
// need a record under the cursor stay unreachable from here — Create is the one
// that names no record, which is why it is the one the readout can ask for.
func (m *AppModel) saveViewAsLens() tea.Cmd {
	if m == nil || m.operationExecutor == nil {
		return nil
	}
	ctx := route.PresentationCommandContext(m.currentView())
	ctx.Capabilities.Add(operation.CapabilityLensPicker)
	result := m.operationExecutor.Execute(operation.OperationExecutionRequest{
		ID:      lens.OperationLensCreate,
		Context: ctx,
	})
	if result.ValueRequired != nil {
		return m.openOperationValueStep(*result.ValueRequired)
	}
	if result.Err != nil {
		m.err = result.Err
		return status.StatusTimeout(5 * time.Second)
	}
	return operationResultCmd(result)
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
func (m *AppModel) openNamedLensActions(msg lens.NamedLensActionsRequestMsg) tea.Cmd {
	if m == nil || m.lensPicker == nil {
		return nil
	}
	sheet, err := listsurface.NewContextActionSheetModel(
		m.operationRegistry,
		m.operationExecutor,
		m.lensPicker.CommandContext(msg.Entity),
		lens.NamedLensActionSheetOperations,
		listsurface.ContextActionSheetConfig{
			Host:         listsurface.ContextActionHostLensPicker,
			Title:        "Lens actions",
			Subtitle:     msg.Name,
			ItemIDPrefix: "lens-action:",
			SecondaryFor: operationActionSecondaryText,
			Targets:      1,
		},
	)
	if err != nil {
		m.err = err
		return status.StatusTimeout(5 * time.Second)
	}
	m.contextActionSheet = sheet
	return nil
}

// applyNamedLensPickerResult applies one execution the picker raised itself.
// A failure is already retained on the picker, a pending value or confirmation
// opens above it, and a completion refreshes the list — except for Apply,
// Create, and Add built-in view, whose whole point is the view behind the
// overlays and which have already torn the stack down by the time this runs.
func (m *AppModel) applyNamedLensPickerResult(msg lens.NamedLensPickerResultMsg) tea.Cmd {
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

// closeNamedLensOverlays tears the whole lens stack down at once. Create and Add
// built-in view are the two actions that finish on the view rather than in the
// list — each saves a record and immediately applies it — so both leave nothing
// above the newly active lens. Their nested step differs (a name prompt versus
// the template gallery) but the teardown does not, because clearing the value
// step covers either one.
func (m *AppModel) closeNamedLensOverlays() {
	if m == nil {
		return
	}
	m.lensPicker = nil
	m.contextActionSheet = nil
	m.clearOperationValueStep()
	m.operationConfirm.Cancel()
	m.pendingOperationRequest = operation.OperationExecutionRequest{}
}
