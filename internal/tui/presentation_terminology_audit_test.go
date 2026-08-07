package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var retiredPresentationCopy = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{name: "c scope", pattern: regexp.MustCompile(`(?i)\bc\s+scope\b`)},
	{name: "queue", pattern: regexp.MustCompile(`(?i)\bqueue\b`)},
	{name: "open loops", pattern: regexp.MustCompile(`(?i)\bopen[ -]?loops?\b`)},
	{name: "quoted single-character key", pattern: regexp.MustCompile(`(?i)\bpress\s+['\"][^'\"]['\"]`)},
	{name: "colon-style key hint", pattern: regexp.MustCompile(`(?i)(?:^| · )\s*(?:enter|space|esc|tab|shift\+tab|ctrl\+[a-z]|[a-z]):\s+(?:add|toggle|edit|check|delete|del|move|open|close|cancel|save|apply|navigate|select)\b`)},
}

// TestProductionPresentationCopyUsesFinalGrammar prevents retired labels and
// raw key-hint punctuation from returning to user-visible production strings.
// Identifiers and comments are intentionally outside this copy-only audit.
func TestProductionPresentationCopyUsesFinalGrammar(t *testing.T) {
	fset := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, unquoteErr := strconv.Unquote(literal.Value)
			if unquoteErr != nil {
				t.Errorf("%s: unquote string: %v", fset.Position(literal.Pos()), unquoteErr)
				return true
			}
			for _, retired := range retiredPresentationCopy {
				if retired.pattern.MatchString(value) {
					t.Errorf("%s: production copy contains retired %s grammar: %q", fset.Position(literal.Pos()), retired.name, value)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("audit TUI production copy: %v", err)
	}
}
