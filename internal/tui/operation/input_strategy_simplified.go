package operation

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

// GotoModeOperations maps each direct mode-jump operation to the mode it
// selects. Seven descriptors rather than one preset-valued operation, so the
// grammar never needs a mode value provider and cannot resurrect a mode picker.
var GotoModeOperations = map[OperationID]chrome.OutlinerModeID{
	OperationCoreGotoAll:     chrome.ModeAll,
	OperationCoreGotoInbox:   chrome.ModeInbox,
	OperationCoreGotoToday:   chrome.ModeToday,
	OperationCoreGotoNext:    chrome.ModeNext,
	OperationCoreGotoBacklog: chrome.ModeBacklog,
	OperationCoreGotoDue:     chrome.ModeDue,
	OperationCoreGotoRecent:  chrome.ModeRecent,
}

// SimplifiedInputStrategy returns a fresh copy of the accepted daily-loop
// grammar. Direct keys and chord sequences are declared here and nowhere else.
func SimplifiedInputStrategy() InputStrategy {
	return InputStrategy{
		ID: InputStrategySimplified,
		Bindings: []OperationKeyBinding{
			{OperationID: OperationCoreCapture, Keys: []string{"a"}, DisplayKey: "a", FooterRank: 8, FooterLabel: "capture"},
			// The keys a checklist row shares with a task row are declared
			// checklist-first: shared-key resolution takes the first applicable
			// descriptor, and only one of each pair applies to any given cursor.
			// e is three-way, because a project header edits the project it names:
			// exactly one of the three applies at any cursor.
			{OperationID: OperationChecklistEditItem, Keys: []string{"e"}, DisplayKey: "e", FooterRank: 12, FooterLabel: "item", SharedKey: true},
			{OperationID: OperationScopeRenameProject, Keys: []string{"e"}, DisplayKey: "e", FooterRank: 12, FooterLabel: "project", SharedKey: true},
			{OperationID: OperationCoreEdit, Keys: []string{"e"}, DisplayKey: "e", FooterRank: 12, FooterLabel: "edit", SharedKey: true},
			{OperationID: OperationCoreEditDescription, Keys: []string{"E"}, DisplayKey: "E", FooterRank: 20, FooterLabel: "description"},
			{OperationID: OperationCoreExternalEditor, Keys: []string{"ctrl+e"}, DisplayKey: "Ctrl+E", FooterRank: 21, FooterLabel: "editor"},
			{OperationID: OperationCoreOutlineEditor, Keys: []string{"O"}, DisplayKey: "O", FooterRank: 23, FooterLabel: "outline"},
			{OperationID: OperationTaskNewChild, Keys: []string{"A"}, DisplayKey: "A", FooterRank: 21, FooterLabel: "child"},
			// c appends to the cursor's checklist from either row, so it is one
			// binding rather than a shared pair: the descriptor answers from the
			// owner task and from any of its items alike.
			{OperationID: OperationChecklistAddItem, Keys: []string{"c"}, DisplayKey: "c", FooterRank: 21, FooterLabel: "checklist item"},
			{OperationID: OperationChecklistToggleItem, Keys: []string{"x"}, DisplayKey: "x", FooterRank: 13, FooterLabel: "tick"},
			// Space is a three-way shared key: checklist row, project header, task
			// row. Exactly one of the three applies at any cursor, so declaration
			// order only decides which is asked first, never which one wins.
			{OperationID: OperationChecklistActions, Keys: []string{"space"}, DisplayKey: "Space", FooterRank: 5, FooterLabel: "actions", SharedKey: true},
			{OperationID: OperationProjectActions, Keys: []string{"space"}, DisplayKey: "Space", FooterRank: 5, FooterLabel: "actions", SharedKey: true},
			{OperationID: OperationCoreActions, Keys: []string{"space"}, DisplayKey: "Space", FooterRank: 5, FooterLabel: "actions", SharedKey: true},
			{OperationID: OperationCoreCursorDown, Keys: []string{"j", "down"}, DisplayKey: "j/↓"},
			{OperationID: OperationCoreCursorUp, Keys: []string{"k", "up"}, DisplayKey: "k/↑"},
			{OperationID: OperationCoreCursorTop, Keys: []string{"home"}, Sequence: []string{"g", "g"}, DisplayKey: "gg/Home", HintLabel: "top"},
			{OperationID: OperationCoreCursorBottom, Keys: []string{"G", "end"}, DisplayKey: "G/End"},
			// Ctrl+U and Ctrl+D are the half page of whatever the reader is looking
			// at: the detail pane while a Peek is on screen, the list otherwise. The
			// two meanings are two descriptors on one shared path, Peek first because
			// resolution takes the first applicable binding, and Peek's is applicable
			// on a visible document alone so the pane keeps the key at both ends of
			// its body. PgUp and PgDn are deliberately not on that path: they stay the
			// list's own spelling, so the outline is still reachable by keyboard with
			// a Peek open, which is the whole reason the aliases are declared apart
			// from the Ctrl pair rather than beside it.
			{OperationID: OperationCorePeekHalfPageUp, Keys: []string{"ctrl+u"}, DisplayKey: "Ctrl+U", SharedKey: true},
			{OperationID: OperationCoreCursorPageUp, Keys: []string{"ctrl+u", "pgup"}, DisplayKey: "Ctrl+U/PgUp", SharedKey: true},
			{OperationID: OperationCorePeekHalfPageDown, Keys: []string{"ctrl+d"}, DisplayKey: "Ctrl+D", SharedKey: true},
			{OperationID: OperationCoreCursorPageDown, Keys: []string{"ctrl+d", "pgdown"}, DisplayKey: "Ctrl+D/PgDn", SharedKey: true},
			{OperationID: OperationCoreJumpBackward, Keys: []string{"ctrl+o"}, DisplayKey: "Ctrl+O"},
			{OperationID: OperationCoreJumpForward, Keys: []string{"ctrl+i"}, DisplayKey: "Ctrl+I"},
			{OperationID: OperationCoreFocusLatest, Keys: []string{"ctrl+l"}, DisplayKey: "Ctrl+L"},
			{OperationID: OperationCoreSelectPinned, Keys: []string{"ctrl+p"}, DisplayKey: "Ctrl+P"},
			{OperationID: OperationCoreCollapseParent, Keys: []string{"h", "left"}, DisplayKey: "h/←"},
			{OperationID: OperationCoreExpand, Keys: []string{"l", "right"}, DisplayKey: "l/→"},
			// One lowercase toggle per fold axis, declared together so the family
			// reads as one: s sections, p project groups, t task subtrees.
			{OperationID: OperationCoreToggleSections, Keys: []string{"s"}, DisplayKey: "s"},
			{OperationID: OperationCoreToggleProjectGroup, Keys: []string{"p"}, DisplayKey: "p"},
			{OperationID: OperationCoreToggleSubtrees, Keys: []string{"t"}, DisplayKey: "t"},
			{OperationID: OperationCoreNextHeader, Keys: []string{"n"}, DisplayKey: "n"},
			{OperationID: OperationCorePreviousHeader, Keys: []string{"N"}, DisplayKey: "N"},
			{OperationID: OperationChecklistMoveDown, Keys: []string{"J"}, DisplayKey: "J", SharedKey: true},
			{OperationID: OperationCoreReorderDown, Keys: []string{"J"}, DisplayKey: "J", SharedKey: true},
			{OperationID: OperationChecklistMoveUp, Keys: []string{"K"}, DisplayKey: "K", SharedKey: true},
			{OperationID: OperationCoreReorderUp, Keys: []string{"K"}, DisplayKey: "K", SharedKey: true},
			{OperationID: OperationChecklistMoveBottom, Keys: []string{"ctrl+j"}, DisplayKey: "Ctrl+J", SharedKey: true},
			{OperationID: OperationCoreReorderBottom, Keys: []string{"ctrl+j"}, DisplayKey: "Ctrl+J", SharedKey: true},
			{OperationID: OperationChecklistMoveTop, Keys: []string{"ctrl+k"}, DisplayKey: "Ctrl+K", SharedKey: true},
			{OperationID: OperationCoreReorderTop, Keys: []string{"ctrl+k"}, DisplayKey: "Ctrl+K", SharedKey: true},
			{OperationID: OperationTaskOutdent, Keys: []string{"["}, DisplayKey: "["},
			{OperationID: OperationTaskIndent, Keys: []string{"]"}, DisplayKey: "]"},
			{OperationID: OperationTaskSetParent, Keys: []string{">"}, DisplayKey: ">"},
			{OperationID: OperationTaskAdoptChildren, Keys: []string{"<"}, DisplayKey: "<"},
			{OperationID: OperationTaskReferences, Keys: []string{"~"}, DisplayKey: "~", FooterRank: 16, FooterLabel: "dependencies"},
			{OperationID: OperationTaskMoveToProject, Keys: []string{"%"}, DisplayKey: "%"},
			// The two capture sigils that carry task content keep the same
			// correspondence the other keyed sigils do: the character that writes an
			// attachment or a comment on a capture line opens that panel on a row.
			{OperationID: OperationTaskAttachments, Keys: []string{"&"}, DisplayKey: "&", FooterRank: 14, FooterLabel: "attachments"},
			{OperationID: OperationTaskComments, Keys: []string{";"}, DisplayKey: ";", FooterRank: 15, FooterLabel: "comments", SharedKey: true},
			// `;` reads comments on whatever the cursor names, on the same terms as the
			// `C` pair below it: a project header opens the project's own comments
			// through its own descriptor rather than through one that branches on
			// runtime context. It takes no second footer rank, because the rank above
			// already advertises the key.
			{OperationID: OperationProjectComments, Keys: []string{";"}, DisplayKey: ";", SharedKey: true},
			// Enter zooms in on whatever the cursor names. The four acts carry
			// different targets and different vocabularies, so they stay four
			// descriptors with four honest labels rather than one that branches on
			// runtime context. Open Note is the one that leaves the Outliner: a
			// search result the query answered with a note is read where notes live.
			{OperationID: OperationCoreFocusSubtree, Keys: []string{"enter"}, DisplayKey: "Enter", FooterRank: 9, FooterLabel: "focus", SharedKey: true},
			{OperationID: OperationCoreZoomSection, Keys: []string{"enter"}, DisplayKey: "Enter", FooterRank: 9, FooterLabel: "zoom", SharedKey: true},
			{OperationID: OperationCoreActivateGroup, Keys: []string{"enter"}, DisplayKey: "Enter", FooterRank: 9, FooterLabel: "open", SharedKey: true},
			{OperationID: OperationCoreOpenNote, Keys: []string{"enter"}, DisplayKey: "Enter", FooterRank: 9, FooterLabel: "open note", SharedKey: true},
			{OperationID: OperationCoreFocusParent, Keys: []string{"shift+enter"}, DisplayKey: "Shift+Enter"},
			{OperationID: OperationCorePeek, Keys: []string{"b"}, DisplayKey: "b", FooterRank: 10, FooterLabel: "peek"},
			// Ctrl+F/B page the detail document a whole window at a time, exactly as
			// they do beside the Hub's preview, and they own their keys outright
			// rather than sharing them: the list has no full page of its own to
			// disambiguate against. Neither carries a FooterRank — the Peek guidance
			// advertises them where a body can actually move, and a rank would spend
			// a column on a key that is inert on every other row.
			{OperationID: OperationCorePeekPageDown, Keys: []string{"ctrl+f"}, DisplayKey: "Ctrl+F"},
			{OperationID: OperationCorePeekPageUp, Keys: []string{"ctrl+b"}, DisplayKey: "Ctrl+B"},
			{OperationID: OperationCoreScopeSearch, Keys: []string{"/"}, DisplayKey: "/", FooterRank: 10, FooterLabel: "search"},
			{OperationID: OperationCoreMultiSelect, Keys: []string{"v"}, DisplayKey: "v", FooterRank: 7, FooterLabel: "select"},
			{OperationID: OperationCoreRangeSelect, Keys: []string{"V"}, DisplayKey: "V"},
			{OperationID: OperationCoreSelectAll, Keys: []string{"ctrl+a"}, DisplayKey: "Ctrl+A"},
			{OperationID: OperationCoreSelectProject, Keys: []string{"P"}, DisplayKey: "P", FooterRank: 18, FooterLabel: "projects"},
			// Named Lenses carries no FooterRank on purpose: it is a discoverable
			// entry point rather than a daily-loop act, so `?` teaches it and the
			// compact footer stays as quiet as it is today.
			{OperationID: OperationCoreNamedLenses, Keys: []string{"\\"}, DisplayKey: `\`},
			// The readout expands the narrowing the operational title already
			// summarizes, so it claims a root key of its own rather than joining the
			// f chord: it reads the lens, the scope, the mode, and the query beside
			// the facets, and a "filter" prefix would be dishonest about four of them.
			{OperationID: OperationCoreQueryReadout, Keys: []string{"Q"}, DisplayKey: "Q", FooterRank: 27, FooterLabel: "narrowing"},
			{OperationID: OperationCoreNotes, Keys: []string{"ctrl+n"}, DisplayKey: "Ctrl+N"},
			{OperationID: OperationCoreToggleTheme, Keys: []string{"T"}, DisplayKey: "T", FooterRank: 22, FooterLabel: "theme"},
			// The bar is the one browsable map of this grammar, so its reveal
			// outranks every act it lists: a user who cannot find anything needs the
			// hint that shows them everything before they need any single key.
			{OperationID: OperationCoreToggleMenuBar, Keys: []string{"M"}, DisplayKey: "M", FooterRank: 6, FooterLabel: "menu"},
			// Opening carries no FooterRank of its own. The band's own reveal is
			// already ranked above, and once the band is on screen it is the label
			// row that advertises what opening does; a second footer hint would spend
			// a rank restating chrome the user is already looking at. The keys inside
			// an open menu take none either, per the gated-keymap invariant.
			{OperationID: OperationCoreOpenMenuBar, Keys: []string{"ctrl+m"}, DisplayKey: "Ctrl+M"},
			{OperationID: OperationCoreUndo, Keys: []string{"u"}, DisplayKey: "u"},
			{OperationID: OperationCoreRedo, Keys: []string{"ctrl+r"}, DisplayKey: "Ctrl+R"},
			{OperationID: OperationCoreBack, Keys: []string{"esc"}, DisplayKey: "Esc"},
			{OperationID: OperationCoreHelp, Keys: []string{"?"}, DisplayKey: "?", FooterRank: 11, FooterLabel: "help"},
			{OperationID: OperationCoreQuit, Keys: []string{"ctrl+c"}, DisplayKey: "Ctrl+C"},
			{OperationID: OperationCoreNextWorkMode, Keys: []string{"tab"}, DisplayKey: "Tab"},
			{OperationID: OperationCorePreviousWorkMode, Keys: []string{"shift+tab"}, DisplayKey: "Shift+Tab"},
			{OperationID: OperationTaskSetDueDate, Keys: []string{"@"}, DisplayKey: "@", FooterRank: 3, FooterLabel: "date"},
			// # tags whatever the cursor names. The two operations carry different
			// targets and different vocabularies, so they stay two descriptors with
			// two honest labels rather than one that branches on runtime context.
			{OperationID: OperationTaskSetTags, Keys: []string{"#"}, DisplayKey: "#", FooterRank: 4, FooterLabel: "tags/context", SharedKey: true},
			{OperationID: OperationProjectSetTags, Keys: []string{"#"}, DisplayKey: "#", FooterRank: 4, FooterLabel: "project tags", SharedKey: true},
			// * pins whatever the cursor names, for the same reason # tags it: two
			// targets and two vocabularies stay two descriptors with two honest
			// labels rather than one that branches on runtime context.
			{OperationID: OperationTaskTogglePin, Keys: []string{"*"}, DisplayKey: "*", FooterRank: 24, FooterLabel: "pin", SharedKey: true},
			{OperationID: OperationScopePinProject, Keys: []string{"*"}, DisplayKey: "*", FooterRank: 24, FooterLabel: "project pin", SharedKey: true},
			// = writes the review flag the =review sigil writes and f= narrows by,
			// so the axis answers to one character on all three surfaces. Unlike *
			// and # it names only a task: there is no project review flag, so the key
			// is not shared and never branches on what the cursor holds. It sits at
			// the tail of the ladder because flagging work for a later pass is a
			// deliberate, occasional act rather than a daily-loop one.
			{OperationID: OperationTaskToggleReview, Keys: []string{"="}, DisplayKey: "=", FooterRank: 28, FooterLabel: "review"},
			// R acknowledges what = flagged, and carries no FooterRank: the pass it
			// belongs to is a periodic sweep rather than a daily-loop act, and the
			// footer already spends a rank on the flag itself. The capital is the
			// shifted neighbor of nothing — `r` is the resume note — but it reads as
			// Review to anyone running the sweep, and lowercase `r` was spoken for
			// before this key existed.
			{OperationID: OperationTaskMarkReviewed, Keys: []string{"R"}, DisplayKey: "R"},
			// r carries no FooterRank. The note is written when a task is put down
			// and read from Peek when it is picked back up, so it is an occasional
			// deliberate act like Named Lenses rather than a daily-loop one; `?` and
			// the menu bar teach it and the compact footer stays as quiet as it is.
			{OperationID: OperationTaskSetResumeNote, Keys: []string{"r"}, DisplayKey: "r"},
			// C carries no FooterRank either, and for the same reason: `;` already
			// advertises comments in the footer, and this key is the shortcut past
			// the panel for the one act — appending — that is worth not leaving the
			// list for. The capital is the shifted form of the panel's own sigil
			// neighborhood rather than a new letter: `;` reads comments, `C` writes
			// one.
			// C writes a comment on whatever the cursor names, for the same reason #
			// tags it and * pins it: the task and the project store different records
			// through different use cases, so they stay two descriptors with two
			// honest labels rather than one that branches on runtime context.
			{OperationID: OperationTaskAddComment, Keys: []string{"C"}, DisplayKey: "C", SharedKey: true},
			{OperationID: OperationProjectAddComment, Keys: []string{"C"}, DisplayKey: "C", SharedKey: true},
			// D carries no FooterRank on the same terms as r and C: repeating work
			// is deliberate and occasional, and the footer already spends its ranks
			// on the daily loop. The capital keeps the lowercase letter free and
			// stays clear of `d`, which is retired vocabulary no alias may return.
			{OperationID: OperationTaskRepeat, Keys: []string{"D"}, DisplayKey: "D"},
			{OperationID: OperationTaskCopyTitle, Keys: []string{"y"}, DisplayKey: "y"},
			{OperationID: OperationTaskCopyDetails, Keys: []string{"Y"}, DisplayKey: "Y"},
			{OperationID: OperationTaskCopyTitleDescription, Keys: []string{"ctrl+y"}, DisplayKey: "Ctrl+Y"},
			{OperationID: OperationTaskSetPriority, Sequence: []string{"!", "0"}, DisplayKey: "!0-3", HintLabel: "unset", PresetValue: 0, HasPresetValue: true},
			{OperationID: OperationTaskSetPriority, Sequence: []string{"!", "1"}, DisplayKey: "!0-3", HintLabel: "high", PresetValue: 1, HasPresetValue: true},
			{OperationID: OperationTaskSetPriority, Sequence: []string{"!", "2"}, DisplayKey: "!0-3", HintLabel: "medium", PresetValue: 2, HasPresetValue: true},
			{OperationID: OperationTaskSetPriority, Sequence: []string{"!", "3"}, DisplayKey: "!0-3", HintLabel: "low", PresetValue: 3, HasPresetValue: true},
			// Workflow is the one chord whose prefix is a letter rather than the
			// sigil its terminals write. `$` stays the destination sigil on every
			// typed surface, but these ten acts are the most-pressed chord of the
			// daily loop and a shifted key on the number row is the wrong home for
			// them; `w` is the unshifted initial of the word the chord is named
			// after, and the sigil grammar had already retired it. The trade is
			// made once, here, and the states facet below follows the keyboard
			// rather than the sigil so `w` and `fw` still read as one family.
			{OperationID: OperationTaskSetState, Sequence: []string{"w", "i"}, DisplayKey: "wi/g/w/p/m/d/c", HintLabel: "inbox", PresetValue: models.StateInbox, HasPresetValue: true},
			{OperationID: OperationCoreBacklog, Sequence: []string{"w", "b"}, DisplayKey: "wb", HintLabel: "backlog"},
			{OperationID: OperationCoreNext, Sequence: []string{"w", "n"}, DisplayKey: "wn", HintLabel: "next"},
			{OperationID: OperationCoreToday, Sequence: []string{"w", "t"}, DisplayKey: "wt", HintLabel: "today"},
			{OperationID: OperationTaskSetState, Sequence: []string{"w", "g"}, DisplayKey: "wi/g/w/p/m/d/c", HintLabel: "doing", PresetValue: models.StateDoing, HasPresetValue: true},
			{OperationID: OperationTaskSetState, Sequence: []string{"w", "w"}, DisplayKey: "wi/g/w/p/m/d/c", HintLabel: "waiting", PresetValue: models.StateWaiting, HasPresetValue: true},
			{OperationID: OperationTaskSetState, Sequence: []string{"w", "p"}, DisplayKey: "wi/g/w/p/m/d/c", HintLabel: "paused", PresetValue: models.StatePaused, HasPresetValue: true},
			{OperationID: OperationTaskSetState, Sequence: []string{"w", "m"}, DisplayKey: "wi/g/w/p/m/d/c", HintLabel: "someday", PresetValue: models.StateSomeday, HasPresetValue: true},
			{OperationID: OperationTaskSetState, Sequence: []string{"w", "d"}, DisplayKey: "wi/g/w/p/m/d/c", HintLabel: "done", PresetValue: models.StateDone, HasPresetValue: true},
			{OperationID: OperationTaskSetState, Sequence: []string{"w", "c"}, DisplayKey: "wi/g/w/p/m/d/c", HintLabel: "canceled", PresetValue: models.StateCanceled, HasPresetValue: true},
			{OperationID: OperationFilterPriority, Sequence: []string{"f", "!", "0"}, DisplayKey: "f!0-3", HintLabel: "clear", PresetValue: 0, HasPresetValue: true},
			{OperationID: OperationFilterPriority, Sequence: []string{"f", "!", "1"}, DisplayKey: "f!0-3", HintLabel: "high", PresetValue: 1, HasPresetValue: true},
			{OperationID: OperationFilterPriority, Sequence: []string{"f", "!", "2"}, DisplayKey: "f!0-3", HintLabel: "medium+", PresetValue: 2, HasPresetValue: true},
			{OperationID: OperationFilterPriority, Sequence: []string{"f", "!", "3"}, DisplayKey: "f!0-3", HintLabel: "low+", PresetValue: 3, HasPresetValue: true},
			{OperationID: OperationFilterDue, Sequence: []string{"f", "@"}, DisplayKey: "f@", HintLabel: "due/deferred"},
			{OperationID: OperationFilterCompletionAge, Sequence: []string{"f", "a"}, DisplayKey: "fa", HintLabel: "completion age"},
			// The states facet is the stated exception to spelling a second key
			// like the sigil it narrows: the workflow chord above took the letter
			// w, and a facet over the same axis has to answer to the same key or
			// the two halves stop reading as one family. f$ is retired outright.
			{OperationID: OperationFilterStates, Sequence: []string{"f", "w"}, DisplayKey: "fw", HintLabel: "states"},
			{OperationID: OperationFilterTags, Sequence: []string{"f", "#"}, DisplayKey: "f#", HintLabel: "tags"},
			// Context is declared immediately after tags so the which-key row
			// reads tags then contexts, which is the order the two sigils teach.
			{OperationID: OperationFilterContexts, Sequence: []string{"f", "c"}, DisplayKey: "fc", HintLabel: "contexts"},
			{OperationID: OperationFilterReview, Sequence: []string{"f", "="}, DisplayKey: "f=", HintLabel: "review"},
			{OperationID: OperationFilterHierarchy, Sequence: []string{"f", "h"}, DisplayKey: "fh", HintLabel: "hierarchy"},
			// A facet carrying a sigil answers to that sigil, so the reference axis
			// is f~ and no letter is bound to it.
			{OperationID: OperationFilterDependency, Sequence: []string{"f", "~"}, DisplayKey: "f~", HintLabel: "?blocked/?blocking"},
			{OperationID: OperationFilterStale, Sequence: []string{"f", "s"}, DisplayKey: "fs", HintLabel: "stale"},
			// Pinned answers to its own sigil like every other sigil-carrying facet:
			// *pin is what the clause spells, so f* is what narrows by it. The letter
			// fp stays retired, because it used to mean priority and a letter still
			// bound would silently apply the wrong facet.
			{OperationID: OperationFilterPinned, Sequence: []string{"f", "*"}, DisplayKey: "f*", HintLabel: "pinned"},
			{OperationID: OperationFilterClear, Sequence: []string{"f", "x"}, DisplayKey: "fx", HintLabel: "clear all"},
			{OperationID: OperationFilterProjectTags, Sequence: []string{"F", "#"}, DisplayKey: "F#", HintLabel: "project tags"},
			{OperationID: OperationFilterShowParked, Sequence: []string{"F", "p"}, DisplayKey: "Fp", HintLabel: "include parked"},
			{OperationID: OperationFilterOnlyParked, Sequence: []string{"F", "P"}, DisplayKey: "FP", HintLabel: "only parked"},
			{OperationID: OperationFilterIncludeArchived, Sequence: []string{"F", "a"}, DisplayKey: "Fa", HintLabel: "include archived"},
			{OperationID: OperationFilterOnlyArchived, Sequence: []string{"F", "A"}, DisplayKey: "FA", HintLabel: "only archived"},
			{OperationID: OperationFilterProjectClear, Sequence: []string{"F", "x"}, DisplayKey: "Fx", HintLabel: "clear project filters"},
			{OperationID: OperationSortDueDate, Sequence: []string{"o", "@"}, DisplayKey: "o@", HintLabel: "due date"},
			{OperationID: OperationSortPriority, Sequence: []string{"o", "!"}, DisplayKey: "o!", HintLabel: "priority"},
			{OperationID: OperationSortCreatedAt, Sequence: []string{"o", "c"}, DisplayKey: "oc", HintLabel: "created"},
			{OperationID: OperationSortUpdatedAt, Sequence: []string{"o", "u"}, DisplayKey: "ou", HintLabel: "updated"},
			{OperationID: OperationSortTitle, Sequence: []string{"o", "t"}, DisplayKey: "ot", HintLabel: "title"},
			{OperationID: OperationSortClear, Sequence: []string{"o", "x"}, DisplayKey: "ox", HintLabel: "clear sort"},
			{OperationID: OperationSortRanked, Sequence: []string{"o", "r"}, DisplayKey: "or", HintLabel: "ranked list"},
			// 0 leads the digit row and is the only one of the eight that names no
			// mode: it puts the whole view back where a lens-free launch starts. It
			// takes no footer rank, because it is the escape hatch a user reaches for
			// once rather than an act of the daily loop.
			{OperationID: OperationCoreResetToLaunchView, Keys: []string{"0"}, DisplayKey: "0"},
			// Each mode jump carries both its go-to chord and the digit at its
			// Tab-cycle position, so the mnemonic route and the cheap one are one
			// binding with one composed spelling rather than two descriptors.
			{OperationID: OperationCoreGotoAll, Keys: []string{"1"}, Sequence: []string{"g", "a"}, DisplayKey: "ga/1", HintLabel: "all"},
			{OperationID: OperationCoreGotoInbox, Keys: []string{"3"}, Sequence: []string{"g", "i"}, DisplayKey: "gi/3", HintLabel: "inbox"},
			{OperationID: OperationCoreGotoToday, Keys: []string{"2"}, Sequence: []string{"g", "t"}, DisplayKey: "gt/2", HintLabel: "today"},
			{OperationID: OperationCoreGotoNext, Keys: []string{"4"}, Sequence: []string{"g", "n"}, DisplayKey: "gn/4", HintLabel: "next"},
			{OperationID: OperationCoreGotoBacklog, Keys: []string{"5"}, Sequence: []string{"g", "b"}, DisplayKey: "gb/5", HintLabel: "backlog"},
			{OperationID: OperationCoreGotoDue, Keys: []string{"6"}, Sequence: []string{"g", "@"}, DisplayKey: "g@/6", HintLabel: "due"},
			{OperationID: OperationCoreGotoRecent, Keys: []string{"7"}, Sequence: []string{"g", "r"}, DisplayKey: "gr/7", HintLabel: "recent"},
			{OperationID: OperationCoreGotoTaskProject, Sequence: []string{"g", "p"}, DisplayKey: "gp", HintLabel: "task project"},
			// `~` is the reference sigil everywhere else, so the relationship walk
			// spells itself the same way under the go-to chord.
			{OperationID: OperationCoreGotoRelation, Sequence: []string{"g", "~"}, DisplayKey: "g~", HintLabel: "related task"},
			// Conflicts joins the go-to chord rather than claiming a root key: what
			// the strip already names as workspace state is something the user goes
			// to look at, and root `c` appends a checklist item.
			{OperationID: OperationCoreConflicts, Sequence: []string{"g", "c"}, DisplayKey: "gc", HintLabel: "conflicts"},
			{OperationID: OperationCoreScrollCursorTop, Sequence: []string{"z", "t"}, DisplayKey: "zt", HintLabel: "row to top"},
			{OperationID: OperationCoreScrollCursorMiddle, Sequence: []string{"z", "z"}, DisplayKey: "zz", HintLabel: "row to middle"},
			{OperationID: OperationCoreScrollCursorBottom, Sequence: []string{"z", "b"}, DisplayKey: "zb", HintLabel: "row to bottom"},
		},
		ChordGroups: []OperationChordGroup{
			{Sequence: []string{"!"}, HintLabel: "priority", FooterRank: 2, FooterLabel: "priority"},
			{Sequence: []string{"w"}, HintLabel: "workflow", FooterRank: 1, FooterLabel: "workflow"},
			{Sequence: []string{"f"}, HintLabel: "filter", FooterRank: 19, FooterLabel: "filter"},
			{Sequence: []string{"f", "!"}, HintLabel: "priority threshold"},
			{Sequence: []string{"F"}, HintLabel: "project filter", FooterRank: 25, FooterLabel: "project filter"},
			{Sequence: []string{"o"}, HintLabel: "sort", FooterRank: 23, FooterLabel: "sort"},
			{Sequence: []string{"g"}, HintLabel: "go to", FooterRank: 26, FooterLabel: "go to"},
			{Sequence: []string{"z"}, HintLabel: "scroll"},
		},
	}
}
