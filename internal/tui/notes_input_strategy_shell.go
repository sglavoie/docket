package tui

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/presentation"
)

// openNotesActionSheet raises the shell-owned sheet over the Notes manager. The
// frozen context already names the note under the cursor, and the sheet owns
// every key while it is open, so the row it lists cannot move beneath it.
func (m *AppModel) openNotesActionSheet(ctx CommandContext) error {
	if currentEntityIdentity(ctx).Kind != EntityKindNote {
		return errors.New("select a note to open actions")
	}
	sheet, err := NewContextActionSheetModel(
		m.GetOperationRegistry(),
		m.operationExecutor,
		ctx,
		notesActionSheetOperations,
		ContextActionSheetConfig{
			Host:         ContextActionHostNotes,
			Completion:   ContextActionCompleteCloseSheet,
			Title:        "Note Actions",
			Subtitle:     m.noteTitleForContext(ctx),
			ItemIDPrefix: "note-action:",
			SecondaryFor: operationActionSecondaryText,
			Targets:      1,
			EmptyState: ListEmptyState{
				Title:  "No note actions available.",
				Detail: "Move the cursor to a note.",
			},
			NoMatchState: ListEmptyState{
				Title:  "No matching actions.",
				Detail: "Clear or change the search.",
			},
		},
	)
	if err != nil {
		return err
	}
	m.contextActionSheet = sheet
	return nil
}

// noteTitleForContext names the sheet's frozen note. A note whose file vanished
// since the row was built yields no subtitle rather than a stale one.
func (m *AppModel) noteTitleForContext(ctx CommandContext) string {
	resolved, err := m.resolveNoteTarget(currentEntityIdentity(ctx))
	if err != nil {
		return ""
	}
	return resolved.Note.Title
}

// registerNotesSurfaceOperations installs the manager's own vocabulary and then
// gives the four note operations a second, Notes-scoped binding. The global
// note bindings prompt and confirm through the shell because their host names a
// note it cannot see; the manager owns an inline rename, a location picker, and
// a confirmation of its own, so inside Notes those steps belong to the surface
// and the binding delegates all four executor stages to it.
//
// The two copies are absent on purpose. They prompt nothing, pick nothing, and
// confirm nothing, so the manager has no step to own and the global binding from
// registerNoteOperations is what the executor selects for them here.
func (m *AppModel) registerNotesSurfaceOperations() error {
	for _, descriptor := range notesSurfaceOperationDescriptors() {
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities:     NewCapabilitySet(CapabilityNotesManager),
			Handler:                  m.executeNotesSurfaceOperation,
			DelegateTargetResolution: true,
			DelegateValueProvider:    true,
			DelegateConfirmation:     true,
			DelegateTransaction:      true,
		}); err != nil {
			return err
		}
	}
	for _, id := range notesHostedNoteOperations() {
		if err := m.operationExecutor.Register(OperationBinding{
			OperationID:              id,
			Views:                    []presentation.PresentationID{presentation.ViewNotes, presentation.ViewGlobalNotes},
			RequiredCapabilities:     NewCapabilitySet(CapabilityNotesManager),
			Handler:                  m.executeNotesSurfaceOperation,
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
func (m *AppModel) executeNotesSurfaceOperation(invocation OperationInvocation) OperationHandlerResult {
	notes := m.activeNotesManager()
	if notes == nil {
		return OperationHandlerResult{Err: fmt.Errorf("the Notes manager is not open")}
	}
	command := func(cmd tea.Cmd) OperationHandlerResult {
		return OperationHandlerResult{Message: OperationCommandResultMsg{Cmd: cmd}}
	}
	switch invocation.Descriptor.ID {
	case operationNotesCreate:
		return command(notes.beginDestinationPicker())
	case operationNotesSearch:
		return command(notes.beginSearch())
	case operationNotesScope:
		return command(notes.beginScopePicker())
	case operationNotesActions:
		if err := m.openNotesActionSheet(invocation.Context); err != nil {
			return OperationHandlerResult{Err: err}
		}
	case operationNoteOpen:
		return command(notes.editCurrentNote())
	case operationNoteMove:
		return command(notes.beginMovePicker())
	case operationNoteRename:
		notes.startRename()
	case operationNoteDelete:
		notes.deleteCurrentNote()
	}
	return OperationHandlerResult{}
}

// activeNotesManager returns the hub-mounted Notes manager, or nil when Manage
// is on another module. getActiveNotesView answers the presentation registry so
// a note mutation result can find its renderer from anywhere; a dispatched key
// already knows Manage is what it came from.
func (m *AppModel) activeNotesManager() *NotesViewModel {
	if m == nil || m.manageHub == nil || m.manageHub.Params().Module != ManageNotes {
		return nil
	}
	notes, _ := m.manageHub.ActiveModuleView().(*NotesViewModel)
	return notes
}
