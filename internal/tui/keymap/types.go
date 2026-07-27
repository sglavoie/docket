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
	// OperationID links this direct key to the same semantic operation used by
	// commands and palettes. Empty retains the legacy action path.
	OperationID string
	Action      Action
	Gate        Gate // nil = always active
	Hidden      bool // excluded from help, included in HandledKeys
}

// Entry holds one direct binding.
type Entry struct {
	Binding *Binding
}

// KeyMap is the complete set of keybindings for a view.
type KeyMap struct {
	ViewName string
	Entries  []Entry
}

// BindOption is a functional option for Bind.
type BindOption func(*Binding)

// WithGate sets a gate on a Binding.
func WithGate(g Gate) BindOption {
	return func(b *Binding) { b.Gate = g }
}

// AsHidden marks a binding as hidden from help output.
func AsHidden() BindOption {
	return func(b *Binding) { b.Hidden = true }
}

// WithOperationID routes a binding through the shell operation executor when
// one is installed, preserving Action as the compatibility fallback.
func WithOperationID(id string) BindOption {
	return func(b *Binding) { b.OperationID = id }
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
