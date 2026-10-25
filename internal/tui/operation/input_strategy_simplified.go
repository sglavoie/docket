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
//
// FooterRank is one ladder over the whole grammar, and its first six rungs are
// the acts a task row is read for: `a capture`, `w workflow`, `e edit`,
// `/ search`, `b peek`, `? help`. That head is sized rather than merely ordered
// — the six entries and their separators are exactly chrome.MinTerminalWidth
// wide, so the smallest supported terminal keeps all six and the packer's first
// refusal falls on rank 7. Adding a rank ahead of them, or lengthening one of
// their labels, costs the narrow footer whichever of the six sits last; the
// contract that says so is TestOutlinerFooterKeepsCaptureThroughHelpAtTheFloor.
//
// The ranks below that head are the second tier — the axes a row is changed on
// (date, priority, tags), then the sheets and selections that reach the rest —
// and the tail is the deliberate, occasional work. Zero excludes a binding from
// the footer entirely, and the menu bar and the generated `?` help project from
// the registry regardless of rank, so a key kept out of the ladder is quiet
// rather than undiscoverable.
func SimplifiedInputStrategy() InputStrategy {
	return InputStrategy{
		ID: InputStrategySimplified,
		Bindings: []OperationKeyBinding{
			// Capture leads the ladder outright: the act that puts a thought into
			// the workspace is the one a user must never have to look for, and it
			// is the one act on this row that is about work not yet on screen.
			{HelpGroup: "Everyday", HelpOrder: 10, OperationID: OperationCoreCapture, Keys: []string{"a"}, DisplayKey: "a", FooterRank: 1, FooterLabel: "capture"},
			// The keys a checklist row shares with a task row are declared
			// checklist-first: shared-key resolution takes the first applicable
			// descriptor, and only one of each pair applies to any given cursor.
			// e is three-way, because a project header edits the project it names:
			// exactly one of the three applies at any cursor.
			// All three share the ladder's third rung, because the ladder ranks the
			// key and the cursor decides which of the three it reached: whichever
			// one applies is the same act at the same place in the row's reading.
			{OperationID: OperationChecklistEditItem, Keys: []string{"e"}, DisplayKey: "e", FooterRank: 3, FooterLabel: "item", SharedKey: true},
			{OperationID: OperationScopeRenameProject, Keys: []string{"e"}, DisplayKey: "e", FooterRank: 3, FooterLabel: "project", SharedKey: true},
			{HelpGroup: "Everyday", HelpOrder: 20, OperationID: OperationCoreEdit, Keys: []string{"e"}, DisplayKey: "e", FooterRank: 3, FooterLabel: "edit", SharedKey: true},
			{OperationID: OperationCoreEditDescription, Keys: []string{"E"}, DisplayKey: "E", FooterRank: 20, FooterLabel: "description"},
			{OperationID: OperationCoreExternalEditor, Keys: []string{"ctrl+e"}, DisplayKey: "Ctrl+E", FooterRank: 21, FooterLabel: "editor"},
			{OperationID: OperationCoreOutlineEditor, Keys: []string{"O"}, DisplayKey: "O", FooterRank: 23, FooterLabel: "outline"},
			{HelpGroup: "Hierarchy", HelpOrder: 500, OperationID: OperationTaskNewChild, Keys: []string{"A"}, DisplayKey: "A", FooterRank: 21, FooterLabel: "child"},
			// c appends to the cursor's checklist from either row, so it is one
			// binding rather than a shared pair: the descriptor answers from the
			// owner task and from any of its items alike.
			{HelpGroup: "Hierarchy", HelpOrder: 501, OperationID: OperationChecklistAddItem, Keys: []string{"c"}, DisplayKey: "c", FooterRank: 21, FooterLabel: "checklist item"},
			{OperationID: OperationChecklistToggleItem, Keys: []string{"x"}, DisplayKey: "x", FooterRank: 14, FooterLabel: "tick"},
			// S converts the selected checklist item or child task in either direction.
			// The two descriptors share a key but have disjoint row applicability, so
			// help and menus name the action appropriate to the selected row.
			{OperationID: OperationChecklistPromoteItem, Keys: []string{"S"}, DisplayKey: "S", SharedKey: true},
			{OperationID: OperationChecklistDemoteChild, Keys: []string{"S"}, DisplayKey: "S", SharedKey: true},
			// Space is a three-way shared key: checklist row, project header, task
			// row. Exactly one of the three applies at any cursor, so declaration
			// order only decides which is asked first, never which one wins.
			{OperationID: OperationChecklistActions, Keys: []string{"space"}, DisplayKey: "Space", FooterRank: 10, FooterLabel: "actions", SharedKey: true},
			{OperationID: OperationProjectActions, Keys: []string{"space"}, DisplayKey: "Space", FooterRank: 10, FooterLabel: "actions", SharedKey: true},
			{OperationID: OperationCoreActions, Keys: []string{"space"}, DisplayKey: "Space", FooterRank: 10, FooterLabel: "actions", SharedKey: true},
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
			{OperationID: OperationCoreJumpForward, Keys: []string{"alt+right", "ctrl+i"}, DisplayKey: "Alt+→"},
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
			{HelpGroup: "Hierarchy", HelpOrder: 505, OperationID: OperationTaskOutdent, Keys: []string{"["}, DisplayKey: "["},
			{HelpGroup: "Hierarchy", HelpOrder: 504, OperationID: OperationTaskIndent, Keys: []string{"]"}, DisplayKey: "]"},
			{HelpGroup: "Hierarchy", HelpOrder: 502, OperationID: OperationTaskSetParent, Keys: []string{">"}, DisplayKey: ">"},
			{HelpGroup: "Hierarchy", HelpOrder: 503, OperationID: OperationTaskAdoptChildren, Keys: []string{"<"}, DisplayKey: "<"},
			{OperationID: OperationTaskReferences, Keys: []string{"~"}, DisplayKey: "~", FooterRank: 17, FooterLabel: "dependencies"},
			{OperationID: OperationTaskMoveToProject, Keys: []string{"%"}, DisplayKey: "%"},
			// The two capture sigils that carry task content keep the same
			// correspondence the other keyed sigils do: the character that writes an
			// attachment or a comment on a capture line opens that panel on a row.
			{OperationID: OperationTaskAttachments, Keys: []string{"&"}, DisplayKey: "&", FooterRank: 15, FooterLabel: "attachments"},
			{OperationID: OperationTaskComments, Keys: []string{";"}, DisplayKey: ";", FooterRank: 16, FooterLabel: "comments", SharedKey: true},
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
			{OperationID: OperationCoreFocusSubtree, Keys: []string{"enter"}, DisplayKey: "Enter", FooterRank: 13, FooterLabel: "focus", SharedKey: true},
			{OperationID: OperationCoreZoomSection, Keys: []string{"enter"}, DisplayKey: "Enter", FooterRank: 13, FooterLabel: "zoom", SharedKey: true},
			{OperationID: OperationCoreActivateGroup, Keys: []string{"enter"}, DisplayKey: "Enter", FooterRank: 13, FooterLabel: "open", SharedKey: true},
			{OperationID: OperationCoreOpenNote, Keys: []string{"enter"}, DisplayKey: "Enter", FooterRank: 13, FooterLabel: "open note", SharedKey: true},
			{OperationID: OperationCoreFocusParent, Keys: []string{"alt+up", "shift+enter"}, DisplayKey: "Alt+↑"},
			{OperationID: OperationCorePeekSize, Keys: []string{"B"}, DisplayKey: "B"},
			{HelpGroup: "Everyday", HelpOrder: 12, HelpCategory: "Tasks", OperationID: OperationCorePeek, Keys: []string{"b"}, DisplayKey: "b", FooterRank: 5, FooterLabel: "peek"},
			// Ctrl+F/B page the detail document a whole window at a time, exactly as
			// they do beside the Hub's preview, and they own their keys outright
			// rather than sharing them: the list has no full page of its own to
			// disambiguate against. Neither carries a FooterRank — the Peek guidance
			// advertises them where a body can actually move, and a rank would spend
			// a column on a key that is inert on every other row.
			{OperationID: OperationCorePeekPageDown, Keys: []string{"ctrl+f"}, DisplayKey: "Ctrl+F"},
			{OperationID: OperationCorePeekPageUp, Keys: []string{"ctrl+b"}, DisplayKey: "Ctrl+B"},
			{HelpGroup: "Everyday", HelpOrder: 11, HelpCategory: "Tasks", OperationID: OperationCoreScopeSearch, Keys: []string{"/"}, DisplayKey: "/", FooterRank: 4, FooterLabel: "search"},
			{HelpGroup: "Selection", HelpOrder: 600, OperationID: OperationCoreMultiSelect, Keys: []string{"v"}, DisplayKey: "v", FooterRank: 12, FooterLabel: "select"},
			{HelpGroup: "Selection", HelpOrder: 601, OperationID: OperationCoreRangeSelect, Keys: []string{"V"}, DisplayKey: "V"},
			{HelpGroup: "Selection", HelpOrder: 602, OperationID: OperationCoreSelectAll, Keys: []string{"ctrl+a"}, DisplayKey: "Ctrl+A"},
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
			// The bar is the one browsable map of this grammar, so its reveal stands
			// at the head of the second tier rather than among the daily acts: a
			// user who cannot find anything reaches it before any single key below
			// it, but `? help` is already on the narrow row naming the same thing
			// more directly, so spending one of the six leading rungs on the bar
			// would advertise two routes to discovery and no route to work.
			{OperationID: OperationCoreToggleMenuBar, Keys: []string{"M"}, DisplayKey: "M", FooterRank: 11, FooterLabel: "menu"},
			// Opening carries no FooterRank of its own. The band's own reveal is
			// already ranked above, and once the band is on screen it is the label
			// row that advertises what opening does; a second footer hint would spend
			// a rank restating chrome the user is already looking at. The keys inside
			// an open menu take none either, per the gated-keymap invariant.
			{OperationID: OperationCoreOpenMenuBar, Keys: []string{"f10", "ctrl+m"}, DisplayKey: "F10/Ctrl+M"},
			{HelpGroup: "Recovery", HelpOrder: 300, HelpCategory: "Tasks", OperationID: OperationCoreUndo, Keys: []string{"u"}, DisplayKey: "u"},
			{HelpGroup: "Recovery", HelpOrder: 301, HelpCategory: "Tasks", OperationID: OperationCoreRedo, Keys: []string{"ctrl+r"}, DisplayKey: "Ctrl+R"},
			{OperationID: OperationCoreBack, Keys: []string{"esc"}, DisplayKey: "Esc"},
			// Help closes the leading six. It is last of them because it is the rung
			// a user reaches when the other five did not answer, and it is among
			// them because a footer that sheds `? help` first leaves the narrowest
			// terminal with no statement that the rest of the grammar exists.
			{OperationID: OperationCoreHelp, Keys: []string{"?"}, DisplayKey: "?", FooterRank: 6, FooterLabel: "help"},
			// q is the key a terminal program is expected to quit on, and nothing
			// on the Outliner reads it as a letter: text inputs, pending chords,
			// and gated sub-states consume it before the root grammar sees it.
			// Nothing is lost by leaving, since every change is already committed.
			{OperationID: OperationCoreQuit, Keys: []string{"q", "ctrl+c"}, DisplayKey: "q/Ctrl+C"},
			{OperationID: OperationCoreNextWorkMode, Keys: []string{"tab"}, DisplayKey: "Tab"},
			{OperationID: OperationCorePreviousWorkMode, Keys: []string{"shift+tab"}, DisplayKey: "Shift+Tab"},
			// The date axis opens the second tier of acts, the ones that change the
			// row rather than reach it, and it is the first thing a terminal wider
			// than the floor buys back.
			{OperationID: OperationTaskSetDueDate, Keys: []string{"@"}, DisplayKey: "@", FooterRank: 7, FooterLabel: "date"},
			// - and + step the deadline @ sets, one calendar day at a time, along the
			// line earlier ↔ yesterday ↔ unset ↔ today ↔ tomorrow ↔ later. They are
			// spelled as the characters they produce and nothing else: a terminal
			// reporting shift+= for + still delivers "+" as the key's text, so a
			// second shifted spelling would be dead vocabulary that also shadows the
			// = review key. Neither carries a FooterRank — @ already spends rank 7
			// on the date axis, and the generated ? help and menu bar name both keys
			// regardless of the footer ladder.
			{OperationID: OperationTaskStepDueEarlier, Keys: []string{"-"}, DisplayKey: "-"},
			{OperationID: OperationTaskStepDueLater, Keys: []string{"+"}, DisplayKey: "+"},
			// # tags whatever the cursor names. The two operations carry different
			// targets and different vocabularies, so they stay two descriptors with
			// two honest labels rather than one that branches on runtime context.
			{OperationID: OperationTaskSetTags, Keys: []string{"#"}, DisplayKey: "#", FooterRank: 9, FooterLabel: "tags/context", SharedKey: true},
			{OperationID: OperationProjectSetTags, Keys: []string{"#"}, DisplayKey: "#", FooterRank: 9, FooterLabel: "project tags", SharedKey: true},
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
			//
			// The ten terminals are declared as two runs because they are two
			// kinds of act: `wn` and `wt` commit a task to a planning collection
			// and leave it in Backlog, while the other eight set the lifecycle
			// state outright. Declaration order is the only thing that changed —
			// every sequence, operation, and preset is the one it always was, so
			// the division is what the which-key row reads and never what the
			// keypress does. Plan leads because two entries at the front of the
			// run are two the narrow row never sheds.
			{HelpGroup: "Plan", HelpOrder: 101, OperationID: OperationCoreNext, Sequence: []string{"w", "n"}, DisplayKey: "wn", HintLabel: "next", HintGroup: chrome.PlanHintGroup},
			{HelpGroup: "Plan", HelpOrder: 100, OperationID: OperationCoreToday, Sequence: []string{"w", "t"}, DisplayKey: "wt", HintLabel: "today", HintGroup: chrome.PlanHintGroup},
			{HelpGroup: "Other statuses", HelpOrder: 350, OperationID: OperationTaskSetState, Sequence: []string{"w", "i"}, DisplayKey: "wi/g/w/p/m/d/c", HintLabel: "inbox", HintGroup: chrome.StatusHintGroup, HelpLabel: "Move to Inbox", PresetValue: models.StateInbox, HasPresetValue: true},
			{HelpGroup: "Other statuses", HelpOrder: 356, OperationID: OperationCoreBacklog, Sequence: []string{"w", "b"}, DisplayKey: "wb", HintLabel: "backlog", HintGroup: chrome.StatusHintGroup},
			{HelpGroup: "Status", HelpOrder: 200, OperationID: OperationTaskSetState, Sequence: []string{"w", "g"}, DisplayKey: "wi/g/w/p/m/d/c", HintLabel: "doing", HintGroup: chrome.StatusHintGroup, HelpLabel: "Start Task", PresetValue: models.StateDoing, HasPresetValue: true},
			{HelpGroup: "Other statuses", HelpOrder: 351, OperationID: OperationTaskSetState, Sequence: []string{"w", "w"}, DisplayKey: "wi/g/w/p/m/d/c", HintLabel: "waiting", HintGroup: chrome.StatusHintGroup, HelpLabel: "Wait on Task", PresetValue: models.StateWaiting, HasPresetValue: true},
			{HelpGroup: "Other statuses", HelpOrder: 352, OperationID: OperationTaskSetState, Sequence: []string{"w", "p"}, DisplayKey: "wi/g/w/p/m/d/c", HintLabel: "paused", HintGroup: chrome.StatusHintGroup, HelpLabel: "Pause Task", PresetValue: models.StatePaused, HasPresetValue: true},
			{HelpGroup: "Other statuses", HelpOrder: 353, OperationID: OperationTaskSetState, Sequence: []string{"w", "m"}, DisplayKey: "wi/g/w/p/m/d/c", HintLabel: "someday", HintGroup: chrome.StatusHintGroup, HelpLabel: "Save for Someday", PresetValue: models.StateSomeday, HasPresetValue: true},
			{HelpGroup: "Status", HelpOrder: 201, OperationID: OperationTaskSetState, Sequence: []string{"w", "d"}, DisplayKey: "wi/g/w/p/m/d/c", HintLabel: "done", HintGroup: chrome.StatusHintGroup, HintKeep: true, HelpLabel: "Complete Task", PresetValue: models.StateDone, HasPresetValue: true},
			{HelpGroup: "Other statuses", HelpOrder: 354, OperationID: OperationTaskSetState, Sequence: []string{"w", "c"}, DisplayKey: "wi/g/w/p/m/d/c", HintLabel: "canceled", HintGroup: chrome.StatusHintGroup, HelpLabel: "Cancel Task", PresetValue: models.StateCanceled, HasPresetValue: true},
			// Hidden is not a status, but moving a task out of every view is a
			// workflow act and the chord has two headings and no third word, so it
			// closes the Status run as the entry the narrow row sheds first.
			{HelpGroup: "Other statuses", HelpOrder: 357, OperationID: OperationTaskToggleHidden, Sequence: []string{"w", "h"}, DisplayKey: "wh", HintLabel: "hide/unhide", HintGroup: chrome.StatusHintGroup, HelpLabel: "Hide / unhide task"},
			// The f chord's facets are declared as two runs, the way the workflow
			// chord's terminals are: the fields a task stores, then the facts
			// computed from it. Declaration order is all the division changed, so
			// every sequence, operation, and preset is the one it always was. The
			// four f! bindings carry no group of their own; their prefix's chord
			// group does, which is what seats `!` in the Field run.
			{OperationID: OperationFilterPriority, Sequence: []string{"f", "!", "0"}, DisplayKey: "f!0-3", HintLabel: "clear", PresetValue: 0, HasPresetValue: true},
			{OperationID: OperationFilterPriority, Sequence: []string{"f", "!", "1"}, DisplayKey: "f!0-3", HintLabel: "high", PresetValue: 1, HasPresetValue: true},
			{OperationID: OperationFilterPriority, Sequence: []string{"f", "!", "2"}, DisplayKey: "f!0-3", HintLabel: "medium+", PresetValue: 2, HasPresetValue: true},
			{OperationID: OperationFilterPriority, Sequence: []string{"f", "!", "3"}, DisplayKey: "f!0-3", HintLabel: "low+", PresetValue: 3, HasPresetValue: true},
			{OperationID: OperationFilterDue, Sequence: []string{"f", "@"}, DisplayKey: "f@", HintLabel: "due/deferred", HintGroup: chrome.FilterFieldHintGroup},
			// The states facet is the stated exception to spelling a second key
			// like the sigil it narrows: the workflow chord above took the letter
			// w, and a facet over the same axis has to answer to the same key or
			// the two halves stop reading as one family. f$ is retired outright.
			{OperationID: OperationFilterStates, Sequence: []string{"f", "w"}, DisplayKey: "fw", HintLabel: "states", HintGroup: chrome.FilterFieldHintGroup},
			{OperationID: OperationFilterTags, Sequence: []string{"f", "#"}, DisplayKey: "f#", HintLabel: "tags", HintGroup: chrome.FilterFieldHintGroup},
			// Context is declared immediately after tags so the which-key row
			// reads tags then contexts, which is the order the two sigils teach.
			{OperationID: OperationFilterContexts, Sequence: []string{"f", "c"}, DisplayKey: "fc", HintLabel: "contexts", HintGroup: chrome.FilterFieldHintGroup},
			{OperationID: OperationFilterReview, Sequence: []string{"f", "="}, DisplayKey: "f=", HintLabel: "review", HintGroup: chrome.FilterFieldHintGroup},
			// Pinned answers to its own sigil like every other sigil-carrying facet:
			// *pin is what the clause spells, so f* is what narrows by it. The letter
			// fp stays retired, because it used to mean priority and a letter still
			// bound would silently apply the wrong facet.
			{OperationID: OperationFilterPinned, Sequence: []string{"f", "*"}, DisplayKey: "f*", HintLabel: "pinned", HintGroup: chrome.FilterFieldHintGroup},
			// Stale leads and completion age closes the run because the row sheds
			// from its tail: at the minimum width the short entry is the one that
			// still fits beside the heading, and the completion window, which only
			// a terminal population reads, is the one given up.
			{OperationID: OperationFilterStale, Sequence: []string{"f", "s"}, DisplayKey: "fs", HintLabel: "stale", HintGroup: chrome.FilterDerivedHintGroup},
			{OperationID: OperationFilterHierarchy, Sequence: []string{"f", "h"}, DisplayKey: "fh", HintLabel: "hierarchy", HintGroup: chrome.FilterDerivedHintGroup},
			// A facet carrying a sigil answers to that sigil, so the reference axis
			// is f~ and no letter is bound to it.
			{OperationID: OperationFilterDependency, Sequence: []string{"f", "~"}, DisplayKey: "f~", HintLabel: "references", HintGroup: chrome.FilterDerivedHintGroup},
			{OperationID: OperationFilterCompletionAge, Sequence: []string{"f", "a"}, DisplayKey: "fa", HintLabel: "completion age", HintGroup: chrome.FilterDerivedHintGroup},
			// Clear all and the project facet close the row outside both divisions.
			// Clear all is kept: it is the one way back from a facet a narrow row
			// has stopped naming, so it outlasts every facet the row sheds.
			{OperationID: OperationFilterClear, Sequence: []string{"f", "x"}, DisplayKey: "fx", HintLabel: "clear all", HintKeep: true},
			// The project facet answers to its sigil, %project, never to P, which
			// is the single-project scope rather than a facet. It is declared after
			// fx so a narrow which-key row sheds it before clear all.
			{OperationID: OperationFilterProjects, Sequence: []string{"f", "%"}, DisplayKey: "f%", HintLabel: "projects"},
			// Hidden narrows to hidden tasks only (+hidden:only), and H because fh is
			// the hierarchy facet. It would close the Field run, but there it costs
			// the floor f~, so it is declared last with no group: at the minimum
			// width it is the first entry the row sheds and every other entry is
			// drawn exactly as before. The empty state and the hide toast name it.
			{OperationID: OperationFilterHidden, Sequence: []string{"f", "H"}, DisplayKey: "fH", HintLabel: "hidden"},
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
			{OperationID: OperationCoreGotoAll, Keys: []string{"1"}, Sequence: []string{"g", "a"}, DisplayKey: "ga/1", HintLabel: "all", HintGroup: chrome.GotoModeHintGroup},
			{OperationID: OperationCoreGotoInbox, Keys: []string{"3"}, Sequence: []string{"g", "i"}, DisplayKey: "gi/3", HintLabel: "inbox", HintGroup: chrome.GotoModeHintGroup},
			{OperationID: OperationCoreGotoToday, Keys: []string{"2"}, Sequence: []string{"g", "t"}, DisplayKey: "gt/2", HintLabel: "today", HintGroup: chrome.GotoModeHintGroup},
			{OperationID: OperationCoreGotoNext, Keys: []string{"4"}, Sequence: []string{"g", "n"}, DisplayKey: "gn/4", HintLabel: "next", HintGroup: chrome.GotoModeHintGroup},
			{OperationID: OperationCoreGotoBacklog, Keys: []string{"5"}, Sequence: []string{"g", "b"}, DisplayKey: "gb/5", HintLabel: "backlog", HintGroup: chrome.GotoModeHintGroup},
			{OperationID: OperationCoreGotoDue, Keys: []string{"6"}, Sequence: []string{"g", "@"}, DisplayKey: "g@/6", HintLabel: "due", HintGroup: chrome.GotoModeHintGroup},
			{OperationID: OperationCoreGotoRecent, Keys: []string{"7"}, Sequence: []string{"g", "r"}, DisplayKey: "gr/7", HintLabel: "recent", HintGroup: chrome.GotoModeHintGroup},
			// The go-to chord reads as three runs: the modes, the places the
			// cursor's own task names, and the workspace's sync state. The last is
			// the pair a task row sheds at the minimum width, since neither of its
			// entries depends on where the cursor is.
			{OperationID: OperationCoreGotoTaskProject, Sequence: []string{"g", "p"}, DisplayKey: "gp", HintLabel: "task project", HintGroup: chrome.GotoTaskHintGroup},
			{OperationID: OperationCoreGotoParentTask, Sequence: []string{"g", "P"}, DisplayKey: "gP", HintLabel: "parent task", HintGroup: chrome.GotoTaskHintGroup},
			// `~` is the reference sigil everywhere else, so the relationship walk
			// spells itself the same way under the go-to chord.
			{OperationID: OperationCoreGotoRelation, Sequence: []string{"g", "~"}, DisplayKey: "g~", HintLabel: "related task", HintGroup: chrome.GotoTaskHintGroup},
			{OperationID: OperationCoreSyncNow, Sequence: []string{"g", "s"}, DisplayKey: "gs", HintLabel: "sync now", HintGroup: chrome.GotoSyncHintGroup},
			// Conflicts joins the go-to chord rather than claiming a root key: what
			// the strip already names as workspace state is something the user goes
			// to look at, and root `c` appends a checklist item.
			{OperationID: OperationCoreConflicts, Sequence: []string{"g", "c"}, DisplayKey: "gc", HintLabel: "conflicts", HintGroup: chrome.GotoSyncHintGroup},
			{OperationID: OperationCoreScrollCursorTop, Sequence: []string{"z", "t"}, DisplayKey: "zt", HintLabel: "row to top"},
			{OperationID: OperationCoreScrollCursorMiddle, Sequence: []string{"z", "z"}, DisplayKey: "zz", HintLabel: "row to middle"},
			{OperationID: OperationCoreScrollCursorBottom, Sequence: []string{"z", "b"}, DisplayKey: "zb", HintLabel: "row to bottom"},
		},
		ChordGroups: []OperationChordGroup{
			{Sequence: []string{"!"}, HintLabel: "priority", FooterRank: 8, FooterLabel: "priority"},
			// Workflow is the one chord among the leading six: moving work between
			// plans and states is what the daily loop is, and the chord's own
			// which-key row is what teaches the ten terminals once it is pressed.
			{Sequence: []string{"w"}, HintLabel: "workflow", FooterRank: 2, FooterLabel: "workflow"},
			{Sequence: []string{"f"}, HintLabel: "filter", FooterRank: 19, FooterLabel: "filter"},
			{Sequence: []string{"f", "!"}, HintLabel: "priority threshold", HintGroup: chrome.FilterFieldHintGroup},
			{Sequence: []string{"F"}, HintLabel: "project filter", FooterRank: 25, FooterLabel: "project filter"},
			{Sequence: []string{"o"}, HintLabel: "sort", FooterRank: 23, FooterLabel: "sort"},
			{Sequence: []string{"g"}, HintLabel: "go to", FooterRank: 26, FooterLabel: "go to"},
			{Sequence: []string{"z"}, HintLabel: "scroll"},
		},
	}
}
