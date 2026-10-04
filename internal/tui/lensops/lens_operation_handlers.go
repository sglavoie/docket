package lensops

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/operation"
)

func (c *Controller) executeLensOperation(
	shell Shell,
	invocation operation.OperationInvocation,
	target operation.OperationTarget,
) operation.OperationHandlerResult {
	if target.Entity.Kind != operation.EntityKindLens {
		return operation.OperationHandlerResult{
			Err: fmt.Errorf("%s needs a lens target, got %s", invocation.Descriptor.Label, target.Entity),
		}
	}
	saved, err := c.ResolveLensTarget(shell, target.Entity)
	if err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	switch invocation.Descriptor.ID {
	case lens.OperationLensApply:
		return c.applyLensOperation(shell, saved)
	case lens.OperationLensRename:
		return c.renameLensOperation(shell, invocation, saved)
	case lens.OperationLensReplace:
		return c.replaceLensOperation(shell, saved)
	case lens.OperationLensDelete:
		return c.deleteLensOperation(shell, saved)
	default:
		return operation.OperationHandlerResult{Err: fmt.Errorf("%w: %s", operation.ErrOperationHandler, invocation.Descriptor.ID)}
	}
}

// applyLensOperation installs a saved projection. The only durable byte it
// writes is the machine-local active-lens pointer, and that one is written from
// publishLens rather than ordered ahead of the application here: it is a resume
// hint, not a record of the act, so a failed pointer commit degrades to a status
// warning and the projection stands. ApplyNamedLens still decides every refusal
// before it touches an axis.
func (c *Controller) applyLensOperation(shell Shell, saved config.NamedLens) operation.OperationHandlerResult {
	out := shell.Outliner()
	if out == nil {
		return operation.OperationHandlerResult{Err: errors.New("the Outliner is unavailable")}
	}
	cmd, err := out.ApplyNamedLens(saved)
	if err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	return lensCommandResult(cmd, "Lens: "+saved.Name)
}

// executeLensCreateOperation saves the current view under a new UUID and makes
// it the live lens. Applying afterwards is what retains an existing baseline:
// lensRestorationPoint carries the original view forward when a lens is already
// live, so saving from inside one still leaves a single Esc to the pre-lens view.
func (c *Controller) executeLensCreateOperation(shell Shell, invocation operation.OperationInvocation) operation.OperationHandlerResult {
	out := shell.Outliner()
	if out == nil {
		return operation.OperationHandlerResult{Err: errors.New("the Outliner is unavailable")}
	}
	name, err := config.NormalizeNamedLensName(operation.StringValue(invocation))
	if err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	// A user lens mints its own UUID here, in canonical lowercase hyphenated
	// form, which is the only spelling ValidateNamedLensID accepts.
	minted := out.CaptureLensState().NamedLens(uuid.NewString(), name)
	if err = minted.Validate(); err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	if err = c.CommitNamedLensMutation(shell, func(machine *config.MachineState) error {
		if namedLensNameTaken(machine.NamedLenses, minted.Name, "") {
			return fmt.Errorf("a lens named %q already exists", minted.Name)
		}
		machine.NamedLenses = append(machine.NamedLenses, minted)
		return nil
	}); err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	cmd, err := out.ApplyNamedLens(minted)
	if err != nil {
		return operation.OperationHandlerResult{
			Err: fmt.Errorf("saved lens %q but could not apply it: %w", minted.Name, err),
		}
	}
	return lensCommandResult(cmd, "Saved lens: "+minted.Name)
}

// resolveBuiltinLensTemplate reads the selected template out of the invocation
// and reloads it from the catalog. The value is a template ID and nothing else:
// a definition captured by the UI could be stale or edited, so only the ID is
// trusted and the record written is the catalog's own fresh copy.
func resolveBuiltinLensTemplate(invocation operation.OperationInvocation) (config.NamedLens, error) {
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
func (c *Controller) executeLensAddTemplateOperation(shell Shell, invocation operation.OperationInvocation) operation.OperationHandlerResult {
	out := shell.Outliner()
	if out == nil {
		return operation.OperationHandlerResult{Err: errors.New("the Outliner is unavailable")}
	}
	template, err := resolveBuiltinLensTemplate(invocation)
	if err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	if err = template.Validate(); err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	if err = c.CommitNamedLensMutation(shell, func(machine *config.MachineState) error {
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
		return operation.OperationHandlerResult{Err: err}
	}
	cmd, err := out.ApplyNamedLens(template)
	if err != nil {
		return operation.OperationHandlerResult{
			Err: fmt.Errorf("added built-in view %q but could not apply it: %w", template.Name, err),
		}
	}
	return lensCommandResult(cmd, "Added built-in view: "+template.Name)
}

// renameLensOperation changes the label alone. The projection is untouched, so
// the active frame keeps its saved state and fingerprint and a modified lens
// stays modified across the rename.
func (c *Controller) renameLensOperation(
	shell Shell,
	invocation operation.OperationInvocation,
	existing config.NamedLens,
) operation.OperationHandlerResult {
	name, err := config.NormalizeNamedLensName(operation.StringValue(invocation))
	if err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	if name == existing.Name {
		return operation.OperationHandlerResult{Status: "Rename canceled: same name"}
	}
	if err := c.CommitNamedLensMutation(shell, func(machine *config.MachineState) error {
		if namedLensNameTaken(machine.NamedLenses, name, existing.ID) {
			return fmt.Errorf("a lens named %q already exists", name)
		}
		saved := machine.NamedLensByID(existing.ID)
		if saved == nil {
			return fmt.Errorf("lens %s is no longer available; reopen the lens list", existing.ID)
		}
		saved.Name = name
		return nil
	}); err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	if out := shell.Outliner(); out != nil {
		out.Lens.RenameActive(existing.ID, name)
	}
	return operation.OperationHandlerResult{Status: fmt.Sprintf("Renamed lens: %s → %s", existing.Name, name)}
}

// replaceLensOperation overwrites the saved definition with the live view under
// the same identity. Replacing the active lens re-bases its frame, which is what
// clears the modified marker: the view and the record now agree.
func (c *Controller) replaceLensOperation(shell Shell, existing config.NamedLens) operation.OperationHandlerResult {
	out := shell.Outliner()
	if out == nil {
		return operation.OperationHandlerResult{Err: errors.New("the Outliner is unavailable")}
	}
	state := out.CaptureLensState()
	replacement := state.NamedLens(existing.ID, existing.Name)
	if err := replacement.Validate(); err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	if err := c.CommitNamedLensMutation(shell, func(machine *config.MachineState) error {
		saved := machine.NamedLensByID(replacement.ID)
		if saved == nil {
			return fmt.Errorf("lens %s is no longer available; reopen the lens list", replacement.ID)
		}
		*saved = replacement.Clone()
		return nil
	}); err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	out.Lens.RebaseActive(replacement.ID, state)
	return operation.OperationHandlerResult{Status: "Replaced lens: " + replacement.Name}
}

// deleteLensOperation drops the definition and, when it was the live one, hands
// the pre-lens view back. The commit runs first so a delete that fails leaves
// the user inside the lens they still have.
func (c *Controller) deleteLensOperation(shell Shell, existing config.NamedLens) operation.OperationHandlerResult {
	if err := c.CommitNamedLensMutation(shell, func(machine *config.MachineState) error {
		if !machine.RemoveNamedLens(existing.ID) {
			return fmt.Errorf("lens %s is no longer available; reopen the lens list", existing.ID)
		}
		return nil
	}); err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	if out := shell.Outliner(); out != nil && out.Lens.ActiveIs(existing.ID) {
		out.RestoreLensBaseline()
	}
	return operation.OperationHandlerResult{Status: "Deleted lens: " + existing.Name}
}

// lensCommandResult carries an application's follow-up load, if it started one.
// The status travels either way so the surface names what happened even when a
// command owns the message slot.
func lensCommandResult(cmd tea.Cmd, status string) operation.OperationHandlerResult {
	if cmd == nil {
		return operation.OperationHandlerResult{Status: status}
	}
	return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{Cmd: cmd}, Status: status}
}
