package notesops

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/external"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/notes"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/route"
)

// The tui.note.* operation IDs, their descriptors, and the parse-error refusal
// set are declared beside the Notes grammar that binds them, in
// notes_input_strategy.go and note_operations_catalog.go, because that grammar
// sits a package below this registrar. What stays here is the half a descriptor
// cannot carry: the handler, which needs the shell's services.

func (c *Controller) RegisterNoteOperations(shell Shell) error {
	if shell == nil || shell.Executor() == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	if err := shell.Executor().RegisterValueProvider(
		operation.ValueProviderNoteTitle,
		operation.PendingTextValueProvider,
	); err != nil {
		return err
	}
	for _, descriptor := range notes.NoteOperationDescriptors() {
		if err := shell.RegisterOperation(descriptor, operation.OperationBinding{
			SingleHandler: func(invocation operation.OperationInvocation, target operation.OperationTarget) operation.OperationHandlerResult {
				return c.executeNoteOperation(shell, invocation, target)
			},
			DelegateTransaction: true,
		}); err != nil {
			return err
		}
	}
	return nil
}

// ResolvedNote is one note reloaded from its collection at execution time,
// together with the collection it came from.
type ResolvedNote struct {
	Location app.NoteLocation
	Project  *models.Project
	Note     *models.Note
}

// ResolveNoteTarget reloads the note the identity names. A note whose file or
// owning project disappeared since the row was built fails loudly instead of
// acting on stale metadata.
func (c *Controller) ResolveNoteTarget(shell Shell, identity operation.EntityIdentity) (ResolvedNote, error) {
	resolved := ResolvedNote{}
	switch identity.Scope {
	case operation.EntityScopeGlobal:
		resolved.Location = app.GlobalNoteLocation()
	case operation.EntityScopeProject:
		project := shell.ProjectByID(identity.ProjectID)
		if project == nil {
			return ResolvedNote{}, fmt.Errorf("project %s is no longer available", identity.ProjectID)
		}
		resolved.Project = project
		resolved.Location = app.ProjectNoteLocation(project.ID)
	default:
		return ResolvedNote{}, fmt.Errorf("note %s names no collection", identity)
	}
	if shell.DomainService() == nil {
		return ResolvedNote{}, fmt.Errorf("note service is unavailable")
	}
	collection, err := shell.DomainService().NotesAt(resolved.Location)
	if err != nil {
		return ResolvedNote{}, err
	}
	for _, note := range collection {
		if note != nil && note.Filename == identity.Name {
			resolved.Note = note
			return resolved, nil
		}
	}
	return ResolvedNote{}, fmt.Errorf("note %s is no longer available", identity)
}

func (c *Controller) noteOperationDir(shell Shell, resolved ResolvedNote) string {
	if resolved.Location.Scope == app.NoteScopeGlobal {
		return storage.GlobalNotesDir(shell.DomainRuntime().DataDir())
	}
	return storage.NotesDir(resolved.Project)
}

// noteOperationPath names the note's Markdown file on disk. The two collections
// live in different directories — the workspace's global-notes folder and the
// owning project's own notes folder — so the full path is the one copied value
// that says which note is meant without a second word of context, whichever
// collection the row came from.
func (c *Controller) noteOperationPath(shell Shell, resolved ResolvedNote) string {
	return filepath.Join(c.noteOperationDir(shell, resolved), resolved.Note.Filename)
}

func (c *Controller) executeNoteOperation(shell Shell,
	invocation operation.OperationInvocation,
	target operation.OperationTarget,
) operation.OperationHandlerResult {
	if target.Entity.Kind != operation.EntityKindNote {
		return operation.OperationHandlerResult{
			Err: fmt.Errorf("%s needs a note target, got %s", invocation.Descriptor.Label, target.Entity),
		}
	}
	resolved, err := c.ResolveNoteTarget(shell, target.Entity)
	if err != nil {
		return operation.OperationHandlerResult{Err: err}
	}
	if resolved.Note.ParseError != "" && notes.NoteOperationUnavailableWithParseError(invocation.Descriptor.ID) {
		return operation.OperationHandlerResult{
			Err: fmt.Errorf(
				"%s is unavailable while %s has a parse error; edit the note to repair it",
				invocation.Descriptor.Label,
				resolved.Note.Filename,
			),
		}
	}
	return c.noteOperationResult(shell, invocation, resolved)
}

//nolint:gocyclo,exhaustive // One auditable switch keeps the note catalog in a single place.
func (c *Controller) noteOperationResult(shell Shell,
	invocation operation.OperationInvocation,
	resolved ResolvedNote,
) operation.OperationHandlerResult {
	note := resolved.Note
	command := func(cmd tea.Cmd) operation.OperationHandlerResult {
		return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{Cmd: cmd}}
	}

	switch invocation.Descriptor.ID {
	case notes.OperationNoteOpen:
		editor, err := external.ResolveNoteEditor()
		if err != nil {
			return operation.OperationHandlerResult{Err: err}
		}
		return command(notes.NoteEditCmd(note, resolved.Location, c.noteOperationDir(shell, resolved), editor))

	case notes.OperationNoteRename:
		return c.renameNoteOperation(shell, invocation, resolved)

	case notes.OperationNoteMove:
		return operation.OperationHandlerResult{Message: notes.ShowNotesManagerMsg{
			Scope: route.AllNotesScope(),
			Focus: notes.NoteRowIdentity{
				Location: resolved.Location,
				Filename: note.Filename,
			},
			MoveSource: notes.NewNoteMutationRef(resolved.Location, note),
		}}

	case notes.OperationNoteCopyTitle:
		return copyNoteValue(note.Title, "Note title copied to clipboard")

	case notes.OperationNoteCopyPath:
		return copyNoteValue(c.noteOperationPath(shell, resolved), "Note path copied to clipboard")

	case notes.OperationNoteDelete:
		return command(notes.NoteMutationCmd(shell.DomainService(), resolved.Location, app.NoteBatchRequest{
			Operations:  []app.NoteMutationRequest{{Kind: app.NoteDelete, Location: resolved.Location, Note: note}},
			UndoAction:  "Delete Note",
			UndoContext: note.Title,
		}, "Deleted: "+note.Title))

	default:
		return operation.OperationHandlerResult{Err: fmt.Errorf("%w: %s", operation.ErrOperationHandler, invocation.Descriptor.ID)}
	}
}

// copyNoteValue is the one clipboard seam both note copies share. Neither carries
// a message, so the handler's Status is the whole outcome and the shell's status
// strip is what confirms the copy on every host that lists a note row.
func copyNoteValue(value string, status string) operation.OperationHandlerResult {
	if err := external.CopyText(value); err != nil {
		return operation.OperationHandlerResult{Err: fmt.Errorf("couldn't copy to clipboard: %w", err)}
	}
	return operation.OperationHandlerResult{Status: status}
}

// renameNoteOperation renames through the same atomic create/delete note
// mutation the Notes manager uses. The new title arrives as the invocation
// value because the host owns the prompt.
func (c *Controller) renameNoteOperation(shell Shell,
	invocation operation.OperationInvocation,
	resolved ResolvedNote,
) operation.OperationHandlerResult {
	title := strings.TrimSpace(operation.StringValue(invocation))
	if title == "" {
		return operation.OperationHandlerResult{Err: fmt.Errorf("%s requires a new title", invocation.Descriptor.Label)}
	}
	if title == resolved.Note.Title {
		return operation.OperationHandlerResult{Status: "Rename canceled: same title"}
	}
	return operation.OperationHandlerResult{Message: operation.OperationCommandResultMsg{
		Cmd: notes.NoteMutationCmd(shell.DomainService(), resolved.Location, app.NoteBatchRequest{
			Operations: []app.NoteMutationRequest{{
				Kind:     app.NoteRename,
				Location: resolved.Location,
				Note:     resolved.Note,
				NewTitle: title,
			}},
			UndoAction:  "Rename Note",
			UndoContext: resolved.Note.Title,
		}, fmt.Sprintf("Renamed: %s → %s", resolved.Note.Title, title)),
	}}
}
