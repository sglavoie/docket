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

	"github.com/sglavoie/tt/internal/textgrammar"
)

// The capture half of this contract — the autocomplete triggers, the static
// suggestion values, and the composer's syntax guide — moved into
// internal/tui/grammar with the surfaces it projects, and the `?` grammar
// section followed its projection into internal/tui/help in Session 2.21. What
// stays here is the half that names shell declarations: the outline header
// forms, and the file scan that holds every side to one registry.

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
	// The rule follows the file, not the package, so this stays one contract
	// spanning the shell's outline editor, the grammar it delegates to, and the
	// frontend-free capture parser in internal/capturetext.
	consumerFiles := []string{
		"outline_editor.go",
		"grammar/outline_editor_parse.go",
		"grammar/quick_capture_autocomplete.go",
		"grammar/quick_capture_input.go",
		"../capturetext/parse.go",
		"../capturetext/fields.go",
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
