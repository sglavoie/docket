package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// Named lens definitions are portable: they live in the synced
// .tt/lenses.json, and only the active-lens pointer stays machine-local. This
// file still edits them through the machine domain, because that domain carries
// the in-memory collection and its commit publishes the portable home behind
// it. That is the whole shape of this file. Every mutation here clones the
// machine snapshot, validates the clone, and commits it through RuntimeState,
// which means no workspace transaction is opened and no undo entry is recorded.
// A handler therefore returns neither an Undo nor a Redo callback, and
// OperationHistory.record drops the entry on its own.
//
// The ordering rule is the other half: a TUI effect — applying a lens,
// relabeling the active one, clearing its modified marker, restoring its
// baseline — is published only after the commit returns nil. A failed commit
// leaves both the cached machine snapshot and the Outliner exactly as they
// were, so a refusal is indistinguishable from never having asked.
//
// The active-lens pointer is the one documented exception, and it is one
// precisely because it records nothing: it is a hint about which lens to reopen
// with, written by publishLens after the projection is already live, so it has
// no act to be ordered ahead of. Create, replace, rename, and delete all still
// commit first.
const (
	operationLensApply       operation.OperationID = "tui.lens.apply"
	operationLensCreate      operation.OperationID = "tui.lens.create"
	operationLensAddTemplate operation.OperationID = "tui.lens.add-template"
	operationLensRename      operation.OperationID = "tui.lens.rename"
	operationLensReplace     operation.OperationID = "tui.lens.replace"
	operationLensDelete      operation.OperationID = "tui.lens.delete"
)

// lensOperationDescriptors is the authoritative catalog of named-lens
// operations. Four of them target one saved lens by its UUID; Create and Add
// built-in view resolve no target, because the record each writes does not
// exist yet.
func lensOperationDescriptors() []operation.OperationDescriptor {
	lens := func(
		id operation.OperationID,
		label string,
		description string,
		risk operation.OperationRisk,
	) operation.OperationDescriptor {
		return operation.OperationDescriptor{
			ID:          id,
			Label:       label,
			Description: description,
			// A lens is a saved projection: mode, scope, sort, and one query holding
			// every facet it narrows by.
			// It groups with the facets that decide the same axes rather than with
			// an entity family, because it owns no domain entity of its own.
			Category:             operation.OperationCategoryFilter,
			TargetSource:         operation.OperationTargetCurrentEntity,
			TargetCardinality:    operation.TargetCardinalityOne,
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilityLensPicker),
			Applicability:        lensOperationApplicable,
			Risk:                 risk,
		}
	}
	return []operation.OperationDescriptor{
		lens(operationLensApply, "Apply lens", "Open this saved view", operation.OperationRiskReadOnly),
		{
			ID:                   operationLensCreate,
			Label:                "Save current view",
			Description:          "Save the current view as a new lens",
			Category:             operation.OperationCategoryFilter,
			TargetSource:         operation.OperationTargetGlobal,
			TargetCardinality:    operation.TargetCardinalityNone,
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilityLensPicker),
			Risk:                 operation.OperationRiskMutation,
			ValueProviderID:      operation.ValueProviderLensName,
		},
		{
			// Adding a template writes an ordinary lens under the catalog's own
			// stable UUID, so it resolves no target either: the value is which
			// template, not which saved record.
			ID:                   operationLensAddTemplate,
			Label:                "Add built-in view",
			Description:          "Add one of the built-in views as a saved lens",
			Category:             operation.OperationCategoryFilter,
			TargetSource:         operation.OperationTargetGlobal,
			TargetCardinality:    operation.TargetCardinalityNone,
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilityLensPicker),
			Risk:                 operation.OperationRiskMutation,
			ValueProviderID:      operation.ValueProviderLensTemplate,
		},
		operation.WithValueProvider(
			lens(operationLensRename, "Rename lens", "Give this lens a new name", operation.OperationRiskMutation),
			operation.ValueProviderLensName,
		),
		// Replace and Delete overwrite or drop a saved record with no undo entry
		// behind them, so both carry destructive risk and fall through to the
		// executor's own confirmation gate rather than prompting for themselves.
		lens(
			operationLensReplace,
			// Named for the source rather than the target, because the sheet row
			// sits under the lens's own name: what needs saying is which view wins.
			"Replace from current view",
			"Overwrite this lens with the current view",
			operation.OperationRiskDestructive,
		),
		lens(operationLensDelete, "Delete lens", "Delete this saved lens", operation.OperationRiskDestructive),
	}
}

// lensOperationApplicable admits a lens operation only when the focused entity
// is a lens. CurrentEntityIdentity degrades to the focused task and then the
// scoped project, so an unguarded descriptor would borrow an unrelated target.
func lensOperationApplicable(ctx operation.CommandContext) bool {
	return operation.CurrentEntityIdentity(ctx).Kind == operation.EntityKindLens
}

func (m *AppModel) registerLensOperations() error {
	if m == nil || m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	if err := m.operationExecutor.RegisterValueProvider(
		operation.ValueProviderLensName,
		pendingTextValueProvider,
	); err != nil {
		return err
	}
	if err := m.operationExecutor.RegisterValueProvider(
		operation.ValueProviderLensTemplate,
		// Returning no immediate value asks the shell to open the built-in view
		// gallery. Registering it here is also what makes the executor's own value
		// gate reachable: an add with no template selected stops before the handler.
		pendingPickedValueProvider,
	); err != nil {
		return err
	}
	for _, descriptor := range lensOperationDescriptors() {
		binding := operation.OperationBinding{
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilityLensPicker),
			Applicability:        descriptor.Applicability,
			// Machine-local state lives outside the synced workspace set, so no
			// workspace transaction is opened for any of these.
			DelegateTransaction: true,
		}
		if descriptor.TargetCardinality == operation.TargetCardinalityNone {
			handler := m.zeroTargetLensHandler(descriptor.ID)
			if handler == nil {
				return fmt.Errorf(
					"lens operation %s resolves no target and has no target-free handler",
					descriptor.ID,
				)
			}
			binding.Handler = handler
		} else {
			binding.SingleHandler = m.executeLensOperation
		}
		if err := m.registerOperation(descriptor, binding); err != nil {
			return err
		}
	}
	return nil
}

// zeroTargetLensHandler names the handler for a lens operation that resolves no
// target. Saving the current view and adding a built-in view both write a record
// that does not exist yet, but they write different records from different
// values, so each dispatches to its own handler rather than sharing one branch.
// An unmapped zero-target descriptor fails registration instead of borrowing a
// neighbor's handler.
func (m *AppModel) zeroTargetLensHandler(id operation.OperationID) operation.OperationHandler {
	switch id {
	case operationLensCreate:
		return m.executeLensCreateOperation
	case operationLensAddTemplate:
		return m.executeLensAddTemplateOperation
	default:
		return nil
	}
}

// resolveLensTarget reloads the definition the identity names from the latest
// machine snapshot. A UUID the snapshot no longer carries — a lens another
// window deleted, or a row built before a reload — fails loudly here instead of
// acting on a stale copy the picker still shows.
func (m *AppModel) resolveLensTarget(identity operation.EntityIdentity) (config.NamedLens, error) {
	if identity.Kind != operation.EntityKindLens || !identity.Valid() {
		return config.NamedLens{}, fmt.Errorf("%s is not a lens", identity)
	}
	if m == nil || m.runtime == nil {
		return config.NamedLens{}, errors.New("workspace runtime is unavailable")
	}
	machine := m.runtime.Machine()
	if machine == nil {
		return config.NamedLens{}, errors.New("machine-local state is unavailable")
	}
	lens := machine.NamedLensByID(identity.Name)
	if lens == nil {
		return config.NamedLens{}, fmt.Errorf(
			"lens %s is no longer available; reopen the lens list",
			identity.Name,
		)
	}
	return lens.Clone(), nil
}

// commitNamedLensMutation applies one lens edit to a deep clone of the machine
// snapshot and commits it. The clone is what protects the cached snapshot: a
// mutate or validation failure returns before any commit, and a failed commit
// leaves RuntimeState holding the state it already had.
func (m *AppModel) commitNamedLensMutation(mutate func(*config.MachineState) error) error {
	if m == nil || m.runtime == nil {
		return errors.New("workspace runtime is unavailable")
	}
	snapshot := m.runtime.Machine()
	if snapshot == nil {
		return errors.New("machine-local state is unavailable")
	}
	machine := cloneMachineState(snapshot)
	if err := mutate(machine); err != nil {
		return err
	}
	if err := machine.Validate(); err != nil {
		return err
	}
	if m.runtime.DataDir() == "" {
		// No writable workspace is rooted, so the in-memory seam is the whole
		// commit: the runtime accessors observe the new list exactly as they would
		// after a disk write.
		m.runtime.SetMachineInMemory(machine)
		return nil
	}
	return m.runtime.CommitMachine(machine)
}

// commitActiveLensPointer records which lens the Outliner should reopen with, or
// clears the pointer when passed the empty string. It is the one lens write that
// is not ordered before its TUI effect, because it records a resume hint rather
// than a lens: publishLens has already installed the projection by the time this
// runs, and a failed commit costs the user nothing but the hint. Writing an
// unchanged pointer is skipped so an ordinary Esc or global reset does not
// rewrite identical bytes on every press.
func (m *AppModel) commitActiveLensPointer(id string) error {
	if m == nil || m.runtime == nil {
		return nil
	}
	machine := m.runtime.Machine()
	if machine == nil || machine.ActiveLensID == id {
		return nil
	}
	return m.commitNamedLensMutation(func(machine *config.MachineState) error {
		machine.ActiveLensID = id
		return nil
	})
}

// restorePendingLaunchLens applies the lens the machine-local pointer named when
// this session was constructed, exactly once, and forgets it either way. It runs
// after the clean-root reset has already completed, which is the whole design:
// the lens is layered on top of the canonical launch view rather than replacing
// it, so lensRestorationPoint captures that view and the first Esc lands exactly
// where a lens-free launch would have started.
//
// A lens that no longer applies — a mode the build no longer knows, a query the
// grammar cannot honor, a scope naming a project that was deleted, archived, or
// parked elsewhere — fails closed to no lens, silently. The pointer is a hint the
// user last expressed on this machine, not a record they are owed an error about,
// and the reset already cleared it, so the next launch simply does not ask again.
func (m *AppModel) restorePendingLaunchLens() tea.Cmd {
	if m == nil || m.pendingLaunchLens == nil {
		return nil
	}
	lens := *m.pendingLaunchLens
	m.pendingLaunchLens = nil
	if m.crossOutliner == nil {
		return nil
	}
	cmd, err := m.crossOutliner.ApplyNamedLens(lens)
	if err != nil {
		return nil
	}
	return cmd
}

// namedLensNameTaken reports whether another record already answers to the
// name. Uniqueness is case-insensitive, matching NamedLensList.Validate; this
// check exists to name the offending lens rather than to add a second rule.
func namedLensNameTaken(list config.NamedLensList, name string, exceptID string) bool {
	for _, lens := range list {
		if lens.ID == exceptID {
			continue
		}
		if strings.EqualFold(lens.Name, name) {
			return true
		}
	}
	return false
}
