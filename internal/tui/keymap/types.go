package keymap

import tea "charm.land/bubbletea/v2"

// Action is executed when a binding fires.
type Action func() (tea.Model, tea.Cmd)

// Gate controls whether a binding is currently active.
type Gate func() bool

// Binding maps a single key to an action.
type Binding struct {
	Key         string
	Description string
	Category    string
	Action      Action
	Gate        Gate // nil = always active
	Hidden      bool // excluded from help, included in HandledKeys
}

// PrefixNode is an intermediate key that awaits a follow-up key.
type PrefixNode struct {
	Key         string
	Description string
	Category    string
	Children    []Entry
	Gate        Gate // nil = always active
}

// Entry holds exactly one non-nil field.
type Entry struct {
	Binding    *Binding
	PrefixNode *PrefixNode
}

// KeyMap is the complete set of keybindings for a view.
type KeyMap struct {
	ViewName string
	Entries  []Entry
}

// BindOption is a functional option for Bind.
type BindOption func(*Binding)

// PrefixOption is a functional option for Prefix.
type PrefixOption func(*PrefixNode)

// WithGate sets a gate on a Binding.
func WithGate(g Gate) BindOption {
	return func(b *Binding) { b.Gate = g }
}

// AsHidden marks a binding as hidden from help output.
func AsHidden() BindOption {
	return func(b *Binding) { b.Hidden = true }
}

// WithPrefixGate sets a gate on a PrefixNode.
func WithPrefixGate(g Gate) PrefixOption {
	return func(p *PrefixNode) { p.Gate = g }
}

// Bind constructs a Binding entry.
func Bind(key, desc, cat string, action Action, opts ...BindOption) Entry {
	b := &Binding{
		Key:         key,
		Description: desc,
		Category:    cat,
		Action:      action,
	}
	for _, o := range opts {
		o(b)
	}
	return Entry{Binding: b}
}

// Prefix constructs a PrefixNode entry.
func Prefix(key, desc, cat string, children []Entry, opts ...PrefixOption) Entry {
	p := &PrefixNode{
		Key:         key,
		Description: desc,
		Category:    cat,
		Children:    children,
	}
	for _, o := range opts {
		o(p)
	}
	return Entry{PrefixNode: p}
}
