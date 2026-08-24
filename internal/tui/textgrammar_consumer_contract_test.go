package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

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
		if got, want := triggers[i].Mode, autocompleteModeForField(definition.Field); got != want {
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
	q := &quickCaptureInput{}
	tests := []struct {
		field       textgrammar.Field
		suggestions []ACSuggestion
	}{
		{textgrammar.FieldPriority, q.getPrioritySuggestions("")},
		{textgrammar.FieldDue, q.getDateSuggestions("")},
		{textgrammar.FieldDestination, q.getDestinationSuggestions("")},
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
	routed := &models.Project{ID: "routed", Name: "Routed", Slug: "project-slug"}
	q := &quickCaptureInput{width: 120}

	var entries []syntaxGuideEntry
	for _, group := range q.syntaxGuideGroups() {
		entries = append(entries, group.entries...)
	}
	// Teach order, not parse order: the guide is the one consumer that reads
	// GuideDefinitions, because the attachment sigil has to be peeled before #tag
	// and must not lead the guide on that account.
	definitions := textgrammar.GuideDefinitions(textgrammar.SurfaceCapture)
	if len(entries) != len(definitions) {
		t.Fatalf("syntax entries = %d, want %d registry definitions", len(entries), len(definitions))
	}
	for i, definition := range definitions {
		entry := entries[i]
		if got, want := entry.forms, strings.Join(definition.Forms, " · "); got != want {
			t.Errorf("%s forms = %q, want %q", definition.Field, got, want)
		}
		if got, want := entry.examples, strings.Join(definition.Examples, "  "); got != want {
			t.Errorf("%s examples = %q, want %q", definition.Field, got, want)
		}
		if len(entry.validation) != len(definition.Examples) {
			t.Errorf("%s validation examples = %d, want %d", definition.Field, len(entry.validation), len(definition.Examples))
		}
		for _, input := range entry.validation {
			if _, err := parseQuickCapture(input, project, []*models.Project{routed}, false); err != nil {
				t.Errorf("registry-backed syntax example %q does not parse: %v", input, err)
			}
		}
	}
}

func TestOutlineHeaderFormsComeFromCanonicalRegistry(t *testing.T) {
	var forms []string
	for _, definition := range textgrammar.Definitions(textgrammar.SurfaceOutline) {
		forms = append(forms, definition.Forms...)
	}
	want := strings.Join(forms, " ")
	if got := outlineGrammarForms(); got != want {
		t.Fatalf("outline forms = %q, want %q", got, want)
	}
}

func TestInlineGrammarConsumersDoNotOwnParallelStaticLists(t *testing.T) {
	consumerFiles := []string{
		"outline_editor.go",
		"outline_editor_parse.go",
		"quick_capture_autocomplete.go",
		"quick_capture_input.go",
		"quick_capture_parser.go",
		"quick_capture_parser_fields.go",
	}
	triggerSet := make(map[string]bool)
	for _, definition := range textgrammar.Definitions(textgrammar.SurfaceCapture) {
		triggerSet[definition.Trigger] = true
	}
	destinationSet := make(map[string]bool)
	destination, ok := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldDestination)
	if !ok {
		t.Fatal("capture destination grammar is missing")
	}
	for _, value := range destination.Values {
		destinationSet[value] = true
	}

	for _, name := range consumerFiles {
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, filepath.Clean(name), source, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.CompositeLit:
				triggers, destinations := grammarLiteralsInNode(value, triggerSet, destinationSet)
				if len(triggers) >= 2 {
					t.Errorf("%s:%d owns a parallel trigger list %v", name, fileSet.Position(value.Pos()).Line, mapKeys(triggers))
				}
				if len(destinations) >= 2 {
					t.Errorf("%s:%d owns a parallel destination list %v", name, fileSet.Position(value.Pos()).Line, mapKeys(destinations))
				}
			case *ast.BasicLit:
				if value.Kind != token.STRING {
					return true
				}
				literal, err := strconv.Unquote(value.Value)
				if err == nil && inlineFormTriggerCount(literal, triggerSet) >= 2 {
					t.Errorf("%s:%d owns a parallel inline-form string %q", name, fileSet.Position(value.Pos()).Line, literal)
				}
			}
			return true
		})
	}
}

func grammarLiteralsInNode(node ast.Node, triggers, destinations map[string]bool) (map[string]bool, map[string]bool) {
	foundTriggers := make(map[string]bool)
	foundDestinations := make(map[string]bool)
	ast.Inspect(node, func(child ast.Node) bool {
		literal, ok := child.(*ast.BasicLit)
		if !ok || (literal.Kind != token.STRING && literal.Kind != token.CHAR) {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			return true
		}
		if triggers[value] {
			foundTriggers[value] = true
		}
		if destinations[value] {
			foundDestinations[value] = true
		}
		return true
	})
	return foundTriggers, foundDestinations
}

func inlineFormTriggerCount(literal string, triggers map[string]bool) int {
	found := make(map[string]bool)
	for _, word := range strings.Fields(literal) {
		normalized := strings.Trim(word, "\t\n\r ,.;:!?()[]{}\"'")
		longest := ""
		for trigger := range triggers {
			if trigger == "%" && isFormattingDirective(normalized) {
				continue
			}
			if len(normalized) > len(trigger) && len(trigger) > len(longest) && strings.HasPrefix(normalized, trigger) {
				longest = trigger
			}
		}
		if longest != "" {
			found[longest] = true
		}
	}
	return len(found)
}

func isFormattingDirective(value string) bool {
	if len(value) != 2 || value[0] != '%' {
		return false
	}
	return strings.ContainsRune("vTtbcdoOqxXUeEfFgGsp", rune(value[1]))
}

func mapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	slices.Sort(keys)
	return keys
}
