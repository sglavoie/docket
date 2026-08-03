package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// Note operation IDs. Every one of them targets a single note identity, which
// names the collection (project or global) and the filename inside it.
const (
	operationNoteOpen          OperationID = "tui.note.open"
	operationNotePreview       OperationID = "tui.note.preview"
	operationNoteEdit          OperationID = "tui.note.edit"
	operationNoteReference     OperationID = "tui.note.open-reference"
	operationNoteRename        OperationID = "tui.note.rename"
	operationNoteTags          OperationID = "tui.note.tags"
	operationNoteLink          OperationID = "tui.note.link-tasks"
	operationNotePin           OperationID = "tui.note.pin"
	operationNoteCopyTitle     OperationID = "tui.note.copy-title"
	operationNoteCopyBody      OperationID = "tui.note.copy-body"
	operationNoteCopyTitleBody OperationID = "tui.note.copy-title-body"
	operationNoteMove          OperationID = "tui.note.move-to-project"
	operationNoteDelete        OperationID = "tui.note.delete"
)

// noteOperationDescriptors is the authoritative catalog of note-targeted
// operations. Each one resolves the frozen note identity at execution, so the
// same descriptor serves the Notes manager and any other host that
// names a note row.
func noteOperationDescriptors() []OperationDescriptor {
	descriptor := func(
		id OperationID,
		label string,
		description string,
		risk OperationRisk,
		applicability OperationApplicability,
		aliases ...string,
	) OperationDescriptor {
		return OperationDescriptor{
			ID:                id,
			Label:             label,
			Description:       description,
			Aliases:           aliases,
			Category:          OperationCategoryNote,
			TargetSource:      OperationTargetCurrentEntity,
			TargetCardinality: TargetCardinalityOne,
			Applicability:     applicability,
			Risk:              risk,
		}
	}
	note := func(
		id OperationID,
		label string,
		description string,
		risk OperationRisk,
		aliases ...string,
	) OperationDescriptor {
		return descriptor(id, label, description, risk, noteOperationApplicable, aliases...)
	}
	return []OperationDescriptor{
		note(operationNoteOpen, "Open note", "Open this note in the external editor", OperationRiskMutation),
		withValueProvider(
			note(operationNoteRename, "Rename note", "Give this note a new title", OperationRiskMutation),
			ValueProviderNoteTitle,
		),
		note(operationNoteMove, "Move note", "Move this note to another location", OperationRiskMutation),
		note(operationNoteDelete, "Delete note", "Delete this note", OperationRiskDestructive),
	}
}

// noteOperationApplicable admits a note operation only when the focused entity
// is a note. currentEntityIdentity degrades to the focused task and then the
// scoped project, so an unguarded descriptor would borrow an unrelated target.
func noteOperationApplicable(ctx CommandContext) bool {
	return currentEntityIdentity(ctx).Kind == EntityKindNote
}

// noteOperationsUnavailableWithParseError lists the restored actions that need parsed
// note content. Open and delete stay available so a malformed note can still be
// repaired or removed.
var noteOperationsUnavailableWithParseError = map[OperationID]bool{
	operationNoteMove: true,
}

func (m *AppModel) registerNoteOperations() error {
	if m == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	if err := m.operationExecutor.RegisterValueProvider(
		ValueProviderNoteTitle,
		pendingTextValueProvider,
	); err != nil {
		return err
	}
	for _, descriptor := range noteOperationDescriptors() {
		if err := m.registerOperation(descriptor, OperationBinding{
			SingleHandler:       m.executeNoteOperation,
			DelegateTransaction: true,
		}); err != nil {
			return err
		}
	}
	return nil
}

// resolvedNote is one note reloaded from its collection at execution time,
// together with the collection it came from.
type resolvedNote struct {
	Location app.NoteLocation
	Project  *models.Project
	Note     *models.Note
	Siblings []*models.Note
}

// resolveNoteTarget reloads the note the identity names. A note whose file or
// owning project disappeared since the row was built fails loudly instead of
// acting on stale metadata.
func (m *AppModel) resolveNoteTarget(identity EntityIdentity) (resolvedNote, error) {
	resolved := resolvedNote{}
	switch identity.Scope {
	case EntityScopeGlobal:
		resolved.Location = app.GlobalNoteLocation()
	case EntityScopeProject:
		project := m.projectByID(identity.ProjectID)
		if project == nil {
			return resolvedNote{}, fmt.Errorf("project %s is no longer available", identity.ProjectID)
		}
		resolved.Project = project
		resolved.Location = app.ProjectNoteLocation(project.ID)
	default:
		return resolvedNote{}, fmt.Errorf("note %s names no collection", identity)
	}
	if m.appService == nil {
		return resolvedNote{}, fmt.Errorf("note service is unavailable")
	}
	notes, err := m.appService.NotesAt(resolved.Location)
	if err != nil {
		return resolvedNote{}, err
	}
	resolved.Siblings = notes
	for _, note := range notes {
		if note != nil && note.Filename == identity.Name {
			resolved.Note = note
			return resolved, nil
		}
	}
	return resolvedNote{}, fmt.Errorf("note %s is no longer available", identity)
}

func (m *AppModel) noteOperationDir(resolved resolvedNote) string {
	if resolved.Location.Scope == app.NoteScopeGlobal {
		return storage.GlobalNotesDir(m.runtime.DataDir())
	}
	return storage.NotesDir(resolved.Project)
}

func (m *AppModel) executeNoteOperation(
	invocation OperationInvocation,
	target OperationTarget,
) OperationHandlerResult {
	if target.Entity.Kind != EntityKindNote {
		return OperationHandlerResult{
			Err: fmt.Errorf("%s needs a note target, got %s", invocation.Descriptor.Label, target.Entity),
		}
	}
	resolved, err := m.resolveNoteTarget(target.Entity)
	if err != nil {
		return OperationHandlerResult{Err: err}
	}
	if resolved.Note.ParseError != "" && noteOperationsUnavailableWithParseError[invocation.Descriptor.ID] {
		return OperationHandlerResult{
			Err: fmt.Errorf(
				"%s is unavailable while %s has a parse error; edit the note to repair it",
				invocation.Descriptor.Label,
				resolved.Note.Filename,
			),
		}
	}
	return m.noteOperationResult(invocation, resolved)
}

//nolint:gocyclo,exhaustive // One auditable switch keeps the note catalog in a single place.
func (m *AppModel) noteOperationResult(
	invocation OperationInvocation,
	resolved resolvedNote,
) OperationHandlerResult {
	note := resolved.Note
	command := func(cmd tea.Cmd) OperationHandlerResult {
		return OperationHandlerResult{Message: OperationCommandResultMsg{Cmd: cmd}}
	}

	switch invocation.Descriptor.ID {
	case operationNoteOpen:
		editor, err := external.ResolveNoteEditor()
		if err != nil {
			return OperationHandlerResult{Err: err}
		}
		return command(noteEditCmd(note, resolved.Location, m.noteOperationDir(resolved), editor))

	case operationNoteRename:
		return m.renameNoteOperation(invocation, resolved)

	case operationNoteMove:
		return OperationHandlerResult{Message: ShowNotesManagerMsg{
			Scope: AllNotesScope(),
			Focus: NoteRowIdentity{
				Location: resolved.Location,
				Filename: note.Filename,
			},
			MoveSource: newNoteMutationRef(resolved.Location, note),
		}}

	case operationNoteDelete:
		return command(noteMutationCmd(m.appService, resolved.Location, app.NoteBatchRequest{
			Operations:  []app.NoteMutationRequest{{Kind: app.NoteDelete, Location: resolved.Location, Note: note}},
			UndoAction:  "Delete Note",
			UndoContext: note.Title,
		}, "Deleted: "+note.Title, false))

	default:
		return OperationHandlerResult{Err: fmt.Errorf("%w: %s", ErrOperationHandler, invocation.Descriptor.ID)}
	}
}

// renameNoteOperation renames through the same atomic create/delete note
// mutation the Notes manager uses. The new title arrives as the invocation
// value because the host owns the prompt.
func (m *AppModel) renameNoteOperation(
	invocation OperationInvocation,
	resolved resolvedNote,
) OperationHandlerResult {
	title := strings.TrimSpace(operationStringValue(invocation))
	if title == "" {
		return OperationHandlerResult{Err: fmt.Errorf("%s requires a new title", invocation.Descriptor.Label)}
	}
	if title == resolved.Note.Title {
		return OperationHandlerResult{Status: "Rename canceled: same title"}
	}
	return OperationHandlerResult{Message: OperationCommandResultMsg{
		Cmd: noteMutationCmd(m.appService, resolved.Location, app.NoteBatchRequest{
			Operations: []app.NoteMutationRequest{{
				Kind:     app.NoteRename,
				Location: resolved.Location,
				Note:     resolved.Note,
				NewTitle: title,
			}},
			UndoAction:  "Rename Note",
			UndoContext: resolved.Note.Title,
		}, fmt.Sprintf("Renamed: %s → %s", resolved.Note.Title, title), false),
	}}
}

// operationStringValue reads a trimmed string invocation value, which is how
// hosts hand a prompted name to a rename operation.
func operationStringValue(invocation OperationInvocation) string {
	if !invocation.HasValue {
		return ""
	}
	value, ok := invocation.Value.(string)
	if !ok {
		return ""
	}
	return value
}
