package keymap

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// ModalGuard intercepts key events before keymap dispatch.
type ModalGuard struct {
	Active func() bool
	Handle func(key string) (tea.Model, tea.Cmd, bool)
}

// Dispatcher routes key events through a KeyMap.
type Dispatcher struct {
	keymap        *KeyMap
	pendingPrefix string
	prefixNode    *PrefixNode
	ModalGuards   []ModalGuard
}

// NewDispatcher creates a Dispatcher for the given KeyMap.
func NewDispatcher(km *KeyMap) *Dispatcher {
	return &Dispatcher{keymap: km}
}

// HasPendingPrefix reports whether a prefix key is awaiting a follow-up.
func (d *Dispatcher) HasPendingPrefix() bool { return d.pendingPrefix != "" }

// PendingPrefix returns the currently pending prefix string.
func (d *Dispatcher) PendingPrefix() string { return d.pendingPrefix }

// ClearPrefix resets pending prefix state.
func (d *Dispatcher) ClearPrefix() {
	d.pendingPrefix = ""
	d.prefixNode = nil
}

// SetPendingPrefix sets the pending prefix and resolves the corresponding node.
// This is an escape hatch for FilterMixin and similar external callers.
func (d *Dispatcher) SetPendingPrefix(prefix string) {
	node := d.findPrefixNode(prefix, d.keymap.Entries)
	if node == nil {
		d.ClearPrefix()
		return
	}
	d.pendingPrefix = prefix
	d.prefixNode = node
}

// PrefixHints returns child key/description pairs for the active prefix node.
func (d *Dispatcher) PrefixHints() []PrefixHint {
	if d.prefixNode == nil {
		return nil
	}
	hints := make([]PrefixHint, 0, len(d.prefixNode.Children))
	for _, e := range d.prefixNode.Children {
		switch {
		case e.Binding != nil:
			if !e.Binding.Hidden && (e.Binding.Gate == nil || e.Binding.Gate()) {
				hints = append(hints, PrefixHint{Key: e.Binding.Key, Description: e.Binding.Description})
			}
		case e.PrefixNode != nil:
			if e.PrefixNode.Gate == nil || e.PrefixNode.Gate() {
				hints = append(hints, PrefixHint{Key: e.PrefixNode.Key, Description: e.PrefixNode.Description})
			}
		}
	}
	return hints
}

// Dispatch processes a key press and returns (model, cmd, consumed).
// When consumed is false the caller is responsible for handling the key.
func (d *Dispatcher) Dispatch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	// Bare modifier during prefix: consume without changing state.
	if d.HasPendingPrefix() && isModifierOnly(msg) {
		return nil, nil, true
	}

	key := msg.String()

	// Modal guards take priority over the keymap.
	for _, g := range d.ModalGuards {
		if g.Active != nil && g.Active() {
			if g.Handle == nil {
				continue
			}
			m, c, handled := g.Handle(key)
			if handled {
				return m, c, true
			}
		}
	}

	if d.HasPendingPrefix() {
		return d.dispatchInPrefix(key)
	}
	return d.dispatchTopLevel(key)
}

func (d *Dispatcher) dispatchInPrefix(key string) (tea.Model, tea.Cmd, bool) {
	if key == "esc" {
		d.ClearPrefix()
		return nil, nil, true
	}

	for _, e := range d.prefixNode.Children {
		switch {
		case e.Binding != nil && e.Binding.Key == key:
			b := e.Binding
			if b.Gate != nil && !b.Gate() {
				continue
			}
			d.ClearPrefix()
			if b.Action == nil {
				return nil, nil, true
			}
			m, c := b.Action()
			return m, c, true

		case e.PrefixNode != nil && e.PrefixNode.Key == key:
			p := e.PrefixNode
			if p.Gate != nil && !p.Gate() {
				continue
			}
			d.pendingPrefix += key
			d.prefixNode = p
			return nil, nil, true
		}
	}

	// No match: clear prefix, silently consume.
	d.ClearPrefix()
	return nil, nil, true
}

func (d *Dispatcher) dispatchTopLevel(key string) (tea.Model, tea.Cmd, bool) {
	for _, e := range d.keymap.Entries {
		switch {
		case e.PrefixNode != nil && e.PrefixNode.Key == key:
			p := e.PrefixNode
			if p.Gate != nil && !p.Gate() {
				continue
			}
			d.pendingPrefix = key
			d.prefixNode = p
			return nil, nil, true

		case e.Binding != nil && e.Binding.Key == key:
			b := e.Binding
			if b.Gate != nil && !b.Gate() {
				continue
			}
			if b.Action == nil {
				return nil, nil, true
			}
			m, c := b.Action()
			return m, c, true
		}
	}
	return nil, nil, false
}

// findPrefixNode resolves the active PrefixNode for a given compound prefix
// string. Each segment in prefix must correspond to a PrefixNode key in the
// tree, and every prefix gate along that path must pass.
func (d *Dispatcher) findPrefixNode(prefix string, entries []Entry) *PrefixNode {
	if prefix == "" {
		return nil
	}

	for _, e := range entries {
		if e.PrefixNode != nil && e.PrefixNode.Key == prefix {
			node := e.PrefixNode
			if node.Gate != nil && !node.Gate() {
				return nil
			}
			return node
		}
	}

	for _, e := range entries {
		if e.PrefixNode != nil && e.PrefixNode.Key != "" && strings.HasPrefix(prefix, e.PrefixNode.Key) {
			node := e.PrefixNode
			if node.Gate != nil && !node.Gate() {
				continue
			}
			if child := d.findPrefixNode(strings.TrimPrefix(prefix, node.Key), node.Children); child != nil {
				return child
			}
		}
	}
	return nil
}

// isModifierOnly reports whether the key event is a bare modifier with no character.
func isModifierOnly(msg tea.KeyPressMsg) bool {
	switch msg.Code {
	case tea.KeyLeftShift, tea.KeyRightShift,
		tea.KeyLeftCtrl, tea.KeyRightCtrl,
		tea.KeyLeftAlt, tea.KeyRightAlt,
		tea.KeyLeftSuper, tea.KeyRightSuper,
		tea.KeyLeftHyper, tea.KeyRightHyper,
		tea.KeyLeftMeta, tea.KeyRightMeta:
		return true
	}
	return false
}
