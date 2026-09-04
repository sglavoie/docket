package tui

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/operation"
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

// TestOperationControllerHoldsOneRegistryAndOneExecutor reads the cluster's own
// field list, which is the one place a second descriptor authority or a second
// dispatch pipeline could be introduced without any call site changing. Exactly
// one of each may be named, and no field may name a router, a palette, or a
// handler table of its own.
func TestOperationControllerHoldsOneRegistryAndOneExecutor(t *testing.T) {
	t.Parallel()

	fields := operationControllerFields(t)
	registries, executors := 0, 0
	for name, typeExpr := range fields {
		switch typeExpr {
		case "*operation.OperationRegistry":
			registries++
		case "*operation.OperationExecutor":
			executors++
		}
		for _, forbidden := range []string{"router", "Router", "palette", "Palette", "handlers", "Handlers"} {
			if name == forbidden {
				t.Errorf("operationController field %s is a second route to an operation", name)
			}
		}
	}
	if registries != 1 {
		t.Errorf("operationController OperationRegistry fields = %d, want exactly 1", registries)
	}
	if executors != 1 {
		t.Errorf("operationController OperationExecutor fields = %d, want exactly 1", executors)
	}
}

func operationControllerFields(t *testing.T) map[string]string {
	t.Helper()

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, shellSourceFile(t, "operation_controller.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse operation_controller.go: %v", err)
	}

	fields := map[string]string{}
	ast.Inspect(parsed, func(node ast.Node) bool {
		spec, isType := node.(*ast.TypeSpec)
		if !isType || spec.Name.Name != "operationController" {
			return true
		}
		controller, isStruct := spec.Type.(*ast.StructType)
		if !isStruct {
			return true
		}
		for _, field := range controller.Fields.List {
			for _, name := range field.Names {
				fields[name.Name] = types.ExprString(field.Type)
			}
		}
		return false
	})
	if len(fields) == 0 {
		t.Fatal("operationController declares no fields")
	}
	return fields
}

// registrarCatalogOrder is the fifteen families, in the order the composition
// root must hand them over. The order is load-bearing: a later family can bind
// a key an earlier one already declared, so reordering the table changes which
// registration fails rather than making both succeed.
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
			if !isSlice || types.ExprString(table.Elt) != "namedOperationRegistrar" {
				return true
			}
			for _, element := range composite.Elts {
				entry, isEntry := element.(*ast.CompositeLit)
				if !isEntry || len(entry.Elts) != 2 {
					continue
				}
				selector, isSelector := entry.Elts[1].(*ast.SelectorExpr)
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

// TestRegisterCatalogStopsAtTheFirstFailingFamily pins the hard-fail: a family
// that will not register stops construction, names itself in the error, and
// leaves every later family unrun, so a half-built catalog can never reach the
// startup checks behind it.
func TestRegisterCatalogStopsAtTheFirstFailingFamily(t *testing.T) {
	t.Parallel()

	controller := newOperationController()
	controller.bindExecutor()

	var ran []string
	record := func(name string) func() error {
		return func() error {
			ran = append(ran, name)
			return nil
		}
	}
	boom := errors.New("descriptor is invalid")
	err := controller.registerCatalog([]namedOperationRegistrar{
		{"first", record("first")},
		{"second", func() error { return boom }},
		{"third", record("third")},
	})
	if !errors.Is(err, boom) {
		t.Fatalf("registerCatalog error = %v, want the family's own error wrapped", err)
	}
	if got := err.Error(); got != fmt.Sprintf("register second operations: %v", boom) {
		t.Errorf("registerCatalog error text = %q, want the failing family named", got)
	}
	if len(ran) != 1 || ran[0] != "first" {
		t.Errorf("registrars run = %v, want only the families before the failure", ran)
	}
}

// TestRegisterCatalogHardFailsOnAnEmptyTable pins the first check behind the
// table. Every surface grammar is declared against the catalog, so a table that
// registered nothing leaves the strategies naming operations that do not exist,
// and startup refuses rather than installing a grammar whose keys resolve to
// nothing.
func TestRegisterCatalogHardFailsOnAnEmptyTable(t *testing.T) {
	t.Parallel()

	controller := newOperationController()
	controller.bindExecutor()

	err := controller.registerCatalog(nil)
	if err == nil {
		t.Fatal("an empty catalog leaves every strategy dangling, so startup must refuse")
	}
	if !strings.Contains(err.Error(), "register simplified input strategy") {
		t.Errorf("registerCatalog error = %q, want the strategy registration refusal", err)
	}
}

// TestRegisterCatalogRunsBothStartupChecksAfterTheTable pins the boot order,
// which the runtime cannot show: reaching menu validation needs a catalog
// complete enough that no earlier check refuses, and a complete catalog passes
// all three. The order is load-bearing in one direction — the grammar is
// validated and installed against the registered descriptors, and the menu bar
// is resolved against the installed grammar — so it is read out of the source
// rather than inferred from a passing boot.
func TestRegisterCatalogRunsBothStartupChecksAfterTheTable(t *testing.T) {
	t.Parallel()

	body := operationControllerFunctionBody(t, "registerCatalog")
	table := strings.Index(body, "registration.register()")
	strategies := strings.Index(body, "c.registerSurfaceInputStrategies()")
	menus := strings.Index(body, "c.validateMenuBarSpecs()")
	switch {
	case table < 0:
		t.Fatal("registerCatalog no longer runs the registrar table")
	case strategies < 0:
		t.Fatal("registerCatalog no longer registers the surface input strategies")
	case menus < 0:
		t.Fatal("registerCatalog no longer validates the menu bar specs")
	case table >= strategies || strategies >= menus:
		t.Errorf("registerCatalog runs the table, strategies and menus at offsets %d, %d, %d, want that order", table, strategies, menus)
	}
}

func operationControllerFunctionBody(t *testing.T, name string) string {
	t.Helper()

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, shellSourceFile(t, "operation_controller.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse operation_controller.go: %v", err)
	}

	for _, decl := range parsed.Decls {
		function, isFunc := decl.(*ast.FuncDecl)
		if !isFunc || function.Name.Name != name || function.Body == nil {
			continue
		}
		var rendered strings.Builder
		if err := printer.Fprint(&rendered, fileSet, function.Body); err != nil {
			t.Fatalf("render %s body: %v", name, err)
		}
		return rendered.String()
	}
	t.Fatalf("operationController declares no %s method", name)
	return ""
}

// TestRegisterRequiresBothRegistries pins the pairing: a descriptor may not be
// declared where no executor can bind behavior to it, because a registry entry
// with nothing behind it is a key that answers with silence.
func TestRegisterRequiresBothRegistries(t *testing.T) {
	t.Parallel()

	unbound := newOperationController()
	if unbound.wired() {
		t.Fatal("a controller with no executor is not wired")
	}
	err := unbound.register(operation.OperationDescriptor{ID: "test.op"}, operation.OperationBinding{})
	if err == nil || err.Error() != "operation registry and executor are required" {
		t.Fatalf("register on an unwired controller = %v, want the required-pair refusal", err)
	}

	unbound.bindExecutor()
	if !unbound.wired() {
		t.Fatal("binding the executor wires the controller")
	}
}

// TestOperationConfirmationIsModalWhileArmed pins the contextual destructive
// prompt's reading. An armed prompt consumes every key; confirming hands the
// frozen request back exactly once; and canceling releases it, so the sheet
// beneath is returned to with nothing executed.
func TestOperationConfirmationIsModalWhileArmed(t *testing.T) {
	t.Parallel()

	request := operation.OperationExecutionRequest{ID: "test.delete", Confirmed: true}

	controller := newOperationController()
	if answer := controller.resolveConfirmKey("y"); answer != confirmAnswerUnarmed {
		t.Errorf("unarmed answer = %v, want the key left to the view beneath", answer)
	}

	controller.armConfirm(request, "Delete 2 targets?")
	if _, armed := controller.confirmOverlay(120); !armed {
		t.Fatal("an armed prompt renders its overlay")
	}
	if answer := controller.resolveConfirmKey("j"); answer != confirmAnswerSwallowed {
		t.Errorf("unanswered key = %v, want it swallowed rather than leaked", answer)
	}
	if answer := controller.resolveConfirmKey("y"); answer != confirmAnswerConfirmed {
		t.Fatalf("y = %v, want confirmed", answer)
	}
	if taken := controller.takeRequest(); taken.ID != request.ID || !taken.Confirmed {
		t.Fatalf("taken request = %+v, want the frozen request", taken)
	}
	if taken := controller.takeRequest(); taken.ID != "" {
		t.Errorf("second take = %+v, want the request released by the first", taken)
	}

	controller.armConfirm(request, "Delete 2 targets?")
	if answer := controller.resolveConfirmKey("esc"); answer != confirmAnswerCanceled {
		t.Fatalf("esc = %v, want canceled", answer)
	}
	if pending := controller.pendingRequest; pending.ID != "" {
		t.Errorf("canceled request = %+v, want it released", pending)
	}
	if _, armed := controller.confirmOverlay(120); armed {
		t.Error("a canceled prompt renders no overlay")
	}
}

// TestClearOverlaysDismissesTheWholeFlow pins that no level of the two-level
// flow outlives the route it was opened over: the sheet, whichever value step
// is mounted, and the confirmation with its frozen request all go together.
func TestClearOverlaysDismissesTheWholeFlow(t *testing.T) {
	t.Parallel()

	controller := newOperationController()
	if controller.overlayMounted() {
		t.Fatal("a fresh controller mounts no overlay")
	}

	controller.textPrompt = &OperationTextPromptModel{}
	if !controller.overlayMounted() {
		t.Error("a mounted text prompt is an active overlay capturing text")
	}
	controller.clearValueStep()
	if controller.overlayMounted() {
		t.Error("clearValueStep dismisses the mounted value step")
	}

	controller.valuePicker = &OperationValuePickerModel{}
	controller.armConfirm(operation.OperationExecutionRequest{ID: "test.op"}, "Sure?")
	controller.clearOverlays()
	if controller.overlayMounted() {
		t.Error("clearOverlays dismisses the value step")
	}
	if _, armed := controller.confirmOverlay(120); armed {
		t.Error("clearOverlays dismisses the confirmation")
	}
	if pending := controller.pendingRequest; pending.ID != "" {
		t.Errorf("cleared request = %+v, want it released with the overlays", pending)
	}
}
