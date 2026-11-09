package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/control"
	"github.com/sglavoie/docket/internal/icons"
	"github.com/sglavoie/docket/internal/tui/presentation"
)

func TestMotionToggleWritesMachineStateAndHoldsArmedMotionOff(t *testing.T) {
	driver, outlinerView := menuBarDriver(t)
	app := driver.App()
	WithMotion()(app)
	if !app.lifecycle.motion || !outlinerView.MotionPreferred() {
		t.Fatal("an armed session with no preference does not animate")
	}
	if err := app.coreToggleMotion(); err != nil {
		t.Fatalf("coreToggleMotion: %v", err)
	}
	if !app.domain.runtime.Machine().MotionDisabled {
		t.Fatal("turning motion off did not reach machine state")
	}
	if app.lifecycle.motion || outlinerView.MotionPreferred() {
		t.Fatal("the session still animates after motion was turned off")
	}
	if app.startUndoCountdown() != nil {
		t.Fatal("the undo countdown started with motion off")
	}
	if err := app.coreToggleMotion(); err != nil {
		t.Fatalf("coreToggleMotion back: %v", err)
	}
	if app.domain.runtime.Machine().MotionDisabled || !app.lifecycle.motion {
		t.Fatal("turning motion back on did not reach machine state and the session")
	}
}

func TestMotionPreferenceIsReadAtLaunchAndNeverArmsMotion(t *testing.T) {
	driver := pointerDriver(t)
	if driver.App().lifecycle.motion {
		t.Fatal("a test launch armed motion")
	}
	if err := driver.App().coreToggleMotion(); err != nil {
		t.Fatalf("coreToggleMotion: %v", err)
	}
	if err := driver.App().coreToggleMotion(); err != nil {
		t.Fatalf("coreToggleMotion back: %v", err)
	}
	if driver.App().lifecycle.motion {
		t.Fatal("the preference armed motion RunApp never armed")
	}
	if err := driver.App().coreToggleMotion(); err != nil {
		t.Fatalf("coreToggleMotion: %v", err)
	}
	if err := driver.Restart(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	WithMotion()(driver.App())
	if driver.App().motionPreferred() || driver.App().lifecycle.motion {
		t.Fatal("a relaunch forgot the machine-local motion preference")
	}
}

// TestControlReduceMotionHoldsSessionMotionOff: a host's Reduce Motion wins
// over the preference for the session, writes nothing, and "off" hands the
// decision back.
func TestControlReduceMotionHoldsSessionMotionOff(t *testing.T) {
	env := controlEnv(t, newBulkPickerTestTask(t, 1, "existing"))
	WithMotion()(env.model)
	setTestPresentationPath(env.model, presentation.ViewCrossProjectOutliner, presentation.ViewNotes)

	reply := env.request(t, control.Request{Op: control.OpReduceMotion, Text: "on"})
	if !reply.OK || env.model.lifecycle.motion {
		t.Fatalf("reduce motion on: reply %+v, motion %v", reply, env.model.lifecycle.motion)
	}
	if env.model.domain.runtime.Machine().MotionDisabled || !env.model.motionPreferred() {
		t.Fatal("reduce motion wrote the machine preference")
	}
	for _, text := range []string{"", "On", "yes"} {
		if refused := env.request(t, control.Request{Op: control.OpReduceMotion, Text: text}); refused.OK {
			t.Fatalf("reduce motion %q answered %+v", text, refused)
		}
	}
	if env.model.lifecycle.motion {
		t.Fatal("a refused request released motion")
	}
	reply = env.request(t, control.Request{Op: control.OpReduceMotion, Text: "off"})
	if !reply.OK || !env.model.lifecycle.motion {
		t.Fatalf("reduce motion off: reply %+v, motion %v", reply, env.model.lifecycle.motion)
	}
}

func TestGlyphOverrideFromEnv(t *testing.T) {
	for value, want := range map[string][2]bool{
		"nerd": {true, true}, " NERD ": {true, true}, "unicode": {false, true},
		"": {false, false}, "emoji": {false, false},
	} {
		nerd, ok := glyphOverrideFromEnv(value)
		if nerd != want[0] || ok != want[1] {
			t.Errorf("glyphOverrideFromEnv(%q) = %v, %v; want %v", value, nerd, ok, want)
		}
	}
}

func TestNerdGlyphToggleSwitchesTheRegistryAndPersists(t *testing.T) {
	t.Cleanup(func() { icons.SetNerdGlyphs(false) })
	driver, outlinerView := menuBarDriver(t)
	app := driver.App()
	if nerdGlyphsSelected() || outlinerView.NerdGlyphs() {
		t.Fatal("the fixture launched on the Nerd set")
	}
	if err := app.coreToggleNerdGlyphs(); err != nil {
		t.Fatalf("coreToggleNerdGlyphs: %v", err)
	}
	if !app.domain.runtime.Machine().NerdGlyphs || !nerdGlyphsSelected() || !outlinerView.NerdGlyphs() {
		t.Fatal("the toggle did not reach machine state, the registry, and the View row")
	}
	if err := driver.Restart(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if !driver.App().nerdGlyphs() || !nerdGlyphsSelected() {
		t.Fatal("a relaunch forgot the machine-local glyph preference")
	}
}

// TestGlyphEnvOverrideMakesTheToggleSessionOnly: under DOCKET_GLYPHS the toggle
// changes what is drawn and leaves machine.json alone.
func TestGlyphEnvOverrideMakesTheToggleSessionOnly(t *testing.T) {
	t.Cleanup(func() { icons.SetNerdGlyphs(false) })
	driver := pointerDriver(t)
	app := driver.App()
	WithGlyphOverride(true)(app)
	app.applyGlyphs()
	if !nerdGlyphsSelected() {
		t.Fatal("the override did not select the Nerd set")
	}
	if err := app.coreToggleNerdGlyphs(); err != nil {
		t.Fatalf("coreToggleNerdGlyphs: %v", err)
	}
	if nerdGlyphsSelected() || app.nerdGlyphs() {
		t.Fatal("the toggle did not change the session under an override")
	}
	if app.domain.runtime.Machine().NerdGlyphs {
		t.Fatal("a toggle under an override wrote machine state")
	}
	if !strings.Contains(app.feedback.message, glyphsEnv) {
		t.Fatalf("the toggle did not say the change is session-only: %q", app.feedback.message)
	}
}

// TestNerdGlyphsKeepEveryLineItsWidth renders the same rich Outliner on both
// sets: every line keeps the cell width it had, so no row grows or shrinks
// when the glyphs change.
func TestNerdGlyphsKeepEveryLineItsWidth(t *testing.T) {
	t.Cleanup(func() { icons.SetNerdGlyphs(false) })
	driver := pointerDriver(t)
	widths := func() ([]int, string) {
		obs := driver.Observe(ObserveOptions{IncludeANSI: true})
		if len(obs.LayoutIssues) > 0 {
			t.Fatalf("layout issues: %+v", obs.LayoutIssues)
		}
		var out []int
		for _, line := range strings.Split(obs.ANSIScreen, "\n") {
			out = append(out, ansi.StringWidth(line))
		}
		return out, obs.PlainScreen
	}
	unicodeWidths, unicodeScreen := widths()
	icons.SetNerdGlyphs(true)
	nerdWidths, nerdScreen := widths()
	if !slices.Equal(unicodeWidths, nerdWidths) {
		t.Fatalf("line widths changed with the glyph set:\nunicode %v\nnerd    %v", unicodeWidths, nerdWidths)
	}
	// A lane leaves its own state's glyph to the header, so the priority badge
	// is the glyph every fixture row is sure to draw.
	if nerdScreen == unicodeScreen || !strings.Contains(nerdScreen, icons.Get("priority.1")) {
		t.Fatalf("the Nerd set drew no Nerd glyph:\n%s", nerdScreen)
	}
}

// nerdGlyphsSelected reads the process-wide switch back through Get, the one
// way production code reads it.
func nerdGlyphsSelected() bool { return icons.Get("status.paused") != "‖" }
