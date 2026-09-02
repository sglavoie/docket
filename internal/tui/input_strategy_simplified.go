package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// gotoModeOperations maps each direct mode-jump operation to the mode it
// selects. Seven descriptors rather than one preset-valued operation, so the
// grammar never needs a mode value provider and cannot resurrect a mode picker.
var gotoModeOperations = map[OperationID]OutlinerModeID{
	operationCoreGotoAll:     ModeAll,
	operationCoreGotoInbox:   ModeInbox,
	operationCoreGotoToday:   ModeToday,
	operationCoreGotoNext:    ModeNext,
	operationCoreGotoBacklog: ModeBacklog,
	operationCoreGotoDue:     ModeDue,
	operationCoreGotoRecent:  ModeRecent,
}

// SimplifiedInputStrategy returns a fresh copy of the accepted daily-loop
// grammar. Direct keys and chord sequences are declared here and nowhere else.
func SimplifiedInputStrategy() InputStrategy {
	return InputStrategy{
		ID: InputStrategySimplified,
		Bindings: []OperationKeyBinding{
			{OperationID: operationCoreCapture, Keys: []string{"a"}, DisplayKey: "a", FooterRank: 8, FooterLabel: "capture"},
			// The eight keys a checklist row shares with a task row are declared
			// checklist-first: shared-key resolution takes the first applicable
			// descriptor, and only one of each pair applies to any given cursor.
			{OperationID: operationChecklistEditItem, Keys: []string{"e"}, DisplayKey: "e", FooterRank: 12, FooterLabel: "item", SharedKey: true},
			{OperationID: operationCoreEdit, Keys: []string{"e"}, DisplayKey: "e", FooterRank: 12, FooterLabel: "edit", SharedKey: true},
			{OperationID: operationCoreEditDescription, Keys: []string{"E"}, DisplayKey: "E", FooterRank: 20, FooterLabel: "description"},
			{OperationID: operationCoreExternalEditor, Keys: []string{"ctrl+e"}, DisplayKey: "Ctrl+E", FooterRank: 21, FooterLabel: "editor"},
			{OperationID: operationCoreOutlineEditor, Keys: []string{"O"}, DisplayKey: "O", FooterRank: 23, FooterLabel: "outline"},
			{OperationID: operationChecklistAddItem, Keys: []string{"A"}, DisplayKey: "A", FooterRank: 21, FooterLabel: "item", SharedKey: true},
			{OperationID: operationTaskNewChild, Keys: []string{"A"}, DisplayKey: "A", FooterRank: 21, FooterLabel: "child", SharedKey: true},
			{OperationID: operationChecklistToggleItem, Keys: []string{"x"}, DisplayKey: "x", FooterRank: 13, FooterLabel: "tick", SharedKey: true},
			{OperationID: operationChecklistStart, Keys: []string{"x"}, DisplayKey: "x", FooterRank: 13, FooterLabel: "checklist", SharedKey: true},
			// Space is a three-way shared key: checklist row, project header, task
			// row. Exactly one of the three applies at any cursor, so declaration
			// order only decides which is asked first, never which one wins.
			{OperationID: operationChecklistActions, Keys: []string{"space"}, DisplayKey: "Space", FooterRank: 5, FooterLabel: "actions", SharedKey: true},
			{OperationID: operationProjectActions, Keys: []string{"space"}, DisplayKey: "Space", FooterRank: 5, FooterLabel: "actions", SharedKey: true},
			{OperationID: operationCoreActions, Keys: []string{"space"}, DisplayKey: "Space", FooterRank: 5, FooterLabel: "actions", SharedKey: true},
			{OperationID: operationCoreCursorDown, Keys: []string{"j", "down"}, DisplayKey: "j/↓"},
			{OperationID: operationCoreCursorUp, Keys: []string{"k", "up"}, DisplayKey: "k/↑"},
			{OperationID: operationCoreCursorTop, Keys: []string{"home"}, Sequence: []string{"g", "g"}, DisplayKey: "gg/Home", HintLabel: "top"},
			{OperationID: operationCoreCursorBottom, Keys: []string{"G", "end"}, DisplayKey: "G/End"},
			{OperationID: operationCoreCursorPageUp, Keys: []string{"ctrl+u", "pgup"}, DisplayKey: "Ctrl+U/PgUp"},
			{OperationID: operationCoreCursorPageDown, Keys: []string{"ctrl+d", "pgdown"}, DisplayKey: "Ctrl+D/PgDn"},
			{OperationID: operationCoreJumpBackward, Keys: []string{"ctrl+o"}, DisplayKey: "Ctrl+O"},
			{OperationID: operationCoreJumpForward, Keys: []string{"ctrl+i"}, DisplayKey: "Ctrl+I"},
			{OperationID: operationCoreFocusLatest, Keys: []string{"ctrl+l"}, DisplayKey: "Ctrl+L"},
			{OperationID: operationCoreSelectPinned, Keys: []string{"ctrl+p"}, DisplayKey: "Ctrl+P"},
			{OperationID: operationCoreCollapseParent, Keys: []string{"h", "left"}, DisplayKey: "h/←"},
			{OperationID: operationCoreExpand, Keys: []string{"l", "right"}, DisplayKey: "l/→"},
			// One lowercase toggle per fold axis, declared together so the family
			// reads as one: s sections, p project groups, t task subtrees.
			{OperationID: operationCoreToggleSections, Keys: []string{"s"}, DisplayKey: "s"},
			{OperationID: operationCoreToggleProjectGroup, Keys: []string{"p"}, DisplayKey: "p"},
			{OperationID: operationCoreToggleSubtrees, Keys: []string{"t"}, DisplayKey: "t"},
			{OperationID: operationCoreNextHeader, Keys: []string{"n"}, DisplayKey: "n"},
			{OperationID: operationCorePreviousHeader, Keys: []string{"N"}, DisplayKey: "N"},
			{OperationID: operationChecklistMoveDown, Keys: []string{"J"}, DisplayKey: "J", SharedKey: true},
			{OperationID: operationCoreReorderDown, Keys: []string{"J"}, DisplayKey: "J", SharedKey: true},
			{OperationID: operationChecklistMoveUp, Keys: []string{"K"}, DisplayKey: "K", SharedKey: true},
			{OperationID: operationCoreReorderUp, Keys: []string{"K"}, DisplayKey: "K", SharedKey: true},
			{OperationID: operationChecklistMoveBottom, Keys: []string{"ctrl+j"}, DisplayKey: "Ctrl+J", SharedKey: true},
			{OperationID: operationCoreReorderBottom, Keys: []string{"ctrl+j"}, DisplayKey: "Ctrl+J", SharedKey: true},
			{OperationID: operationChecklistMoveTop, Keys: []string{"ctrl+k"}, DisplayKey: "Ctrl+K", SharedKey: true},
			{OperationID: operationCoreReorderTop, Keys: []string{"ctrl+k"}, DisplayKey: "Ctrl+K", SharedKey: true},
			{OperationID: operationTaskOutdent, Keys: []string{"["}, DisplayKey: "["},
			{OperationID: operationTaskIndent, Keys: []string{"]"}, DisplayKey: "]"},
			{OperationID: operationTaskSetParent, Keys: []string{">"}, DisplayKey: ">"},
			{OperationID: operationTaskAdoptChildren, Keys: []string{"<"}, DisplayKey: "<"},
			{OperationID: operationTaskReferences, Keys: []string{"~"}, DisplayKey: "~", FooterRank: 16, FooterLabel: "dependencies"},
			{OperationID: operationTaskMoveToProject, Keys: []string{"%"}, DisplayKey: "%"},
			// The two capture sigils that carry task content keep the same
			// correspondence the other keyed sigils do: the character that writes an
			// attachment or a comment on a capture line opens that panel on a row.
			{OperationID: operationTaskAttachments, Keys: []string{"&"}, DisplayKey: "&", FooterRank: 14, FooterLabel: "attachments"},
			{OperationID: operationTaskComments, Keys: []string{";"}, DisplayKey: ";", FooterRank: 15, FooterLabel: "comments"},
			// Enter zooms in on whatever the cursor names. The four acts carry
			// different targets and different vocabularies, so they stay four
			// descriptors with four honest labels rather than one that branches on
			// runtime context. Open Note is the one that leaves the Outliner: a
			// search result the query answered with a note is read where notes live.
			{OperationID: operationCoreFocusSubtree, Keys: []string{"enter"}, DisplayKey: "Enter", FooterRank: 9, FooterLabel: "focus", SharedKey: true},
			{OperationID: operationCoreZoomSection, Keys: []string{"enter"}, DisplayKey: "Enter", FooterRank: 9, FooterLabel: "zoom", SharedKey: true},
			{OperationID: operationCoreActivateGroup, Keys: []string{"enter"}, DisplayKey: "Enter", FooterRank: 9, FooterLabel: "open", SharedKey: true},
			{OperationID: operationCoreOpenNote, Keys: []string{"enter"}, DisplayKey: "Enter", FooterRank: 9, FooterLabel: "open note", SharedKey: true},
			{OperationID: operationCoreFocusParent, Keys: []string{"shift+enter"}, DisplayKey: "Shift+Enter"},
			{OperationID: operationCorePeek, Keys: []string{"ctrl+enter"}, DisplayKey: "Ctrl+Enter", FooterRank: 10, FooterLabel: "peek"},
			{OperationID: operationCoreScopeSearch, Keys: []string{"/"}, DisplayKey: "/", FooterRank: 10, FooterLabel: "search"},
			{OperationID: operationCoreMultiSelect, Keys: []string{"v"}, DisplayKey: "v", FooterRank: 7, FooterLabel: "select"},
			{OperationID: operationCoreRangeSelect, Keys: []string{"V"}, DisplayKey: "V"},
			{OperationID: operationCoreSelectAll, Keys: []string{"ctrl+a"}, DisplayKey: "Ctrl+A"},
			{OperationID: operationCoreSelectProject, Keys: []string{"P"}, DisplayKey: "P", FooterRank: 18, FooterLabel: "projects"},
			// Named Lenses carries no FooterRank on purpose: it is a discoverable
			// entry point rather than a daily-loop act, so `?` teaches it and the
			// compact footer stays as quiet as it is today.
			{OperationID: operationCoreNamedLenses, Keys: []string{"\\"}, DisplayKey: `\`},
			// The readout expands the narrowing the operational title already
			// summarizes, so it claims a root key of its own rather than joining the
			// f chord: it reads the lens, the scope, the mode, and the query beside
			// the facets, and a "filter" prefix would be dishonest about four of them.
			{OperationID: operationCoreQueryReadout, Keys: []string{"Q"}, DisplayKey: "Q", FooterRank: 27, FooterLabel: "narrowing"},
			{OperationID: operationCoreNotes, Keys: []string{"ctrl+n"}, DisplayKey: "Ctrl+N"},
			{OperationID: operationCoreToggleTheme, Keys: []string{"T"}, DisplayKey: "T", FooterRank: 22, FooterLabel: "theme"},
			// The bar is the one browsable map of this grammar, so its reveal
			// outranks every act it lists: a user who cannot find anything needs the
			// hint that shows them everything before they need any single key.
			{OperationID: operationCoreToggleMenuBar, Keys: []string{"M"}, DisplayKey: "M", FooterRank: 6, FooterLabel: "menu"},
			// Opening carries no FooterRank of its own. The band's own reveal is
			// already ranked above, and once the band is on screen it is the label
			// row that advertises what opening does; a second footer hint would spend
			// a rank restating chrome the user is already looking at. The keys inside
			// an open menu take none either, per the gated-keymap invariant.
			{OperationID: operationCoreOpenMenuBar, Keys: []string{"ctrl+m"}, DisplayKey: "Ctrl+M"},
			{OperationID: operationCoreUndo, Keys: []string{"u"}, DisplayKey: "u"},
			{OperationID: operationCoreRedo, Keys: []string{"ctrl+r"}, DisplayKey: "Ctrl+R"},
			{OperationID: operationCoreBack, Keys: []string{"esc"}, DisplayKey: "Esc"},
			{OperationID: operationCoreHelp, Keys: []string{"?"}, DisplayKey: "?", FooterRank: 11, FooterLabel: "help"},
			{OperationID: operationCoreQuit, Keys: []string{"ctrl+c"}, DisplayKey: "Ctrl+C"},
			{OperationID: operationCoreNextWorkMode, Keys: []string{"tab"}, DisplayKey: "Tab"},
			{OperationID: operationCorePreviousWorkMode, Keys: []string{"shift+tab"}, DisplayKey: "Shift+Tab"},
			{OperationID: operationTaskSetDueDate, Keys: []string{"@"}, DisplayKey: "@", FooterRank: 3, FooterLabel: "date"},
			// # tags whatever the cursor names. The two operations carry different
			// targets and different vocabularies, so they stay two descriptors with
			// two honest labels rather than one that branches on runtime context.
			{OperationID: operationTaskSetTags, Keys: []string{"#"}, DisplayKey: "#", FooterRank: 4, FooterLabel: "tags/context", SharedKey: true},
			{OperationID: operationProjectSetTags, Keys: []string{"#"}, DisplayKey: "#", FooterRank: 4, FooterLabel: "project tags", SharedKey: true},
			// * pins whatever the cursor names, for the same reason # tags it: two
			// targets and two vocabularies stay two descriptors with two honest
			// labels rather than one that branches on runtime context.
			{OperationID: operationTaskTogglePin, Keys: []string{"*"}, DisplayKey: "*", FooterRank: 24, FooterLabel: "pin", SharedKey: true},
			{OperationID: operationScopePinProject, Keys: []string{"*"}, DisplayKey: "*", FooterRank: 24, FooterLabel: "project pin", SharedKey: true},
			{OperationID: operationTaskCopyTitle, Keys: []string{"y"}, DisplayKey: "y"},
			{OperationID: operationTaskCopyDetails, Keys: []string{"Y"}, DisplayKey: "Y"},
			{OperationID: operationTaskCopyTitleDescription, Keys: []string{"ctrl+y"}, DisplayKey: "Ctrl+Y"},
			{OperationID: operationTaskSetPriority, Sequence: []string{"!", "0"}, DisplayKey: "!0-3", HintLabel: "unset", PresetValue: 0, HasPresetValue: true},
			{OperationID: operationTaskSetPriority, Sequence: []string{"!", "1"}, DisplayKey: "!0-3", HintLabel: "high", PresetValue: 1, HasPresetValue: true},
			{OperationID: operationTaskSetPriority, Sequence: []string{"!", "2"}, DisplayKey: "!0-3", HintLabel: "medium", PresetValue: 2, HasPresetValue: true},
			{OperationID: operationTaskSetPriority, Sequence: []string{"!", "3"}, DisplayKey: "!0-3", HintLabel: "low", PresetValue: 3, HasPresetValue: true},
			{OperationID: operationTaskSetState, Sequence: []string{"$", "i"}, DisplayKey: "$i/g/w/p/m/d/c", HintLabel: "inbox", PresetValue: models.StateInbox, HasPresetValue: true},
			{OperationID: operationCoreBacklog, Sequence: []string{"$", "b"}, DisplayKey: "$b", HintLabel: "backlog"},
			{OperationID: operationCoreNext, Sequence: []string{"$", "n"}, DisplayKey: "$n", HintLabel: "next"},
			{OperationID: operationCoreToday, Sequence: []string{"$", "t"}, DisplayKey: "$t", HintLabel: "today"},
			{OperationID: operationTaskSetState, Sequence: []string{"$", "g"}, DisplayKey: "$i/g/w/p/m/d/c", HintLabel: "doing", PresetValue: models.StateDoing, HasPresetValue: true},
			{OperationID: operationTaskSetState, Sequence: []string{"$", "w"}, DisplayKey: "$i/g/w/p/m/d/c", HintLabel: "waiting", PresetValue: models.StateWaiting, HasPresetValue: true},
			{OperationID: operationTaskSetState, Sequence: []string{"$", "p"}, DisplayKey: "$i/g/w/p/m/d/c", HintLabel: "paused", PresetValue: models.StatePaused, HasPresetValue: true},
			{OperationID: operationTaskSetState, Sequence: []string{"$", "m"}, DisplayKey: "$i/g/w/p/m/d/c", HintLabel: "someday", PresetValue: models.StateSomeday, HasPresetValue: true},
			{OperationID: operationTaskSetState, Sequence: []string{"$", "d"}, DisplayKey: "$i/g/w/p/m/d/c", HintLabel: "done", PresetValue: models.StateDone, HasPresetValue: true},
			{OperationID: operationTaskSetState, Sequence: []string{"$", "c"}, DisplayKey: "$i/g/w/p/m/d/c", HintLabel: "canceled", PresetValue: models.StateCanceled, HasPresetValue: true},
			{OperationID: operationFilterPriority, Sequence: []string{"f", "!", "0"}, DisplayKey: "f!0-3", HintLabel: "clear", PresetValue: 0, HasPresetValue: true},
			{OperationID: operationFilterPriority, Sequence: []string{"f", "!", "1"}, DisplayKey: "f!0-3", HintLabel: "high", PresetValue: 1, HasPresetValue: true},
			{OperationID: operationFilterPriority, Sequence: []string{"f", "!", "2"}, DisplayKey: "f!0-3", HintLabel: "medium+", PresetValue: 2, HasPresetValue: true},
			{OperationID: operationFilterPriority, Sequence: []string{"f", "!", "3"}, DisplayKey: "f!0-3", HintLabel: "low+", PresetValue: 3, HasPresetValue: true},
			{OperationID: operationFilterDue, Sequence: []string{"f", "@"}, DisplayKey: "f@", HintLabel: "due/deferred"},
			{OperationID: operationFilterCompletionAge, Sequence: []string{"f", "a"}, DisplayKey: "fa", HintLabel: "completion age"},
			{OperationID: operationFilterStates, Sequence: []string{"f", "$"}, DisplayKey: "f$", HintLabel: "states"},
			{OperationID: operationFilterTags, Sequence: []string{"f", "#"}, DisplayKey: "f#", HintLabel: "tags"},
			// Context is declared immediately after tags so the which-key row
			// reads tags then contexts, which is the order the two sigils teach.
			{OperationID: operationFilterContexts, Sequence: []string{"f", "c"}, DisplayKey: "fc", HintLabel: "contexts"},
			{OperationID: operationFilterReview, Sequence: []string{"f", "="}, DisplayKey: "f=", HintLabel: "review"},
			{OperationID: operationFilterHierarchy, Sequence: []string{"f", "h"}, DisplayKey: "fh", HintLabel: "hierarchy"},
			// A facet carrying a sigil answers to that sigil, so the reference axis
			// is f~ and no letter is bound to it.
			{OperationID: operationFilterDependency, Sequence: []string{"f", "~"}, DisplayKey: "f~", HintLabel: "?blocked/?blocking"},
			{OperationID: operationFilterStale, Sequence: []string{"f", "s"}, DisplayKey: "fs", HintLabel: "stale"},
			// Pinned answers to its own sigil like every other sigil-carrying facet:
			// *pin is what the clause spells, so f* is what narrows by it. The letter
			// fp stays retired, because it used to mean priority and a letter still
			// bound would silently apply the wrong facet.
			{OperationID: operationFilterPinned, Sequence: []string{"f", "*"}, DisplayKey: "f*", HintLabel: "pinned"},
			{OperationID: operationFilterClear, Sequence: []string{"f", "x"}, DisplayKey: "fx", HintLabel: "clear all"},
			{OperationID: operationFilterProjectTags, Sequence: []string{"F", "#"}, DisplayKey: "F#", HintLabel: "project tags"},
			{OperationID: operationFilterShowParked, Sequence: []string{"F", "p"}, DisplayKey: "Fp", HintLabel: "include parked"},
			{OperationID: operationFilterOnlyParked, Sequence: []string{"F", "P"}, DisplayKey: "FP", HintLabel: "only parked"},
			{OperationID: operationFilterIncludeArchived, Sequence: []string{"F", "a"}, DisplayKey: "Fa", HintLabel: "include archived"},
			{OperationID: operationFilterOnlyArchived, Sequence: []string{"F", "A"}, DisplayKey: "FA", HintLabel: "only archived"},
			{OperationID: operationFilterProjectClear, Sequence: []string{"F", "x"}, DisplayKey: "Fx", HintLabel: "clear project filters"},
			{OperationID: operationSortDueDate, Sequence: []string{"o", "@"}, DisplayKey: "o@", HintLabel: "due date"},
			{OperationID: operationSortPriority, Sequence: []string{"o", "!"}, DisplayKey: "o!", HintLabel: "priority"},
			{OperationID: operationSortCreatedAt, Sequence: []string{"o", "c"}, DisplayKey: "oc", HintLabel: "created"},
			{OperationID: operationSortUpdatedAt, Sequence: []string{"o", "u"}, DisplayKey: "ou", HintLabel: "updated"},
			{OperationID: operationSortTitle, Sequence: []string{"o", "t"}, DisplayKey: "ot", HintLabel: "title"},
			{OperationID: operationSortClear, Sequence: []string{"o", "x"}, DisplayKey: "ox", HintLabel: "clear sort"},
			{OperationID: operationSortRanked, Sequence: []string{"o", "r"}, DisplayKey: "or", HintLabel: "ranked list"},
			// 0 leads the digit row and is the only one of the eight that names no
			// mode: it puts the whole view back where a lens-free launch starts. It
			// takes no footer rank, because it is the escape hatch a user reaches for
			// once rather than an act of the daily loop.
			{OperationID: operationCoreResetToLaunchView, Keys: []string{"0"}, DisplayKey: "0"},
			// Each mode jump carries both its go-to chord and the digit at its
			// Tab-cycle position, so the mnemonic route and the cheap one are one
			// binding with one composed spelling rather than two descriptors.
			{OperationID: operationCoreGotoAll, Keys: []string{"1"}, Sequence: []string{"g", "a"}, DisplayKey: "ga/1", HintLabel: "all"},
			{OperationID: operationCoreGotoInbox, Keys: []string{"3"}, Sequence: []string{"g", "i"}, DisplayKey: "gi/3", HintLabel: "inbox"},
			{OperationID: operationCoreGotoToday, Keys: []string{"2"}, Sequence: []string{"g", "t"}, DisplayKey: "gt/2", HintLabel: "today"},
			{OperationID: operationCoreGotoNext, Keys: []string{"4"}, Sequence: []string{"g", "n"}, DisplayKey: "gn/4", HintLabel: "next"},
			{OperationID: operationCoreGotoBacklog, Keys: []string{"5"}, Sequence: []string{"g", "b"}, DisplayKey: "gb/5", HintLabel: "backlog"},
			{OperationID: operationCoreGotoDue, Keys: []string{"6"}, Sequence: []string{"g", "@"}, DisplayKey: "g@/6", HintLabel: "due"},
			{OperationID: operationCoreGotoRecent, Keys: []string{"7"}, Sequence: []string{"g", "r"}, DisplayKey: "gr/7", HintLabel: "recent"},
			{OperationID: operationCoreGotoTaskProject, Sequence: []string{"g", "p"}, DisplayKey: "gp", HintLabel: "task project"},
			// Conflicts joins the go-to chord rather than claiming a root key: `c` is
			// retired at the root, and what the strip already names as workspace state
			// is something the user goes to look at.
			{OperationID: operationCoreConflicts, Sequence: []string{"g", "c"}, DisplayKey: "gc", HintLabel: "conflicts"},
			{OperationID: operationCoreScrollCursorTop, Sequence: []string{"z", "t"}, DisplayKey: "zt", HintLabel: "row to top"},
			{OperationID: operationCoreScrollCursorMiddle, Sequence: []string{"z", "z"}, DisplayKey: "zz", HintLabel: "row to middle"},
			{OperationID: operationCoreScrollCursorBottom, Sequence: []string{"z", "b"}, DisplayKey: "zb", HintLabel: "row to bottom"},
		},
		ChordGroups: []OperationChordGroup{
			{Sequence: []string{"!"}, HintLabel: "priority", FooterRank: 2, FooterLabel: "priority"},
			{Sequence: []string{"$"}, HintLabel: "workflow", FooterRank: 1, FooterLabel: "workflow"},
			{Sequence: []string{"f"}, HintLabel: "filter", FooterRank: 19, FooterLabel: "filter"},
			{Sequence: []string{"f", "!"}, HintLabel: "priority threshold"},
			{Sequence: []string{"F"}, HintLabel: "project filter", FooterRank: 25, FooterLabel: "project filter"},
			{Sequence: []string{"o"}, HintLabel: "sort", FooterRank: 23, FooterLabel: "sort"},
			{Sequence: []string{"g"}, HintLabel: "go to", FooterRank: 26, FooterLabel: "go to"},
			{Sequence: []string{"z"}, HintLabel: "scroll"},
		},
	}
}
