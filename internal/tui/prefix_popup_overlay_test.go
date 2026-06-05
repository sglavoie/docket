package tui

import "testing"

func TestBottomPopupTopRowUsesBottomPopupRule(t *testing.T) {
	got := bottomPopupTopRow(10, 3)
	if got != 6 {
		t.Fatalf("bottomPopupTopRow(10, 3) = %d, want 6", got)
	}
}

func TestBottomPopupTopRowHandlesSingleLinePopup(t *testing.T) {
	got := bottomPopupTopRow(5, 1)
	if got != 3 {
		t.Fatalf("bottomPopupTopRow(5, 1) = %d, want 3", got)
	}
}

func TestBottomPopupTopRowClampsWhenPopupExceedsBaseHeight(t *testing.T) {
	got := bottomPopupTopRow(2, 5)
	if got != 0 {
		t.Fatalf("bottomPopupTopRow(2, 5) = %d, want 0", got)
	}
}

func TestBottomPopupTopRowClampsTinyTerminalHeight(t *testing.T) {
	got := bottomPopupTopRow(0, 1)
	if got != 0 {
		t.Fatalf("bottomPopupTopRow(0, 1) = %d, want 0", got)
	}
}
