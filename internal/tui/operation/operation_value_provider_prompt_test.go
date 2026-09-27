package operation

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// TestEveryValueProviderDeclaresItsPrompt reads the provider constants from
// their declaring file, so a provider added without a phrase fails here rather
// than reaching a menu row as its hyphenated registry key.
func TestEveryValueProviderDeclaresItsPrompt(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "operation_value_provider_ids.go", nil, 0)
	if err != nil {
		t.Fatalf("parse provider ids: %v", err)
	}
	seen := 0
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if !ok || len(spec.Values) != len(spec.Names) {
			return true
		}
		for i, value := range spec.Values {
			literal, ok := value.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				continue
			}
			raw, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatalf("unquote %s: %v", literal.Value, err)
			}
			id := ValueProviderID(raw)
			seen++
			fallback := "choose " + strings.ReplaceAll(raw, "-", " ")
			if got := id.Prompt(); got == fallback {
				t.Errorf("%s (%q) has no prompt of its own; menu rows would read %q", spec.Names[i].Name, raw, got)
			}
		}
		return true
	})
	if seen == 0 {
		t.Fatal("found no provider constants to check")
	}
}
