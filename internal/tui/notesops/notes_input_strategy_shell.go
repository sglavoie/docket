package notesops

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/notes"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/route"
)

// OpenNotesActionSheet raises the shell-owned sheet over the Notes manager. The
// frozen context already names the note under the cursor, and the sheet owns
// every key while it is open, so the row it lists cannot move beneath it.
func (c *Controller) OpenNotesActionSheet(shell Shell, ctx operation.CommandContext) error {
	if operation.CurrentEntityIdentity(ctx).Kind != operation.EntityKindNote {
		return errors.New("select a note to open actions")
	}
	sheet, err := listsurface.NewContextActionSheetModel(
		shell.Registry(),
		shell.Executor(),
		ctx,
		notes.NotesActionSheetOperations,
		listsurface.ContextActionSheetConfig{
			Host:         listsurface.ContextActionHostNotes,
			Completion:   listsurface.ContextActionCompleteCloseSheet,
			Title:        "Note Actions",
			Subtitle:     c.NoteTitleForContext(shell, ctx),
			ItemIDPrefix: "note-action:",
			SecondaryFor: operation.ActionSecondaryText,
			Targets:      1,
			EmptyState: listsurface.ListEmptyState{
				Title:  "No note actions available.",
				Detail: "Move the cursor to a note.",
			},
			NoMatchState: listsurface.ListEmptyState{
				Title:  "No matching actions.",
				Detail: "Clear or change the search.",
			},
		},
	)
	if err != nil {
		return err
	}
	shell.MountActionSheet(sheet)
	return nil
}

// NoteTitleForContext names the sheet's frozen note. A note whose file vanished
// since the row was built yields no subtitle rather than a stale one.
func (c *Controller) NoteTitleForContext(shell Shell, ctx operation.CommandContext) string {
	resolved, err := c.ResolveNoteTarget(shell, operation.CurrentEntityIdentity(ctx))
	if err != nil {
		return ""
	}
	return resolved.Note.Title
}

// RegisterNotesSurfaceOperations installs the manager's own vocabulary and then
// gives the four note operations a second, Notes-scoped binding. The global
// note bindings prompt and confirm through the shell because their host names a
// note it cannot see; the manager owns an inline rename, a location picker, and
// a confirmation of its own, so inside Notes those steps belong to the surface
// and the binding delegates all four executor stages to it.
//
// The two copies are absent on purpose. They prompt nothing, pick nothing, and
// confirm nothing, so the manager has no step to own and the global binding from
// RegisterNoteOperations is what the executor selects for them here.
func (c *Controller) RegisterNotesSurfaceOperations(shell Shell) error {
	for _, descriptor := range notes.NotesSurfaceOperationDescriptors() {
		if err := shell.RegisterOperation(descriptor, operation.OperationBinding{
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilityNotesManager),
			Handler: func(invocation operation.OperationInvocation) operation.OperationHandlerResult {
				return c.executeNotesSurfaceOperation(shell, invocation)
			},
			DelegateTargetResolution: true,
			DelegateValueProvider:    true,
			DelegateConfirmation:     true,
			DelegateTransaction:      true,
		}); err != nil {
			return err
		}
	}
	for _, id := range notes.NotesHostedNoteOperations() {
		if err := shell.Executor().Register(operation.OperationBinding{
			OperationID:          id,
			Views:                []presentation.PresentationID{presentation.ViewNotes, presentation.ViewGlobalNotes},
			RequiredCapabilities: operation.NewCapabilitySet(operation.CapabilityNotesManager),
			Handler: func(invocation operation.OperationInvocation) operation.OperationHandlerResult {
				return c.executeNotesSurfaceOperation(shell, invocation)
			},
			DelegateTargetResolution: true,
			DelegateValueProvider:    true,
			DelegateConfirmation:     true,
			DelegateTransaction:      true,
		}); err != nil {
			return err
		}
	}
	return nil
}

// executeNotesSurfaceOperation applies one Notes operation to the live manager.
// Every one of them moves the model the user is looking at, so the handler
// reaches for the mounted module rather than rebuilding one.
//
//nolint:exhaustive // Only the Notes grammar's operations are registered with this handler.
func (c *Controller) executeNotesSurfaceOperation(shell Shell, invocation operation.OperationInvocation) operation.OperationHandlerResult {
	manager := c.ActiveNotesManager(shell)
	if manager == nil {
		return operation.OperationHandlerResult{Err: fmt.Errorf("the Notes manager is not open")}
	}
	command := func(cmd tea.Cmd) operation.OperationHandlerResult {
		return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{Cmd: cmd}}
	}
	switch invocation.Descriptor.ID {
	case notes.OperationNotesCreate:
		return command(manager.BeginDestinationPicker())
	case notes.OperationNotesSearch:
		return command(manager.BeginSearch())
	case notes.OperationNotesScope:
		return command(manager.BeginScopePicker())
	case notes.OperationNotesActions:
		if err := c.OpenNotesActionSheet(shell, invocation.Context); err != nil {
			return operation.OperationHandlerResult{Err: err}
		}
	case notes.OperationNoteOpen:
		return command(manager.EditCurrentNote())
	case notes.OperationNoteMove:
		return command(manager.BeginMovePicker())
	case notes.OperationNoteRename:
		manager.StartRename()
	case notes.OperationNoteDelete:
		manager.DeleteCurrentNote()
	}
	return operation.OperationHandlerResult{}
}

// ActiveNotesManager returns the hub-mounted Notes manager, or nil when Manage
// is on another module. getActiveNotesView answers the presentation registry so
// a note mutation result can find its renderer from anywhere; a dispatched key
// already knows Manage is what it came from.
func (c *Controller) ActiveNotesManager(shell Shell) *notes.NotesViewModel {
	if shell == nil || shell.ManageHub() == nil || shell.ManageHub().Params().Module != route.ManageNotes {
		return nil
	}
	manager, _ := shell.ManageHub().ActiveModuleView().(*notes.NotesViewModel)
	return manager
}
