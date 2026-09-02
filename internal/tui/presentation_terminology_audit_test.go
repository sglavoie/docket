package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"regexp"
	"runtime"
	"strconv"
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
	// Anchored on this file's own source path so the audit covers the whole
	// internal/tui subtree from whatever package depth it ends up at.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate the presentation terminology audit")
	}

	fset := token.NewFileSet()
	err := walkTUIProductionSourcesFrom(t, thisFile, func(path, relative string, _ fs.DirEntry) error {
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		// Parse under the subtree-relative name so a position still reads as a
		// path a reader can type, not an absolute one.
		file, parseErr := parser.ParseFile(fset, relative, source, 0)
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
