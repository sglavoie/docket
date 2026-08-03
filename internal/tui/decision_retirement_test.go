package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestDecisionVocabularyIsAbsentFromProductionTUI(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate decision retirement audit")
	}
	tuiRoot := filepath.Dir(testFile)
	fset := token.NewFileSet()
	var violations []string
	err := filepath.WalkDir(tuiRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if containsDecisionVocabulary(entry.Name()) {
			violations = append(violations, entry.Name()+": production filename")
		}
		file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			var value string
			switch node := node.(type) {
			case *ast.Ident:
				value = node.Name
			case *ast.BasicLit:
				if node.Kind != token.STRING {
					return true
				}
				unquoted, unquoteErr := strconv.Unquote(node.Value)
				if unquoteErr != nil {
					value = node.Value
				} else {
					value = unquoted
				}
			default:
				return true
			}
			if containsDecisionVocabulary(value) {
				position := fset.Position(node.Pos())
				relative, _ := filepath.Rel(tuiRoot, path)
				violations = append(violations, filepath.ToSlash(relative)+":"+strconv.Itoa(position.Line)+": "+value)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan production TUI sources: %v", err)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("decision-specific TUI vocabulary returned:\n%s", strings.Join(violations, "\n"))
	}
}

func containsDecisionVocabulary(value string) bool {
	return strings.Contains(strings.ToLower(value), "decision")
}
