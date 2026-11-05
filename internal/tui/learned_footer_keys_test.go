package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/tui/operation"
)

func footerRow(obs UITestObservation) string {
	lines := strings.Split(strings.TrimRight(obs.PlainScreen, "\n"), "\n")
	return lines[len(lines)-1]
}

// Pressing a footer key often enough retires its hint from the task-row
// footer, the hints behind it move up, and the count survives a restart of
// the session through the machine state. `? help` is never retired.
func TestLearnedFooterKeyRetiresItsHint(t *testing.T) {
	driver := launchReadmeFixture(t)
	obs := pressKeys(t, driver, "j", "j")
	if !strings.Contains(footerRow(obs), "b peek") {
		t.Fatalf("the fresh footer does not name b:\n%s", obs.PlainScreen)
	}
	for i := range config.FooterKeyLearnedUses {
		pressKeys(t, driver, "b")
		if i < config.FooterKeyLearnedUses-1 && driver.App().machineState().FooterKeyLearned("b") {
			t.Fatalf("b was learned after %d presses", i+1)
		}
	}
	if !driver.App().machineState().FooterKeyLearned("b") {
		t.Fatalf("b was not recorded as learned after %d presses", config.FooterKeyLearnedUses)
	}
	obs = pressKeys(t, driver, "esc")
	footer := footerRow(obs)
	if strings.Contains(footer, "b peek") {
		t.Fatalf("a learned key is still advertised:\n%s", footer)
	}
	if !strings.Contains(footer, "? help") {
		t.Fatalf("? help left the footer:\n%s", footer)
	}

	for range config.FooterKeyLearnedUses {
		pressKeys(t, driver, "?", "?")
	}
	if driver.App().machineState().LearnedFooterKeys["?"] != 0 {
		t.Fatal("? help was counted")
	}
	if driver.App().machineState().LearnedFooterKeys["Esc"] != 0 {
		t.Fatal("Esc, which the ranked footer never names, was counted")
	}

	result := driver.App().operations.Executor.Execute(operation.OperationExecutionRequest{
		ID: operation.OperationCoreResetFooterHints, Context: driver.App().operationCommandContext(),
	})
	if result.Err != nil {
		t.Fatalf("reset: %v", result.Err)
	}
	if len(driver.App().machineState().LearnedFooterKeys) != 0 {
		t.Fatalf("reset left %v", driver.App().machineState().LearnedFooterKeys)
	}
	obs = pressKeys(t, driver, "j", "k")
	if !strings.Contains(footerRow(obs), "b peek") {
		t.Fatalf("reset did not restore b:\n%s", footerRow(obs))
	}
}

// Presses below the threshold ride in memory and reach the machine state when
// the session quits, so the count carries into the next session without a
// write per keypress.
func TestFooterKeyUsesFlushOnQuit(t *testing.T) {
	driver := launchReadmeFixture(t)
	pressKeys(t, driver, "j", "j", "b", "b", "b")
	if got := driver.App().machineState().LearnedFooterKeys["b"]; got != 0 {
		t.Fatalf("a press below the threshold was written at once: %d", got)
	}
	driver.App().flushFooterKeyUses()
	if got := driver.App().machineState().LearnedFooterKeys["b"]; got != 3 {
		t.Fatalf("flushed count = %d, want 3", got)
	}
}
