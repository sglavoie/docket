package tui

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/sglavoie/tt/internal/models"
)

// InputStrategyID identifies a complete top-level key grammar.
// InputStrategySimplified is the production daily-loop grammar and is built
// from OperationRegistry identities. The four surface grammars beside it name
// the other entity surfaces that dispatch through the same registry.
type InputStrategyID string

const (
	InputStrategySimplified  InputStrategyID = "simplified"
	InputStrategyProjectHub  InputStrategyID = "project-hub"
	InputStrategyNotes       InputStrategyID = "notes"
	InputStrategyAttachments InputStrategyID = "attachments"
	InputStrategyComments    InputStrategyID = "comments"
)

// OperationKeyBinding assigns one operation any number of equivalent direct key
// events (Keys) and at most one multi-key chord path (Sequence). Every element
// is a physical Bubble Tea key name; a direct key is a length-1 sequence, so
// both spellings resolve through one dispatch path, and a binding may declare
// both when an operation answers to a direct key and a chord alike. DisplayKey is
// the compact help spelling. FooterRank zero excludes an operation from compact
// footer hints. HintLabel names the binding in the which-key row shown while its
// chord prefix is pending. A preset value supplies the operation's value without
// opening a picker and is the only reason one operation may own several
// bindings.
//
// SharedKey admits the binding onto a key path another operation already owns.
// Every binding on that path must declare it, and resolution picks the first
// applicable one in declaration order, so the operations sharing a key must be
// applicable over disjoint cursors. Without the flag a repeated path stays a
// grammar defect.
type OperationKeyBinding struct {
	OperationID    OperationID
	Keys           []string
	Sequence       []string
	DisplayKey     string
	FooterRank     int
	FooterLabel    string
	HintLabel      string
	PresetValue    any
	HasPresetValue bool
	SharedKey      bool
}

// OperationChordGroup labels one non-terminal chord prefix. Every prefix in a
// strategy needs a label so a pending chord always has a which-key row to show;
// a ranked group also appears among the compact footer hints.
type OperationChordGroup struct {
	Sequence    []string
	HintLabel   string
	FooterRank  int
	FooterLabel string
}

// InputStrategy is one collision-free, prefix-free operation key grammar.
// Bindings and their chord group labels are the single declaration source for
// dispatch, help, footer hints, and which-key rows.
type InputStrategy struct {
	ID          InputStrategyID
	Bindings    []OperationKeyBinding
	ChordGroups []OperationChordGroup
}

// ChordResolution is the outcome of resolving one key path against a strategy.
type ChordResolution int

const (
	// ChordNoMatch means the path leads nowhere applicable and must be abandoned.
	ChordNoMatch ChordResolution = iota
	// ChordPending means the path is a prefix waiting for its next key.
	ChordPending
	// ChordTerminal means the path resolves to one applicable binding.
	ChordTerminal
)

// paths returns every key path that resolves this binding. Direct keys are
// length-1 paths, so a chord and a direct key are the same kind of value, and a
// binding may declare both when one operation genuinely answers to a direct key
// and a chord alike.
func (binding OperationKeyBinding) paths() [][]string {
	paths := make([][]string, 0, len(binding.Keys)+1)
	for _, key := range binding.Keys {
		paths = append(paths, []string{key})
	}
	if len(binding.Sequence) > 0 {
		paths = append(paths, binding.Sequence)
	}
	return paths
}

// inputChordNode is one node of a strategy's key trie. bindings is empty on
// prefix nodes; childOrder keeps which-key hints in declaration order. A
// terminal normally holds exactly one binding, and holds several only when every
// one of them opted into SharedKey.
type inputChordNode struct {
	children   map[string]*inputChordNode
	childOrder []string
	bindings   []int
	owners     []OperationID
	shared     bool
}

func newInputChordNode() *inputChordNode {
	return &inputChordNode{children: make(map[string]*inputChordNode)}
}

func (node *inputChordNode) child(key string) *inputChordNode {
	if node == nil {
		return nil
	}
	return node.children[key]
}

func (node *inputChordNode) terminal() bool {
	return node != nil && len(node.bindings) > 0
}

func (node *inputChordNode) ownerList() string {
	names := make([]string, 0, len(node.owners))
	for _, owner := range node.owners {
		names = append(names, string(owner))
	}
	return strings.Join(names, ", ")
}

// insert adds one prefix-free path. A path that passes through or stops on an
// existing terminal, or that ends on a node another chord continues past, is a
// grammar defect rather than a runtime ambiguity to resolve later.
func (node *inputChordNode) insert(
	strategyID InputStrategyID,
	operationID OperationID,
	path []string,
	bindingIndex int,
	shared bool,
) error {
	current := node
	for _, key := range path {
		if current.terminal() {
			return fmt.Errorf(
				"input strategy %s chord prefix collision: %s already ends a binding inside %s (%s vs %s)",
				strategyID,
				chordPathString(path[:len(path)-1]),
				chordPathString(path),
				current.ownerList(),
				operationID,
			)
		}
		next := current.children[key]
		if next == nil {
			next = newInputChordNode()
			current.children[key] = next
			current.childOrder = append(current.childOrder, key)
		}
		current = next
	}
	if current.terminal() {
		if !shared || !current.shared || slices.Contains(current.owners, operationID) {
			return fmt.Errorf(
				"input strategy %s key collision: %s (%s vs %s)",
				strategyID,
				chordPathString(path),
				current.ownerList(),
				operationID,
			)
		}
	}
	if len(current.children) > 0 {
		return fmt.Errorf(
			"input strategy %s chord prefix collision: %s is a prefix of a longer chord (%s vs %s)",
			strategyID,
			chordPathString(path),
			current.ownerList(),
			operationID,
		)
	}
	current.bindings = append(current.bindings, bindingIndex)
	current.owners = append(current.owners, operationID)
	current.shared = shared
	return nil
}

// prefixPaths returns every non-root prefix path in declaration order.
func (node *inputChordNode) prefixPaths(prefix []string) [][]string {
	if node == nil {
		return nil
	}
	paths := make([][]string, 0)
	for _, key := range node.childOrder {
		child := node.children[key]
		if child.terminal() {
			continue
		}
		path := append(append([]string(nil), prefix...), key)
		paths = append(paths, path)
		paths = append(paths, child.prefixPaths(path)...)
	}
	return paths
}

func chordPathString(path []string) string {
	if len(path) == 0 {
		return "(root)"
	}
	return strings.Join(path, " ")
}

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
			{OperationID: operationCoreCursorScreenTop, Keys: []string{"H"}, DisplayKey: "H"},
			{OperationID: operationCoreCursorScreenMiddle, Keys: []string{"M"}, DisplayKey: "M"},
			{OperationID: operationCoreCursorScreenBottom, Keys: []string{"L"}, DisplayKey: "L"},
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
			{OperationID: operationCoreNotes, Keys: []string{"ctrl+n"}, DisplayKey: "Ctrl+N"},
			{OperationID: operationCoreToggleTheme, Keys: []string{"T"}, DisplayKey: "T", FooterRank: 22, FooterLabel: "theme"},
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
			{OperationID: operationFilterReview, Sequence: []string{"f", "r"}, DisplayKey: "fr", HintLabel: "review"},
			{OperationID: operationFilterHierarchy, Sequence: []string{"f", "h"}, DisplayKey: "fh", HintLabel: "hierarchy"},
			{OperationID: operationFilterStale, Sequence: []string{"f", "s"}, DisplayKey: "fs", HintLabel: "stale"},
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
			{OperationID: operationCoreGotoAll, Sequence: []string{"g", "a"}, DisplayKey: "ga", HintLabel: "all"},
			{OperationID: operationCoreGotoInbox, Sequence: []string{"g", "i"}, DisplayKey: "gi", HintLabel: "inbox"},
			{OperationID: operationCoreGotoToday, Sequence: []string{"g", "t"}, DisplayKey: "gt", HintLabel: "today"},
			{OperationID: operationCoreGotoNext, Sequence: []string{"g", "n"}, DisplayKey: "gn", HintLabel: "next"},
			{OperationID: operationCoreGotoBacklog, Sequence: []string{"g", "b"}, DisplayKey: "gb", HintLabel: "backlog"},
			{OperationID: operationCoreGotoDue, Sequence: []string{"g", "@"}, DisplayKey: "g@", HintLabel: "due"},
			{OperationID: operationCoreGotoRecent, Sequence: []string{"g", "r"}, DisplayKey: "gr", HintLabel: "recent"},
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

// RegisterInputStrategy validates and stores a strategy. Descriptor existence,
// direct-event syntax, preset value types, and prefix-freeness across every key
// path are checked together so a registered strategy cannot contain an ambiguous
// or unlabeled prefix dependency.
func (registry *OperationRegistry) RegisterInputStrategy(strategy InputStrategy) error {
	if registry == nil {
		return fmt.Errorf("operation registry is required")
	}
	strategy.ID = InputStrategyID(strings.TrimSpace(string(strategy.ID)))
	if strategy.ID == "" {
		return fmt.Errorf("input strategy ID is required")
	}
	if registry.inputStrategies == nil {
		registry.inputStrategies = make(map[InputStrategyID]InputStrategy)
	}
	if _, exists := registry.inputStrategies[strategy.ID]; exists {
		return fmt.Errorf("input strategy already registered: %s", strategy.ID)
	}

	// Only preset-valued bindings may share an operation: p1 through p5 are five
	// spellings of one set-priority operation, while any other repeat is a
	// duplicate grammar entry.
	presetOnlyOperations := make(map[OperationID]bool)
	trie := newInputChordNode()
	normalized := InputStrategy{ID: strategy.ID, Bindings: make([]OperationKeyBinding, 0, len(strategy.Bindings))}
	for _, binding := range strategy.Bindings {
		binding.OperationID = OperationID(strings.TrimSpace(string(binding.OperationID)))
		if binding.OperationID == "" {
			return fmt.Errorf("input strategy %s has a binding without an operation ID", strategy.ID)
		}
		descriptor, ok := registry.Descriptor(binding.OperationID)
		if !ok {
			return fmt.Errorf("input strategy %s references unknown operation %s", strategy.ID, binding.OperationID)
		}
		if len(binding.Keys) == 0 && len(binding.Sequence) == 0 {
			return fmt.Errorf("input strategy %s operation %s has no keys", strategy.ID, binding.OperationID)
		}
		if len(binding.Sequence) == 1 {
			return fmt.Errorf(
				"input strategy %s operation %s declares a one-key chord sequence; use Keys instead",
				strategy.ID,
				binding.OperationID,
			)
		}
		if binding.HasPresetValue {
			if descriptor.ValueProviderID == "" {
				return fmt.Errorf(
					"input strategy %s operation %s presets a value but takes none",
					strategy.ID,
					binding.OperationID,
				)
			}
			if !descriptor.ValueProviderID.acceptsPresetValue(binding.PresetValue) {
				return fmt.Errorf(
					"input strategy %s operation %s presets %T, which its %s value provider does not accept",
					strategy.ID,
					binding.OperationID,
					binding.PresetValue,
					descriptor.ValueProviderID,
				)
			}
		}
		if firstWasPreset, exists := presetOnlyOperations[binding.OperationID]; exists {
			if !firstWasPreset || !binding.HasPresetValue {
				return fmt.Errorf(
					"input strategy %s binds operation %s more than once",
					strategy.ID,
					binding.OperationID,
				)
			}
		}
		presetOnlyOperations[binding.OperationID] = binding.HasPresetValue
		binding.DisplayKey = strings.TrimSpace(binding.DisplayKey)
		binding.FooterLabel = strings.TrimSpace(binding.FooterLabel)
		binding.HintLabel = strings.TrimSpace(binding.HintLabel)

		owner := "operation " + string(binding.OperationID)
		normalizedKeys, err := normalizeChordPath(strategy.ID, owner, binding.Keys)
		if err != nil {
			return err
		}
		normalizedSequence, err := normalizeChordPath(strategy.ID, owner, binding.Sequence)
		if err != nil {
			return err
		}
		binding.Keys = normalizedKeys
		binding.Sequence = normalizedSequence
		if binding.DisplayKey == "" {
			binding.DisplayKey = strings.Join(binding.Keys, "/") + strings.Join(binding.Sequence, "")
		}

		index := len(normalized.Bindings)
		for _, path := range binding.paths() {
			if err := trie.insert(strategy.ID, binding.OperationID, path, index, binding.SharedKey); err != nil {
				return err
			}
		}
		normalized.Bindings = append(normalized.Bindings, cloneOperationKeyBinding(binding))
	}

	groups, err := normalizeChordGroups(strategy, trie)
	if err != nil {
		return err
	}
	normalized.ChordGroups = groups

	if registry.inputChords == nil {
		registry.inputChords = make(map[InputStrategyID]*inputChordNode)
	}
	registry.inputStrategies[strategy.ID] = normalized
	registry.inputChords[strategy.ID] = trie
	return nil
}

func normalizeChordPath(strategyID InputStrategyID, owner string, path []string) ([]string, error) {
	if len(path) == 0 {
		return nil, nil
	}
	normalized := make([]string, 0, len(path))
	for _, raw := range path {
		key := normalizeOperationKey(raw)
		if !isDirectInputKey(key) {
			return nil, fmt.Errorf(
				"input strategy %s %s key %q is not one direct key event",
				strategyID,
				owner,
				raw,
			)
		}
		normalized = append(normalized, key)
	}
	return normalized, nil
}

// normalizeChordGroups pairs every prefix in the trie with exactly one declared
// label, so a pending chord can never render a blank which-key row.
func normalizeChordGroups(strategy InputStrategy, trie *inputChordNode) ([]OperationChordGroup, error) {
	declared := make(map[string]OperationChordGroup, len(strategy.ChordGroups))
	for _, group := range strategy.ChordGroups {
		sequence, err := normalizeChordPath(strategy.ID, "chord group", group.Sequence)
		if err != nil {
			return nil, err
		}
		if len(sequence) == 0 {
			return nil, fmt.Errorf("input strategy %s has a chord group without a sequence", strategy.ID)
		}
		group.Sequence = sequence
		group.HintLabel = strings.TrimSpace(group.HintLabel)
		group.FooterLabel = strings.TrimSpace(group.FooterLabel)
		if group.HintLabel == "" {
			return nil, fmt.Errorf(
				"input strategy %s chord group %s has no hint label",
				strategy.ID,
				chordPathString(sequence),
			)
		}
		key := chordPathString(sequence)
		if _, exists := declared[key]; exists {
			return nil, fmt.Errorf("input strategy %s labels chord group %s more than once", strategy.ID, key)
		}
		declared[key] = group
	}

	prefixes := trie.prefixPaths(nil)
	groups := make([]OperationChordGroup, 0, len(prefixes))
	for _, prefix := range prefixes {
		key := chordPathString(prefix)
		group, exists := declared[key]
		if !exists {
			return nil, fmt.Errorf("input strategy %s chord prefix %s has no group label", strategy.ID, key)
		}
		delete(declared, key)
		groups = append(groups, cloneOperationChordGroup(group))
	}
	for key := range declared {
		return nil, fmt.Errorf("input strategy %s chord group %s does not label a chord prefix", strategy.ID, key)
	}
	return groups, nil
}

// InputStrategy returns a defensive copy of a registered strategy.
func (registry *OperationRegistry) InputStrategy(id InputStrategyID) (InputStrategy, bool) {
	if registry == nil {
		return InputStrategy{}, false
	}
	strategy, ok := registry.inputStrategies[id]
	if !ok {
		return InputStrategy{}, false
	}
	return cloneInputStrategy(strategy), true
}

// InputBindingForSequence resolves one key path through the selected strategy
// and active operation context. A direct key is a length-1 path, so this is the
// only resolution seam: it reports whether the path ends a binding, waits for
// more keys, or leads nowhere applicable.
func (registry *OperationRegistry) InputBindingForSequence(
	id InputStrategyID,
	sequence []string,
	ctx CommandContext,
) (OperationKeyBinding, OperationDescriptor, ChordResolution) {
	if registry == nil || len(sequence) == 0 {
		return OperationKeyBinding{}, OperationDescriptor{}, ChordNoMatch
	}
	strategy, ok := registry.inputStrategies[id]
	if !ok {
		return OperationKeyBinding{}, OperationDescriptor{}, ChordNoMatch
	}
	node := registry.inputChords[id]
	for _, raw := range sequence {
		node = node.child(normalizeOperationKey(raw))
		if node == nil {
			return OperationKeyBinding{}, OperationDescriptor{}, ChordNoMatch
		}
	}
	if node.terminal() {
		// A shared key holds several bindings; the first applicable one in
		// declaration order wins, so the cursor decides which operation a key means.
		for _, index := range node.bindings {
			binding := strategy.Bindings[index]
			descriptor, exists := registry.Descriptor(binding.OperationID)
			if exists && descriptor.Applicable(ctx) {
				return cloneOperationKeyBinding(binding), descriptor, ChordTerminal
			}
		}
		return OperationKeyBinding{}, OperationDescriptor{}, ChordNoMatch
	}
	// A prefix whose whole subtree is inapplicable would trap input in a chord
	// that can never complete, so it fails closed like an unbound key.
	if !registry.chordSubtreeIsApplicable(strategy, node, ctx) {
		return OperationKeyBinding{}, OperationDescriptor{}, ChordNoMatch
	}
	return OperationKeyBinding{}, OperationDescriptor{}, ChordPending
}

// InputPrefixRefusedOnChecklistRow reports whether a key path reaches any
// operation the checklist rule turns off. It answers the question a category
// cannot when the path's operations disagree, which is what `#` does: one of
// its two meanings needs the owner task and the other needs a project header,
// so neither category alone explains why the key did nothing.
func (registry *OperationRegistry) InputPrefixRefusedOnChecklistRow(
	id InputStrategyID,
	sequence []string,
) bool {
	strategy, ok := registry.InputStrategy(id)
	if !ok {
		return false
	}
	var walk func(node *inputChordNode) bool
	walk = func(node *inputChordNode) bool {
		for _, index := range node.bindings {
			descriptor, exists := registry.Descriptor(strategy.Bindings[index].OperationID)
			if exists && operationRefusedOnChecklistRow(descriptor) {
				return true
			}
		}
		for _, key := range node.childOrder {
			if walk(node.children[key]) {
				return true
			}
		}
		return false
	}
	node := registry.chordNodeForPrefix(id, sequence)
	return node != nil && walk(node)
}

// InputCategoryForPrefix returns the target-rejection category every operation
// still reachable from a key path shares, whether or not any of them is currently
// applicable. It is how a refused key still names what it needed, including a
// navigation operation that targets a task and a chord prefix abandoned because
// its whole subtree turned off. A path that ends nowhere, or whose reachable
// operations disagree, reports false and stays silent rather than guessing at a
// sentence.
func (registry *OperationRegistry) InputCategoryForPrefix(
	id InputStrategyID,
	sequence []string,
) (OperationCategory, bool) {
	strategy, ok := registry.InputStrategy(id)
	if !ok {
		return "", false
	}
	category, resolved := OperationCategory(""), false
	var walk func(node *inputChordNode) bool
	walk = func(node *inputChordNode) bool {
		for _, index := range node.bindings {
			descriptor, exists := registry.Descriptor(strategy.Bindings[index].OperationID)
			if !exists {
				return false
			}
			rejectionCategory := operationTargetRejectionCategory(descriptor)
			if resolved && rejectionCategory != category {
				return false
			}
			category, resolved = rejectionCategory, true
		}
		for _, key := range node.childOrder {
			if !walk(node.children[key]) {
				return false
			}
		}
		return true
	}
	node := registry.chordNodeForPrefix(id, sequence)
	if node == nil || !walk(node) {
		return "", false
	}
	return category, resolved
}

// ChordGroupLabelForPrefix returns the declared label of the longest chord
// prefix a key path passes through. It is how an abandoned chord names the path
// it was on, which a resolution failure alone cannot say.
func (registry *OperationRegistry) ChordGroupLabelForPrefix(
	id InputStrategyID,
	sequence []string,
) (string, bool) {
	strategy, ok := registry.InputStrategy(id)
	if !ok || len(sequence) < 2 {
		return "", false
	}
	keys := make([]string, 0, len(sequence)-1)
	for _, key := range sequence[:len(sequence)-1] {
		keys = append(keys, normalizeOperationKey(key))
	}
	prefix := chordPathString(keys)
	for _, group := range strategy.ChordGroups {
		if chordPathString(group.Sequence) == prefix {
			return group.HintLabel, true
		}
	}
	return "", false
}

// InputBindingForKey resolves one direct key through the selected strategy and
// active operation context.
func (registry *OperationRegistry) InputBindingForKey(
	id InputStrategyID,
	key string,
	ctx CommandContext,
) (OperationKeyBinding, OperationDescriptor, bool) {
	binding, descriptor, resolution := registry.InputBindingForSequence(id, []string{key}, ctx)
	return binding, descriptor, resolution == ChordTerminal
}

func (registry *OperationRegistry) chordSubtreeIsApplicable(
	strategy InputStrategy,
	node *inputChordNode,
	ctx CommandContext,
) bool {
	if node == nil {
		return false
	}
	if node.terminal() {
		for _, index := range node.bindings {
			descriptor, exists := registry.Descriptor(strategy.Bindings[index].OperationID)
			if exists && descriptor.Applicable(ctx) {
				return true
			}
		}
		return false
	}
	for _, key := range node.childOrder {
		if registry.chordSubtreeIsApplicable(strategy, node.children[key], ctx) {
			return true
		}
	}
	return false
}

func isDirectInputKey(key string) bool {
	if key == "" || strings.ContainsAny(key, " \t\r\n") {
		return false
	}
	if utf8.RuneCountInString(key) == 1 {
		return true
	}
	switch key {
	case "up", "down", "left", "right", "enter", "space", "tab", "esc",
		"backspace", "delete", "home", "end", "pgup", "pgdown":
		return true
	}
	if !strings.Contains(key, "+") {
		return false
	}
	parts := strings.Split(key, "+")
	if len(parts) < 2 {
		return false
	}
	for _, modifier := range parts[:len(parts)-1] {
		switch modifier {
		case "ctrl", "alt", "shift", "meta", "super":
		default:
			return false
		}
	}
	return isDirectInputKey(parts[len(parts)-1])
}

func cloneOperationKeyBinding(binding OperationKeyBinding) OperationKeyBinding {
	binding.Keys = append([]string(nil), binding.Keys...)
	binding.Sequence = append([]string(nil), binding.Sequence...)
	return binding
}

func cloneOperationChordGroup(group OperationChordGroup) OperationChordGroup {
	group.Sequence = append([]string(nil), group.Sequence...)
	return group
}

func cloneInputStrategy(strategy InputStrategy) InputStrategy {
	strategy.Bindings = append([]OperationKeyBinding(nil), strategy.Bindings...)
	for i := range strategy.Bindings {
		strategy.Bindings[i] = cloneOperationKeyBinding(strategy.Bindings[i])
	}
	strategy.ChordGroups = append([]OperationChordGroup(nil), strategy.ChordGroups...)
	for i := range strategy.ChordGroups {
		strategy.ChordGroups[i] = cloneOperationChordGroup(strategy.ChordGroups[i])
	}
	return strategy
}
