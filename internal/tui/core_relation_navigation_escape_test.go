package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

// armScope puts the view inside the origin project through the real picker.
func (env *relationLifecycleEnv) armScope(t *testing.T) {
	t.Helper()
	env.press(t, "P")
	env.selectPickerRow(t, "Origin")
	env.settle(t, "enter")
	if env.outliner().Scope.ProjectID != env.originProjectID {
		t.Fatalf("the picker scoped to %q, want the origin", env.outliner().Scope.ProjectID)
	}
}

// TestEscClaimedByModalInputAndTheMenuBeforeTheCascade holds the two exemptions.
// A composer is value entry rather than a grammar and an open menu is the
// innermost thing there is, so neither press reaches the axes behind them.
func TestEscClaimedByModalInputAndTheMenuBeforeTheCascade(t *testing.T) {
	env := newRelationLifecycleEnv(t)
	env.armScope(t)

	env.press(t, "a")
	if got := env.driver.App().currentView(); got != presentation.ViewTaskComposer {
		t.Fatalf("a opened %v, want the composer", got)
	}
	env.press(t, "esc")
	if got := env.driver.App().currentView(); got == presentation.ViewTaskComposer {
		t.Fatal("Esc did not put the composer down")
	}
	if !env.outliner().IsProjectScoped() {
		t.Fatal("closing the composer also spent the scope")
	}

	env.press(t, "ctrl+m")
	if !env.outliner().MenuBar.Open {
		t.Fatal("Ctrl+M did not open the menu bar")
	}
	env.press(t, "esc")
	if env.outliner().MenuBar.Open {
		t.Fatal("Esc did not close the menu bar")
	}
	if !env.outliner().IsProjectScoped() {
		t.Fatal("closing the menu also spent the scope")
	}
}

// TestEscSpendsPeekAndTheSelectionPairBeforeTheTrail is the top of the cascade.
// Each of the three costs its own press and none of them reaches the way back.
func TestEscSpendsPeekAndTheSelectionPairBeforeTheTrail(t *testing.T) {
	env := newRelationLifecycleEnv(t)
	env.hop(t)

	env.press(t, "b")
	if !env.outliner().PeekMode {
		t.Fatal("b did not open Peek")
	}
	env.press(t, "esc")
	if env.outliner().PeekMode {
		t.Fatal("Esc did not close Peek")
	}
	if !env.outliner().HasRelationTrail() {
		t.Fatal("closing Peek also spent the trail")
	}

	env.press(t, "V")
	if !env.outliner().IsRangeSelectionActive() {
		t.Fatal("V did not start a range")
	}
	env.press(t, "esc")
	if env.outliner().IsRangeSelectionActive() {
		t.Fatal("Esc did not put the live range down")
	}
	if !env.outliner().MultiSelectMode {
		t.Fatal("the range left no frozen selection behind")
	}
	if !env.outliner().HasRelationTrail() {
		t.Fatal("freezing the range also spent the trail")
	}

	env.press(t, "esc")
	if env.outliner().MultiSelectMode {
		t.Fatal("Esc did not clear the frozen selection")
	}
	if !env.outliner().HasRelationTrail() {
		t.Fatal("clearing the selection also spent the trail")
	}

	env.settle(t, "esc")
	env.requireReturnedToOrigin(t)
}

// TestEscUnwindsTheRestoredNarrowingsInOrder is the wide leg. The origin is a
// scope holding a zoom holding a focus; the hop replaced all three at once, so
// one press hands them all back and each further press spends exactly one,
// narrowest first.
func TestEscUnwindsTheRestoredNarrowingsInOrder(t *testing.T) {
	env := newRelationLifecycleEnv(t)
	env.armScope(t)

	// A focused scope leads with its project header, which takes no Enter, so the
	// walk steps off it onto the first section header below.
	env.press(t, "home", "n", "enter")
	if !env.outliner().Focus.Zoom.Active() {
		t.Fatal("Enter on a section header did not zoom it")
	}
	env.cursorTo(t, env.spareID)
	env.press(t, "enter")
	if len(env.outliner().Focus.CrossStack) != 1 {
		t.Fatalf("Enter on a parent pushed %d focus level(s), want one",
			len(env.outliner().Focus.CrossStack))
	}

	env.hop(t)
	// The destination is the target's own project in All: none of the origin's
	// narrowings traveled with it.
	if env.outliner().Focus.Zoom.Active() || len(env.outliner().Focus.CrossStack) != 0 {
		t.Fatal("the hop carried the origin's narrowings into the destination")
	}

	env.settle(t, "esc")
	if env.outliner().HasRelationTrail() {
		t.Fatal("Esc did not pop the trail")
	}
	if !env.outliner().Focus.Zoom.Active() || len(env.outliner().Focus.CrossStack) != 1 ||
		env.outliner().Scope.ProjectID != env.originProjectID {
		t.Fatalf("the frame did not hand the whole projection back: zoom=%v focus=%d scope=%q",
			env.outliner().Focus.Zoom.Active(), len(env.outliner().Focus.CrossStack),
			env.outliner().Scope.ProjectID)
	}

	env.press(t, "esc")
	if len(env.outliner().Focus.CrossStack) != 0 {
		t.Fatal("Esc did not pop the focus level first")
	}
	if !env.outliner().Focus.Zoom.Active() {
		t.Fatal("popping the focus also exited the zoom")
	}

	env.press(t, "esc")
	if env.outliner().Focus.Zoom.Active() {
		t.Fatal("Esc did not exit the zoom")
	}
	if !env.outliner().IsProjectScoped() {
		t.Fatal("exiting the zoom also popped the scope")
	}

	env.settle(t, "esc")
	if env.outliner().IsProjectScoped() {
		t.Fatal("Esc did not pop the project scope")
	}
	if env.outliner().PrimaryMode != chrome.ModeAll {
		t.Fatalf("the unwound view is in %s, want the launch mode", env.outliner().PrimaryMode)
	}
}

// TestEscSpendsTheTrailBeforeARestoredSearch pins the one ordering the wide leg
// cannot show: a search projection and a live lens never coexist, because
// publishing a lens clears the projection under it. Search is therefore paired
// with the trail above it and the scope below it.
func TestEscSpendsTheTrailBeforeARestoredSearch(t *testing.T) {
	env := newRelationLifecycleEnv(t)
	env.armScope(t)
	env.press(t, "/", "i", "m", "p", "o", "r", "t", "e", "r", "enter")
	if !env.outliner().HasSearchProjection() {
		t.Fatal("the query did not project")
	}

	env.hop(t)
	if env.outliner().HasSearchProjection() {
		t.Fatal("the hop carried the origin's search into the destination")
	}

	env.settle(t, "esc")
	if env.outliner().HasRelationTrail() {
		t.Fatal("Esc did not pop the trail")
	}
	if !env.outliner().HasSearchProjection() {
		t.Fatal("the return did not restore the search projection")
	}

	env.press(t, "esc")
	if env.outliner().HasSearchProjection() {
		t.Fatal("Esc did not clear the search")
	}
	if !env.outliner().IsProjectScoped() {
		t.Fatal("clearing the search also popped the scope")
	}

	env.settle(t, "esc")
	if env.outliner().IsProjectScoped() {
		t.Fatal("Esc did not pop the project scope")
	}
}

// TestEscSpendsTheTrailBeforeARestoredLens is the lens leg. The hop suspends the
// lens and the trail hands it back whole, so dismissing it is a second press —
// and dismissing it is what restores the baseline underneath.
func TestEscSpendsTheTrailBeforeARestoredLens(t *testing.T) {
	env := newRelationLifecycleEnv(t)
	env.press(t, "\\")
	env.selectPickerRow(t, "Save Current View…")
	env.settle(t, "enter")
	env.press(t, "C", "a", "s", "c", "a", "d", "e")
	env.settle(t, "enter")
	if !env.outliner().HasActiveLens() {
		t.Fatal("saving the current view did not make it live")
	}

	env.hop(t)
	if env.outliner().HasActiveLens() {
		t.Fatal("the hop left the lens applied over the destination")
	}

	env.settle(t, "esc")
	if env.outliner().HasRelationTrail() {
		t.Fatal("Esc did not pop the trail")
	}
	if !env.outliner().HasActiveLens() {
		t.Fatal("the return did not put the suspended lens back")
	}

	env.settle(t, "esc")
	if env.outliner().HasActiveLens() {
		t.Fatal("Esc did not dismiss the lens")
	}
	if env.outliner().PrimaryMode != chrome.ModeAll {
		t.Fatalf("the unwound view is in %s, want the launch mode", env.outliner().PrimaryMode)
	}
}
