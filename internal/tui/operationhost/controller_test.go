package operationhost

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/valuepicker"
)

// TestControllerHoldsOneRegistryAndOneExecutor reads the cluster's own field
// list, which is the one place a second descriptor authority or a second
// dispatch pipeline could be introduced without any call site changing. Exactly
// one of each may be named, and no field may name a router, a palette, or a
// handler table of its own.
func TestControllerHoldsOneRegistryAndOneExecutor(t *testing.T) {
	t.Parallel()

	fields := controllerFields(t)
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
				t.Errorf("Controller field %s is a second route to an operation", name)
			}
		}
	}
	if registries != 1 {
		t.Errorf("Controller OperationRegistry fields = %d, want exactly 1", registries)
	}
	if executors != 1 {
		t.Errorf("Controller OperationExecutor fields = %d, want exactly 1", executors)
	}
}

func controllerFields(t *testing.T) map[string]string {
	t.Helper()

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, "controller.go", nil, 0)
	if err != nil {
		t.Fatalf("parse controller.go: %v", err)
	}

	fields := make(map[string]string)
	ast.Inspect(parsed, func(node ast.Node) bool {
		spec, isSpec := node.(*ast.TypeSpec)
		if !isSpec || spec.Name.Name != "Controller" {
			return true
		}
		structType, isStruct := spec.Type.(*ast.StructType)
		if !isStruct {
			return false
		}
		for _, field := range structType.Fields.List {
			var rendered strings.Builder
			if err := printer.Fprint(&rendered, fileSet, field.Type); err != nil {
				t.Fatalf("render field type: %v", err)
			}
			for _, name := range field.Names {
				fields[name.Name] = rendered.String()
			}
		}
		return false
	})
	if len(fields) == 0 {
		t.Fatal("controller.go declares no Controller fields")
	}
	return fields
}

// TestRegisterCatalogStopsAtTheFirstFailingFamily pins the hard-fail: a family
// that will not register stops construction, names itself in the error, and
// leaves every later family unrun, so a half-built catalog can never reach the
// startup checks behind it.
func TestRegisterCatalogStopsAtTheFirstFailingFamily(t *testing.T) {
	t.Parallel()

	controller := NewController()
	controller.BindExecutor()

	var ran []string
	record := func(name string) func() error {
		return func() error {
			ran = append(ran, name)
			return nil
		}
	}
	boom := errors.New("descriptor is invalid")
	err := controller.RegisterCatalog([]NamedRegistrar{
		{Name: "first", Register: record("first")},
		{Name: "second", Register: func() error { return boom }},
		{Name: "third", Register: record("third")},
	}, nil)
	if !errors.Is(err, boom) {
		t.Fatalf("RegisterCatalog error = %v, want the family's own error wrapped", err)
	}
	if got := err.Error(); got != fmt.Sprintf("register second operations: %v", boom) {
		t.Errorf("RegisterCatalog error text = %q, want the failing family named", got)
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

	controller := NewController()
	controller.BindExecutor()

	gates := []SurfaceGate{{Strategy: operation.InputStrategy{
		ID:       "simplified",
		Bindings: []operation.OperationKeyBinding{{OperationID: "never.registered", Keys: []string{"x"}}},
	}}}
	err := controller.RegisterCatalog(nil, gates)
	if err == nil {
		t.Fatal("an empty catalog leaves every strategy dangling, so startup must refuse")
	}
	if !strings.Contains(err.Error(), "register simplified input strategy") {
		t.Errorf("RegisterCatalog error = %q, want the strategy registration refusal", err)
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

	body := controllerFunctionBody(t, "RegisterCatalog")
	table := strings.Index(body, "registration.Register()")
	strategies := strings.Index(body, "c.registerSurfaceInputStrategies(gates)")
	menus := strings.Index(body, "c.validateMenuBarSpecs()")
	switch {
	case table < 0:
		t.Fatal("RegisterCatalog no longer runs the registrar table")
	case strategies < 0:
		t.Fatal("RegisterCatalog no longer registers the surface input strategies")
	case menus < 0:
		t.Fatal("RegisterCatalog no longer validates the menu bar specs")
	case table >= strategies || strategies >= menus:
		t.Errorf(
			"RegisterCatalog runs the table, strategies and menus at offsets %d, %d, %d, want that order",
			table, strategies, menus,
		)
	}
}

func controllerFunctionBody(t *testing.T, name string) string {
	t.Helper()

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, "controller.go", nil, 0)
	if err != nil {
		t.Fatalf("parse controller.go: %v", err)
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
	t.Fatalf("Controller declares no %s method", name)
	return ""
}

// TestRegisterRequiresBothRegistries pins the pairing: a descriptor may not be
// declared where no executor can bind behavior to it, because a registry entry
// with nothing behind it is a key that answers with silence.
func TestRegisterRequiresBothRegistries(t *testing.T) {
	t.Parallel()

	unbound := NewController()
	if unbound.Wired() {
		t.Fatal("a controller with no executor is not wired")
	}
	err := unbound.Register(operation.OperationDescriptor{ID: "test.op"}, operation.OperationBinding{})
	if err == nil || err.Error() != "operation registry and executor are required" {
		t.Fatalf("Register on an unwired controller = %v, want the required-pair refusal", err)
	}

	unbound.BindExecutor()
	if !unbound.Wired() {
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

	controller := NewController()
	if answer := controller.ResolveConfirmKey("y"); answer != chrome.ConfirmAnswerUnarmed {
		t.Errorf("unarmed answer = %v, want the key left to the view beneath", answer)
	}

	controller.ArmConfirm(request, "Delete 2 targets?")
	if _, armed := controller.ConfirmOverlay(120); !armed {
		t.Fatal("an armed prompt renders its overlay")
	}
	if answer := controller.ResolveConfirmKey("j"); answer != chrome.ConfirmAnswerSwallowed {
		t.Errorf("unanswered key = %v, want it swallowed rather than leaked", answer)
	}
	if answer := controller.ResolveConfirmKey("y"); answer != chrome.ConfirmAnswerConfirmed {
		t.Fatalf("y = %v, want confirmed", answer)
	}
	if taken := controller.TakeRequest(); taken.ID != request.ID || !taken.Confirmed {
		t.Fatalf("taken request = %+v, want the frozen request", taken)
	}
	if taken := controller.TakeRequest(); taken.ID != "" {
		t.Errorf("second take = %+v, want the request released by the first", taken)
	}

	controller.ArmConfirm(request, "Delete 2 targets?")
	if answer := controller.ResolveConfirmKey("esc"); answer != chrome.ConfirmAnswerCanceled {
		t.Fatalf("esc = %v, want canceled", answer)
	}
	if pending := controller.pendingRequest; pending.ID != "" {
		t.Errorf("canceled request = %+v, want it released", pending)
	}
	if _, armed := controller.ConfirmOverlay(120); armed {
		t.Error("a canceled prompt renders no overlay")
	}
}

// TestClearOverlaysDismissesTheWholeFlow pins that no level of the two-level
// flow outlives the route it was opened over: the sheet, whichever value step
// is mounted, and the confirmation with its frozen request all go together.
func TestClearOverlaysDismissesTheWholeFlow(t *testing.T) {
	t.Parallel()

	controller := NewController()
	if controller.OverlayMounted() {
		t.Fatal("a fresh controller mounts no overlay")
	}

	controller.TextPrompt = &OperationTextPromptModel{}
	if !controller.OverlayMounted() {
		t.Error("a mounted text prompt is an active overlay capturing text")
	}
	controller.ClearValueStep()
	if controller.OverlayMounted() {
		t.Error("ClearValueStep dismisses the mounted value step")
	}

	controller.ValuePicker = &valuepicker.OperationValuePickerModel{}
	controller.ArmConfirm(operation.OperationExecutionRequest{ID: "test.op"}, "Sure?")
	controller.ClearOverlays()
	if controller.OverlayMounted() {
		t.Error("ClearOverlays dismisses the value step")
	}
	if _, armed := controller.ConfirmOverlay(120); armed {
		t.Error("ClearOverlays dismisses the confirmation")
	}
	if pending := controller.pendingRequest; pending.ID != "" {
		t.Errorf("cleared request = %+v, want it released with the overlays", pending)
	}
}
