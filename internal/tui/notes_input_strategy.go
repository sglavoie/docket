package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

// Notes surface operation IDs. They name what the Notes grammar does to the
// manager itself rather than to a note, so they stay apart from the tui.note.*
// operations a note row carries into any host that lists it.
const (
	operationNotesCreate  OperationID = "tui.notes.create"
	operationNotesSearch  OperationID = "tui.notes.search"
	operationNotesScope   OperationID = "tui.notes.scope"
	operationNotesActions OperationID = "tui.notes.actions"
)

// NotesInputStrategy is the Notes manager's whole direct key grammar and the
// single declaration of what each of its keys means. Five of its operations are
// note descriptors any host reaches; the four beside them belong to the manager,
// because creating, searching, rescoping, and opening the action sheet act on
// the list rather than on the note under the cursor.
//
// y and Y copy the note under the cursor, taking the row's primary text and its
// fuller qualified form the way they do on the Outliner, Comments, and
// Attachments: the title, and the full path of the Markdown file behind it.
//
// Deleting a note is the one note operation with no direct key. It is
// destructive, so it reaches the user only through the Space sheet and its
// confirmation, which is why operationNoteDelete is absent from this table.
//
// The cursor keys, the note ledger's undo and redo, help, and Esc resolve to the
// core operations because they are the same acts everywhere. Those descriptors
// answer only to the Outliner, so they document the Notes grammar without
// dispatching here: the manager moves its own cursor and unwinds its own modal
// rungs.
//
// No binding carries footer metadata. The manager's footer has to narrow with
// the rename, search, and confirmation sub-states, which only the gated keymap
// beside this strategy can express, so ranking a key here would be a second
// description of the same footer with no way to stay in step.
func NotesInputStrategy() InputStrategy {
	return InputStrategy{
		ID: InputStrategyNotes,
		Bindings: []OperationKeyBinding{
			{OperationID: operationNoteOpen, Keys: []string{"enter"}, DisplayKey: "Enter"},
			{OperationID: operationNotesCreate, Keys: []string{"a"}, DisplayKey: "a"},
			{OperationID: operationNoteRename, Keys: []string{"e"}, DisplayKey: "e"},
			{OperationID: operationNoteMove, Keys: []string{"%"}, DisplayKey: "%"},
			{OperationID: operationNotesSearch, Keys: []string{"/"}, DisplayKey: "/"},
			{OperationID: operationNotesScope, Keys: []string{"p"}, DisplayKey: "p"},
			{OperationID: operationNoteCopyTitle, Keys: []string{"y"}, DisplayKey: "y"},
			{OperationID: operationNoteCopyPath, Keys: []string{"Y"}, DisplayKey: "Y"},
			{OperationID: operationNotesActions, Keys: []string{"space"}, DisplayKey: "Space"},
			{OperationID: operationCoreUndo, Keys: []string{"u"}, DisplayKey: "u"},
			{OperationID: operationCoreRedo, Keys: []string{"ctrl+r"}, DisplayKey: "Ctrl+R"},
			{OperationID: operationCoreCursorDown, Keys: []string{"j", "down"}, DisplayKey: "j/↓"},
			{OperationID: operationCoreCursorUp, Keys: []string{"k", "up"}, DisplayKey: "k/↑"},
			{OperationID: operationCoreCursorTop, Keys: []string{"home"}, Sequence: []string{"g", "g"}, DisplayKey: "gg/Home", HintLabel: "top"},
			{OperationID: operationCoreCursorBottom, Keys: []string{"G", "end"}, DisplayKey: "G/End"},
			{OperationID: operationCoreCursorPageDown, Keys: []string{"ctrl+d", "pgdown"}, DisplayKey: "Ctrl+D"},
			{OperationID: operationCoreCursorPageUp, Keys: []string{"ctrl+u", "pgup"}, DisplayKey: "Ctrl+U"},
			{OperationID: operationCoreHelp, Keys: []string{"?"}, DisplayKey: "?"},
			{OperationID: operationCoreBack, Keys: []string{"esc"}, DisplayKey: "Esc"},
		},
		ChordGroups: []OperationChordGroup{
			{Sequence: []string{"g"}, HintLabel: "go to"},
		},
	}
}

// dispatchNotesKey resolves one browse key through the Notes grammar and runs
// whatever operation it reaches. The row under the cursor is frozen into the
// context here because every note operation resolves its target from it, and an
// empty list therefore names nothing and refuses them.
func (m *NotesViewModel) dispatchNotesKey(key string) tea.Cmd {
	if m.dispatch == nil {
		return nil
	}
	_, cmd := m.dispatch(InputStrategyNotes, []string{key}, m.notesCommandContext())
	return cmd
}

func (m *NotesViewModel) notesCommandContext() CommandContext {
	ctx := PresentationCommandContext(m.presentationID())
	ctx.Capabilities.Add(CapabilityNotesManager)
	if row := m.currentRow(); row != nil {
		ctx.CurrentEntity = noteRowEntity(*row)
	}
	return ctx
}

// presentationID names the renderer the current scope selects, matching the
// hub's own mapping so a Notes-scoped binding resolves under either spelling.
func (m *NotesViewModel) presentationID() presentation.PresentationID {
	if m.scope.Kind == NotesScopeProject {
		return presentation.ViewNotes
	}
	return presentation.ViewGlobalNotes
}

// noteRowEntity names the note a row carries. A row without a note is not a
// note cursor, so it yields the zero identity rather than a half-named one.
func noteRowEntity(row LocatedNoteRow) EntityIdentity {
	if row.Note == nil {
		return EntityIdentity{}
	}
	if row.Location.Scope == app.NoteScopeGlobal {
		return GlobalNoteEntity(row.Note.Filename)
	}
	return ProjectNoteEntity(row.Location.ProjectID, row.Note.Filename)
}

// notesSurfaceOperationDescriptors declares what the Notes grammar reaches that
// no note row carries: creating a note, and the list vocabulary the manager
// applies to its own query and scope. None of them names a row, so all three
// stay available while the list is empty, which is what makes an empty
// collection recoverable.
func notesSurfaceOperationDescriptors() []OperationDescriptor {
	surface := func(
		id OperationID,
		label string,
		description string,
		category OperationCategory,
		risk OperationRisk,
	) OperationDescriptor {
		return OperationDescriptor{
			ID:                   id,
			Label:                label,
			Description:          description,
			Category:             category,
			TargetSource:         OperationTargetGlobal,
			TargetCardinality:    TargetCardinalityNone,
			RequiredCapabilities: NewCapabilitySet(CapabilityNotesManager),
			Risk:                 risk,
		}
	}
	return []OperationDescriptor{
		surface(
			operationNotesCreate,
			"New note",
			"Create a note after choosing its location",
			OperationCategoryNote,
			OperationRiskMutation,
		),
		surface(
			operationNotesSearch,
			"Search notes",
			"Search note titles and bodies",
			OperationCategoryFilter,
			OperationRiskReadOnly,
		),
		surface(
			operationNotesScope,
			"Choose Notes scope",
			"Show all notes, global notes, or one project's notes",
			OperationCategoryNote,
			OperationRiskReadOnly,
		),
		surface(
			operationNotesActions,
			"Note actions",
			"Open the action sheet for the note under the cursor",
			OperationCategoryNote,
			OperationRiskReadOnly,
		),
	}
}

// notesOperationManifest is the authoritative ordered Notes action list. Nine
// of the ten entries are DirectOnly: they keep their descriptor and value
// provider and leave the Space sheet to Delete, which is destructive and so
// carries no direct key at all.
//
// Space itself is manifested for the same reason the other eight are: a manifest
// is all validateOperationOnePath inspects, so leaving the key that opens the
// sheet out of it was the one binding the gate could not check. The sheet lists
// notesActionSheetOperations, not this manifest, so the row it opens is
// unchanged.
func notesOperationManifest() []operationManifestEntry {
	return []operationManifestEntry{
		{ID: operationNoteOpen, Label: "Open note", Risk: OperationRiskMutation, DirectOnly: true},
		{ID: operationNotesCreate, Label: "New note", Risk: OperationRiskMutation, DirectOnly: true},
		{ID: operationNoteRename, Label: "Rename note", Provider: ValueProviderNoteTitle, Risk: OperationRiskMutation, DirectOnly: true},
		{ID: operationNoteMove, Label: "Move note", Risk: OperationRiskMutation, DirectOnly: true},
		{ID: operationNotesSearch, Label: "Search notes", Risk: OperationRiskReadOnly, DirectOnly: true},
		{ID: operationNotesScope, Label: "Choose Notes scope", Risk: OperationRiskReadOnly, DirectOnly: true},
		{ID: operationNoteCopyTitle, Label: "Copy note title", Risk: OperationRiskReadOnly, DirectOnly: true},
		{ID: operationNoteCopyPath, Label: "Copy note path", Risk: OperationRiskReadOnly, DirectOnly: true},
		{ID: operationNotesActions, Label: "Note actions", Risk: OperationRiskReadOnly, DirectOnly: true},
		{ID: operationNoteDelete, Label: "Delete note", Risk: OperationRiskDestructive},
	}
}

// notesActionSheetOperations is the Notes Space sheet. It holds Delete alone:
// Notes has no multi-select, so this is one note's action sheet rather than a
// bulk menu, and Delete reaches the user only here behind a confirmation.
var notesActionSheetOperations = []OperationID{operationNoteDelete}

// validateNotesOperationOnePath holds Notes to the shared one-path rule: a
// manifested operation is either bound directly and marked DirectOnly or listed
// in the Space sheet, never both.
func validateNotesOperationOnePath(strategy InputStrategy) error {
	return validateOperationOnePath(notesOperationManifest(), strategy)
}

// notesHostedNoteOperations are the note operations the manager performs itself.
func notesHostedNoteOperations() []OperationID {
	return []OperationID{
		operationNoteOpen,
		operationNoteRename,
		operationNoteMove,
		operationNoteDelete,
	}
}
