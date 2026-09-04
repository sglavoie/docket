package tui

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// gotoModeOperations maps each direct mode-jump operation to the mode it
// selects. Seven descriptors rather than one preset-valued operation, so the
// grammar never needs a mode value provider and cannot resurrect a mode picker.
var gotoModeOperations = map[operation.OperationID]chrome.OutlinerModeID{
	operation.OperationCoreGotoAll:     chrome.ModeAll,
	operation.OperationCoreGotoInbox:   chrome.ModeInbox,
	operation.OperationCoreGotoToday:   chrome.ModeToday,
	operation.OperationCoreGotoNext:    chrome.ModeNext,
	operation.OperationCoreGotoBacklog: chrome.ModeBacklog,
	operation.OperationCoreGotoDue:     chrome.ModeDue,
	operation.OperationCoreGotoRecent:  chrome.ModeRecent,
}

// SimplifiedInputStrategy returns a fresh copy of the accepted daily-loop
// grammar. Direct keys and chord sequences are declared here and nowhere else.
func SimplifiedInputStrategy() operation.InputStrategy {
	return operation.InputStrategy{
		ID: operation.InputStrategySimplified,
		Bindings: []operation.OperationKeyBinding{
			{OperationID: operation.OperationCoreCapture, Keys: []string{"a"}, DisplayKey: "a", FooterRank: 8, FooterLabel: "capture"},
			// The eight keys a checklist row shares with a task row are declared
			// checklist-first: shared-key resolution takes the first applicable
			// descriptor, and only one of each pair applies to any given cursor.
			{OperationID: operation.OperationChecklistEditItem, Keys: []string{"e"}, DisplayKey: "e", FooterRank: 12, FooterLabel: "item", SharedKey: true},
			{OperationID: operation.OperationCoreEdit, Keys: []string{"e"}, DisplayKey: "e", FooterRank: 12, FooterLabel: "edit", SharedKey: true},
			{OperationID: operation.OperationCoreEditDescription, Keys: []string{"E"}, DisplayKey: "E", FooterRank: 20, FooterLabel: "description"},
			{OperationID: operation.OperationCoreExternalEditor, Keys: []string{"ctrl+e"}, DisplayKey: "Ctrl+E", FooterRank: 21, FooterLabel: "editor"},
			{OperationID: operation.OperationCoreOutlineEditor, Keys: []string{"O"}, DisplayKey: "O", FooterRank: 23, FooterLabel: "outline"},
			{OperationID: operation.OperationChecklistAddItem, Keys: []string{"A"}, DisplayKey: "A", FooterRank: 21, FooterLabel: "item", SharedKey: true},
			{OperationID: operationTaskNewChild, Keys: []string{"A"}, DisplayKey: "A", FooterRank: 21, FooterLabel: "child", SharedKey: true},
			{OperationID: operation.OperationChecklistToggleItem, Keys: []string{"x"}, DisplayKey: "x", FooterRank: 13, FooterLabel: "tick", SharedKey: true},
			{OperationID: operation.OperationChecklistStart, Keys: []string{"x"}, DisplayKey: "x", FooterRank: 13, FooterLabel: "checklist", SharedKey: true},
			// Space is a three-way shared key: checklist row, project header, task
			// row. Exactly one of the three applies at any cursor, so declaration
			// order only decides which is asked first, never which one wins.
			{OperationID: operation.OperationChecklistActions, Keys: []string{"space"}, DisplayKey: "Space", FooterRank: 5, FooterLabel: "actions", SharedKey: true},
			{OperationID: operationProjectActions, Keys: []string{"space"}, DisplayKey: "Space", FooterRank: 5, FooterLabel: "actions", SharedKey: true},
			{OperationID: operation.OperationCoreActions, Keys: []string{"space"}, DisplayKey: "Space", FooterRank: 5, FooterLabel: "actions", SharedKey: true},
			{OperationID: operation.OperationCoreCursorDown, Keys: []string{"j", "down"}, DisplayKey: "j/↓"},
			{OperationID: operation.OperationCoreCursorUp, Keys: []string{"k", "up"}, DisplayKey: "k/↑"},
			{OperationID: operation.OperationCoreCursorTop, Keys: []string{"home"}, Sequence: []string{"g", "g"}, DisplayKey: "gg/Home", HintLabel: "top"},
			{OperationID: operation.OperationCoreCursorBottom, Keys: []string{"G", "end"}, DisplayKey: "G/End"},
			{OperationID: operation.OperationCoreCursorPageUp, Keys: []string{"ctrl+u", "pgup"}, DisplayKey: "Ctrl+U/PgUp"},
			{OperationID: operation.OperationCoreCursorPageDown, Keys: []string{"ctrl+d", "pgdown"}, DisplayKey: "Ctrl+D/PgDn"},
			{OperationID: operation.OperationCoreJumpBackward, Keys: []string{"ctrl+o"}, DisplayKey: "Ctrl+O"},
			{OperationID: operation.OperationCoreJumpForward, Keys: []string{"ctrl+i"}, DisplayKey: "Ctrl+I"},
			{OperationID: operation.OperationCoreFocusLatest, Keys: []string{"ctrl+l"}, DisplayKey: "Ctrl+L"},
			{OperationID: operation.OperationCoreSelectPinned, Keys: []string{"ctrl+p"}, DisplayKey: "Ctrl+P"},
			{OperationID: operation.OperationCoreCollapseParent, Keys: []string{"h", "left"}, DisplayKey: "h/←"},
			{OperationID: operation.OperationCoreExpand, Keys: []string{"l", "right"}, DisplayKey: "l/→"},
			// One lowercase toggle per fold axis, declared together so the family
			// reads as one: s sections, p project groups, t task subtrees.
			{OperationID: operation.OperationCoreToggleSections, Keys: []string{"s"}, DisplayKey: "s"},
			{OperationID: operation.OperationCoreToggleProjectGroup, Keys: []string{"p"}, DisplayKey: "p"},
			{OperationID: operation.OperationCoreToggleSubtrees, Keys: []string{"t"}, DisplayKey: "t"},
			{OperationID: operation.OperationCoreNextHeader, Keys: []string{"n"}, DisplayKey: "n"},
			{OperationID: operation.OperationCorePreviousHeader, Keys: []string{"N"}, DisplayKey: "N"},
			{OperationID: operation.OperationChecklistMoveDown, Keys: []string{"J"}, DisplayKey: "J", SharedKey: true},
			{OperationID: operation.OperationCoreReorderDown, Keys: []string{"J"}, DisplayKey: "J", SharedKey: true},
			{OperationID: operation.OperationChecklistMoveUp, Keys: []string{"K"}, DisplayKey: "K", SharedKey: true},
			{OperationID: operation.OperationCoreReorderUp, Keys: []string{"K"}, DisplayKey: "K", SharedKey: true},
			{OperationID: operation.OperationChecklistMoveBottom, Keys: []string{"ctrl+j"}, DisplayKey: "Ctrl+J", SharedKey: true},
			{OperationID: operation.OperationCoreReorderBottom, Keys: []string{"ctrl+j"}, DisplayKey: "Ctrl+J", SharedKey: true},
			{OperationID: operation.OperationChecklistMoveTop, Keys: []string{"ctrl+k"}, DisplayKey: "Ctrl+K", SharedKey: true},
			{OperationID: operation.OperationCoreReorderTop, Keys: []string{"ctrl+k"}, DisplayKey: "Ctrl+K", SharedKey: true},
			{OperationID: operation.OperationTaskOutdent, Keys: []string{"["}, DisplayKey: "["},
			{OperationID: operation.OperationTaskIndent, Keys: []string{"]"}, DisplayKey: "]"},
			{OperationID: operation.OperationTaskSetParent, Keys: []string{">"}, DisplayKey: ">"},
			{OperationID: operation.OperationTaskAdoptChildren, Keys: []string{"<"}, DisplayKey: "<"},
			{OperationID: operation.OperationTaskReferences, Keys: []string{"~"}, DisplayKey: "~", FooterRank: 16, FooterLabel: "dependencies"},
			{OperationID: operation.OperationTaskMoveToProject, Keys: []string{"%"}, DisplayKey: "%"},
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
			{OperationID: operation.OperationCoreFocusSubtree, Keys: []string{"enter"}, DisplayKey: "Enter", FooterRank: 9, FooterLabel: "focus", SharedKey: true},
			{OperationID: operation.OperationCoreZoomSection, Keys: []string{"enter"}, DisplayKey: "Enter", FooterRank: 9, FooterLabel: "zoom", SharedKey: true},
			{OperationID: operation.OperationCoreActivateGroup, Keys: []string{"enter"}, DisplayKey: "Enter", FooterRank: 9, FooterLabel: "open", SharedKey: true},
			{OperationID: operation.OperationCoreOpenNote, Keys: []string{"enter"}, DisplayKey: "Enter", FooterRank: 9, FooterLabel: "open note", SharedKey: true},
			{OperationID: operation.OperationCoreFocusParent, Keys: []string{"shift+enter"}, DisplayKey: "Shift+Enter"},
			{OperationID: operation.OperationCorePeek, Keys: []string{"ctrl+enter"}, DisplayKey: "Ctrl+Enter", FooterRank: 10, FooterLabel: "peek"},
			{OperationID: operation.OperationCoreScopeSearch, Keys: []string{"/"}, DisplayKey: "/", FooterRank: 10, FooterLabel: "search"},
			{OperationID: operation.OperationCoreMultiSelect, Keys: []string{"v"}, DisplayKey: "v", FooterRank: 7, FooterLabel: "select"},
			{OperationID: operation.OperationCoreRangeSelect, Keys: []string{"V"}, DisplayKey: "V"},
			{OperationID: operation.OperationCoreSelectAll, Keys: []string{"ctrl+a"}, DisplayKey: "Ctrl+A"},
			{OperationID: operation.OperationCoreSelectProject, Keys: []string{"P"}, DisplayKey: "P", FooterRank: 18, FooterLabel: "projects"},
			// Named Lenses carries no FooterRank on purpose: it is a discoverable
			// entry point rather than a daily-loop act, so `?` teaches it and the
			// compact footer stays as quiet as it is today.
			{OperationID: operation.OperationCoreNamedLenses, Keys: []string{"\\"}, DisplayKey: `\`},
			// The readout expands the narrowing the operational title already
			// summarizes, so it claims a root key of its own rather than joining the
			// f chord: it reads the lens, the scope, the mode, and the query beside
			// the facets, and a "filter" prefix would be dishonest about four of them.
			{OperationID: operation.OperationCoreQueryReadout, Keys: []string{"Q"}, DisplayKey: "Q", FooterRank: 27, FooterLabel: "narrowing"},
			{OperationID: operation.OperationCoreNotes, Keys: []string{"ctrl+n"}, DisplayKey: "Ctrl+N"},
			{OperationID: operation.OperationCoreToggleTheme, Keys: []string{"T"}, DisplayKey: "T", FooterRank: 22, FooterLabel: "theme"},
			// The bar is the one browsable map of this grammar, so its reveal
			// outranks every act it lists: a user who cannot find anything needs the
			// hint that shows them everything before they need any single key.
			{OperationID: operation.OperationCoreToggleMenuBar, Keys: []string{"M"}, DisplayKey: "M", FooterRank: 6, FooterLabel: "menu"},
			// Opening carries no FooterRank of its own. The band's own reveal is
			// already ranked above, and once the band is on screen it is the label
			// row that advertises what opening does; a second footer hint would spend
			// a rank restating chrome the user is already looking at. The keys inside
			// an open menu take none either, per the gated-keymap invariant.
			{OperationID: operation.OperationCoreOpenMenuBar, Keys: []string{"ctrl+m"}, DisplayKey: "Ctrl+M"},
			{OperationID: operation.OperationCoreUndo, Keys: []string{"u"}, DisplayKey: "u"},
			{OperationID: operation.OperationCoreRedo, Keys: []string{"ctrl+r"}, DisplayKey: "Ctrl+R"},
			{OperationID: operation.OperationCoreBack, Keys: []string{"esc"}, DisplayKey: "Esc"},
			{OperationID: operation.OperationCoreHelp, Keys: []string{"?"}, DisplayKey: "?", FooterRank: 11, FooterLabel: "help"},
			{OperationID: operation.OperationCoreQuit, Keys: []string{"ctrl+c"}, DisplayKey: "Ctrl+C"},
			{OperationID: operation.OperationCoreNextWorkMode, Keys: []string{"tab"}, DisplayKey: "Tab"},
			{OperationID: operation.OperationCorePreviousWorkMode, Keys: []string{"shift+tab"}, DisplayKey: "Shift+Tab"},
			{OperationID: operation.OperationTaskSetDueDate, Keys: []string{"@"}, DisplayKey: "@", FooterRank: 3, FooterLabel: "date"},
			// # tags whatever the cursor names. The two operations carry different
			// targets and different vocabularies, so they stay two descriptors with
			// two honest labels rather than one that branches on runtime context.
			{OperationID: operation.OperationTaskSetTags, Keys: []string{"#"}, DisplayKey: "#", FooterRank: 4, FooterLabel: "tags/context", SharedKey: true},
			{OperationID: operationProjectSetTags, Keys: []string{"#"}, DisplayKey: "#", FooterRank: 4, FooterLabel: "project tags", SharedKey: true},
			// * pins whatever the cursor names, for the same reason # tags it: two
			// targets and two vocabularies stay two descriptors with two honest
			// labels rather than one that branches on runtime context.
			{OperationID: operationTaskTogglePin, Keys: []string{"*"}, DisplayKey: "*", FooterRank: 24, FooterLabel: "pin", SharedKey: true},
			{OperationID: operationScopePinProject, Keys: []string{"*"}, DisplayKey: "*", FooterRank: 24, FooterLabel: "project pin", SharedKey: true},
			{OperationID: operationTaskCopyTitle, Keys: []string{"y"}, DisplayKey: "y"},
			{OperationID: operationTaskCopyDetails, Keys: []string{"Y"}, DisplayKey: "Y"},
			{OperationID: operationTaskCopyTitleDescription, Keys: []string{"ctrl+y"}, DisplayKey: "Ctrl+Y"},
			{OperationID: operation.OperationTaskSetPriority, Sequence: []string{"!", "0"}, DisplayKey: "!0-3", HintLabel: "unset", PresetValue: 0, HasPresetValue: true},
			{OperationID: operation.OperationTaskSetPriority, Sequence: []string{"!", "1"}, DisplayKey: "!0-3", HintLabel: "high", PresetValue: 1, HasPresetValue: true},
			{OperationID: operation.OperationTaskSetPriority, Sequence: []string{"!", "2"}, DisplayKey: "!0-3", HintLabel: "medium", PresetValue: 2, HasPresetValue: true},
			{OperationID: operation.OperationTaskSetPriority, Sequence: []string{"!", "3"}, DisplayKey: "!0-3", HintLabel: "low", PresetValue: 3, HasPresetValue: true},
			{OperationID: operation.OperationTaskSetState, Sequence: []string{"$", "i"}, DisplayKey: "$i/g/w/p/m/d/c", HintLabel: "inbox", PresetValue: models.StateInbox, HasPresetValue: true},
			{OperationID: operation.OperationCoreBacklog, Sequence: []string{"$", "b"}, DisplayKey: "$b", HintLabel: "backlog"},
			{OperationID: operation.OperationCoreNext, Sequence: []string{"$", "n"}, DisplayKey: "$n", HintLabel: "next"},
			{OperationID: operation.OperationCoreToday, Sequence: []string{"$", "t"}, DisplayKey: "$t", HintLabel: "today"},
			{OperationID: operation.OperationTaskSetState, Sequence: []string{"$", "g"}, DisplayKey: "$i/g/w/p/m/d/c", HintLabel: "doing", PresetValue: models.StateDoing, HasPresetValue: true},
			{OperationID: operation.OperationTaskSetState, Sequence: []string{"$", "w"}, DisplayKey: "$i/g/w/p/m/d/c", HintLabel: "waiting", PresetValue: models.StateWaiting, HasPresetValue: true},
			{OperationID: operation.OperationTaskSetState, Sequence: []string{"$", "p"}, DisplayKey: "$i/g/w/p/m/d/c", HintLabel: "paused", PresetValue: models.StatePaused, HasPresetValue: true},
			{OperationID: operation.OperationTaskSetState, Sequence: []string{"$", "m"}, DisplayKey: "$i/g/w/p/m/d/c", HintLabel: "someday", PresetValue: models.StateSomeday, HasPresetValue: true},
			{OperationID: operation.OperationTaskSetState, Sequence: []string{"$", "d"}, DisplayKey: "$i/g/w/p/m/d/c", HintLabel: "done", PresetValue: models.StateDone, HasPresetValue: true},
			{OperationID: operation.OperationTaskSetState, Sequence: []string{"$", "c"}, DisplayKey: "$i/g/w/p/m/d/c", HintLabel: "canceled", PresetValue: models.StateCanceled, HasPresetValue: true},
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
			{OperationID: operation.OperationCoreResetToLaunchView, Keys: []string{"0"}, DisplayKey: "0"},
			// Each mode jump carries both its go-to chord and the digit at its
			// Tab-cycle position, so the mnemonic route and the cheap one are one
			// binding with one composed spelling rather than two descriptors.
			{OperationID: operation.OperationCoreGotoAll, Keys: []string{"1"}, Sequence: []string{"g", "a"}, DisplayKey: "ga/1", HintLabel: "all"},
			{OperationID: operation.OperationCoreGotoInbox, Keys: []string{"3"}, Sequence: []string{"g", "i"}, DisplayKey: "gi/3", HintLabel: "inbox"},
			{OperationID: operation.OperationCoreGotoToday, Keys: []string{"2"}, Sequence: []string{"g", "t"}, DisplayKey: "gt/2", HintLabel: "today"},
			{OperationID: operation.OperationCoreGotoNext, Keys: []string{"4"}, Sequence: []string{"g", "n"}, DisplayKey: "gn/4", HintLabel: "next"},
			{OperationID: operation.OperationCoreGotoBacklog, Keys: []string{"5"}, Sequence: []string{"g", "b"}, DisplayKey: "gb/5", HintLabel: "backlog"},
			{OperationID: operation.OperationCoreGotoDue, Keys: []string{"6"}, Sequence: []string{"g", "@"}, DisplayKey: "g@/6", HintLabel: "due"},
			{OperationID: operation.OperationCoreGotoRecent, Keys: []string{"7"}, Sequence: []string{"g", "r"}, DisplayKey: "gr/7", HintLabel: "recent"},
			{OperationID: operation.OperationCoreGotoTaskProject, Sequence: []string{"g", "p"}, DisplayKey: "gp", HintLabel: "task project"},
			// Conflicts joins the go-to chord rather than claiming a root key: `c` is
			// retired at the root, and what the strip already names as workspace state
			// is something the user goes to look at.
			{OperationID: operation.OperationCoreConflicts, Sequence: []string{"g", "c"}, DisplayKey: "gc", HintLabel: "conflicts"},
			{OperationID: operation.OperationCoreScrollCursorTop, Sequence: []string{"z", "t"}, DisplayKey: "zt", HintLabel: "row to top"},
			{OperationID: operation.OperationCoreScrollCursorMiddle, Sequence: []string{"z", "z"}, DisplayKey: "zz", HintLabel: "row to middle"},
			{OperationID: operation.OperationCoreScrollCursorBottom, Sequence: []string{"z", "b"}, DisplayKey: "zb", HintLabel: "row to bottom"},
		},
		ChordGroups: []operation.OperationChordGroup{
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
