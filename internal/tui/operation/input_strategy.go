package operation

import (
	"fmt"
	"strings"
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
// HintGroup names the presentation division a which-key row draws this binding
// under. It is metadata over the one declaration, never a second key list: it
// changes no sequence, no operation identity, and no dispatch path, and an empty
// group means the binding stands ungrouped. HintKeep is the same kind of
// metadata: it asks a which-key row too narrow for its whole run to shed this
// binding last of that run, and moves it nowhere in the row.
//
// SharedKey admits the binding onto a key path another operation already owns.
// Every binding on that path must declare it, and resolution picks the first
// applicable one in declaration order, so the operations sharing a key must be
// applicable over disjoint cursors. Without the flag a repeated path stays a
// grammar defect.
type OperationKeyBinding struct {
	// Help presentation metadata never changes dispatch or footer order.
	// HelpLabel optionally spells a preset's action; its state remains an alias.
	HelpLabel      string
	HelpGroup      string
	HelpOrder      int
	HelpCategory   string
	OperationID    OperationID
	Keys           []string
	Sequence       []string
	DisplayKey     string
	FooterRank     int
	FooterLabel    string
	HintLabel      string
	HintGroup      string
	HintKeep       bool
	PresetValue    any
	HasPresetValue bool
	SharedKey      bool
}

// OperationChordGroup labels one non-terminal chord prefix. Every prefix in a
// strategy needs a label so a pending chord always has a which-key row to show;
// a ranked group also appears among the compact footer hints. HintGroup is the
// binding's own field read the same way: the division the parent prefix's
// which-key row draws this prefix under, so a nested chord joins the run its
// terminal siblings declared.
type OperationChordGroup struct {
	Sequence    []string
	HintLabel   string
	HintGroup   string
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

// Paths returns every key path that resolves this binding. Direct keys are
// length-1 paths, so a chord and a direct key are the same kind of value, and a
// binding may declare both when one operation genuinely answers to a direct key
// and a chord alike.
func (binding OperationKeyBinding) Paths() [][]string {
	paths := make([][]string, 0, len(binding.Keys)+1)
	for _, key := range binding.Keys {
		paths = append(paths, []string{key})
	}
	if len(binding.Sequence) > 0 {
		paths = append(paths, binding.Sequence)
	}
	return paths
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
			if !descriptor.ValueProviderID.AcceptsPresetValue(binding.PresetValue) {
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
		for _, path := range binding.Paths() {
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
		key := NormalizeOperationKey(raw)
		if !IsDirectInputKey(key) {
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
