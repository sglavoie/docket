package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
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
	operationLensApply       OperationID = "tui.lens.apply"
	operationLensCreate      OperationID = "tui.lens.create"
	operationLensAddTemplate OperationID = "tui.lens.add-template"
	operationLensRename      OperationID = "tui.lens.rename"
	operationLensReplace     OperationID = "tui.lens.replace"
	operationLensDelete      OperationID = "tui.lens.delete"
)

// lensOperationDescriptors is the authoritative catalog of named-lens
// operations. Four of them target one saved lens by its UUID; Create and Add
// built-in view resolve no target, because the record each writes does not
// exist yet.
func lensOperationDescriptors() []OperationDescriptor {
	lens := func(
		id OperationID,
		label string,
		description string,
		risk OperationRisk,
	) OperationDescriptor {
		return OperationDescriptor{
			ID:          id,
			Label:       label,
			Description: description,
			// A lens is a saved projection: mode, scope, sort, and one query holding
			// every facet it narrows by.
			// It groups with the facets that decide the same axes rather than with
			// an entity family, because it owns no domain entity of its own.
			Category:             OperationCategoryFilter,
			TargetSource:         OperationTargetCurrentEntity,
			TargetCardinality:    TargetCardinalityOne,
			RequiredCapabilities: NewCapabilitySet(CapabilityLensPicker),
			Applicability:        lensOperationApplicable,
			Risk:                 risk,
		}
	}
	return []OperationDescriptor{
		lens(operationLensApply, "Apply lens", "Open this saved view", OperationRiskReadOnly),
		{
			ID:                   operationLensCreate,
			Label:                "Save current view",
			Description:          "Save the current view as a new lens",
			Category:             OperationCategoryFilter,
			TargetSource:         OperationTargetGlobal,
			TargetCardinality:    TargetCardinalityNone,
			RequiredCapabilities: NewCapabilitySet(CapabilityLensPicker),
			Risk:                 OperationRiskMutation,
			ValueProviderID:      ValueProviderLensName,
		},
		{
			// Adding a template writes an ordinary lens under the catalog's own
			// stable UUID, so it resolves no target either: the value is which
			// template, not which saved record.
			ID:                   operationLensAddTemplate,
			Label:                "Add built-in view",
			Description:          "Add one of the built-in views as a saved lens",
			Category:             OperationCategoryFilter,
			TargetSource:         OperationTargetGlobal,
			TargetCardinality:    TargetCardinalityNone,
			RequiredCapabilities: NewCapabilitySet(CapabilityLensPicker),
			Risk:                 OperationRiskMutation,
			ValueProviderID:      ValueProviderLensTemplate,
		},
		withValueProvider(
			lens(operationLensRename, "Rename lens", "Give this lens a new name", OperationRiskMutation),
			ValueProviderLensName,
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
			OperationRiskDestructive,
		),
		lens(operationLensDelete, "Delete lens", "Delete this saved lens", OperationRiskDestructive),
	}
}

// lensOperationApplicable admits a lens operation only when the focused entity
// is a lens. currentEntityIdentity degrades to the focused task and then the
// scoped project, so an unguarded descriptor would borrow an unrelated target.
func lensOperationApplicable(ctx CommandContext) bool {
	return currentEntityIdentity(ctx).Kind == EntityKindLens
}

func (m *AppModel) registerLensOperations() error {
	if m == nil || m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	if err := m.operationExecutor.RegisterValueProvider(
		ValueProviderLensName,
		pendingTextValueProvider,
	); err != nil {
		return err
	}
	if err := m.operationExecutor.RegisterValueProvider(
		ValueProviderLensTemplate,
		// Returning no immediate value asks the shell to open the built-in view
		// gallery. Registering it here is also what makes the executor's own value
		// gate reachable: an add with no template selected stops before the handler.
		pendingPickedValueProvider,
	); err != nil {
		return err
	}
	for _, descriptor := range lensOperationDescriptors() {
		binding := OperationBinding{
			RequiredCapabilities: NewCapabilitySet(CapabilityLensPicker),
			Applicability:        descriptor.Applicability,
			// Machine-local state lives outside the synced workspace set, so no
			// workspace transaction is opened for any of these.
			DelegateTransaction: true,
		}
		if descriptor.TargetCardinality == TargetCardinalityNone {
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
func (m *AppModel) zeroTargetLensHandler(id OperationID) OperationHandler {
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
func (m *AppModel) resolveLensTarget(identity EntityIdentity) (config.NamedLens, error) {
	if identity.Kind != EntityKindLens || !identity.Valid() {
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

func (m *AppModel) executeLensOperation(
	invocation OperationInvocation,
	target OperationTarget,
) OperationHandlerResult {
	if target.Entity.Kind != EntityKindLens {
		return OperationHandlerResult{
			Err: fmt.Errorf("%s needs a lens target, got %s", invocation.Descriptor.Label, target.Entity),
		}
	}
	lens, err := m.resolveLensTarget(target.Entity)
	if err != nil {
		return OperationHandlerResult{Err: err}
	}
	switch invocation.Descriptor.ID {
	case operationLensApply:
		return m.applyLensOperation(lens)
	case operationLensRename:
		return m.renameLensOperation(invocation, lens)
	case operationLensReplace:
		return m.replaceLensOperation(lens)
	case operationLensDelete:
		return m.deleteLensOperation(lens)
	default:
		return OperationHandlerResult{Err: fmt.Errorf("%w: %s", ErrOperationHandler, invocation.Descriptor.ID)}
	}
}

// applyLensOperation installs a saved projection. The only durable byte it
// writes is the machine-local active-lens pointer, and that one is written from
// publishLens rather than ordered ahead of the application here: it is a resume
// hint, not a record of the act, so a failed pointer commit degrades to a status
// warning and the projection stands. ApplyNamedLens still decides every refusal
// before it touches an axis.
func (m *AppModel) applyLensOperation(lens config.NamedLens) OperationHandlerResult {
	out := m.crossOutliner
	if out == nil {
		return OperationHandlerResult{Err: errors.New("the Outliner is unavailable")}
	}
	cmd, err := out.ApplyNamedLens(lens)
	if err != nil {
		return OperationHandlerResult{Err: err}
	}
	return lensCommandResult(cmd, "Lens: "+lens.Name)
}

// executeLensCreateOperation saves the current view under a new UUID and makes
// it the live lens. Applying afterwards is what retains an existing baseline:
// lensRestorationPoint carries the original view forward when a lens is already
// live, so saving from inside one still leaves a single Esc to the pre-lens view.
func (m *AppModel) executeLensCreateOperation(invocation OperationInvocation) OperationHandlerResult {
	out := m.crossOutliner
	if out == nil {
		return OperationHandlerResult{Err: errors.New("the Outliner is unavailable")}
	}
	name, err := config.NormalizeNamedLensName(operationStringValue(invocation))
	if err != nil {
		return OperationHandlerResult{Err: err}
	}
	// A user lens mints its own UUID here, in canonical lowercase hyphenated
	// form, which is the only spelling ValidateNamedLensID accepts.
	lens := out.captureLensState().namedLens(uuid.NewString(), name)
	if err = lens.Validate(); err != nil {
		return OperationHandlerResult{Err: err}
	}
	if err = m.commitNamedLensMutation(func(machine *config.MachineState) error {
		if namedLensNameTaken(machine.NamedLenses, lens.Name, "") {
			return fmt.Errorf("a lens named %q already exists", lens.Name)
		}
		machine.NamedLenses = append(machine.NamedLenses, lens)
		return nil
	}); err != nil {
		return OperationHandlerResult{Err: err}
	}
	cmd, err := out.ApplyNamedLens(lens)
	if err != nil {
		return OperationHandlerResult{
			Err: fmt.Errorf("saved lens %q but could not apply it: %w", lens.Name, err),
		}
	}
	return lensCommandResult(cmd, "Saved lens: "+lens.Name)
}

// resolveBuiltinLensTemplate reads the selected template out of the invocation
// and reloads it from the catalog. The value is a template ID and nothing else:
// a definition captured by the UI could be stale or edited, so only the ID is
// trusted and the record written is the catalog's own fresh copy.
func resolveBuiltinLensTemplate(invocation OperationInvocation) (config.NamedLens, error) {
	if !invocation.HasValue {
		return config.NamedLens{}, errors.New("no built-in view was selected")
	}
	id, ok := invocation.Value.(string)
	if !ok {
		return config.NamedLens{}, fmt.Errorf(
			"a built-in view selection must name a template ID, got %T",
			invocation.Value,
		)
	}
	for _, template := range config.BuiltinNamedLensTemplates() {
		if template.ID == id {
			return template, nil
		}
	}
	return config.NamedLens{}, fmt.Errorf("unknown built-in view %q", id)
}

// executeLensAddTemplateOperation persists one catalog template as an ordinary
// lens and applies it, in that order and under the same rules as Create: the
// commit is what publishes the record, and the application afterwards is what
// keeps an existing restoration baseline. Availability is rechecked here against
// the latest machine snapshot, because the gallery decided it against the one it
// was rendered from.
func (m *AppModel) executeLensAddTemplateOperation(invocation OperationInvocation) OperationHandlerResult {
	out := m.crossOutliner
	if out == nil {
		return OperationHandlerResult{Err: errors.New("the Outliner is unavailable")}
	}
	template, err := resolveBuiltinLensTemplate(invocation)
	if err != nil {
		return OperationHandlerResult{Err: err}
	}
	if err = template.Validate(); err != nil {
		return OperationHandlerResult{Err: err}
	}
	if err = m.commitNamedLensMutation(func(machine *config.MachineState) error {
		// The stable ID answers first: a template already added is already added
		// whatever it has since been renamed to, and saying so by its current name
		// is the more useful refusal.
		if existing := machine.NamedLensByID(template.ID); existing != nil {
			return fmt.Errorf("%q is already added as %q", template.Name, existing.Name)
		}
		if namedLensNameTaken(machine.NamedLenses, template.Name, template.ID) {
			return fmt.Errorf("a lens named %q already exists", template.Name)
		}
		machine.NamedLenses = append(machine.NamedLenses, template)
		return nil
	}); err != nil {
		return OperationHandlerResult{Err: err}
	}
	cmd, err := out.ApplyNamedLens(template)
	if err != nil {
		return OperationHandlerResult{
			Err: fmt.Errorf("added built-in view %q but could not apply it: %w", template.Name, err),
		}
	}
	return lensCommandResult(cmd, "Added built-in view: "+template.Name)
}

// renameLensOperation changes the label alone. The projection is untouched, so
// the active frame keeps its saved state and fingerprint and a modified lens
// stays modified across the rename.
func (m *AppModel) renameLensOperation(
	invocation OperationInvocation,
	lens config.NamedLens,
) OperationHandlerResult {
	name, err := config.NormalizeNamedLensName(operationStringValue(invocation))
	if err != nil {
		return OperationHandlerResult{Err: err}
	}
	if name == lens.Name {
		return OperationHandlerResult{Status: "Rename canceled: same name"}
	}
	if err := m.commitNamedLensMutation(func(machine *config.MachineState) error {
		if namedLensNameTaken(machine.NamedLenses, name, lens.ID) {
			return fmt.Errorf("a lens named %q already exists", name)
		}
		saved := machine.NamedLensByID(lens.ID)
		if saved == nil {
			return fmt.Errorf("lens %s is no longer available; reopen the lens list", lens.ID)
		}
		saved.Name = name
		return nil
	}); err != nil {
		return OperationHandlerResult{Err: err}
	}
	if out := m.crossOutliner; out != nil && out.activeLens != nil && out.activeLens.lensID == lens.ID {
		out.activeLens.name = name
	}
	return OperationHandlerResult{Status: fmt.Sprintf("Renamed lens: %s → %s", lens.Name, name)}
}

// replaceLensOperation overwrites the saved definition with the live view under
// the same identity. Replacing the active lens re-bases its frame, which is what
// clears the modified marker: the view and the record now agree.
func (m *AppModel) replaceLensOperation(lens config.NamedLens) OperationHandlerResult {
	out := m.crossOutliner
	if out == nil {
		return OperationHandlerResult{Err: errors.New("the Outliner is unavailable")}
	}
	state := out.captureLensState()
	replacement := state.namedLens(lens.ID, lens.Name)
	if err := replacement.Validate(); err != nil {
		return OperationHandlerResult{Err: err}
	}
	if err := m.commitNamedLensMutation(func(machine *config.MachineState) error {
		saved := machine.NamedLensByID(replacement.ID)
		if saved == nil {
			return fmt.Errorf("lens %s is no longer available; reopen the lens list", replacement.ID)
		}
		*saved = replacement.Clone()
		return nil
	}); err != nil {
		return OperationHandlerResult{Err: err}
	}
	if out.activeLens != nil && out.activeLens.lensID == replacement.ID {
		out.activeLens.savedFingerprint = state.fingerprint()
	}
	return OperationHandlerResult{Status: "Replaced lens: " + replacement.Name}
}

// deleteLensOperation drops the definition and, when it was the live one, hands
// the pre-lens view back. The commit runs first so a delete that fails leaves
// the user inside the lens they still have.
func (m *AppModel) deleteLensOperation(lens config.NamedLens) OperationHandlerResult {
	if err := m.commitNamedLensMutation(func(machine *config.MachineState) error {
		if !machine.RemoveNamedLens(lens.ID) {
			return fmt.Errorf("lens %s is no longer available; reopen the lens list", lens.ID)
		}
		return nil
	}); err != nil {
		return OperationHandlerResult{Err: err}
	}
	if out := m.crossOutliner; out != nil && out.activeLens != nil && out.activeLens.lensID == lens.ID {
		out.restoreLensBaseline()
	}
	return OperationHandlerResult{Status: "Deleted lens: " + lens.Name}
}

// lensCommandResult carries an application's follow-up load, if it started one.
// The status travels either way so the surface names what happened even when a
// command owns the message slot.
func lensCommandResult(cmd tea.Cmd, status string) OperationHandlerResult {
	if cmd == nil {
		return OperationHandlerResult{Status: status}
	}
	return OperationHandlerResult{Message: OperationCommandResultMsg{Cmd: cmd}, Status: status}
}
