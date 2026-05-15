package tui

import "testing"

func TestCyclePerspectiveLens_FromInactive(t *testing.T) {
	if got := CyclePerspectiveLens(PerspectiveInactive, 1); got != 1 {
		t.Errorf("forward from inactive: got %d, want 1 (All)", got)
	}
	if got := CyclePerspectiveLens(PerspectiveInactive, -1); got != 9 {
		t.Errorf("backward from inactive: got %d, want 9 (Review)", got)
	}
}

func TestCyclePerspectiveLens_WrapsModulo10(t *testing.T) {
	if got := CyclePerspectiveLens(9, 1); got != 0 {
		t.Errorf("9 + 1: got %d, want 0", got)
	}
	if got := CyclePerspectiveLens(0, -1); got != 9 {
		t.Errorf("0 - 1: got %d, want 9", got)
	}
	if got := CyclePerspectiveLens(5, 1); got != 6 {
		t.Errorf("5 + 1: got %d, want 6", got)
	}
}

func TestPerspectiveLensName(t *testing.T) {
	if got := PerspectiveLensName(PerspectiveInactive); got != "" {
		t.Errorf("inactive: got %q, want empty", got)
	}
	if got := PerspectiveLensName(10); got != "" {
		t.Errorf("out-of-range 10: got %q, want empty", got)
	}
	if got := PerspectiveLensName(1); got != "All" {
		t.Errorf("preset 1: got %q, want All", got)
	}
}

func TestPerspectiveLensFilter_InactiveReturnsNil(t *testing.T) {
	if got := PerspectiveLensFilter(PerspectiveInactive); got != nil {
		t.Error("inactive should return nil filter")
	}
	if got := PerspectiveLensFilter(99); got != nil {
		t.Error("out-of-range should return nil filter")
	}
}

func TestPerspectiveLensFilter_AppliesPreset(t *testing.T) {
	filter := PerspectiveLensFilter(1) // "All" — excludes complete + snoozed
	if filter == nil {
		t.Fatal("preset 1 should return non-nil filter")
	}
}
