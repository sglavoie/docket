package keymap

// Gate controls whether a binding is currently active.
type Gate func() bool

// Binding maps a single key to an action.
type Binding struct {
	Key            string
	Description    string
	Category       string
	Gate           Gate // nil = always active
	Hidden         bool // excluded from help and guidance, included in HandledKeys
	HelpKey        string
	FooterRank     int // zero excludes the binding from compact guidance
	FooterKey      string
	FooterLabel    string
	FooterRequired bool
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

// WithFooter exposes a binding as one compact guidance entry. Lower positive
// ranks are retained first; required entries are reserved before optional
// entries by the shared TUI guidance renderer.
func WithFooter(rank int, label string, required bool) BindOption {
	return func(b *Binding) {
		b.FooterRank = rank
		b.FooterLabel = label
		b.FooterRequired = required
	}
}

// WithFooterKey overrides only the compact guidance spelling. The physical
// key retained by Binding remains authoritative for dispatch and HandledKeys,
// while aliases such as j/k or Ctrl+J/K can be presented as one concise hint.
func WithFooterKey(key string) BindOption {
	return func(b *Binding) { b.FooterKey = key }
}

// WithHelpKey overrides only the `?` help spelling, the way WithFooterKey
// overrides the compact one. The physical key retained by Binding stays
// authoritative for dispatch and HandledKeys, so a surface whose grammar reaches
// one act through a chord and an alternate key — gg/Home, G/End — can name that
// complete path once in help without declaring a second dispatch spelling or
// listing the aliases as separate rows.
func WithHelpKey(key string) BindOption {
	return func(b *Binding) { b.HelpKey = key }
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
