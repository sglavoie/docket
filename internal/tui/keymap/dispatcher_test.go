package keymap

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// charKey constructs a KeyPressMsg for a single printable character.
func charKey(ch rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: ch, Text: string(ch)}
}

// escKey constructs a KeyPressMsg for the Escape key.
func escKey() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEscape}
}

// modKey constructs a KeyPressMsg for a bare modifier key (no character).
func modKey() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyLeftCtrl}
}

// noopAction returns an Action that records it was called and returns nil model+cmd.
func noopAction(called *bool) Action {
	return func() (tea.Model, tea.Cmd) {
		*called = true
		return nil, nil
	}
}

// TestDispatch_DirectKeyHit verifies a matching top-level binding fires its action.
func TestDispatch_DirectKeyHit(t *testing.T) {
	called := false
	km := &KeyMap{Entries: []Entry{
		Bind("a", "do a", "cat", noopAction(&called)),
	}}
	d := NewDispatcher(km)

	_, _, consumed := d.Dispatch(charKey('a'))
	if !consumed {
		t.Fatal("expected consumed")
	}
	if !called {
		t.Fatal("expected action called")
	}
}

// TestDispatch_DirectKeyMiss verifies an unregistered key is not consumed.
func TestDispatch_DirectKeyMiss(t *testing.T) {
	km := &KeyMap{Entries: []Entry{
		Bind("a", "do a", "cat", func() (tea.Model, tea.Cmd) { return nil, nil }),
	}}
	d := NewDispatcher(km)

	_, _, consumed := d.Dispatch(charKey('b'))
	if consumed {
		t.Fatal("expected not consumed")
	}
}

// TestDispatch_DirectKeyGated verifies a gated binding is not consumed when gate is false.
func TestDispatch_DirectKeyGated(t *testing.T) {
	called := false
	km := &KeyMap{Entries: []Entry{
		Bind("a", "do a", "cat", noopAction(&called), WithGate(func() bool { return false })),
	}}
	d := NewDispatcher(km)

	_, _, consumed := d.Dispatch(charKey('a'))
	if consumed {
		t.Fatal("expected not consumed when gate blocks")
	}
	if called {
		t.Fatal("expected action not called when gate blocks")
	}
}

// TestDispatch_SingleLevelPrefix verifies activate → dispatch → clear.
func TestDispatch_SingleLevelPrefix(t *testing.T) {
	called := false
	km := &KeyMap{Entries: []Entry{
		Prefix(",", "comma", "cat", []Entry{
			Bind("s", "save", "cat", noopAction(&called)),
		}),
	}}
	d := NewDispatcher(km)

	// Activate prefix.
	_, _, consumed := d.Dispatch(charKey(','))
	if !consumed {
		t.Fatal("expected consumed on prefix activation")
	}
	if !d.HasPendingPrefix() {
		t.Fatal("expected pending prefix after activation")
	}
	if d.PendingPrefix() != "," {
		t.Fatalf("expected pending prefix %q, got %q", ",", d.PendingPrefix())
	}

	// Dispatch child key.
	_, _, consumed = d.Dispatch(charKey('s'))
	if !consumed {
		t.Fatal("expected consumed on prefix child dispatch")
	}
	if !called {
		t.Fatal("expected action called")
	}
	if d.HasPendingPrefix() {
		t.Fatal("expected prefix cleared after dispatch")
	}
}

// TestDispatch_NestedPrefix verifies a two-level prefix sequence (`,D` → `t`).
func TestDispatch_NestedPrefix(t *testing.T) {
	called := false
	km := &KeyMap{Entries: []Entry{
		Prefix(",", "comma", "cat", []Entry{
			Prefix("D", "deep", "cat", []Entry{
				Bind("t", "do t", "cat", noopAction(&called)),
			}),
		}),
	}}
	d := NewDispatcher(km)

	d.Dispatch(charKey(','))
	if d.PendingPrefix() != "," {
		t.Fatalf("expected prefix %q, got %q", ",", d.PendingPrefix())
	}

	d.Dispatch(charKey('D'))
	if d.PendingPrefix() != ",D" {
		t.Fatalf("expected prefix %q, got %q", ",D", d.PendingPrefix())
	}

	_, _, consumed := d.Dispatch(charKey('t'))
	if !consumed {
		t.Fatal("expected consumed")
	}
	if !called {
		t.Fatal("expected action called")
	}
	if d.HasPendingPrefix() {
		t.Fatal("expected prefix cleared")
	}
}

// TestDispatch_EscCancelsPrefix verifies ESC resets pending prefix without action.
func TestDispatch_EscCancelsPrefix(t *testing.T) {
	km := &KeyMap{Entries: []Entry{
		Prefix(",", "comma", "cat", []Entry{
			Bind("s", "save", "cat", func() (tea.Model, tea.Cmd) { return nil, nil }),
		}),
	}}
	d := NewDispatcher(km)

	d.Dispatch(charKey(','))
	if !d.HasPendingPrefix() {
		t.Fatal("expected pending prefix")
	}

	_, _, consumed := d.Dispatch(escKey())
	if !consumed {
		t.Fatal("expected esc consumed")
	}
	if d.HasPendingPrefix() {
		t.Fatal("expected prefix cleared after esc")
	}
}

// TestDispatch_InvalidKeyClearsPrefix verifies an unregistered child key silently clears the prefix.
func TestDispatch_InvalidKeyClearsPrefix(t *testing.T) {
	km := &KeyMap{Entries: []Entry{
		Prefix(",", "comma", "cat", []Entry{
			Bind("s", "save", "cat", func() (tea.Model, tea.Cmd) { return nil, nil }),
		}),
	}}
	d := NewDispatcher(km)

	d.Dispatch(charKey(','))
	_, _, consumed := d.Dispatch(charKey('z'))
	if !consumed {
		t.Fatal("expected invalid prefix child to be consumed (silently)")
	}
	if d.HasPendingPrefix() {
		t.Fatal("expected prefix cleared after no-match key")
	}
}

// TestDispatch_ModifierOnlyIgnoredDuringPrefix verifies bare modifiers do not clear the prefix.
func TestDispatch_ModifierOnlyIgnoredDuringPrefix(t *testing.T) {
	km := &KeyMap{Entries: []Entry{
		Prefix(",", "comma", "cat", []Entry{
			Bind("s", "save", "cat", func() (tea.Model, tea.Cmd) { return nil, nil }),
		}),
	}}
	d := NewDispatcher(km)

	d.Dispatch(charKey(','))
	_, _, consumed := d.Dispatch(modKey())
	if !consumed {
		t.Fatal("expected modifier consumed")
	}
	if !d.HasPendingPrefix() {
		t.Fatal("expected prefix still pending after modifier")
	}
}

// TestDispatch_ModalGuardIntercepts verifies active modal guards intercept before keymap.
func TestDispatch_ModalGuardIntercepts(t *testing.T) {
	keymapCalled := false
	guardCalled := false

	km := &KeyMap{Entries: []Entry{
		Bind("a", "do a", "cat", noopAction(&keymapCalled)),
	}}
	d := NewDispatcher(km)
	d.ModalGuards = []ModalGuard{
		{
			Active: func() bool { return true },
			Handle: func(key string) (tea.Model, tea.Cmd, bool) {
				guardCalled = true
				return nil, nil, true
			},
		},
	}

	_, _, consumed := d.Dispatch(charKey('a'))
	if !consumed {
		t.Fatal("expected consumed by guard")
	}
	if !guardCalled {
		t.Fatal("expected guard called")
	}
	if keymapCalled {
		t.Fatal("expected keymap action NOT called when guard intercepts")
	}
}

// TestDispatch_GateBlocksPrefixActivation verifies a gated prefix is not activated.
func TestDispatch_GateBlocksPrefixActivation(t *testing.T) {
	km := &KeyMap{Entries: []Entry{
		Prefix(",", "comma", "cat", []Entry{
			Bind("s", "save", "cat", func() (tea.Model, tea.Cmd) { return nil, nil }),
		}, WithPrefixGate(func() bool { return false })),
	}}
	d := NewDispatcher(km)

	_, _, consumed := d.Dispatch(charKey(','))
	if consumed {
		t.Fatal("expected not consumed when prefix gate blocks")
	}
	if d.HasPendingPrefix() {
		t.Fatal("expected no pending prefix when gate blocks")
	}
}

func TestDispatch_NilBindingActionIsSafe(t *testing.T) {
	km := &KeyMap{Entries: []Entry{
		Bind("a", "do a", "cat", nil),
	}}
	d := NewDispatcher(km)

	_, _, consumed := d.Dispatch(charKey('a'))
	if !consumed {
		t.Fatal("expected nil-action binding to remain consumed")
	}
}

func TestDispatch_NilPrefixBindingActionIsSafe(t *testing.T) {
	km := &KeyMap{Entries: []Entry{
		Prefix(",", "comma", "cat", []Entry{
			Bind("s", "save", "cat", nil),
		}),
	}}
	d := NewDispatcher(km)

	d.Dispatch(charKey(','))
	_, _, consumed := d.Dispatch(charKey('s'))
	if !consumed {
		t.Fatal("expected nil-action prefix binding to remain consumed")
	}
	if d.HasPendingPrefix() {
		t.Fatal("expected prefix cleared after nil-action binding")
	}
}

func TestDispatch_NilModalGuardHandleFallsThrough(t *testing.T) {
	keymapCalled := false
	km := &KeyMap{Entries: []Entry{
		Bind("a", "do a", "cat", noopAction(&keymapCalled)),
	}}
	d := NewDispatcher(km)
	d.ModalGuards = []ModalGuard{
		{
			Active: func() bool { return true },
			Handle: nil,
		},
	}

	_, _, consumed := d.Dispatch(charKey('a'))
	if !consumed {
		t.Fatal("expected keymap binding to consume after nil guard handler")
	}
	if !keymapCalled {
		t.Fatal("expected keymap action to run when guard handler is nil")
	}
}

// TestDispatch_SetPendingPrefixResolvesNode verifies SetPendingPrefix resolves the correct node.
func TestDispatch_SetPendingPrefixResolvesNode(t *testing.T) {
	called := false
	km := &KeyMap{Entries: []Entry{
		Prefix(",", "comma", "cat", []Entry{
			Prefix("D", "deep", "cat", []Entry{
				Bind("t", "do t", "cat", noopAction(&called)),
			}),
		}),
	}}
	d := NewDispatcher(km)

	// Resolve top-level prefix.
	d.SetPendingPrefix(",")
	if d.PendingPrefix() != "," {
		t.Fatalf("expected %q, got %q", ",", d.PendingPrefix())
	}
	if d.prefixNode == nil || d.prefixNode.Key != "," {
		t.Fatal("expected prefixNode with key ','")
	}

	// Resolve nested prefix.
	d.SetPendingPrefix(",D")
	if d.PendingPrefix() != ",D" {
		t.Fatalf("expected %q, got %q", ",D", d.PendingPrefix())
	}
	if d.prefixNode == nil || d.prefixNode.Key != "D" {
		t.Fatal("expected prefixNode with key 'D'")
	}

	// Dispatch leaf from resolved nested prefix.
	d.Dispatch(charKey('t'))
	if !called {
		t.Fatal("expected action called after SetPendingPrefix + dispatch")
	}
}

// TestDispatch_SetPendingPrefixUnknownClears verifies unknown prefix clears state.
func TestDispatch_SetPendingPrefixUnknownClears(t *testing.T) {
	km := &KeyMap{Entries: []Entry{
		Bind("a", "do a", "cat", func() (tea.Model, tea.Cmd) { return nil, nil }),
	}}
	d := NewDispatcher(km)
	d.pendingPrefix = "x"

	d.SetPendingPrefix("z")
	if d.HasPendingPrefix() {
		t.Fatal("expected prefix cleared for unknown prefix")
	}
}

// TestDispatch_PrefixHintsReturnsChildren verifies PrefixHints returns active node children.
func TestDispatch_PrefixHintsReturnsChildren(t *testing.T) {
	km := &KeyMap{Entries: []Entry{
		Prefix(",", "comma", "cat", []Entry{
			Bind("s", "save", "cat", func() (tea.Model, tea.Cmd) { return nil, nil }),
			Bind("t", "trash", "cat", func() (tea.Model, tea.Cmd) { return nil, nil }),
			Bind("h", "hidden", "cat", func() (tea.Model, tea.Cmd) { return nil, nil },
				WithGate(func() bool { return false })),
		}),
	}}
	d := NewDispatcher(km)

	// No prefix → empty hints.
	if hints := d.PrefixHints(); len(hints) != 0 {
		t.Fatalf("expected 0 hints before prefix, got %d", len(hints))
	}

	d.Dispatch(charKey(','))
	hints := d.PrefixHints()
	// Gated-off binding excluded.
	if len(hints) != 2 {
		t.Fatalf("expected 2 hints, got %d", len(hints))
	}
	keys := []string{hints[0].Key, hints[1].Key}
	if !slices.Contains(keys, "s") || !slices.Contains(keys, "t") {
		t.Fatalf("unexpected hint keys: %v", keys)
	}
}

// TestHelpItems_CompoundKeys verifies HelpItems produces correct compound keys for nested bindings.
func TestHelpItems_CompoundKeys(t *testing.T) {
	km := &KeyMap{Entries: []Entry{
		Bind("a", "direct", "cat", func() (tea.Model, tea.Cmd) { return nil, nil }),
		Prefix(",", "comma", "cat", []Entry{
			Bind("s", "save", "cat", func() (tea.Model, tea.Cmd) { return nil, nil }),
			Bind("t", "trash", "cat", func() (tea.Model, tea.Cmd) { return nil, nil }),
		}),
		Bind("h", "hidden", "cat", func() (tea.Model, tea.Cmd) { return nil, nil }, AsHidden()),
	}}

	items := km.HelpItems()
	keys := make([]string, len(items))
	for i, it := range items {
		keys[i] = it.Key
	}

	// Direct binding.
	if !slices.Contains(keys, "a") {
		t.Errorf("expected 'a' in help items, got %v", keys)
	}
	// Compound keys through prefix.
	if !slices.Contains(keys, ",s") {
		t.Errorf("expected ',s' in help items, got %v", keys)
	}
	if !slices.Contains(keys, ",t") {
		t.Errorf("expected ',t' in help items, got %v", keys)
	}
	// Hidden binding excluded.
	if slices.Contains(keys, "h") {
		t.Errorf("expected 'h' excluded from help items, got %v", keys)
	}
	// Prefix node itself not a help item.
	if slices.Contains(keys, ",") {
		t.Errorf("expected bare ',' excluded from help items, got %v", keys)
	}
}

// TestHandledKeys_IncludesPrefixActivatorsAndLeaves verifies HandledKeys output.
func TestHandledKeys_IncludesPrefixActivatorsAndLeaves(t *testing.T) {
	km := &KeyMap{Entries: []Entry{
		Bind("a", "direct", "cat", func() (tea.Model, tea.Cmd) { return nil, nil }),
		Prefix(",", "comma", "cat", []Entry{
			Bind("s", "save", "cat", func() (tea.Model, tea.Cmd) { return nil, nil }),
			Prefix("D", "deep", "cat", []Entry{
				Bind("t", "do t", "cat", func() (tea.Model, tea.Cmd) { return nil, nil }),
			}),
		}),
		Bind("h", "hidden", "cat", func() (tea.Model, tea.Cmd) { return nil, nil }, AsHidden()),
	}}

	got := km.HandledKeys()

	mustContain := []string{"a", ",", ",s", ",D", ",Dt", "h"}
	for _, k := range mustContain {
		if !slices.Contains(got, k) {
			t.Errorf("expected %q in HandledKeys, got %v", k, got)
		}
	}
}
