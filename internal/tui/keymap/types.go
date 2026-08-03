package keymap

// Gate controls whether a binding is currently active.
type Gate func() bool

// Binding maps a single key to an action.
type Binding struct {
	Key         string
	Description string
	Category    string
	Gate        Gate // nil = always active
	Hidden      bool // excluded from help, included in HandledKeys
}

// Entry holds one direct binding.
type Entry struct {
	Binding *Binding
}

// KeyMap is the complete set of keybindings for a view.
type KeyMap struct {
	Entries []Entry
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

// Bind constructs a Binding entry.
func Bind(key, desc, cat string, opts ...BindOption) Entry {
	b := &Binding{
		Key:         key,
		Description: desc,
		Category:    cat,
	}
	for _, o := range opts {
		o(b)
	}
	return Entry{Binding: b}
}
