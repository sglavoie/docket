package tui

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/tui/keymap"
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
// holds the seventh, and the rest navigate, explain, or close, so the table is
// the panel's complete contract. `n`, `d`, and `x` are absent because the panel
// retired them. gg is spelled as the complete path it is, the way the Hub table
// below spells it.
func assertCommentsOwnsEveryDocumentedKeyOnce(t *testing.T) {
	direct := map[string]OperationID{
		"enter": operationCommentExpand,
		"a":     operationCommentAdd,
		"e":     operationCommentEdit,
		"E":     operationCommentEditExternal,
		"space": operationCommentActions,
		"y":     operationCommentCopy,
		"Y":     operationCommentCopyAll,
		"j":     operationCoreCursorDown,
		"down":  operationCoreCursorDown,
		"k":     operationCoreCursorUp,
		"up":    operationCoreCursorUp,
		"gg":    operationCoreCursorTop,
		"home":  operationCoreCursorTop,
		"G":     operationCoreCursorBottom,
		"end":   operationCoreCursorBottom,
		"?":     operationCoreHelp,
		"esc":   operationCoreBack,
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
		"H":      operationCoreCursorScreenTop,
		"M":      operationCoreCursorScreenMiddle,
		"L":      operationCoreCursorScreenBottom,
		"ctrl+o": operationCoreJumpBackward, "ctrl+i": operationCoreJumpForward,
		"ctrl+l": operationCoreFocusLatest,
		"ctrl+p": operationCoreGotoTaskProject,
		"h":      operationCoreCollapseParent, "left": operationCoreCollapseParent,
		"l": operationCoreExpand, "right": operationCoreExpand,
		"S": operationCoreToggleSections,
		"n": operationCoreNextHeader, "N": operationCorePreviousHeader,
		"[": operationTaskOutdent, "]": operationTaskIndent,
		">":           operationTaskSetParent,
		"<":           operationTaskAdoptChildren,
		"%":           operationTaskMoveToProject,
		"shift+enter": operationCoreFocusParent,
		"ctrl+enter":  operationCorePeek,
		"/":           operationCoreScopeSearch,
		"v":           operationCoreMultiSelect, "V": operationCoreRangeSelect,
		"ctrl+a": operationCoreSelectAll,
		"p":      operationCoreSelectProject,
		"ctrl+n": operationCoreNotes, "T": operationCoreToggleTheme,
		"u": operationCoreUndo, "ctrl+r": operationCoreRedo,
		"esc": operationCoreBack, "?": operationCoreHelp, "ctrl+c": operationCoreQuit,
		"tab": operationCoreNextWorkMode, "shift+tab": operationCorePreviousWorkMode,
		"@": operationTaskSetDueDate,
		"*": operationTaskTogglePin,
		"y": operationTaskCopyTitle, "Y": operationTaskCopyDetails,
		"ctrl+y": operationTaskCopyTitleDescription,
	}

	// A shared key is one the cursor disambiguates, and each meaning keeps its own
	// descriptor so help and the footer can name both honestly. # asks whether the
	// cursor names a task or a project; the rest ask whether it names a checklist
	// item, and the checklist meaning is listed first because resolution takes the
	// first applicable binding.
	shared := map[string][]OperationID{
		"#":      {operationTaskSetTags, operationProjectSetTags},
		"e":      {operationChecklistEditItem, operationCoreEdit},
		"A":      {operationChecklistAddItem, operationTaskNewChild},
		"x":      {operationChecklistToggleItem, operationChecklistStart},
		"space":  {operationChecklistActions, operationCoreActions},
		"J":      {operationChecklistMoveDown, operationCoreReorderDown},
		"K":      {operationChecklistMoveUp, operationCoreReorderUp},
		"ctrl+j": {operationChecklistMoveBottom, operationCoreReorderBottom},
		"ctrl+k": {operationChecklistMoveTop, operationCoreReorderTop},
		"enter":  {operationCoreFocusSubtree, operationCoreZoomSection, operationCoreActivateGroup},
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
	// and d went with the defer picker when @ absorbed both date fields.
	for _, removed := range []string{
		"b", "c", "d", "g", "t", "z", ",", "m", ":", "C", "R", "ctrl+esc",
	} {
		if owners := owners[removed]; len(owners) > 0 {
			t.Errorf("removed key %q still resolves to %s", removed, owners[0])
		}
	}
}

// sigilKeyRoute is how one text-grammar trigger reaches the keyboard.
type sigilKeyRoute int

const (
	sigilRootKey sigilKeyRoute = iota
	sigilChordPrefix
	sigilPickerOnly
)

// grammarSigilRoutes pairs every advertised text-grammar trigger with the route
// the Outliner gives it. The eight keyed sigils are the correspondence itself:
// the character that writes a field in a capture line is the key that edits it
// on a row. The three picker-only triggers are named rather than omitted so a
// new sigil forces a deliberate decision instead of silently passing, and a
// renamed one breaks the keymap loudly.
var grammarSigilRoutes = map[string]sigilKeyRoute{
	"!": sigilChordPrefix,
	"#": sigilRootKey,
	"@": sigilRootKey,
	"$": sigilChordPrefix,
	"*": sigilRootKey,
	"%": sigilRootKey,
	">": sigilRootKey,
	"<": sigilRootKey,
	// Two runes are not one key event, and the # picker already unchecks a tag.
	"-#": sigilPickerOnly,
	// The trie is prefix-free, so ## and @@ cannot be chords under keys # and @
	// that already resolve. Both live as modes inside the picker their first
	// character opens: ## is the context radio, @@ the defer section.
	"##": sigilPickerOnly,
	"@@": sigilPickerOnly,
}

func TestSigilKeysMatchTextGrammarSigils(t *testing.T) {
	strategy := SimplifiedInputStrategy()
	rootKeys := make(map[string]OperationID)
	prefixes := make(map[string]OperationID)
	for _, binding := range strategy.Bindings {
		for _, key := range binding.Keys {
			rootKeys[key] = binding.OperationID
		}
		if len(binding.Sequence) > 0 {
			prefixes[binding.Sequence[0]] = binding.OperationID
		}
	}

	declared := make(map[string]bool, len(grammarSigilRoutes))
	for _, surface := range []textgrammar.Surface{
		textgrammar.SurfaceCapture,
		textgrammar.SurfaceEdit,
		textgrammar.SurfaceOutline,
		textgrammar.SurfaceSearch,
	} {
		for _, definition := range textgrammar.Definitions(surface) {
			trigger := definition.Trigger
			if declared[trigger] {
				continue
			}
			declared[trigger] = true
			route, named := grammarSigilRoutes[trigger]
			if !named {
				t.Errorf(
					"text grammar trigger %q (%s) has no declared keyboard route; bind it or name it picker-only",
					trigger,
					definition.Field,
				)
				continue
			}
			_, isRoot := rootKeys[trigger]
			_, isPrefix := prefixes[trigger]
			switch route {
			case sigilRootKey:
				if !isRoot {
					t.Errorf("sigil %q (%s) is not bound as a root key", trigger, definition.Field)
				}
				if isPrefix {
					t.Errorf("sigil %q (%s) is a root key and a chord prefix at once", trigger, definition.Field)
				}
			case sigilChordPrefix:
				if !isPrefix {
					t.Errorf("sigil %q (%s) is not a chord prefix", trigger, definition.Field)
				}
				if isRoot {
					t.Errorf("sigil %q (%s) is a chord prefix and a root key at once", trigger, definition.Field)
				}
			case sigilPickerOnly:
				if isRoot || isPrefix {
					t.Errorf(
						"sigil %q (%s) is documented as picker-only but the keymap binds it",
						trigger,
						definition.Field,
					)
				}
			}
		}
	}
	for trigger := range grammarSigilRoutes {
		if !declared[trigger] {
			t.Errorf("keyboard route names %q, which the text grammar no longer declares", trigger)
		}
	}

	// A retired sigil is recognized only to produce a migration error, so it must
	// never reach the advertised set the keymap answers to.
	for _, field := range []textgrammar.Field{textgrammar.FieldTaskType, textgrammar.FieldRecurrence} {
		if definition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, field); ok {
			t.Errorf("retired field %s is advertised as %q", field, definition.Trigger)
		}
	}
}

// TestReviewHasNoDirectKey holds the one sigil the grammar explicitly rejects.
// $review is a refused destination, so binding a review setter anywhere would
// teach a route the parser refuses to honor. Review survives as the ordinary
// review tag, its promoted All-mode lane, and the fr presentation facet.
func TestReviewHasNoDirectKey(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	if _, exists := registry.Descriptor(OperationID("tui.task.toggle-review")); exists {
		t.Error("the retired review setter is registered again")
	}
	for _, entry := range taskOperationDescriptorManifest() {
		if strings.Contains(string(entry.ID), "review") {
			t.Errorf("task operation manifest lists review setter %s", entry.ID)
		}
	}
	strategy := SimplifiedInputStrategy()
	for _, binding := range strategy.Bindings {
		if !strings.Contains(string(binding.OperationID), "review") {
			continue
		}
		if binding.OperationID != operationFilterReview {
			t.Errorf("%s is bound to %v; review has no setter key", binding.OperationID, binding.paths())
		}
	}
	binding, _, resolution := registry.InputBindingForSequence(
		InputStrategySimplified,
		[]string{"f", "r"},
		simplifiedOperationContext(),
	)
	if resolution != ChordTerminal || binding.OperationID != operationFilterReview {
		t.Fatalf("fr = %s (%v), want %s terminal", binding.OperationID, resolution, operationFilterReview)
	}
}

// TestFilterAndSortSecondKeysUseSigilsWhereOneExists keeps the second key of a
// chord spelled like the field it narrows. A filter over a sigil-carrying field
// answers to that sigil, and the mnemonic initials those chords used to take are
// gone rather than kept as aliases.
func TestFilterAndSortSecondKeysUseSigilsWhereOneExists(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	ctx := simplifiedOperationContext()
	for _, testCase := range []struct {
		sequence []string
		want     ChordResolution
	}{
		{sequence: []string{"f", "!"}, want: ChordPending},
		{sequence: []string{"f", "!", "1"}, want: ChordTerminal},
		{sequence: []string{"f", "#"}, want: ChordTerminal},
		{sequence: []string{"f", "$"}, want: ChordTerminal},
		{sequence: []string{"f", "@"}, want: ChordTerminal},
		{sequence: []string{"f", "s"}, want: ChordTerminal},
		{sequence: []string{"F", "#"}, want: ChordTerminal},
		{sequence: []string{"o", "!"}, want: ChordTerminal},
		{sequence: []string{"o", "@"}, want: ChordTerminal},
		{sequence: []string{"f", "p"}, want: ChordNoMatch},
		{sequence: []string{"f", "t"}, want: ChordNoMatch},
		{sequence: []string{"f", "d"}, want: ChordNoMatch},
		{sequence: []string{"o", "p"}, want: ChordNoMatch},
		{sequence: []string{"F", "t"}, want: ChordNoMatch},
	} {
		t.Run(strings.Join(testCase.sequence, ""), func(t *testing.T) {
			_, _, resolution := registry.InputBindingForSequence(
				InputStrategySimplified,
				testCase.sequence,
				ctx,
			)
			if resolution != testCase.want {
				t.Fatalf(
					"%s = %v, want %v",
					strings.Join(testCase.sequence, ""),
					resolution,
					testCase.want,
				)
			}
		})
	}
}

func TestNotesDirectGrammarMatchesTheWorkspaceManager(t *testing.T) {
	view := &NotesViewModel{}
	want := []string{
		"j", "k", "down", "up", "enter", "a", "e", "%", "/", "p", "space", "u", "ctrl+r", "y", "Y",
	}
	if got := view.HandledKeys(); !slices.Equal(got, want) {
		t.Fatalf("Notes direct keys = %v, want %v", got, want)
	}
	// n, v, s, r, m and x are the retired spellings the grammar must now leave
	// silent; ctrl+n, t and ctrl+o name vocabulary Notes never regained.
	for _, excluded := range []string{
		"n", "v", "s", "r", "m", "x", "ctrl+n", "t", "ctrl+o",
	} {
		if slices.Contains(view.HandledKeys(), excluded) {
			t.Errorf("Notes direct grammar exposes excluded key %q", excluded)
		}
	}
}

func TestFinalPriorityChordUsesBangPrefix(t *testing.T) {
	strategy := SimplifiedInputStrategy()
	got := make(map[string]any)
	for _, binding := range strategy.Bindings {
		if binding.OperationID != operationTaskSetPriority ||
			len(binding.Sequence) != 2 ||
			binding.Sequence[0] != "!" {
			continue
		}
		got[binding.Sequence[1]] = binding.PresetValue
	}
	want := map[string]any{"0": 0, "1": 1, "2": 2, "3": 3}
	if len(got) != len(want) {
		t.Fatalf("priority chord terminals = %v, want %v", got, want)
	}
	for key, priority := range want {
		if got[key] != priority {
			t.Errorf("!%s = %v, want %v", key, got[key], priority)
		}
	}
	for _, binding := range strategy.Bindings {
		if binding.OperationID == operationTaskSetPriority &&
			len(binding.Sequence) > 0 &&
			binding.Sequence[0] == "p" {
			t.Errorf("retired priority prefix remains registered: %v", binding.Sequence)
		}
	}
}

func TestFinalStateChordOwnsEveryStateAndPlanningAction(t *testing.T) {
	strategy := SimplifiedInputStrategy()
	type terminal struct {
		operation OperationID
		value     any
		hasValue  bool
		display   string
	}
	got := make(map[string]terminal)
	gotOrder := make([]string, 0, 10)
	for _, binding := range strategy.Bindings {
		if len(binding.Sequence) != 2 || binding.Sequence[0] != "$" {
			continue
		}
		key := binding.Sequence[1]
		gotOrder = append(gotOrder, key)
		got[key] = terminal{
			operation: binding.OperationID,
			value:     binding.PresetValue,
			hasValue:  binding.HasPresetValue,
			display:   binding.DisplayKey,
		}
	}
	const stateSpelling = "$i/g/w/p/m/d/c"
	want := map[string]terminal{
		"i": {operation: operationTaskSetState, value: models.StateInbox, hasValue: true, display: stateSpelling},
		"b": {operation: operationCoreBacklog, display: "$b"},
		"n": {operation: operationCoreNext, display: "$n"},
		"t": {operation: operationCoreToday, display: "$t"},
		"g": {operation: operationTaskSetState, value: models.StateDoing, hasValue: true, display: stateSpelling},
		"w": {operation: operationTaskSetState, value: models.StateWaiting, hasValue: true, display: stateSpelling},
		"p": {operation: operationTaskSetState, value: models.StatePaused, hasValue: true, display: stateSpelling},
		"m": {operation: operationTaskSetState, value: models.StateSomeday, hasValue: true, display: stateSpelling},
		"d": {operation: operationTaskSetState, value: models.StateDone, hasValue: true, display: stateSpelling},
		"c": {operation: operationTaskSetState, value: models.StateCanceled, hasValue: true, display: stateSpelling},
	}
	if len(got) != len(want) {
		t.Fatalf("state chord terminals = %v, want %v", got, want)
	}
	if wantOrder := []string{"i", "b", "n", "t", "g", "w", "p", "m", "d", "c"}; !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Fatalf("state chord terminal order = %v, want %v", gotOrder, wantOrder)
	}
	for key, terminal := range want {
		if !reflect.DeepEqual(got[key], terminal) {
			t.Errorf("$%s = %+v, want %+v", key, got[key], terminal)
		}
	}
	for _, binding := range strategy.Bindings {
		if len(binding.Sequence) > 0 && binding.Sequence[0] == "," {
			t.Errorf("retired comma state prefix remains registered: %v", binding.Sequence)
		}
	}
}

func TestDirectTaskGrammarRemainsDocumentedWithoutACursorTarget(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	items := registry.HelpItemsForInputStrategy(
		InputStrategySimplified,
		simplifiedOperationContext(),
	)
	keys := make(map[string]bool, len(items))
	for _, item := range items {
		keys[item.Key] = true
	}
	for _, key := range []string{"A", "@", "#", "*", "[", "]", ">", "<", "y", "Y", "Ctrl+Y"} {
		if !keys[key] {
			t.Errorf("target-free help omits documented direct key %q", key)
		}
	}
}

func TestTaskOperationsCannotAppearInBothDirectGrammarAndMenu(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	strategy := SimplifiedInputStrategy()
	if err := validateTaskOperationOnePath(strategy); err != nil {
		t.Fatalf("production grammar: %v", err)
	}

	bound := make(map[OperationID]bool)
	for _, binding := range strategy.Bindings {
		bound[binding.OperationID] = true
	}
	menu := make(map[OperationID]bool)
	for _, id := range taskOperationManifestIDs() {
		menu[id] = true
	}
	for _, entry := range taskOperationDescriptorManifest() {
		if _, ok := registry.Descriptor(entry.ID); !ok {
			t.Errorf("manifested task operation %s is not registered", entry.ID)
		}
		if bound[entry.ID] == menu[entry.ID] {
			t.Errorf(
				"registered task operation %s has %d invocation paths; want exactly one",
				entry.ID,
				invocationPathCount(bound[entry.ID])+invocationPathCount(menu[entry.ID]),
			)
		}
	}

	t.Run("duplicate placement", func(t *testing.T) {
		candidate := cloneInputStrategy(strategy)
		candidate.Bindings = append(candidate.Bindings, OperationKeyBinding{
			OperationID: operationTaskComments,
			Keys:        []string{"Z"},
		})
		err := validateTaskOperationOnePath(candidate)
		if err == nil || !strings.Contains(err.Error(), "bound directly and listed") {
			t.Fatalf("error = %v, want duplicate-placement rejection", err)
		}
	})

	t.Run("unreachable direct-only operation", func(t *testing.T) {
		candidate := cloneInputStrategy(strategy)
		candidate.Bindings = slices.DeleteFunc(candidate.Bindings, func(binding OperationKeyBinding) bool {
			return binding.OperationID == operationTaskSetDueDate
		})
		err := validateTaskOperationOnePath(candidate)
		if err == nil || !strings.Contains(err.Error(), "direct-only but has no direct binding") {
			t.Fatalf("error = %v, want unreachable-direct rejection", err)
		}
	})
}

func invocationPathCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

// TestEverySurfaceManifestSatisfiesTheOnePathRule runs the rule the Outliner has
// always been held to over all five entity surfaces at once, so a surface cannot
// pass startup validation because nothing thought to check it. The table is
// compared against surfaceOperationGates so a sixth surface fails here rather
// than joining the app unexamined.
func TestEverySurfaceManifestSatisfiesTheOnePathRule(t *testing.T) {
	surfaces := []struct {
		name     string
		strategy InputStrategy
		manifest []operationManifestEntry
	}{
		{"task", SimplifiedInputStrategy(), taskOperationDescriptorManifest()},
		{"project hub", ProjectHubInputStrategy(), projectOperationManifest()},
		{"notes", NotesInputStrategy(), notesOperationManifest()},
		{"attachments", AttachmentsInputStrategy(), attachmentOperationManifest()},
		{"comments", CommentsInputStrategy(), commentOperationManifest()},
	}

	gated := make([]string, 0, len(surfaceOperationGates()))
	for _, gate := range surfaceOperationGates() {
		gated = append(gated, string(gate.strategy.ID))
		if err := gate.validate(gate.strategy); err != nil {
			t.Errorf("production grammar for %s: %v", gate.strategy.ID, err)
		}
	}
	tabled := make([]string, 0, len(surfaces))
	for _, surface := range surfaces {
		tabled = append(tabled, string(surface.strategy.ID))
	}
	slices.Sort(gated)
	slices.Sort(tabled)
	if !slices.Equal(gated, tabled) {
		t.Fatalf("gated surfaces = %v, tabled = %v", gated, tabled)
	}

	for _, surface := range surfaces {
		t.Run(surface.name, func(t *testing.T) {
			var firstMenu, firstDirect OperationID
			for _, entry := range surface.manifest {
				if entry.Risk == OperationRiskDestructive && entry.DirectOnly {
					t.Errorf("destructive operation %s is direct; it belongs behind the menu", entry.ID)
				}
				switch {
				case entry.DirectOnly && firstDirect == "":
					firstDirect = entry.ID
				case !entry.DirectOnly && firstMenu == "":
					firstMenu = entry.ID
				}
			}
			if firstMenu == "" || firstDirect == "" {
				t.Fatalf("manifest has no menu entry (%q) or no direct entry (%q)", firstMenu, firstDirect)
			}

			duplicate := cloneInputStrategy(surface.strategy)
			duplicate.Bindings = append(duplicate.Bindings, OperationKeyBinding{
				OperationID: firstMenu,
				Keys:        []string{"Z"},
			})
			err := validateOperationOnePath(surface.manifest, duplicate)
			if err == nil || !strings.Contains(err.Error(), "bound directly and listed") {
				t.Errorf("duplicate placement error = %v, want rejection", err)
			} else if !strings.Contains(err.Error(), string(surface.strategy.ID)) {
				t.Errorf("error %q does not name the surface it came from", err)
			}

			unreachable := cloneInputStrategy(surface.strategy)
			unreachable.Bindings = slices.DeleteFunc(unreachable.Bindings, func(binding OperationKeyBinding) bool {
				return binding.OperationID == firstDirect
			})
			err = validateOperationOnePath(surface.manifest, unreachable)
			if err == nil || !strings.Contains(err.Error(), "direct-only but has no direct binding") {
				t.Errorf("unreachable direct-only error = %v, want rejection", err)
			}
		})
	}
}

// TestPanelFootersOnlyAdvertiseKeysTheStrategyBinds covers the three surfaces
// whose footers come from the gated keymap rather than from the registry. Their
// keymaps are the only footer source, so nothing else notices when one promises
// a key the strategy stopped binding. Modal keys are the exception and are named
// here rather than waved through, because a composer's save and newline keys
// exist only while its textarea has focus.
func TestPanelFootersOnlyAdvertiseKeysTheStrategyBinds(t *testing.T) {
	for _, surface := range []struct {
		name     string
		strategy InputStrategy
		keys     *keymap.KeyMap
		modal    []string
	}{
		{"notes", NotesInputStrategy(), (&NotesViewModel{}).buildKeyMap(), nil},
		{"attachments", AttachmentsInputStrategy(), (&TaskAttachmentsModel{}).buildKeyMap(), nil},
		{
			name:     "comments",
			strategy: CommentsInputStrategy(),
			keys:     (&TaskCommentsModel{}).buildKeyMap(),
			modal:    []string{"ctrl+s", "ctrl+e", "shift+enter"},
		},
	} {
		t.Run(surface.name, func(t *testing.T) {
			for _, binding := range surface.strategy.Bindings {
				if binding.FooterRank != 0 || binding.FooterLabel != "" {
					t.Errorf(
						"%s carries footer metadata nothing reads; the keymap is the footer source",
						binding.OperationID,
					)
				}
			}

			bound := make(map[string]bool)
			for _, binding := range surface.strategy.Bindings {
				for _, key := range binding.Keys {
					bound[key] = true
				}
			}
			advertised := 0
			for _, entry := range surface.keys.Entries {
				if entry.Binding == nil || entry.Binding.FooterRank <= 0 {
					continue
				}
				advertised++
				key := entry.Binding.Key
				if !bound[key] && !slices.Contains(surface.modal, key) {
					t.Errorf("footer advertises %q, which the strategy does not bind", key)
				}
			}
			if advertised == 0 {
				t.Fatal("keymap advertises no footer hints")
			}
		})
	}
}

func TestDirectThemeAndQuitOperationsUseTheirDescriptors(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureFresh, 100, 30)
	if err != nil {
		t.Fatalf("NewFixtureDriver: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	app := driver.App()
	beforeTheme := app.runtime.Machine().Theme
	t.Cleanup(func() {
		ApplyTheme(NewThemeConfig(app.runtime.Machine()))
	})

	themeResult := executeDirectPath(
		t,
		app,
		app.operationCommandContext(),
		[]string{"T"},
	)
	if themeResult.Err != nil {
		t.Fatalf("toggle theme: %v", themeResult.Err)
	}
	timeout, ok := themeResult.Message.(OperationCommandResultMsg)
	if !ok || timeout.Cmd == nil {
		t.Fatalf("toggle theme message = %T, want a status timeout command", themeResult.Message)
	}
	if app.statusMessage != "Theme: "+app.runtime.Machine().Theme {
		t.Fatalf("theme status = %q, want the active theme", app.statusMessage)
	}
	if handled, _ := app.handleViewMessages(StatusClearMsg{ScheduledAt: app.statusUpdatedAt.Add(time.Nanosecond)}); !handled {
		t.Fatal("theme status clear message was not handled")
	}
	if app.statusMessage != "" {
		t.Fatalf("theme status = %q after timeout, want empty", app.statusMessage)
	}
	afterTheme := app.runtime.Machine().Theme
	if afterTheme == beforeTheme || (afterTheme != "dark" && afterTheme != "light") {
		t.Fatalf("theme changed from %q to %q, want the opposite maintained variant", beforeTheme, afterTheme)
	}
	reloaded, err := config.LoadRuntimeState(workspace.DataDir)
	if err != nil {
		t.Fatalf("reload machine theme: %v", err)
	}
	if reloaded.Machine().Theme != afterTheme {
		t.Fatalf("persisted machine theme = %q, want %q", reloaded.Machine().Theme, afterTheme)
	}
	app.pushView(ViewProjectHub)
	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{
		Code: 'c',
		Mod:  tea.ModCtrl,
	})
	if !handled || cmd == nil {
		t.Fatalf("Ctrl+C over a subordinate route = handled %v cmd %v, want descriptor dispatch", handled, cmd)
	}
	if _, ok := cmd().(QuitMsg); !ok {
		t.Fatalf("Ctrl+C command emitted %T, want QuitMsg", cmd())
	}
}
