package tui

import (
	"sort"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// Characterization tests for the filter-keybinding-consistency plan (session 1).
//
// These tests freeze the *current* behavior of the shared "F" filter prefix
// across every view the plan treats as a shared-filter view. They are designed
// to all PASS today; sub-tests whose names end in "_KnownGap" record parity
// gaps that later sessions (2-4) will close. When a gap is closed the
// corresponding expectation should flip — that is the signal the session is
// done.
//
// Scope per view:
//   - Top-level F children (keys directly under "F", sorted).
//   - Presence of the canonical sub-prefixes: Ft, Fs, Fd, Fa, FS.
//   - Cross-view parity for Fh (hide-dimmed) and for the FS score sub-prefix
//     using lowercase "c" (stored custom threshold) — not uppercase "C".
//   - Dispatcher.PrefixHints() and KeyMap.HelpItems() agree on the F subtree.
//
// Test names follow the verify regex Test(FocusBoard|ProjectFocus|Filter|
// PrefixPopup|Keybinding|Score) so the plan's run recipe exercises them all.

// filterParityCase describes a shared-filter view under characterization.
type filterParityCase struct {
	view PresentationID
	name string
	km   *keymap.KeyMap
}

// sharedFilterViewCases mirrors the views the plan calls "shared-filter views"
// and constructs each view's keymap from a zero-value model — the same
// approach migratedViewKeymaps() uses, since buildKeyMap only creates closures
// that capture the model pointer (the closures themselves are never executed).
func sharedFilterViewCases() []filterParityCase {
	return []filterParityCase{
		{ViewOutliner, "Outliner", (&OutlinerModel{project: &models.Project{}}).buildKeyMap()},
		{ViewCrossProjectOutliner, "CrossProjectOutliner", (&OutlinerModel{}).buildKeyMap()},
		{ViewWeeklyReview, "WeeklyReview", (&WeeklyReviewModel{}).buildKeyMap()},
	}
}

// findRootFPrefix returns the top-level "F" PrefixNode for the keymap, or nil
// when the view does not expose the shared filter prefix at all.
func findRootFPrefix(km *keymap.KeyMap) *keymap.PrefixNode {
	if km == nil {
		return nil
	}
	for _, e := range km.Entries {
		if e.PrefixNode != nil && e.PrefixNode.Key == "F" {
			return e.PrefixNode
		}
	}
	return nil
}

// fChildKeys returns the sorted set of immediate child keys (both bindings and
// sub-prefixes) under the F prefix. Returns nil when the view has no F prefix.
func fChildKeys(km *keymap.KeyMap) []string {
	f := findRootFPrefix(km)
	if f == nil {
		return nil
	}
	keys := make([]string, 0, len(f.Children))
	for _, c := range f.Children {
		switch {
		case c.Binding != nil:
			keys = append(keys, c.Binding.Key)
		case c.PrefixNode != nil:
			keys = append(keys, c.PrefixNode.Key)
		}
	}
	sort.Strings(keys)
	return keys
}

// fChildIsBinding reports whether the F child at key is a direct binding leaf.
func fChildIsBinding(km *keymap.KeyMap, key string) bool {
	f := findRootFPrefix(km)
	if f == nil {
		return false
	}
	for _, c := range f.Children {
		if c.Binding != nil && c.Binding.Key == key {
			return true
		}
	}
	return false
}

// fSubChildKeys returns the sorted child keys of the F.<sub> prefix, or nil
// when that sub-prefix doesn't exist.
func fSubChildKeys(km *keymap.KeyMap, sub string) []string {
	f := findRootFPrefix(km)
	if f == nil {
		return nil
	}
	for _, c := range f.Children {
		if c.PrefixNode != nil && c.PrefixNode.Key == sub {
			keys := make([]string, 0, len(c.PrefixNode.Children))
			for _, cc := range c.PrefixNode.Children {
				switch {
				case cc.Binding != nil:
					keys = append(keys, cc.Binding.Key)
				case cc.PrefixNode != nil:
					keys = append(keys, cc.PrefixNode.Key)
				}
			}
			sort.Strings(keys)
			return keys
		}
	}
	return nil
}

// helpKeysWithPrefix returns the sorted set of HelpItem.Key values that start
// with the given compound prefix (e.g. "F" returns Ft, Fs, Fa, FS, F#, Fp,
// Fh, Fi, Fx, etc. — every leaf reachable through the F sub-tree).
func helpKeysWithPrefix(km *keymap.KeyMap, prefix string) []string {
	var out []string
	for _, item := range km.HelpItems() {
		if strings.HasPrefix(item.Key, prefix) {
			out = append(out, item.Key)
		}
	}
	sort.Strings(out)
	return out
}

func prefixHintKeys(hints []PrefixHint) []string {
	keys := make([]string, 0, len(hints))
	for _, h := range hints {
		keys = append(keys, h.Key)
	}
	sort.Strings(keys)
	return keys
}

func newFilterParityConfig() *config.RuntimeState {
	return newTestConfig()
}

func dispatcherPendingF(km *keymap.KeyMap) *keymap.Dispatcher {
	disp := keymap.NewDispatcher(km)
	disp.SetPendingPrefix("F")
	return disp
}

func runtimeFilterPopupInfo(t *testing.T, name string) *PrefixPopupInfo {
	t.Helper()
	cfg := newFilterParityConfig()
	runtime := testRuntimeAt(cfg.DataDir(), cfg)
	filterMixin := func() FilterMixin {
		return FilterMixin{filter: newFilterState(), filterMachine: runtime.Machine()}
	}

	switch name {
	case "Outliner":
		m := &OutlinerModel{
			PrimaryViewBundle: NewPrimaryViewBundle(nil, testRuntimeAt(cfg.DataDir(), cfg)),
			project:           &models.Project{ID: "p1", Name: "Project"},
			filter:            NewFilterState(),
			sort:              NewSortState(),
			activePerspective: 1,
		}
		m.Dispatcher = dispatcherPendingF(m.buildKeyMap())
		return m.GetPrefixPopupInfo()
	case "CrossProjectOutliner":
		m := &OutlinerModel{
			PrimaryViewBundle: NewPrimaryViewBundle(nil, testRuntimeAt(cfg.DataDir(), cfg)),
			filter:            NewFilterState(),
			sort:              NewSortState(),
			activePerspective: 1,
		}
		m.Dispatcher = dispatcherPendingF(m.buildKeyMap())
		return m.GetPrefixPopupInfo()
	case "WeeklyReview":
		m := &WeeklyReviewModel{PrimaryViewBundle: NewPrimaryViewBundle(nil, runtime), FilterMixin: filterMixin()}
		m.pendingPrefix = "F"
		return m.GetPrefixPopupInfo()
	default:
		t.Fatalf("missing runtime popup setup for %s", name)
		return nil
	}
}

// TestFilterParity_SharedFilterViews_RootChildKeys characterizes the top-level
// children directly under "F" for each shared-filter view. Frozen snapshots so
// sessions 2-4 can diff against them when they touch wiring.
func TestFilterParity_SharedFilterViews_RootChildKeys(t *testing.T) {
	wants := map[string][]string{
		// The Outliner appends view-specific filters for
		// completed/actionable/section/columns plus Fv saved filters.
		"Outliner":             {"#", "@", "D", "S", "a", "c", "d", "h", "i", "k", "p", "r", "s", "t", "v", "x"},
		"CrossProjectOutliner": {"#", "@", "D", "S", "a", "c", "d", "h", "i", "k", "p", "r", "s", "t", "v", "x"},
		"WeeklyReview":         {"#", "S", "a", "d", "h", "i", "k", "p", "s", "t", "x"},
	}

	for _, tc := range sharedFilterViewCases() {
		t.Run(tc.name, func(t *testing.T) {
			got := fChildKeys(tc.km)
			want, ok := wants[tc.name]
			if !ok {
				t.Fatalf("missing expectation for view %q", tc.name)
			}
			if !equalStringSlices(got, want) {
				t.Errorf("F children for %s\n  got:  %v\n  want: %v", tc.name, got, want)
			}
		})
	}
}

// TestFilterParity_SharedFilterViews_AdvertiseFh confirms every shared-filter
// view that wires FilterGroup advertises Fh (toggle hide-dimmed). The plan's
// acceptance criterion is that Fh works and is advertised consistently.
func TestFilterParity_SharedFilterViews_AdvertiseFh(t *testing.T) {
	for _, tc := range sharedFilterViewCases() {
		t.Run(tc.name, func(t *testing.T) {
			if !fChildIsBinding(tc.km, "h") {
				t.Errorf("%s does not advertise Fh (hide-dimmed)", tc.name)
			}
			helpKeys := helpKeysWithPrefix(tc.km, "Fh")
			if len(helpKeys) == 0 {
				t.Errorf("%s HelpItems() omit Fh", tc.name)
			}
		})
	}
}

// TestFilterParity_SharedFilterViews_AdvertiseFk confirms every shared-filter
// view that wires FilterGroup advertises Fk (parked tasks only). The parked
// filter is a common filter, so it must be reachable from keymap help wherever
// the shared F prefix is present.
func TestFilterParity_SharedFilterViews_AdvertiseFk(t *testing.T) {
	for _, tc := range sharedFilterViewCases() {
		t.Run(tc.name, func(t *testing.T) {
			if !fChildIsBinding(tc.km, "k") {
				t.Errorf("%s does not advertise Fk (parked)", tc.name)
			}
			helpKeys := helpKeysWithPrefix(tc.km, "Fk")
			if len(helpKeys) == 0 {
				t.Errorf("%s HelpItems() omit Fk", tc.name)
			}
		})
	}
}

// TestFilterParity_ScoreSubPrefix_CapabilityGated documents the explicit
// capability-driven asymmetry: no view exposes the retired lowercase `c`
// machine-stored threshold, while views that pass a non-nil ScoreCustomInput
// action through FilterDeps advertise uppercase `C` (session-local input).
// Adding a new view that wires ScoreCustomInput must add it to the
// supportsCustomInput set below so the keymap, ? help, popup, and explorer
// stay in lockstep.
func TestFilterParity_ScoreSubPrefix_CapabilityGated(t *testing.T) {
	supportsCustomInput := map[string]bool{
		"Outliner":             true,
		"CrossProjectOutliner": true,
	}
	for _, tc := range sharedFilterViewCases() {
		t.Run(tc.name, func(t *testing.T) {
			children := fSubChildKeys(tc.km, "S")
			if len(children) == 0 {
				t.Fatalf("%s does not expose the FS score sub-prefix", tc.name)
			}
			hasLowerC, hasUpperC := false, false
			for _, k := range children {
				switch k {
				case "c":
					hasLowerC = true
				case "C":
					hasUpperC = true
				}
			}
			if hasLowerC {
				t.Errorf("%s FS sub-prefix restored lowercase c machine threshold; got children %v", tc.name, children)
			}
			wantUpperC := supportsCustomInput[tc.name]
			if hasUpperC != wantUpperC {
				t.Errorf("%s FS uppercase C presence: got %v, want %v (children=%v)", tc.name, hasUpperC, wantUpperC, children)
			}
		})
	}
}

// TestFilterParity_DispatcherPrefixHints_MatchHelpItems_OnFPrefix verifies the
// invariant that Dispatcher.PrefixHints() (used to render the which-key popup)
// and KeyMap.HelpItems() (used to render ? help)
// describe the same top-level F children. This catches drift between the
// popup and the help overlay — exactly the parity
// the plan wants enforced.
func TestFilterParity_DispatcherPrefixHints_MatchHelpItems_OnFPrefix(t *testing.T) {
	for _, tc := range sharedFilterViewCases() {
		if findRootFPrefix(tc.km) == nil {
			// Focus Board only — characterized elsewhere.
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			disp := keymap.NewDispatcher(tc.km)
			disp.SetPendingPrefix("F")
			hints := disp.PrefixHints()
			if len(hints) == 0 {
				t.Fatalf("%s PrefixHints() for F returned no entries", tc.name)
			}

			hintKeys := make([]string, 0, len(hints))
			for _, h := range hints {
				hintKeys = append(hintKeys, h.Key)
			}
			sort.Strings(hintKeys)

			// Top-level F children visible to HelpItems() are the leaves whose
			// compound key is exactly "F<key>" (one rune of additional depth,
			// or zero — leaves include both direct binds like Fh and prefix
			// activator entries like Ft, which HelpItems descends into).
			// We collect the immediate keys by stripping the F prefix from
			// the *shortest* compound entries and from the prefix activators.
			helpRoots := map[string]bool{}
			for _, item := range tc.km.HelpItems() {
				if !strings.HasPrefix(item.Key, "F") || item.Key == "F" {
					continue
				}
				// Take the next rune after "F" as the immediate child key.
				rest := item.Key[len("F"):]
				if rest == "" {
					continue
				}
				// `#` is a single-byte rune; treat the first rune as child.
				firstRune := []rune(rest)[0]
				helpRoots[string(firstRune)] = true
			}
			helpKeys := make([]string, 0, len(helpRoots))
			for k := range helpRoots {
				helpKeys = append(helpKeys, k)
			}
			sort.Strings(helpKeys)

			if !equalStringSlices(hintKeys, helpKeys) {
				t.Errorf("%s F-prefix popup and help disagree\n  popup: %v\n  help:  %v", tc.name, hintKeys, helpKeys)
			}
		})
	}
}

// TestFilterParity_RuntimePrefixPopup_MatchesDeclarativeFChildren verifies the
// actual view-level GetPrefixPopupInfo() output, including custom popup maps,
// not only the dispatcher fallback. This catches ghost popup hints such as an
// advertised FD key that the current view's keymap cannot dispatch.
func TestFilterParity_RuntimePrefixPopup_MatchesDeclarativeFChildren(t *testing.T) {
	for _, tc := range sharedFilterViewCases() {
		t.Run(tc.name, func(t *testing.T) {
			info := runtimeFilterPopupInfo(t, tc.name)
			if info == nil {
				t.Fatalf("%s returned nil PrefixPopupInfo for pending F", tc.name)
			}
			got := prefixHintKeys(info.Hints["F"])
			want := fChildKeys(tc.km)
			if !equalStringSlices(got, want) {
				t.Errorf("%s runtime F popup keys disagree with keymap\n  popup: %v\n  keymap: %v", tc.name, got, want)
			}
		})
	}
}

// TestFilterParity_StateSubPrefix_MatchesPopup verifies the Fs sub-prefix now
// exposes the ten canonical WorkflowState children (the legacy TaskStatus status
// cycle was retired) and that keymap and runtime popup agree on them.
func TestFilterParity_StateSubPrefix_MatchesPopup(t *testing.T) {
	want := []string{"0", "d", "i", "m", "n", "p", "r", "s", "v", "w", "x"}
	for _, tc := range sharedFilterViewCases() {
		t.Run(tc.name, func(t *testing.T) {
			gotKeymap := fSubChildKeys(tc.km, "s")
			if !equalStringSlices(gotKeymap, want) {
				t.Errorf("%s Fs keymap children\n  got:  %v\n  want: %v", tc.name, gotKeymap, want)
			}

			info := runtimeFilterPopupInfo(t, tc.name)
			if info == nil {
				t.Fatalf("%s returned nil PrefixPopupInfo for pending F", tc.name)
			}
			gotPopup := prefixHintKeys(info.Hints["Fs"])
			if !equalStringSlices(gotPopup, want) {
				t.Errorf("%s Fs runtime popup keys\n  got:  %v\n  want: %v", tc.name, gotPopup, want)
			}
		})
	}
}

// TestFilterParity_HandledKeys_CoverAdvertisedFKeys is the bidirectional
// invariant the keybinding sync convention requires, restricted to the F
// subtree. Every compound F key the keymap advertises through HelpItems() must
// also be reachable through HandledKeys() — otherwise it would be a
// "ghost binding" the user sees but can't trigger. The reverse direction
// (no invisible bindings) is enforced more globally by TestKeybindingSyncAllViews.
func TestFilterParity_HandledKeys_CoverAdvertisedFKeys(t *testing.T) {
	for _, tc := range sharedFilterViewCases() {
		if findRootFPrefix(tc.km) == nil {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			handled := map[string]bool{}
			for _, k := range tc.km.HandledKeys() {
				handled[k] = true
			}
			for _, item := range tc.km.HelpItems() {
				if !strings.HasPrefix(item.Key, "F") {
					continue
				}
				if !handled[item.Key] {
					t.Errorf("%s advertises %q in help but HandledKeys() omits it", tc.name, item.Key)
				}
			}
		})
	}
}

// TestScoreSubPrefix_AllSharedViewsAdvertiseFullLadder freezes the FS score
// ladder children every wired view advertises today: 1-9 and 0 — plus the
// capability-gated uppercase C in views that wire ScoreCustomInput.
func TestScoreSubPrefix_AllSharedViewsAdvertiseFullLadder(t *testing.T) {
	wantNoCustomInput := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
	wantWithCustomInput := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "C"}
	supportsCustomInput := map[string]bool{
		"Outliner":             true,
		"CrossProjectOutliner": true,
	}
	for _, tc := range sharedFilterViewCases() {
		t.Run(tc.name, func(t *testing.T) {
			want := wantNoCustomInput
			if supportsCustomInput[tc.name] {
				want = wantWithCustomInput
			}
			got := fSubChildKeys(tc.km, "S")
			if !equalStringSlices(got, want) {
				t.Errorf("%s FS children\n  got:  %v\n  want: %v", tc.name, got, want)
			}
		})
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
