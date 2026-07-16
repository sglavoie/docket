package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

func TestCyclePerspectiveLens_FromInactive(t *testing.T) {
	if got := CyclePerspectiveLens(PerspectiveInactive, 1); got != PerspectiveAll {
		t.Errorf("forward from inactive: got %d, want %d (All)", got, PerspectiveAll)
	}
	if got := CyclePerspectiveLens(PerspectiveInactive, -1); got != PerspectiveCompleted {
		t.Errorf("backward from inactive: got %d, want %d (Completed)", got, PerspectiveCompleted)
	}
}

func TestCyclePerspectiveLens_UsesCanonicalOrder(t *testing.T) {
	if got := CyclePerspectiveLens(PerspectiveReview, 1); got != PerspectiveSnoozed {
		t.Errorf("Review next: got %d, want Snoozed", got)
	}
	if got := CyclePerspectiveLens(PerspectiveSnoozed, 1); got != PerspectiveCompleted {
		t.Errorf("Snoozed next: got %d, want Completed", got)
	}
	if got := CyclePerspectiveLens(PerspectiveCompleted, 1); got != PerspectiveAll {
		t.Errorf("Completed next: got %d, want All", got)
	}
	if got := CyclePerspectiveLens(PerspectiveAll, -1); got != PerspectiveCompleted {
		t.Errorf("All previous: got %d, want Completed", got)
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

func TestBuildPerspectivePrefixEntryUsesRemappedShortcutIDs(t *testing.T) {
	selected := PerspectiveInactive
	noop := func() (tea.Model, tea.Cmd) { return nil, nil }
	entry := BuildPerspectivePrefixEntry(PerspectivePrefixDeps{
		SelectByID: func(id int) (tea.Model, tea.Cmd) {
			selected = id
			return nil, nil
		},
		CycleNext: noop,
		CyclePrev: noop,
	})

	for _, tc := range []struct {
		key  string
		want int
	}{
		{key: "0", want: PerspectiveCompleted},
		{key: "8", want: PerspectiveReview},
		{key: "9", want: PerspectiveSnoozed},
	} {
		selected = PerspectiveInactive
		action := findPrefixChildBinding(t, []keymap.Entry{entry}, "P", tc.key)
		_, _ = action()
		if selected != tc.want {
			t.Errorf("P%s selected perspective %d, want %d", tc.key, selected, tc.want)
		}
	}
}
