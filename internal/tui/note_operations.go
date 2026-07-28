package tui

import (
	"fmt"
	"strings"
	"time"

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

// The note clipboard writers are package vars so tests can exercise what the
// copy actions report without writing to the real system clipboard.
var (
	copyNoteTitleToClipboard        = external.CopyNoteTitle
	copyNoteBodyToClipboard         = external.CopyNoteBody
	copyNoteTitleAndBodyToClipboard = external.CopyNoteTitleAndBody
)

// noteOperationDescriptors is the authoritative catalog of note-targeted
// operations. Each one resolves the frozen note identity at execution, so the
// same descriptor serves Finder, the Notes manager, and any other host that
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
			Placement:         PlacementFinderOperation,
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
		note(operationNoteOpen, "Open note", "Open the Notes manager on this note", OperationRiskReadOnly, "focus note"),
		note(operationNotePreview, "Preview note", "Render this note with glow", OperationRiskReadOnly, "view note"),
		note(operationNoteEdit, "Edit note", "Open this note in the editor", OperationRiskMutation),
		note(operationNoteReference, "Open note reference", "Open a read-only snapshot of this note", OperationRiskReadOnly),
		withValueProvider(
			note(operationNoteRename, "Rename note", "Give this note a new title", OperationRiskMutation),
			ValueProviderNoteTitle,
		),
		note(operationNoteTags, "Edit note tags", "Choose the tags on this note", OperationRiskMutation, "tag note"),
		descriptor(
			operationNoteLink,
			"Link note to tasks",
			"Choose the tasks in this note's project that it documents",
			OperationRiskMutation,
			projectNoteOperationApplicable,
		),
		note(operationNotePin, "Toggle note pin", "Pin or unpin this note", OperationRiskMutation, "pin note", "unpin note"),
		note(operationNoteCopyTitle, "Copy note title", "Copy this note's title to the clipboard", OperationRiskReadOnly),
		note(operationNoteCopyBody, "Copy note body", "Copy this note's body to the clipboard", OperationRiskReadOnly),
		note(operationNoteCopyTitleBody, "Copy note title and body", "Copy this note's title and body to the clipboard", OperationRiskReadOnly),
		descriptor(
			operationNoteMove,
			"Move note to project",
			"Move this global note into a project",
			OperationRiskMutation,
			globalNoteOperationApplicable,
		),
		note(operationNoteDelete, "Delete note", "Delete this note", OperationRiskDestructive),
	}
}

// noteOperationApplicable admits a note operation only when the focused entity
// is a note. currentEntityIdentity degrades to the focused task and then the
// scoped project, so an unguarded descriptor would borrow an unrelated target.
func noteOperationApplicable(ctx CommandContext) bool {
	return currentEntityIdentity(ctx).Kind == EntityKindNote
}

// globalNoteOperationApplicable restricts the move action to workspace scratch
// notes: a project note already lives in a project collection.
func globalNoteOperationApplicable(ctx CommandContext) bool {
	identity := currentEntityIdentity(ctx)
	return identity.Kind == EntityKindNote && identity.Scope == EntityScopeGlobal
}

// projectNoteOperationApplicable restricts task linking to project notes. A note
// link names a task inside the note's own project, so a workspace scratch note
// has no task scope to link within; `tt note link` rejects --global for the same
// reason. Move the note into a project first.
func projectNoteOperationApplicable(ctx CommandContext) bool {
	identity := currentEntityIdentity(ctx)
	return identity.Kind == EntityKindNote && identity.Scope == EntityScopeProject
}

// noteOperationsBlockedByParseError lists the actions that need parsed note
// content. Edit, reference, rename, and delete stay available so a malformed
// note can still be repaired or removed.
var noteOperationsBlockedByParseError = map[OperationID]bool{
	operationNotePreview:       true,
	operationNoteTags:          true,
	operationNoteLink:          true,
	operationNotePin:           true,
	operationNoteCopyTitle:     true,
	operationNoteCopyBody:      true,
	operationNoteCopyTitleBody: true,
	operationNoteMove:          true,
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
	if resolved.Note.ParseError != "" && noteOperationsBlockedByParseError[invocation.Descriptor.ID] {
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
	copyNote := func(copy func(*models.Note) (string, error), status string) OperationHandlerResult {
		if _, err := copy(note); err != nil {
			return OperationHandlerResult{Err: fmt.Errorf("copy failed: %w", err)}
		}
		return OperationHandlerResult{Status: status}
	}

	switch invocation.Descriptor.ID {
	case operationNoteOpen:
		if resolved.Location.Scope == app.NoteScopeGlobal {
			return OperationHandlerResult{Message: ShowGlobalNotesMsg{Focus: note.Filename}}
		}
		return OperationHandlerResult{Message: ShowNotesMsg{Project: resolved.Project, Focus: note.Filename}}

	case operationNotePreview:
		cmd, err := notePreviewCmd(note)
		if err != nil {
			return OperationHandlerResult{Err: err}
		}
		if cmd == nil {
			return OperationHandlerResult{Err: fmt.Errorf("glow is not available for note preview")}
		}
		return command(cmd)

	case operationNoteEdit:
		editor, err := external.ResolveEditor(m.runtime.Machine().GetEditor())
		if err != nil {
			return OperationHandlerResult{Err: err}
		}
		return command(noteEditCmd(note, resolved.Location, m.noteOperationDir(resolved), editor))

	case operationNoteReference:
		status, err := openNoteReference(
			note,
			m.noteOperationDir(resolved),
			m.runtime.Machine().GetExternalEditor(),
			notesLinkedTasks(note, m.noteOperationTasks(resolved)),
		)
		if err != nil {
			return OperationHandlerResult{Err: err}
		}
		return OperationHandlerResult{Status: status}

	case operationNoteRename:
		return m.renameNoteOperation(invocation, resolved)

	case operationNoteTags:
		return OperationHandlerResult{Message: ShowNoteTagSelectorMsg{
			Source:        newNoteMutationRef(resolved.Location, note),
			AvailableTags: buildNoteTagList(resolved.Siblings, resolved.Project),
			CurrentTags:   append([]string(nil), note.Tags...),
		}}

	case operationNoteLink:
		return OperationHandlerResult{Message: ShowNoteTaskLinkerMsg{Project: resolved.Project, Note: note}}

	case operationNotePin:
		updated := cloneNoteForCommand(note)
		updated.Pinned = !updated.Pinned
		updated.ModifiedAt = time.Now()
		status := "Pinned: " + note.Title
		if note.Pinned {
			status = "Unpinned: " + note.Title
		}
		return command(noteMutationCmd(m.appService, resolved.Location, app.NoteBatchRequest{
			Operations:  []app.NoteMutationRequest{{Kind: app.NoteUpdate, Location: resolved.Location, Note: updated}},
			UndoAction:  "Toggle Note Pin",
			UndoContext: note.Title,
		}, status, false))

	case operationNoteCopyTitle:
		return copyNote(copyNoteTitleToClipboard, "Copied title to clipboard")
	case operationNoteCopyBody:
		return copyNote(copyNoteBodyToClipboard, "Copied body to clipboard")
	case operationNoteCopyTitleBody:
		return copyNote(copyNoteTitleAndBodyToClipboard, "Copied title and body to clipboard")

	case operationNoteMove:
		return OperationHandlerResult{Message: ShowGlobalNoteMoveMsg{
			Source: newNoteMutationRef(app.GlobalNoteLocation(), note),
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

func (m *AppModel) noteOperationTasks(resolved resolvedNote) []*models.Task {
	if resolved.Project != nil {
		return resolved.Project.Tasks
	}
	tasks := make([]*models.Task, 0)
	for _, project := range m.activeProjects() {
		if project != nil {
			tasks = append(tasks, project.Tasks...)
		}
	}
	return tasks
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
