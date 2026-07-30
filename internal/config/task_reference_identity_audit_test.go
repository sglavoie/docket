package config

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestKnownOwnerProductionPathsAvoidIDOnlyTaskReferenceHelpers(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate audit source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	forbidden := map[string]bool{
		"RemoveDurableTaskRefs":          true,
		"RemoveMachineLocalTaskRefs":     true,
		"ReassignDurablePlanningProject": true,
		"RemoveFromNextOrder":            true,
		"RemoveFromToday":                true,
	}

	for _, relDir := range []string{"cmd", "internal/app", "internal/tui"} {
		dir := filepath.Join(repoRoot, relDir)
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			fileSet := token.NewFileSet()
			file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
			if parseErr != nil {
				return parseErr
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, isCall := node.(*ast.CallExpr)
				if !isCall {
					return true
				}
				selector, isSelector := call.Fun.(*ast.SelectorExpr)
				if !isSelector || !forbidden[selector.Sel.Name] {
					return true
				}
				pos := fileSet.Position(selector.Sel.Pos())
				t.Errorf("%s:%d calls ID-only compatibility helper %s; use the project-qualified seam", filepath.ToSlash(strings.TrimPrefix(path, repoRoot+string(filepath.Separator))), pos.Line, selector.Sel.Name)
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("audit %s: %v", relDir, err)
		}
	}
}
