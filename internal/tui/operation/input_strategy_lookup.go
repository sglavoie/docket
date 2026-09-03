package operation

import (
	"strings"
	"unicode/utf8"
)

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
		node = node.child(NormalizeOperationKey(raw))
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
		keys = append(keys, NormalizeOperationKey(key))
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

func IsDirectInputKey(key string) bool {
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
	return IsDirectInputKey(parts[len(parts)-1])
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
