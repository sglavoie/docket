package tui

import (
	"slices"
	"strings"
	"testing"
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
	direct := map[string]OperationID{
		"enter":  operationCommentExpand,
		"a":      operationCommentAdd,
		"e":      operationCommentEdit,
		"E":      operationCommentEditExternal,
		"space":  operationCommentActions,
		"y":      operationCommentCopy,
		"Y":      operationCommentCopyAll,
		"u":      operationCoreUndo,
		"ctrl+r": operationCoreRedo,
		"j":      operationCoreCursorDown,
		"down":   operationCoreCursorDown,
		"k":      operationCoreCursorUp,
		"up":     operationCoreCursorUp,
		"gg":     operationCoreCursorTop,
		"home":   operationCoreCursorTop,
		"G":      operationCoreCursorBottom,
		"end":    operationCoreCursorBottom,
		"?":      operationCoreHelp,
		"esc":    operationCoreBack,
	}
	assertStrategyOwnsEveryKeyOnce(t, CommentsInputStrategy(), direct)
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
	direct := map[string]OperationID{
		"enter":  operationAttachmentOpen,
		"a":      operationAttachmentAdd,
		"E":      operationAttachmentOpenInEditor,
		"y":      operationAttachmentCopyPath,
		"Y":      operationAttachmentCopyPathWithLine,
		"J":      operationAttachmentReorderDown,
		"K":      operationAttachmentReorderUp,
		"space":  operationAttachmentActions,
		"u":      operationCoreUndo,
		"ctrl+r": operationCoreRedo,
		"j":      operationCoreCursorDown,
		"down":   operationCoreCursorDown,
		"k":      operationCoreCursorUp,
		"up":     operationCoreCursorUp,
		"gg":     operationCoreCursorTop,
		"home":   operationCoreCursorTop,
		"G":      operationCoreCursorBottom,
		"end":    operationCoreCursorBottom,
		"?":      operationCoreHelp,
		"esc":    operationCoreBack,
	}

	owners := assertStrategyOwnsEveryKeyOnce(t, AttachmentsInputStrategy(), direct)

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
	direct := map[string]OperationID{
		"enter":  operationNoteOpen,
		"a":      operationNotesCreate,
		"e":      operationNoteRename,
		"%":      operationNoteMove,
		"/":      operationNotesSearch,
		"p":      operationNotesScope,
		"y":      operationNoteCopyTitle,
		"Y":      operationNoteCopyPath,
		"space":  operationNotesActions,
		"u":      operationCoreUndo,
		"ctrl+r": operationCoreRedo,
		"j":      operationCoreCursorDown,
		"down":   operationCoreCursorDown,
		"k":      operationCoreCursorUp,
		"up":     operationCoreCursorUp,
		"gg":     operationCoreCursorTop,
		"home":   operationCoreCursorTop,
		"G":      operationCoreCursorBottom,
		"end":    operationCoreCursorBottom,
		"ctrl+d": operationCoreCursorPageDown,
		"pgdown": operationCoreCursorPageDown,
		"ctrl+u": operationCoreCursorPageUp,
		"pgup":   operationCoreCursorPageUp,
		"?":      operationCoreHelp,
		"esc":    operationCoreBack,
	}
	assertStrategyOwnsEveryKeyOnce(t, NotesInputStrategy(), direct)
}

// assertStrategyOwnsEveryKeyOnce holds one surface grammar to the one-key
// rule: every declared path resolves to exactly the operation the table names,
// and the grammar declares no path the table omits.
func assertStrategyOwnsEveryKeyOnce(t *testing.T, strategy InputStrategy, direct map[string]OperationID) map[string][]OperationID {
	t.Helper()
	owners := make(map[string][]OperationID)
	for _, binding := range strategy.Bindings {
		for _, path := range binding.paths() {
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
		if got := owners[key]; !slices.Equal(got, []OperationID{operationID}) {
			t.Errorf("direct key %q = %v, want %s", key, got, operationID)
		}
	}
	return owners
}

// assertProjectHubOwnsEveryDocumentedKeyOnce spells complete paths rather than
// root keys: the Hub grammar holds a single chord, so gg belongs in the same
// table as the presses beside it.
func assertProjectHubOwnsEveryDocumentedKeyOnce(t *testing.T) {
	direct := map[string]OperationID{
		"/":      operationProjectHubSearch,
		"j":      operationCoreCursorDown,
		"down":   operationCoreCursorDown,
		"k":      operationCoreCursorUp,
		"up":     operationCoreCursorUp,
		"gg":     operationCoreCursorTop,
		"home":   operationCoreCursorTop,
		"G":      operationCoreCursorBottom,
		"end":    operationCoreCursorBottom,
		"ctrl+d": operationCoreCursorPageDown,
		"ctrl+u": operationCoreCursorPageUp,
		"ctrl+f": operationProjectHubPreviewPageDown,
		"ctrl+b": operationProjectHubPreviewPageUp,
		"a":      operationScopeCreateProject,
		"e":      operationScopeRenameProject,
		"*":      operationScopePinProject,
		"#":      operationProjectSetTags,
		"ctrl+n": operationScopeOpenProjectNote,
		"v":      operationProjectHubToggleSelection,
		"ctrl+a": operationProjectHubSelectAll,
		"space":  operationProjectHubActions,
		"enter":  operationProjectOpen,
		"ctrl+t": operationProjectHubTaskFilter,
		"u":      operationCoreUndo,
		"ctrl+r": operationCoreRedo,
		"?":      operationCoreHelp,
		"esc":    operationCoreBack,
	}

	owners := assertStrategyOwnsEveryKeyOnce(t, ProjectHubInputStrategy(), direct)

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
	direct := map[string]OperationID{
		"a":      operationCoreCapture,
		"E":      operationCoreEditDescription,
		"ctrl+e": operationCoreExternalEditor,
		"O":      operationCoreOutlineEditor,
		"j":      operationCoreCursorDown, "down": operationCoreCursorDown,
		"k": operationCoreCursorUp, "up": operationCoreCursorUp,
		"home": operationCoreCursorTop,
		"G":    operationCoreCursorBottom, "end": operationCoreCursorBottom,
		"ctrl+u": operationCoreCursorPageUp, "pgup": operationCoreCursorPageUp,
		"ctrl+d": operationCoreCursorPageDown, "pgdown": operationCoreCursorPageDown,
		"ctrl+o": operationCoreJumpBackward, "ctrl+i": operationCoreJumpForward,
		"ctrl+l": operationCoreFocusLatest,
		"ctrl+p": operationCoreSelectPinned,
		"h":      operationCoreCollapseParent, "left": operationCoreCollapseParent,
		"l": operationCoreExpand, "right": operationCoreExpand,
		"s": operationCoreToggleSections,
		"p": operationCoreToggleProjectGroup,
		"t": operationCoreToggleSubtrees,
		"n": operationCoreNextHeader, "N": operationCorePreviousHeader,
		"[": operationTaskOutdent, "]": operationTaskIndent,
		">":           operationTaskSetParent,
		"<":           operationTaskAdoptChildren,
		"~":           operationTaskReferences,
		"%":           operationTaskMoveToProject,
		"&":           operationTaskAttachments,
		";":           operationTaskComments,
		"shift+enter": operationCoreFocusParent,
		"ctrl+enter":  operationCorePeek,
		"/":           operationCoreScopeSearch,
		"v":           operationCoreMultiSelect, "V": operationCoreRangeSelect,
		"ctrl+a": operationCoreSelectAll,
		"P":      operationCoreSelectProject,
		"\\":     operationCoreNamedLenses,
		"ctrl+n": operationCoreNotes, "T": operationCoreToggleTheme,
		"M": operationCoreToggleMenuBar, "ctrl+m": operationCoreOpenMenuBar,
		"Q": operationCoreQueryReadout,
		"u": operationCoreUndo, "ctrl+r": operationCoreRedo,
		"esc": operationCoreBack, "?": operationCoreHelp, "ctrl+c": operationCoreQuit,
		"tab": operationCoreNextWorkMode, "shift+tab": operationCorePreviousWorkMode,
		"@": operationTaskSetDueDate,
		"y": operationTaskCopyTitle, "Y": operationTaskCopyDetails,
		"ctrl+y": operationTaskCopyTitleDescription,
		// The digit row: 0 returns to the launch view and 1-7 are the seven modes
		// in Tab-cycle order, each a second spelling on the go-to chord's binding.
		"0": operationCoreResetToLaunchView,
		"1": operationCoreGotoAll, "2": operationCoreGotoToday,
		"3": operationCoreGotoInbox, "4": operationCoreGotoNext,
		"5": operationCoreGotoBacklog, "6": operationCoreGotoDue,
		"7": operationCoreGotoRecent,
	}

	// A shared key is one the cursor disambiguates, and each meaning keeps its own
	// descriptor so help and the footer can name both honestly. # and * both ask
	// whether the cursor names a task or a project; Space asks the same question
	// with a third answer, the checklist row; the rest ask whether it names a
	// checklist item, and the checklist meaning is listed first because resolution
	// takes the first applicable binding.
	shared := map[string][]OperationID{
		"#":      {operationTaskSetTags, operationProjectSetTags},
		"*":      {operationTaskTogglePin, operationScopePinProject},
		"e":      {operationChecklistEditItem, operationCoreEdit},
		"A":      {operationChecklistAddItem, operationTaskNewChild},
		"x":      {operationChecklistToggleItem, operationChecklistStart},
		"space":  {operationChecklistActions, operationProjectActions, operationCoreActions},
		"J":      {operationChecklistMoveDown, operationCoreReorderDown},
		"K":      {operationChecklistMoveUp, operationCoreReorderUp},
		"ctrl+j": {operationChecklistMoveBottom, operationCoreReorderBottom},
		"ctrl+k": {operationChecklistMoveTop, operationCoreReorderTop},
		"enter": {
			operationCoreFocusSubtree, operationCoreZoomSection,
			operationCoreActivateGroup, operationCoreOpenNote,
		},
	}

	owners := make(map[string][]OperationID)
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
		if got := owners[key]; !slices.Equal(got, []OperationID{operationID}) {
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
	// advertise half of a jump the z chord already owns.
	for _, removed := range []string{
		"b", "c", "d", "g", "z", ",", "m", ":", "C", "R", "ctrl+esc",
		"H", "L",
	} {
		if owners := owners[removed]; len(owners) > 0 {
			t.Errorf("removed key %q still resolves to %s", removed, owners[0])
		}
	}
}
