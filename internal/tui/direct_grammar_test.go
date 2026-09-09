package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/entityops"

	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
)

// TestFinalDirectGrammarOwnsEveryDocumentedKeyOnce runs per strategy because
// every surface with its own grammar owns its keys the same way: one key, one
// operation, unless the cursor disambiguates two honest descriptors.
func TestFinalDirectGrammarOwnsEveryDocumentedKeyOnce(t *testing.T) {
	t.Run("Outliner", assertOutlinerOwnsEveryDocumentedKeyOnce)
	t.Run("Project Hub", assertProjectHubOwnsEveryDocumentedKeyOnce)
	t.Run("Notes", assertNotesOwnsEveryDocumentedKeyOnce)
	t.Run("Attachments", assertAttachmentsOwnsEveryDocumentedKeyOnce)
	t.Run("Comments", assertCommentsOwnsEveryDocumentedKeyOnce)
}

// assertCommentsOwnsEveryDocumentedKeyOnce spells the whole Comments grammar.
// Six keys act on the comment under the cursor, Space raises the sheet that
// holds the seventh, and the rest navigate, undo, explain, or close, so the
// table is the panel's complete contract. `n`, `d`, and `x` are absent because
// the panel retired them; `u` and `Ctrl+R` are present because adding, editing,
// and removing a comment are workspace-ledger transactions, so the panel that
// made them answers the app-wide undo rather than advertising one it consumes in
// silence. gg is spelled as the complete path it is, the way the Hub table below
// spells it.
func assertCommentsOwnsEveryDocumentedKeyOnce(t *testing.T) {
	direct := map[string]operation.OperationID{
		"enter":  entityops.OperationCommentExpand,
		"a":      entityops.OperationCommentAdd,
		"e":      entityops.OperationCommentEdit,
		"E":      entityops.OperationCommentEditExternal,
		"space":  entityops.OperationCommentActions,
		"y":      entityops.OperationCommentCopy,
		"Y":      entityops.OperationCommentCopyAll,
		"u":      operation.OperationCoreUndo,
		"ctrl+r": operation.OperationCoreRedo,
		"j":      operation.OperationCoreCursorDown,
		"down":   operation.OperationCoreCursorDown,
		"k":      operation.OperationCoreCursorUp,
		"up":     operation.OperationCoreCursorUp,
		"gg":     operation.OperationCoreCursorTop,
		"home":   operation.OperationCoreCursorTop,
		"G":      operation.OperationCoreCursorBottom,
		"end":    operation.OperationCoreCursorBottom,
		"?":      operation.OperationCoreHelp,
		"esc":    operation.OperationCoreBack,
	}
	assertStrategyOwnsEveryKeyOnce(t, entityops.CommentsInputStrategy(), direct)
}

// assertAttachmentsOwnsEveryDocumentedKeyOnce spells the whole Attachments
// grammar. Seven keys act on the item under the cursor, Space raises the sheet
// that holds the remaining five acts, and the rest navigate, undo, explain, or
// close, so the table is the panel's complete contract. `e`, `o`, `d`, `x`, `O`,
// `Ctrl+E`, and `Ctrl+O` are absent because the panel retired them; `u` left
// that list once unlink moved to the sheet, and now serves the global undo the
// way it does on every other surface, because adding and removing an attachment
// are workspace-ledger transactions.
func assertAttachmentsOwnsEveryDocumentedKeyOnce(t *testing.T) {
	direct := map[string]operation.OperationID{
		"enter":  entityops.OperationAttachmentOpen,
		"a":      entityops.OperationAttachmentAdd,
		"E":      entityops.OperationAttachmentOpenInEditor,
		"y":      entityops.OperationAttachmentCopyPath,
		"Y":      entityops.OperationAttachmentCopyPathWithLine,
		"J":      entityops.OperationAttachmentReorderDown,
		"K":      entityops.OperationAttachmentReorderUp,
		"space":  entityops.OperationAttachmentActions,
		"u":      operation.OperationCoreUndo,
		"ctrl+r": operation.OperationCoreRedo,
		"j":      operation.OperationCoreCursorDown,
		"down":   operation.OperationCoreCursorDown,
		"k":      operation.OperationCoreCursorUp,
		"up":     operation.OperationCoreCursorUp,
		"gg":     operation.OperationCoreCursorTop,
		"home":   operation.OperationCoreCursorTop,
		"G":      operation.OperationCoreCursorBottom,
		"end":    operation.OperationCoreCursorBottom,
		"?":      operation.OperationCoreHelp,
		"esc":    operation.OperationCoreBack,
	}

	owners := assertStrategyOwnsEveryKeyOnce(t, entityops.AttachmentsInputStrategy(), direct)

	// The external editor is E, the slot it holds on every other surface, so the
	// e that used to open it resolves to nothing here alongside the six
	// spellings the panel retired before it. e is the one the count above cannot
	// guard on its own: an editor binding that drifted back to the lowercase key
	// would keep the same count and only this loop would notice.
	for _, removed := range []string{"e", "o", "d", "x", "O", "ctrl+e", "ctrl+o"} {
		if owners := owners[removed]; len(owners) > 0 {
			t.Errorf("removed key %q still resolves to %s", removed, owners[0])
		}
	}
}

// assertNotesOwnsEveryDocumentedKeyOnce spells the whole Notes grammar. Nine
// keys act and the rest navigate or unwind, so the table is the surface's
// complete contract rather than a sample of it. Delete is absent because the
// Space sheet is its one path. gg is spelled as the complete path it is, the way
// the Hub table below spells it.
func assertNotesOwnsEveryDocumentedKeyOnce(t *testing.T) {
	direct := map[string]operation.OperationID{
		"enter":  notes.OperationNoteOpen,
		"a":      notes.OperationNotesCreate,
		"e":      notes.OperationNoteRename,
		"%":      notes.OperationNoteMove,
		"/":      notes.OperationNotesSearch,
		"p":      notes.OperationNotesScope,
		"y":      notes.OperationNoteCopyTitle,
		"Y":      notes.OperationNoteCopyPath,
		"space":  notes.OperationNotesActions,
		"u":      operation.OperationCoreUndo,
		"ctrl+r": operation.OperationCoreRedo,
		"j":      operation.OperationCoreCursorDown,
		"down":   operation.OperationCoreCursorDown,
		"k":      operation.OperationCoreCursorUp,
		"up":     operation.OperationCoreCursorUp,
		"gg":     operation.OperationCoreCursorTop,
		"home":   operation.OperationCoreCursorTop,
		"G":      operation.OperationCoreCursorBottom,
		"end":    operation.OperationCoreCursorBottom,
		"ctrl+d": operation.OperationCoreCursorPageDown,
		"pgdown": operation.OperationCoreCursorPageDown,
		"ctrl+u": operation.OperationCoreCursorPageUp,
		"pgup":   operation.OperationCoreCursorPageUp,
		"?":      operation.OperationCoreHelp,
		"esc":    operation.OperationCoreBack,
	}
	assertStrategyOwnsEveryKeyOnce(t, notes.NotesInputStrategy(), direct)
}

// assertStrategyOwnsEveryKeyOnce holds one surface grammar to the one-key
// rule: every declared path resolves to exactly the operation the table names,
// and the grammar declares no path the table omits.
func assertStrategyOwnsEveryKeyOnce(t *testing.T, strategy operation.InputStrategy, direct map[string]operation.OperationID) map[string][]operation.OperationID {
	t.Helper()
	owners := make(map[string][]operation.OperationID)
	for _, binding := range strategy.Bindings {
		for _, path := range binding.Paths() {
			spelling := strings.Join(path, "")
			if len(owners[spelling]) > 0 && !binding.SharedKey {
				t.Errorf("direct key %q is owned by both %s and %s", spelling, owners[spelling][0], binding.OperationID)
			}
			owners[spelling] = append(owners[spelling], binding.OperationID)
		}
	}
	if len(owners) != len(direct) {
		t.Fatalf("direct key count = %d, want %d; got %v", len(owners), len(direct), owners)
	}
	for key, operationID := range direct {
		if got := owners[key]; !slices.Equal(got, []operation.OperationID{operationID}) {
			t.Errorf("direct key %q = %v, want %s", key, got, operationID)
		}
	}
	return owners
}

// assertProjectHubOwnsEveryDocumentedKeyOnce spells complete paths rather than
// root keys: the Hub grammar holds a single chord, so gg belongs in the same
// table as the presses beside it.
func assertProjectHubOwnsEveryDocumentedKeyOnce(t *testing.T) {
	direct := map[string]operation.OperationID{
		"/":      projecthub.OperationProjectHubSearch,
		"j":      operation.OperationCoreCursorDown,
		"down":   operation.OperationCoreCursorDown,
		"k":      operation.OperationCoreCursorUp,
		"up":     operation.OperationCoreCursorUp,
		"gg":     operation.OperationCoreCursorTop,
		"home":   operation.OperationCoreCursorTop,
		"G":      operation.OperationCoreCursorBottom,
		"end":    operation.OperationCoreCursorBottom,
		"ctrl+d": operation.OperationCoreCursorPageDown,
		"ctrl+u": operation.OperationCoreCursorPageUp,
		"ctrl+f": projecthub.OperationProjectHubPreviewPageDown,
		"ctrl+b": projecthub.OperationProjectHubPreviewPageUp,
		"a":      operation.OperationScopeCreateProject,
		"e":      operation.OperationScopeRenameProject,
		"*":      operation.OperationScopePinProject,
		"#":      operation.OperationProjectSetTags,
		"ctrl+n": operation.OperationScopeOpenProjectNote,
		"v":      projecthub.OperationProjectHubToggleSelection,
		"ctrl+a": projecthub.OperationProjectHubSelectAll,
		"space":  projecthub.OperationProjectHubActions,
		"enter":  operation.OperationProjectOpen,
		"ctrl+t": projecthub.OperationProjectHubTaskFilter,
		"u":      operation.OperationCoreUndo,
		"ctrl+r": operation.OperationCoreRedo,
		"?":      operation.OperationCoreHelp,
		"esc":    operation.OperationCoreBack,
	}

	owners := assertStrategyOwnsEveryKeyOnce(t, projecthub.ProjectHubInputStrategy(), direct)

	// New is a, select-all is Ctrl+A, and g is the go-to prefix, so the three
	// spellings they replaced resolve to nothing at all.
	for _, removed := range []string{"n", "A", "g"} {
		if owners := owners[removed]; len(owners) > 0 {
			t.Errorf("removed key %q still resolves to %s", removed, owners[0])
		}
	}
}

func assertOutlinerOwnsEveryDocumentedKeyOnce(t *testing.T) {
	strategy := SimplifiedInputStrategy()
	direct := map[string]operation.OperationID{
		"a":      operation.OperationCoreCapture,
		"E":      operation.OperationCoreEditDescription,
		"ctrl+e": operation.OperationCoreExternalEditor,
		"O":      operation.OperationCoreOutlineEditor,
		"A":      operation.OperationTaskNewChild,
		// c appends to whichever checklist the cursor names, the owner task's or
		// the one an item already belongs to, so one descriptor answers both rows
		// and the key is not shared. x is no longer shared for the same reason:
		// starting a checklist and appending to one are now one act.
		"c": operation.OperationChecklistAddItem,
		"x": operation.OperationChecklistToggleItem,
		"j": operation.OperationCoreCursorDown, "down": operation.OperationCoreCursorDown,
		"k": operation.OperationCoreCursorUp, "up": operation.OperationCoreCursorUp,
		"home": operation.OperationCoreCursorTop,
		"G":    operation.OperationCoreCursorBottom, "end": operation.OperationCoreCursorBottom,
		"ctrl+u": operation.OperationCoreCursorPageUp, "pgup": operation.OperationCoreCursorPageUp,
		"ctrl+d": operation.OperationCoreCursorPageDown, "pgdown": operation.OperationCoreCursorPageDown,
		"ctrl+o": operation.OperationCoreJumpBackward, "ctrl+i": operation.OperationCoreJumpForward,
		"ctrl+l": operation.OperationCoreFocusLatest,
		"ctrl+p": operation.OperationCoreSelectPinned,
		"h":      operation.OperationCoreCollapseParent, "left": operation.OperationCoreCollapseParent,
		"l": operation.OperationCoreExpand, "right": operation.OperationCoreExpand,
		"s": operation.OperationCoreToggleSections,
		"p": operation.OperationCoreToggleProjectGroup,
		"t": operation.OperationCoreToggleSubtrees,
		"n": operation.OperationCoreNextHeader, "N": operation.OperationCorePreviousHeader,
		"[": operation.OperationTaskOutdent, "]": operation.OperationTaskIndent,
		">": operation.OperationTaskSetParent,
		"<": operation.OperationTaskAdoptChildren,
		"~": operation.OperationTaskReferences,
		"%": operation.OperationTaskMoveToProject,
		"&": operation.OperationTaskAttachments,
		// r is not shared either: only a task carries a resume note, so the key
		// never asks what the cursor names.
		"r": operation.OperationTaskSetResumeNote,
		// D is not shared either: repeating names one task's fields, and a project
		// has no draft to seed.
		"D":           operation.OperationTaskRepeat,
		"shift+enter": operation.OperationCoreFocusParent,
		// Peek reads as a root letter now rather than a modified Enter, and its two
		// page turns spell the same pair every other pageable body on the product
		// answers to.
		"b":      operation.OperationCorePeek,
		"ctrl+f": operation.OperationCorePeekPageDown,
		"ctrl+b": operation.OperationCorePeekPageUp,
		"/":      operation.OperationCoreScopeSearch,
		"v":      operation.OperationCoreMultiSelect, "V": operation.OperationCoreRangeSelect,
		"ctrl+a": operation.OperationCoreSelectAll,
		"P":      operation.OperationCoreSelectProject,
		"\\":     operation.OperationCoreNamedLenses,
		"ctrl+n": operation.OperationCoreNotes, "T": operation.OperationCoreToggleTheme,
		"M": operation.OperationCoreToggleMenuBar, "ctrl+m": operation.OperationCoreOpenMenuBar,
		"Q": operation.OperationCoreQueryReadout,
		"u": operation.OperationCoreUndo, "ctrl+r": operation.OperationCoreRedo,
		"esc": operation.OperationCoreBack, "?": operation.OperationCoreHelp, "ctrl+c": operation.OperationCoreQuit,
		"tab": operation.OperationCoreNextWorkMode, "shift+tab": operation.OperationCorePreviousWorkMode,
		"@": operation.OperationTaskSetDueDate,
		// = is not shared: there is no project review flag for the cursor to
		// disambiguate, so the key names a task and only a task.
		"=": operation.OperationTaskToggleReview,
		// R acknowledges what = flagged. It is not shared either, and for the same
		// reason: only a task can await review.
		"R": operation.OperationTaskMarkReviewed,
		"y": operation.OperationTaskCopyTitle, "Y": operation.OperationTaskCopyDetails,
		"ctrl+y": operation.OperationTaskCopyTitleDescription,
		// The digit row: 0 returns to the launch view and 1-7 are the seven modes
		// in Tab-cycle order, each a second spelling on the go-to chord's binding.
		"0": operation.OperationCoreResetToLaunchView,
		"1": operation.OperationCoreGotoAll, "2": operation.OperationCoreGotoToday,
		"3": operation.OperationCoreGotoInbox, "4": operation.OperationCoreGotoNext,
		"5": operation.OperationCoreGotoBacklog, "6": operation.OperationCoreGotoDue,
		"7": operation.OperationCoreGotoRecent,
	}

	// A shared key is one the cursor disambiguates, and each meaning keeps its own
	// descriptor so help and the footer can name both honestly. #, *, and C all
	// ask whether the cursor names a task or a project; e and Space ask the same
	// question with a third answer, the checklist row; the rest ask whether it
	// names a checklist item, and the checklist meaning is listed first because
	// resolution takes the first applicable binding.
	shared := map[string][]operation.OperationID{
		"#": {operation.OperationTaskSetTags, operation.OperationProjectSetTags},
		"*": {operation.OperationTaskTogglePin, operation.OperationScopePinProject},
		// C is the shifted write to the comment panel's read: ; opens comments and
		// C appends one without leaving the list. Both learned the project header,
		// because a project now carries comments of its own; they stay two
		// descriptors because they are two records through two use cases.
		"C": {operation.OperationTaskAddComment, operation.OperationProjectAddComment},
		";": {operation.OperationTaskComments, operation.OperationProjectComments},
		"e": {
			operation.OperationChecklistEditItem, operation.OperationScopeRenameProject,
			operation.OperationCoreEdit,
		},
		"space":  {operation.OperationChecklistActions, operation.OperationProjectActions, operation.OperationCoreActions},
		"J":      {operation.OperationChecklistMoveDown, operation.OperationCoreReorderDown},
		"K":      {operation.OperationChecklistMoveUp, operation.OperationCoreReorderUp},
		"ctrl+j": {operation.OperationChecklistMoveBottom, operation.OperationCoreReorderBottom},
		"ctrl+k": {operation.OperationChecklistMoveTop, operation.OperationCoreReorderTop},
		"enter": {
			operation.OperationCoreFocusSubtree, operation.OperationCoreZoomSection,
			operation.OperationCoreActivateGroup, operation.OperationCoreOpenNote,
		},
	}

	owners := make(map[string][]operation.OperationID)
	for _, binding := range strategy.Bindings {
		for _, key := range binding.Keys {
			if len(owners[key]) > 0 && !binding.SharedKey {
				t.Errorf("direct key %q is owned by both %s and %s", key, owners[key][0], binding.OperationID)
			}
			owners[key] = append(owners[key], binding.OperationID)
		}
	}
	if len(owners) != len(direct)+len(shared) {
		t.Fatalf("direct key count = %d, want %d; got %v", len(owners), len(direct)+len(shared), owners)
	}
	for key, operationID := range direct {
		if got := owners[key]; !slices.Equal(got, []operation.OperationID{operationID}) {
			t.Errorf("direct key %q = %v, want %s", key, got, operationID)
		}
	}
	for key, want := range shared {
		if got := owners[key]; !slices.Equal(got, want) {
			t.Errorf("shared key %q = %v, want %v", key, got, want)
		}
	}

	// g became the go-to chord prefix, so cursor-top answers to gg and Home only,
	// and d went with the defer picker when @ absorbed both date fields. H, M,
	// and L went with the screen-relative jumps; the z chord kept the family. M
	// has since been spent on the menu bar and is asserted in direct above, so
	// only its two neighbors are still owed silence: either one alone would
	// advertise half of a jump the z chord already owns. C is likewise absent
	// now: quick Add Comment took it, and shared below is where it is asserted. b
	// has likewise left this list: Peek took it off Ctrl+Enter, and direct above
	// is where it is asserted, as has R, which Mark Reviewed took.
	for _, removed := range []string{
		"d", "g", "z", ",", "m", ":", "ctrl+esc",
		"H", "L",
	} {
		if owners := owners[removed]; len(owners) > 0 {
			t.Errorf("removed key %q still resolves to %s", removed, owners[0])
		}
	}
}
