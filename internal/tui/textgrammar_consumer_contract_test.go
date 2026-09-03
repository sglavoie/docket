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

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/textgrammar"
)

// The capture half of this contract — the autocomplete triggers, the static
// suggestion values, and the composer's syntax guide — moved into
// internal/tui/grammar with the surfaces it projects. What stays here is the
// half that names shell declarations: the `?` grammar section, the outline
// header forms, and the file scan that holds both sides to one registry.

// TestHelpGrammarSectionComesFromCanonicalRegistry holds the `?` grammar section
// to the same teach order the composer guide reads, over the union of the three
// surfaces a sigil can be typed on. It is the second GuideDefinitions consumer,
// so a new sigil that reaches the composer must reach `?` in the same pass.
func TestHelpGrammarSectionComesFromCanonicalRegistry(t *testing.T) {
	surfaces := textgrammar.SurfaceCapture | textgrammar.SurfaceEdit | textgrammar.SurfaceSearch
	definitions := textgrammar.GuideDefinitions(surfaces)
	items := grammarHelpItems()
	if len(items) != len(definitions) {
		t.Fatalf("grammar help rows = %d, want %d registry definitions", len(items), len(definitions))
	}
	for i, definition := range definitions {
		item := items[i]
		if got, want := item.Key, strings.Join(definition.Forms, " "); got != want {
			t.Errorf("%s key = %q, want %q", definition.Field, got, want)
		}
		if !strings.HasPrefix(item.Description, definition.Description) {
			t.Errorf("%s description = %q, want it to lead with %q", definition.Field, item.Description, definition.Description)
		}
		for _, clear := range definition.Clears {
			if !strings.Contains(item.Description, clear.Description) {
				t.Errorf("%s description = %q, want it to name the clear form %q", definition.Field, item.Description, clear.Description)
			}
		}
		if item.Category != helpGrammarCategory {
			t.Errorf("%s category = %q, want %q", definition.Field, item.Category, helpGrammarCategory)
		}
		// The key column is a fixed cell: a wider spelling wraps mid-token and the
		// row stops reading as one form.
		if width := lipgloss.Width(item.Key); width > helpKeyColumnWidth {
			t.Errorf("%s key %q is %d cells, want at most %d", definition.Field, item.Key, width, helpKeyColumnWidth)
		}
		for _, alias := range item.Aliases {
			if strings.TrimSpace(alias) == "" {
				t.Errorf("%s carries a blank alias", definition.Field)
			}
		}
	}

	// The deferral sigil is the whole reason the section exists: it has no key of
	// its own, so `?` is the only place outside the composer that can teach it.
	// The row is found by prefix rather than by substring: -@@defer is a row of its
	// own now, and its key contains the positive form's spelling.
	deferRow := -1
	for i, item := range items {
		if strings.HasPrefix(item.Key, "@@defer") {
			deferRow = i
		}
	}
	if deferRow < 0 {
		t.Fatalf("grammar help omits @@defer: %#v", items)
	}
	if !slices.Contains(items[deferRow].Aliases, "@@any") || !slices.Contains(items[deferRow].Aliases, "@@none") {
		t.Errorf("deferral aliases = %v, want the search-only presets", items[deferRow].Aliases)
	}

	// The @+Nh due window is search-only, so `?` is its only in-app advertising
	// surface beside the README. It rides the due row as a searchable alias the
	// way the deferral presets ride the defer row.
	dueRow := -1
	for i, item := range items {
		if strings.HasPrefix(item.Key, "@date") {
			dueRow = i
		}
	}
	if dueRow < 0 {
		t.Fatalf("grammar help omits @date: %#v", items)
	}
	if !slices.Contains(items[dueRow].Aliases, "@+4h") {
		t.Errorf("due aliases = %v, want the search-only @+4h hour window", items[dueRow].Aliases)
	}
	if !strings.Contains(items[dueRow].Description, "@+4h") {
		t.Errorf("due description = %q, want it to visibly show the @+4h search window", items[dueRow].Description)
	}
	if !strings.Contains(items[deferRow].Description, "@@any") || !strings.Contains(items[deferRow].Description, "@@none") {
		t.Errorf("deferral description = %q, want it to visibly show the @@any/@@none search forms", items[deferRow].Description)
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
	// Five of the six moved into the grammar package in Session 2.11; the rule
	// follows the file, not the package, so this stays one contract spanning the
	// shell's outline editor and the grammar it delegates to.
	consumerFiles := []string{
		"outline_editor.go",
		"grammar/outline_editor_parse.go",
		"grammar/quick_capture_autocomplete.go",
		"grammar/quick_capture_input.go",
		"grammar/quick_capture_parser.go",
		"grammar/quick_capture_parser_fields.go",
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
