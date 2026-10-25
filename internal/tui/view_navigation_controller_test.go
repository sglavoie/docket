package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/route"
)

func TestViewNavigationControllerResizeSizesViewsToBudget(t *testing.T) {
	t.Parallel()

	controller := newViewNavigationController()
	controller.resize(100, 30)

	if width, height := controller.size(); width != 100 || height != 30 {
		t.Fatalf("size() = %dx%d, want 100x30", width, height)
	}
	// Every renderer is sized to height-2, the budget ViewRegistry stores and
	// re-applies on Register.
	if width, height := controller.viewSize(); width != 100 || height != 28 {
		t.Fatalf("viewSize() = %dx%d, want 100x28", width, height)
	}
	if controller.tooSmall() {
		t.Fatal("tooSmall() at 100x30, want false")
	}
}

func TestViewNavigationControllerResizeMarksEitherAxisTooSmall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		width, height int
		tooNarrow     bool
		tooShort      bool
	}{
		{"minimum", chrome.MinTerminalWidth, chrome.MinTerminalHeight, false, false},
		{"narrow", chrome.MinTerminalWidth - 1, chrome.MinTerminalHeight, true, false},
		{"short", chrome.MinTerminalWidth, chrome.MinTerminalHeight - 1, false, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			controller := newViewNavigationController()
			controller.resize(testCase.width, testCase.height)
			if controller.tooNarrow != testCase.tooNarrow || controller.tooShort != testCase.tooShort {
				t.Fatalf("resize(%d, %d) tooNarrow=%v tooShort=%v, want %v/%v",
					testCase.width, testCase.height,
					controller.tooNarrow, controller.tooShort,
					testCase.tooNarrow, testCase.tooShort)
			}
			if got := controller.tooSmall(); got != (testCase.tooNarrow || testCase.tooShort) {
				t.Fatalf("tooSmall() = %v", got)
			}
		})
	}
}

func TestViewNavigationControllerAtRootTracksBothStacks(t *testing.T) {
	t.Parallel()

	controller := newViewNavigationController()
	if !controller.atRoot() {
		t.Fatal("a freshly constructed controller is the bare Outliner root")
	}

	overlay, err := route.NewOverlay(route.OverlayComposer, taskComposerOverlayKey)
	if err != nil {
		t.Fatalf("NewOverlay: %v", err)
	}
	controller.navigation.Overlays.Push(overlay, nil)
	if controller.atRoot() {
		t.Fatal("an open overlay is something Esc can close, so atRoot() must be false")
	}

	controller.navigation.Overlays.CancelAll()
	if !controller.atRoot() {
		t.Fatal("canceling the overlay returns navigation to the root")
	}
}

// TestViewNavigationClusterHasOneHome pins the cluster to its single named
// AppModel field. A later controller split must not re-introduce a second
// geometry, registry or navigation home on the god struct: that is how the two
// paths this session kept apart — showView's registration and View()'s
// paint-time injection — would drift toward reading different state.
func TestViewNavigationClusterHasOneHome(t *testing.T) {
	t.Parallel()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller for package directory")
	}
	appFile := filepath.Join(filepath.Dir(thisFile), "app.go")
	parsed, err := parser.ParseFile(token.NewFileSet(), appFile, nil, 0)
	if err != nil {
		t.Fatalf("parse app.go: %v", err)
	}

	clustered := map[string]bool{
		"navigation": true,
		"views":      true,
		"width":      true,
		"height":     true,
		"tooNarrow":  true,
		"tooShort":   true,
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
					t.Errorf("AppModel field %s belongs to the viewNav controller", name.Name)
				}
				if name.Name == "viewNav" {
					controllerFields++
				}
			}
		}
		return false
	})
	if controllerFields != 1 {
		t.Errorf("AppModel viewNav fields = %d, want exactly 1", controllerFields)
	}
}
