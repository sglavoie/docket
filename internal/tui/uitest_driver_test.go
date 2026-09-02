package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/presentation"
)

// newUITestApp builds a minimal, fully-renderable AppModel (cross-project
// Outliner registered) for driver tests, without the real-data side effects of
// NewAppModel.
func newUITestApp() *AppModel {
	app := newTestAppModel()
	app.operationRegistry = NewOperationRegistry()
	app.operationExecutor = NewOperationExecutor(app.operationRegistry)
	o := NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner = o
	app.views.Register(presentation.ViewCrossProjectOutliner, o)
	setTestPresentationPath(app, presentation.ViewCrossProjectOutliner)
	if err := app.registerCoreInputOperations(); err != nil {
		panic(err)
	}
	if err := app.registerTaskActionOperations(); err != nil {
		panic(err)
	}
	if err := app.registerFilterOperations(); err != nil {
		panic(err)
	}
	if err := app.registerSortOperations(); err != nil {
		panic(err)
	}
	if err := app.registerProjectOperations(); err != nil {
		panic(err)
	}
	if err := app.registerProjectActionSheetOperation(); err != nil {
		panic(err)
	}
	if err := app.registerHarnessOperations(); err != nil {
		panic(err)
	}
	if err := app.registerChecklistRowOperations(); err != nil {
		panic(err)
	}
	if err := app.operationRegistry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		panic(err)
	}
	app.wireOutlinerOperationExecutor(app.crossOutliner)
	return app
}

func TestUITestDriverResizeAndRender(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)

	if got := driver.App().width; got != 100 {
		t.Fatalf("width = %d, want 100", got)
	}
	if got := driver.App().height; got != 30 {
		t.Fatalf("height = %d, want 30", got)
	}

	screen := driver.Render()
	if screen == "" {
		t.Fatal("Render() returned empty screen")
	}
	if strings.Contains(screen, "\x1b") {
		t.Fatalf("Render() screen still contains ANSI escape sequences: %q", screen)
	}

	driver.Resize(60, 20)
	if driver.App().width != 60 || driver.App().height != 20 {
		t.Fatalf("after resize got %dx%d, want 60x20", driver.App().width, driver.App().height)
	}
}

func TestUITestDriverTooSmall(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 40, 10)

	if !driver.App().tooNarrow || !driver.App().tooShort {
		t.Fatalf("expected tooNarrow/tooShort at 40x10, got tooNarrow=%v tooShort=%v",
			driver.App().tooNarrow, driver.App().tooShort)
	}
	if screen := driver.Render(); !strings.Contains(screen, "Terminal too small") {
		t.Fatalf("expected too-small message, got %q", screen)
	}
}

func TestUITestDriverSendKey(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)

	// Sending a valid key must not error and must not panic.
	if err := driver.SendKey("esc"); err != nil {
		t.Fatalf("SendKey(esc) error: %v", err)
	}

	// An invalid key name is reported as an error.
	if err := driver.SendKey("ctrl+"); err == nil {
		t.Fatal("SendKey(ctrl+) expected error, got nil")
	}
}
