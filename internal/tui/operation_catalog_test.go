package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

// TestOperationClusterHasOneHome pins the operation-wiring cluster to its
// single named AppModel field. The registry is the descriptor authority and the
// executor the one invocation pipeline, so a second home on the god struct is
// literally how a parallel router would start.
func TestOperationClusterHasOneHome(t *testing.T) {
	t.Parallel()

	clustered := map[string]bool{
		"operationRegistry":       true,
		"operationExecutor":       true,
		"contextActionSheet":      true,
		"operationValuePicker":    true,
		"operationTextPrompt":     true,
		"operationConfirm":        true,
		"pendingOperationRequest": true,
		"pendingChord":            true,
	}
	assertAppModelClusterHasOneHome(t, "operations", clustered)
}

// registrarCatalogOrder is the fixed boot order of the registrar table, held
// here so a sixteenth family has to be named deliberately.
var registrarCatalogOrder = []string{
	"registerCoreInputOperations",
	"registerTaskActionOperations",
	"registerFilterOperations",
	"registerSortOperations",
	"registerProjectHubOperations",
	"registerProjectOperations",
	"registerProjectActionSheetOperation",
	"registerProjectBulkOperations",
	"registerProjectTagVocabularyOperations",
	"registerNoteOperations",
	"registerNotesSurfaceOperations",
	"registerAttachmentOperations",
	"registerCommentOperations",
	"registerChecklistRowOperations",
	"registerLensOperations",
}

// TestRegisterOperationCatalogNamesFifteenRegistrarsInOrder reads the table out
// of app_init.go's AST rather than out of a running model, because the order is
// only observable there: once the catalog is built, nothing in the registry
// records which registrar declared what. A sixteenth family therefore has to be
// added here deliberately rather than joining the boot sequence unexamined.
func TestRegisterOperationCatalogNamesFifteenRegistrarsInOrder(t *testing.T) {
	t.Parallel()

	parsed, err := parser.ParseFile(token.NewFileSet(), shellSourceFile(t, "app_init.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse app_init.go: %v", err)
	}

	var named []string
	ast.Inspect(parsed, func(node ast.Node) bool {
		decl, isFunc := node.(*ast.FuncDecl)
		if !isFunc || decl.Name.Name != "registerOperationCatalog" {
			return true
		}
		ast.Inspect(decl, func(inner ast.Node) bool {
			composite, isComposite := inner.(*ast.CompositeLit)
			if !isComposite {
				return true
			}
			table, isSlice := composite.Type.(*ast.ArrayType)
			if !isSlice || types.ExprString(table.Elt) != "operationhost.NamedRegistrar" {
				return true
			}
			for _, element := range composite.Elts {
				entry, isEntry := element.(*ast.CompositeLit)
				if !isEntry || len(entry.Elts) != 2 {
					continue
				}
				value := entry.Elts[1]
				if keyed, isKeyed := value.(*ast.KeyValueExpr); isKeyed {
					value = keyed.Value
				}
				selector, isSelector := value.(*ast.SelectorExpr)
				if !isSelector {
					continue
				}
				named = append(named, selector.Sel.Name)
			}
			return false
		})
		return false
	})

	if len(named) != len(registrarCatalogOrder) {
		t.Fatalf("registerOperationCatalog names %d registrars, want %d", len(named), len(registrarCatalogOrder))
	}
	for index, want := range registrarCatalogOrder {
		if named[index] != want {
			t.Errorf("registrar %d is %s, want %s", index, named[index], want)
		}
	}
}
