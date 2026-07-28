package tui

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// InputStrategyID identifies a complete top-level key grammar. Simplified is
// the production daily-loop grammar built from OperationRegistry identities;
// Legacy remains only as a temporary internal discriminator for the follow-up
// compatibility cleanup.
type InputStrategyID string

const (
	InputStrategySimplified InputStrategyID = "simplified"
)

// OperationKeyBinding assigns one operation one or more equivalent direct key
// events. Keys are physical Bubble Tea key names, never prefix sequences.
// DisplayKey is the compact help spelling. FooterRank zero excludes an
// operation from compact footer hints.
type OperationKeyBinding struct {
	OperationID OperationID
	Keys        []string
	DisplayKey  string
	FooterRank  int
	FooterLabel string
}

// InputStrategy is one collision-free, prefix-free operation key grammar.
type InputStrategy struct {
	ID       InputStrategyID
	Bindings []OperationKeyBinding
}

// Stable core operation IDs let the final grammar remain independent of dormant
// legacy command adapters and their retired shortcuts or prefix chords.
const (
	operationCoreCapture          OperationID = "tui.core.capture"
	operationCoreEdit             OperationID = "tui.core.edit"
	operationCoreStart            OperationID = "tui.core.start"
	operationCoreDone             OperationID = "tui.core.done"
	operationCoreBacklog          OperationID = "tui.core.backlog"
	operationCoreToday            OperationID = "tui.core.today"
	operationCoreNext             OperationID = "tui.core.next"
	operationCoreActions          OperationID = "tui.core.actions"
	operationCoreCursorDown       OperationID = "tui.core.cursor-down"
	operationCoreCursorUp         OperationID = "tui.core.cursor-up"
	operationCoreCursorTop        OperationID = "tui.core.cursor-top"
	operationCoreCursorBottom     OperationID = "tui.core.cursor-bottom"
	operationCoreCursorPageUp     OperationID = "tui.core.cursor-page-up"
	operationCoreCursorPageDown   OperationID = "tui.core.cursor-page-down"
	operationCoreCollapseParent   OperationID = "tui.core.collapse-parent"
	operationCoreExpand           OperationID = "tui.core.expand"
	operationCoreReorderDown      OperationID = "tui.core.reorder-down"
	operationCoreReorderUp        OperationID = "tui.core.reorder-up"
	operationCoreDetail           OperationID = "tui.core.detail"
	operationCoreVisibleSearch    OperationID = "tui.core.visible-search"
	operationCoreFinder           OperationID = "tui.core.finder"
	operationCoreModes            OperationID = "tui.core.modes"
	operationCoreScope            OperationID = "tui.core.scope"
	operationCoreMultiSelect      OperationID = "tui.core.multi-select"
	operationCoreNotes            OperationID = "tui.core.notes"
	operationCoreWeeklyReview     OperationID = "tui.core.weekly-review"
	operationCoreTags             OperationID = "tui.core.tags"
	operationCoreNotifications    OperationID = "tui.core.notifications"
	operationCoreUndo             OperationID = "tui.core.undo"
	operationCoreRedo             OperationID = "tui.core.redo"
	operationCoreBack             OperationID = "tui.core.back"
	operationCoreReset            OperationID = "tui.core.reset"
	operationCoreHelp             OperationID = "tui.core.help"
	operationCoreQuit             OperationID = "tui.core.quit"
	operationCoreNextWorkMode     OperationID = "tui.core.next-work-mode"
	operationCorePreviousWorkMode OperationID = "tui.core.previous-work-mode"
)

// SimplifiedInputStrategy returns a fresh copy of the accepted daily-loop
// grammar. Every key is direct: the strategy has no prefix node or chord.
func SimplifiedInputStrategy() InputStrategy {
	return InputStrategy{
		ID: InputStrategySimplified,
		Bindings: []OperationKeyBinding{
			{OperationID: operationCoreCapture, Keys: []string{"a"}, DisplayKey: "a", FooterRank: 1, FooterLabel: "capture"},
			{OperationID: operationCoreEdit, Keys: []string{"e"}, DisplayKey: "e", FooterRank: 10, FooterLabel: "edit"},
			{OperationID: operationCoreStart, Keys: []string{"s"}, DisplayKey: "s", FooterRank: 11, FooterLabel: "start"},
			{OperationID: operationCoreDone, Keys: []string{"d"}, DisplayKey: "d", FooterRank: 3, FooterLabel: "done"},
			{OperationID: operationCoreBacklog, Keys: []string{"b"}, DisplayKey: "b", FooterRank: 12, FooterLabel: "backlog"},
			{OperationID: operationCoreToday, Keys: []string{"t"}, DisplayKey: "t", FooterRank: 13, FooterLabel: "today"},
			{OperationID: operationCoreNext, Keys: []string{"n"}, DisplayKey: "n", FooterRank: 14, FooterLabel: "next"},
			{OperationID: operationCoreActions, Keys: []string{"space"}, DisplayKey: "Space", FooterRank: 2, FooterLabel: "actions"},
			{OperationID: operationCoreCursorDown, Keys: []string{"j", "down"}, DisplayKey: "j/↓"},
			{OperationID: operationCoreCursorUp, Keys: []string{"k", "up"}, DisplayKey: "k/↑"},
			{OperationID: operationCoreCursorTop, Keys: []string{"g", "home"}, DisplayKey: "g/Home"},
			{OperationID: operationCoreCursorBottom, Keys: []string{"G", "end"}, DisplayKey: "G/End"},
			{OperationID: operationCoreCursorPageUp, Keys: []string{"ctrl+u", "pgup"}, DisplayKey: "Ctrl+U/PgUp"},
			{OperationID: operationCoreCursorPageDown, Keys: []string{"ctrl+d", "pgdown"}, DisplayKey: "Ctrl+D/PgDn"},
			{OperationID: operationCoreCollapseParent, Keys: []string{"h", "left"}, DisplayKey: "h/←"},
			{OperationID: operationCoreExpand, Keys: []string{"l", "right"}, DisplayKey: "l/→"},
			{OperationID: operationCoreReorderDown, Keys: []string{"J"}, DisplayKey: "J"},
			{OperationID: operationCoreReorderUp, Keys: []string{"K"}, DisplayKey: "K"},
			{OperationID: operationCoreDetail, Keys: []string{"enter"}, DisplayKey: "Enter"},
			{OperationID: operationCoreVisibleSearch, Keys: []string{"/"}, DisplayKey: "/", FooterRank: 4, FooterLabel: "search"},
			{OperationID: operationCoreFinder, Keys: []string{":"}, DisplayKey: ":", FooterRank: 5, FooterLabel: "finder"},
			{OperationID: operationCoreModes, Keys: []string{"m"}, DisplayKey: "m", FooterRank: 6, FooterLabel: "modes"},
			{OperationID: operationCoreScope, Keys: []string{"c"}, DisplayKey: "c", FooterRank: 7, FooterLabel: "scope"},
			{OperationID: operationCoreMultiSelect, Keys: []string{"v"}, DisplayKey: "v"},
			{OperationID: operationCoreNotes, Keys: []string{"N"}, DisplayKey: "N"},
			{OperationID: operationCoreWeeklyReview, Keys: []string{"R"}, DisplayKey: "R"},
			{OperationID: operationCoreTags, Keys: []string{"T"}, DisplayKey: "T"},
			{OperationID: operationCoreNotifications, Keys: []string{"!"}, DisplayKey: "!"},
			{OperationID: operationCoreUndo, Keys: []string{"u"}, DisplayKey: "u"},
			{OperationID: operationCoreRedo, Keys: []string{"ctrl+r"}, DisplayKey: "Ctrl+R"},
			{OperationID: operationCoreBack, Keys: []string{"esc"}, DisplayKey: "Esc"},
			{OperationID: operationCoreReset, Keys: []string{"ctrl+esc"}, DisplayKey: "Ctrl+Esc", FooterRank: 9, FooterLabel: "root"},
			{OperationID: operationCoreHelp, Keys: []string{"?"}, DisplayKey: "?", FooterRank: 8, FooterLabel: "help"},
			{OperationID: operationCoreQuit, Keys: []string{"ctrl+c"}, DisplayKey: "Ctrl+C"},
			{OperationID: operationCoreNextWorkMode, Keys: []string{"tab"}, DisplayKey: "Tab"},
			{OperationID: operationCorePreviousWorkMode, Keys: []string{"shift+tab"}, DisplayKey: "Shift+Tab"},
		},
	}
}

// RegisterInputStrategy validates and stores a strategy. Descriptor existence,
// exact key uniqueness, and direct-event syntax are checked together so a
// registered strategy cannot contain an undocumented prefix dependency.
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

	seenKeys := make(map[string]OperationID)
	seenOperations := make(map[OperationID]bool)
	normalized := InputStrategy{ID: strategy.ID, Bindings: make([]OperationKeyBinding, 0, len(strategy.Bindings))}
	for _, binding := range strategy.Bindings {
		binding.OperationID = OperationID(strings.TrimSpace(string(binding.OperationID)))
		if binding.OperationID == "" {
			return fmt.Errorf("input strategy %s has a binding without an operation ID", strategy.ID)
		}
		if seenOperations[binding.OperationID] {
			return fmt.Errorf("input strategy %s binds operation %s more than once", strategy.ID, binding.OperationID)
		}
		if _, ok := registry.Descriptor(binding.OperationID); !ok {
			return fmt.Errorf("input strategy %s references unknown operation %s", strategy.ID, binding.OperationID)
		}
		if len(binding.Keys) == 0 {
			return fmt.Errorf("input strategy %s operation %s has no keys", strategy.ID, binding.OperationID)
		}
		seenOperations[binding.OperationID] = true
		binding.DisplayKey = strings.TrimSpace(binding.DisplayKey)
		binding.FooterLabel = strings.TrimSpace(binding.FooterLabel)

		keys := make([]string, 0, len(binding.Keys))
		for _, raw := range binding.Keys {
			key := normalizeOperationKey(raw)
			if !isDirectInputKey(key) {
				return fmt.Errorf(
					"input strategy %s operation %s key %q is not one direct key event",
					strategy.ID,
					binding.OperationID,
					raw,
				)
			}
			if owner, exists := seenKeys[key]; exists {
				return fmt.Errorf(
					"input strategy %s key collision: %s (%s vs %s)",
					strategy.ID,
					key,
					owner,
					binding.OperationID,
				)
			}
			seenKeys[key] = binding.OperationID
			keys = append(keys, key)
		}
		binding.Keys = keys
		if binding.DisplayKey == "" {
			binding.DisplayKey = strings.Join(keys, "/")
		}
		normalized.Bindings = append(normalized.Bindings, cloneOperationKeyBinding(binding))
	}

	registry.inputStrategies[strategy.ID] = normalized
	return nil
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

// InputBindingForKey resolves one direct key through the selected strategy and
// active operation context.
func (registry *OperationRegistry) InputBindingForKey(
	id InputStrategyID,
	key string,
	ctx CommandContext,
) (OperationKeyBinding, OperationDescriptor, bool) {
	strategy, ok := registry.InputStrategy(id)
	if !ok {
		return OperationKeyBinding{}, OperationDescriptor{}, false
	}
	key = normalizeOperationKey(key)
	for _, binding := range strategy.Bindings {
		if !slices.Contains(binding.Keys, key) {
			continue
		}
		descriptor, exists := registry.Descriptor(binding.OperationID)
		if !exists || !descriptor.Applicable(ctx) {
			return OperationKeyBinding{}, OperationDescriptor{}, false
		}
		return binding, descriptor, true
	}
	return OperationKeyBinding{}, OperationDescriptor{}, false
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
	return binding
}

func cloneInputStrategy(strategy InputStrategy) InputStrategy {
	strategy.Bindings = append([]OperationKeyBinding(nil), strategy.Bindings...)
	for i := range strategy.Bindings {
		strategy.Bindings[i] = cloneOperationKeyBinding(strategy.Bindings[i])
	}
	return strategy
}
