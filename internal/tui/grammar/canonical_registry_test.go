package grammar

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/capturetext"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

// The three contracts below are the capture half of the canonical-registry
// consumer contract. They read nothing above this package, so they are homed
// with the surfaces they project. The half that stayed in the shell —
// the `?` grammar section, the outline header forms, and the
// no-parallel-static-list file scan — names help and outline-editor
// declarations the shell owns.

func TestCaptureAutocompleteProjectsCanonicalRegistry(t *testing.T) {
	definitions := textgrammar.Definitions(textgrammar.SurfaceCapture)
	triggers := quickCaptureTriggers(false)
	if len(triggers) != len(definitions) {
		t.Fatalf("autocomplete triggers = %d, want %d registry definitions", len(triggers), len(definitions))
	}
	for i, definition := range definitions {
		if got, want := triggers[i].Text, definition.Trigger; got != want {
			t.Errorf("trigger %d = %q, want %q for %s", i, got, want, definition.Field)
		}
		if got, want := triggers[i].Mode, AutocompleteModeForField(definition.Field); got != want {
			t.Errorf("mode for %s = %d, want %d", definition.Field, got, want)
		}
	}

	withoutProject := quickCaptureTriggers(true)
	for _, trigger := range withoutProject {
		if trigger.Mode == acModeProject {
			t.Fatal("project trigger remained active when project routing was disabled")
		}
	}
	if len(withoutProject) != len(triggers)-1 {
		t.Fatalf("project-disabled triggers = %d, want %d", len(withoutProject), len(triggers)-1)
	}
}

func TestStaticAutocompleteValuesComeFromCanonicalRegistry(t *testing.T) {
	q := &QuickCaptureInput{}
	tests := []struct {
		field       textgrammar.Field
		suggestions []ACSuggestion
	}{
		{textgrammar.FieldPriority, q.getPrioritySuggestions("")},
		{textgrammar.FieldDue, q.getDateSuggestions("")},
		{textgrammar.FieldDestination, q.GetDestinationSuggestions("")},
		{textgrammar.FieldPinned, q.getPinSuggestions("")},
	}
	for _, tt := range tests {
		definition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, tt.field)
		if !ok {
			t.Fatalf("missing definition for %s", tt.field)
		}
		values := make([]string, len(tt.suggestions))
		for i, suggestion := range tt.suggestions {
			values[i] = suggestion.Value
		}
		if !slices.Equal(values, definition.Values) {
			t.Errorf("%s suggestions = %v, want registry values %v", tt.field, values, definition.Values)
		}
	}
}

func TestSyntaxGuideExamplesAndGroupsComeFromCanonicalRegistry(t *testing.T) {
	project := quickCaptureTestProject()
	// The routed project carries a task because the dependency examples teach the
	// qualified alias, which is only resolvable when the named project has one.
	routed := &models.Project{
		ID: "routed", Name: "Routed", Slug: "project-slug",
		Tasks: []*models.Task{{ID: "routed-1", ShortID: 1, Title: "Routed work"}},
	}
	q := &QuickCaptureInput{Width: 120}

	var entries []SyntaxGuideEntry
	for _, group := range q.SyntaxGuideGroups() {
		entries = append(entries, group.Entries...)
	}
	// Teach order, not parse order: the composer guide and the `?` grammar
	// section are the two consumers that read GuideDefinitions, because the
	// attachment sigil has to be peeled before #tag and must not lead either
	// teaching surface on that account.
	definitions := textgrammar.GuideDefinitions(textgrammar.SurfaceCapture)
	if len(entries) != len(definitions) {
		t.Fatalf("syntax entries = %d, want %d registry definitions", len(entries), len(definitions))
	}
	for i, definition := range definitions {
		entry := entries[i]
		if got, want := entry.Forms, strings.Join(definition.Forms, " · "); got != want {
			t.Errorf("%s forms = %q, want %q", definition.Field, got, want)
		}
		if got, want := entry.examples, strings.Join(definition.Examples, "  "); got != want {
			t.Errorf("%s examples = %q, want %q", definition.Field, got, want)
		}
		if len(entry.validation) != len(definition.Examples) {
			t.Errorf("%s validation examples = %d, want %d", definition.Field, len(entry.validation), len(definition.Examples))
		}
		for _, input := range entry.validation {
			if _, err := capturetext.ParseQuickCapture(input, project, []*models.Project{routed}, false); err != nil {
				t.Errorf("registry-backed syntax example %q does not parse: %v", input, err)
			}
		}
	}
}
