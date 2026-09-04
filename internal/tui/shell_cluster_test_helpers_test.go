package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"
)

// The source-reading helpers the controller-cluster tests share. A cluster's
// field list and the shell's boot order are only stated in the source, so these
// assertions read the source rather than a running model — and they live in a
// file that declares no test, because a helper this many files read is
// infrastructure and a filename-based move of its file would orphan its readers.

// shellSourceFile resolves a file in this package by name.
func shellSourceFile(t *testing.T, name string) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller for package directory")
	}
	return filepath.Join(filepath.Dir(thisFile), name)
}

// assertAppModelClusterHasOneHome walks the AppModel type spec and reports both
// halves of a controller extraction: no retired field came back, and the
// controller itself is named exactly once.
func assertAppModelClusterHasOneHome(t *testing.T, controller string, clustered map[string]bool) {
	t.Helper()

	parsed, err := parser.ParseFile(token.NewFileSet(), shellSourceFile(t, "app.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse app.go: %v", err)
	}

	controllerFields := 0
	ast.Inspect(parsed, func(node ast.Node) bool {
		spec, isType := node.(*ast.TypeSpec)
		if !isType || spec.Name.Name != "AppModel" {
			return true
		}
		model, isStruct := spec.Type.(*ast.StructType)
		if !isStruct {
			return true
		}
		for _, field := range model.Fields.List {
			for _, name := range field.Names {
				if clustered[name.Name] {
					t.Errorf("AppModel field %s belongs to the %s controller", name.Name, controller)
				}
				if name.Name == controller {
					controllerFields++
				}
			}
		}
		return false
	})
	if controllerFields != 1 {
		t.Errorf("AppModel %s fields = %d, want exactly 1", controller, controllerFields)
	}
}
