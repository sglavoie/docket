// Package textgrammar owns the inline metadata language shared by capture,
// inline editing, and outline editing.
package textgrammar

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sglavoie/docket/internal/models"
)

func init() {
	if err := validateRegistry(); err != nil {
		panic(err)
	}
}

// validateRegistry checks the invariants that keep parser and presentation
// projections safe to generate from the canonical entries.
func validateRegistry() error {
	seenFields := make(map[Field]bool, len(registry))
	seenTriggers := make(map[string]Field, len(registry))
	seenForms := make(map[string]Field, len(registry))
	for _, spec := range registry {
		if err := validateDefinitionIdentity(spec, seenFields, seenTriggers); err != nil {
			return err
		}
		if err := validateDefinitionValues(spec); err != nil {
			return err
		}
		if err := validateDefinitionForms(spec, seenForms); err != nil {
			return err
		}
		if err := validateDefinitionClears(spec); err != nil {
			return err
		}
		if err := validateDefinitionDescriptions(spec); err != nil {
			return err
		}
	}
	if err := validateGuideOrder(); err != nil {
		return err
	}
	if err := validateTriggerPrefixOrder(); err != nil {
		return err
	}
	if err := validateNegationPlacement(); err != nil {
		return err
	}
	return validateDestinationDefinition()
}

// validateTriggerPrefixOrder asserts that whenever one trigger is a prefix of
// another, the longer one is peeled first. RE2 has no lookaround, so a shorter
// trigger registered ahead of the trigger that extends it would swallow the
// first characters of every token the longer one owns. The registry has always
// depended on this — ## ahead of #, @@ ahead of @ — through comments and a
// hand-held order; the uniform negations doubled the number of prefix pairs, so
// it is checked instead.
func validateTriggerPrefixOrder() error {
	for i, spec := range registry {
		for _, later := range registry[i+1:] {
			if strings.HasPrefix(later.Trigger, spec.Trigger) {
				return fmt.Errorf(
					"grammar trigger %q is parsed before %q, which extends it",
					spec.Trigger, later.Trigger,
				)
			}
		}
	}
	return nil
}

// validateNegationPlacement asserts that a negation names a real twin, sits
// immediately before it in parse order, and reaches no surface the twin does
// not. Adjacency is what makes "-" one operator rather than a family of
// coincidences: the pair is peeled together, so no third pattern can slip
// between a value and its removal and consume half of one.
func validateNegationPlacement() error {
	for i, spec := range registry {
		if spec.negates == "" {
			continue
		}
		if spec.negates == spec.Field {
			return fmt.Errorf("grammar field %s negates itself", spec.Field)
		}
		if i+1 >= len(registry) || registry[i+1].Field != spec.negates {
			return fmt.Errorf(
				"grammar field %s must be registered immediately before %s, the field it negates",
				spec.Field, spec.negates,
			)
		}
		twin := registry[i+1]
		if spec.Surfaces&^twin.Surfaces != 0 {
			return fmt.Errorf(
				"grammar field %s reaches a surface %s does not",
				spec.Field, twin.Field,
			)
		}
	}
	return nil
}

// validateGuideOrder asserts parse order and teach order hold identical sets.
// Splitting the two lists is what lets a field be parsed early and taught late,
// and this is the fence that makes the split safe: a field added to one list and
// forgotten in the other would otherwise silently vanish from the syntax guide
// or gain a second, unreachable entry.
func validateGuideOrder() error {
	seen := make(map[Field]bool, len(guideOrder))
	for _, field := range guideOrder {
		if seen[field] {
			return fmt.Errorf("grammar guide order lists %q twice", field)
		}
		seen[field] = true
		if _, ok := registrySpecFor(field); !ok {
			return fmt.Errorf("grammar guide order names %q, which the registry does not declare", field)
		}
	}
	for _, spec := range registry {
		if !seen[spec.Field] {
			return fmt.Errorf("grammar field %q is missing from the guide order", spec.Field)
		}
	}
	return nil
}

func validateDefinitionIdentity(spec definitionSpec, seenFields map[Field]bool, seenTriggers map[string]Field) error {
	if spec.Field == "" || spec.Trigger == "" || spec.pattern == "" {
		return fmt.Errorf("grammar definition %q is incomplete", spec.Field)
	}
	if seenFields[spec.Field] {
		return fmt.Errorf("grammar field %q is duplicated", spec.Field)
	}
	seenFields[spec.Field] = true
	if previous, exists := seenTriggers[spec.Trigger]; exists {
		return fmt.Errorf("grammar trigger %q is shared by %s and %s", spec.Trigger, previous, spec.Field)
	}
	seenTriggers[spec.Trigger] = spec.Field
	if spec.Surfaces == 0 || spec.Surfaces&^allSurfaces != 0 {
		return fmt.Errorf("grammar field %s has invalid surfaces %d", spec.Field, spec.Surfaces)
	}
	if spec.Cardinality != CardinalitySingleton && spec.Cardinality != CardinalityRepeatable {
		return fmt.Errorf("grammar field %s has invalid cardinality %d", spec.Field, spec.Cardinality)
	}
	return nil
}

func validateDefinitionValues(spec definitionSpec) error {
	seenValues := make(map[string]bool, len(spec.Values)+len(spec.RetiredValues))
	for _, value := range spec.Values {
		if value == "" || seenValues[value] {
			return fmt.Errorf("grammar field %s has invalid static value %q", spec.Field, value)
		}
		seenValues[value] = true
	}
	for _, retired := range spec.RetiredValues {
		if retired == "" || seenValues[retired] {
			return fmt.Errorf("grammar field %s has invalid retired value %q", spec.Field, retired)
		}
		seenValues[retired] = true
	}
	return nil
}

func validateDefinitionForms(spec definitionSpec, seenForms map[string]Field) error {
	for _, form := range spec.forms {
		if form.Text == "" || !strings.HasPrefix(form.Text, spec.Trigger) {
			return fmt.Errorf("grammar field %s has invalid form %q", spec.Field, form.Text)
		}
		if form.Surfaces == 0 || form.Surfaces&^spec.Surfaces != 0 {
			return fmt.Errorf("grammar form %q has invalid surfaces %d", form.Text, form.Surfaces)
		}
		if previous, exists := seenForms[form.Text]; exists {
			return fmt.Errorf("grammar form %q is shared by %s and %s", form.Text, previous, spec.Field)
		}
		seenForms[form.Text] = spec.Field
	}
	return nil
}

func validateDefinitionClears(spec definitionSpec) error {
	for _, clear := range spec.Clears {
		if clear.Value == "" || clear.Surfaces == 0 || clear.Surfaces&^spec.Surfaces != 0 {
			return fmt.Errorf("grammar field %s has invalid clear semantics", spec.Field)
		}
		clearForm := spec.Trigger + clear.Value
		found := slices.ContainsFunc(spec.forms, func(form surfaceForm) bool {
			return form.Text == clearForm && form.Surfaces&clear.Surfaces == clear.Surfaces
		})
		if !found {
			return fmt.Errorf("grammar field %s clear value %q has no matching form", spec.Field, clear.Value)
		}
	}
	return nil
}

// validateDefinitionDescriptions keeps the search projection from advertising a
// mutation sentence: a field reaching search must carry predicate wording, and
// so must every clear value that reaches it. A search-only field states its
// meaning in searchDescription alone, so it is the one shape allowed — and
// required — to leave the mutation sentence empty.
func validateDefinitionDescriptions(spec definitionSpec) error {
	if spec.Surfaces&SurfaceSearch != 0 && !spec.Legacy && spec.searchDescription == "" {
		return fmt.Errorf("grammar field %s has no search description", spec.Field)
	}
	if spec.Surfaces&SurfaceSearch == 0 && spec.searchDescription != "" {
		return fmt.Errorf("grammar field %s declares a search description off the search surface", spec.Field)
	}
	if spec.Surfaces&^SurfaceSearch != 0 && spec.Description == "" {
		return fmt.Errorf("grammar field %s has no mutation description", spec.Field)
	}
	if spec.Surfaces == SurfaceSearch && spec.Description != "" {
		return fmt.Errorf("grammar field %s is search-only and must describe itself once, in its search description", spec.Field)
	}
	for value, text := range spec.searchClearText {
		if text == "" {
			return fmt.Errorf("grammar field %s has empty search text for clear %q", spec.Field, value)
		}
		found := slices.ContainsFunc(spec.Clears, func(clear ClearOperation) bool {
			return clear.Value == value && clear.Surfaces&SurfaceSearch != 0
		})
		if !found {
			return fmt.Errorf("grammar field %s search text %q names no search clear", spec.Field, value)
		}
	}
	for _, clear := range spec.Clears {
		if clear.Surfaces&SurfaceSearch != 0 && spec.searchClearText[clear.Value] == "" {
			return fmt.Errorf("grammar field %s search clear %q has no search text", spec.Field, clear.Value)
		}
	}
	return nil
}

func validateDestinationDefinition() error {
	destination, ok := registrySpecFor(FieldDestination)
	if !ok {
		return fmt.Errorf("destination grammar is missing")
	}
	wantDestinations := []string{"today", "next"}
	for _, state := range models.AllWorkflowStates() {
		wantDestinations = append(wantDestinations, state.String())
	}
	if len(destination.Values) != len(wantDestinations) {
		return fmt.Errorf("destination grammar has %d values; want %d", len(destination.Values), len(wantDestinations))
	}
	for _, want := range wantDestinations {
		if !slices.Contains(destination.Values, want) {
			return fmt.Errorf("destination grammar is missing %q", want)
		}
	}
	return nil
}

func registrySpecFor(field Field) (definitionSpec, bool) {
	for _, spec := range registry {
		if spec.Field == field {
			return spec, true
		}
	}
	return definitionSpec{}, false
}

// StateDescription returns concise presentation text for a canonical state.
