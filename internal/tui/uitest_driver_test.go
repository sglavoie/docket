package tui

import (
	"strings"
	"testing"
)

func TestUITestDriverResizeAndRender(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 100, 30)

	if got := driver.App().viewNav.width; got != 100 {
		t.Fatalf("width = %d, want 100", got)
	}
	if got := driver.App().viewNav.height; got != 30 {
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
	if driver.App().viewNav.width != 60 || driver.App().viewNav.height != 20 {
		t.Fatalf("after resize got %dx%d, want 60x20", driver.App().viewNav.width, driver.App().viewNav.height)
	}
}

func TestUITestDriverTooSmall(t *testing.T) {
	driver := NewUITestDriver(newUITestApp(), 40, 10)

	if !driver.App().viewNav.tooNarrow || !driver.App().viewNav.tooShort {
		t.Fatalf("expected tooNarrow/tooShort at 40x10, got tooNarrow=%v tooShort=%v",
			driver.App().viewNav.tooNarrow, driver.App().viewNav.tooShort)
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
