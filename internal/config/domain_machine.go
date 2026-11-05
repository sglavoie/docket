package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
)

// SearchHistoryLimit bounds submitted queries retained on this machine.
const SearchHistoryLimit = 50

// MachineState contains the machine-local values retained by schema 6: the
// dark/light theme, whether the Outliner draws its menu bar, whether it carries
// ambient advisories, submitted search history, named Outliner lens definitions,
// and the pointer to the live one. Lens definitions also have a portable home
// (WorkspaceLensDefinitions); the copy here is the fallback a workspace that
// defines none of its own still reads. Neither home enters a workspace
// transaction or the undo ledger.
type MachineState struct {
	Theme string `json:"theme"`
	// MenuBarVisible records whether the Outliner draws its menu bar. Chrome
	// follows the terminal you are sitting at, the way Theme does, so it is
	// machine-local rather than portable. The omitted key is the default, which
	// is why it carries omitempty and NewMachineState never seeds it: the bar
	// starts hidden and a machine that has never pressed M writes no key at all.
	MenuBarVisible bool `json:"menu_bar_visible,omitempty"`
	// AdvisoriesHidden records whether the Outliner suppresses ambient advisories
	// — the outstanding-conflict count and the load-failure notices. It is chrome
	// on the same terms as MenuBarVisible: machine-local, omitempty, never seeded,
	// and the omitted key is the default, which is that advisories are shown.
	AdvisoriesHidden bool `json:"advisories_hidden,omitempty"`
	// MouseDisabled records whether the TUI leaves mouse reporting off. It is
	// chrome on the same terms as AdvisoriesHidden: machine-local, omitempty,
	// never seeded, and the omitted key is the default, which is that the mouse
	// is on.
	MouseDisabled bool `json:"mouse_disabled,omitempty"`
	// MotionDisabled records whether an interactive launch leaves motion off: the
	// departure ghost, mode arrival, pulse lighting, undo countdown, and the Today
	// sweep. Chrome on the same terms as MouseDisabled; the omitted key is the
	// default, which is that motion is on.
	MotionDisabled bool `json:"motion_disabled,omitempty"`
	// NerdGlyphs records whether the TUI draws its glyphs from the Nerd Font set
	// rather than plain Unicode. Chrome on the same terms; the omitted key is the
	// default, which is the Unicode set. DOCKET_GLYPHS overrides it per process.
	NerdGlyphs bool `json:"nerd_glyphs,omitempty"`
	// SearchHistory holds submitted slash queries oldest first, per workspace.
	// Drafts, the active query, and the recall position are never persisted.
	SearchHistory []string `json:"search_history,omitempty"`
	// NamedLenses carries no omitempty: an explicit empty array is the durable
	// record of "every lens was deleted", and omitting the key instead would
	// reseed the starters on the next load.
	NamedLenses NamedLensList `json:"named_lenses"`
	// ActiveLensID names the lens the Outliner reopens with, or is empty when
	// no lens is live. It is a soft pointer: nothing here checks that it names
	// a lens that exists, so a hand-edited machine file with a dangling id
	// still loads and simply reopens with no lens.
	ActiveLensID string `json:"active_lens_id,omitempty"`
	// LensHomeAdopted records that this machine has already merged its lens set
	// into the workspace's portable home. The marker has to be machine-local:
	// the catalog schema is the synced artifact, so it can say the workspace has
	// a lens home but never whether *this* machine has joined it. See
	// Manager.AdoptPortableLenses.
	LensHomeAdopted bool `json:"lenses_adopted,omitempty"`
	// GreetedOn is the local calendar day (YYYY-MM-DD) this machine last showed
	// the launch greeting, so the first interactive launch of a day greets and
	// every later one that day does not. It is chrome on the same terms as
	// MouseDisabled: machine-local, omitempty, never seeded.
	GreetedOn string `json:"greeted_on,omitempty"`
	// LearnedFooterKeys counts how often each footer key was pressed on this
	// machine, up to FooterKeyLearnedUses, after which the Outliner's idle footer
	// stops advertising it: a key in the hand needs no reminder on the screen.
	// Chrome on the same terms as GreetedOn — machine-local, omitempty, never
	// seeded — and the omitted key is the default, which is that every hint shows.
	LearnedFooterKeys map[string]int `json:"learned_footer_keys,omitempty"`
}

// FooterKeyLearnedUses is how many presses retire a footer hint.
const FooterKeyLearnedUses = 10

// FooterKeyLearned reports whether key has been pressed often enough on this
// machine that the footer no longer needs to name it.
func (m *MachineState) FooterKeyLearned(key string) bool {
	return m != nil && m.LearnedFooterKeys[key] >= FooterKeyLearnedUses
}

// WithFooterKeyUse returns a copy of the state counting one more press of key,
// and whether the count moved. A learned key stops counting, so the file is
// written at most FooterKeyLearnedUses times per key. The map is cloned rather
// than shared, because a caller commits the copy and the live state must not
// change underneath it if that commit fails.
func (m MachineState) WithFooterKeyUse(key string) (MachineState, bool) {
	if key == "" || m.LearnedFooterKeys[key] >= FooterKeyLearnedUses {
		return m, false
	}
	learned := maps.Clone(m.LearnedFooterKeys)
	if learned == nil {
		learned = make(map[string]int, 1)
	}
	learned[key]++
	m.LearnedFooterKeys = learned
	return m, true
}

// NewMachineState returns the default built-in theme selection and the starter
// lens records. A load decodes into this seeded value, so an omitted
// named_lenses key keeps the starters while an explicit [] overwrites them.
func NewMachineState() *MachineState {
	return &MachineState{Theme: "dark", NamedLenses: StarterNamedLenses()}
}

// Validate reports problems with machine-local preferences. The active lens
// pointer is checked for shape alone: referential integrity is deliberately not
// a load-time rule, so one stale id can never refuse the whole machine file.
func (m *MachineState) Validate() error {
	if len(m.SearchHistory) > SearchHistoryLimit {
		return fmt.Errorf("machine: search history exceeds %d entries", SearchHistoryLimit)
	}
	seen := make(map[string]bool, len(m.SearchHistory))
	for _, query := range m.SearchHistory {
		if query == "" || strings.TrimSpace(query) != query || seen[query] {
			return fmt.Errorf("machine: search history must contain distinct, nonblank, trimmed queries")
		}
		seen[query] = true
	}
	if m.Theme != "" && m.Theme != "dark" && m.Theme != "light" {
		return fmt.Errorf("machine: theme %q must be dark or light", m.Theme)
	}
	if m.GreetedOn != "" {
		if _, err := time.Parse(time.DateOnly, m.GreetedOn); err != nil {
			return fmt.Errorf("machine: greeted_on %q must be a YYYY-MM-DD day", m.GreetedOn)
		}
	}
	for key, uses := range m.LearnedFooterKeys {
		if key == "" || uses < 1 || uses > FooterKeyLearnedUses {
			return fmt.Errorf("machine: learned footer key %q must count 1 to %d uses", key, FooterKeyLearnedUses)
		}
	}
	if m.ActiveLensID != "" {
		if err := ValidateNamedLensID(m.ActiveLensID); err != nil {
			return fmt.Errorf("machine: active lens: %w", err)
		}
	}
	if err := m.NamedLenses.Validate(); err != nil {
		return fmt.Errorf("machine: %w", err)
	}
	return nil
}

// ActiveNamedLens returns the live lens definition, or nil when no lens is
// active and when the pointer dangles. A dangling id reads as "no lens", which
// is what keeps a hand-edited machine file usable.
func (m *MachineState) ActiveNamedLens() *NamedLens {
	if m == nil || m.ActiveLensID == "" {
		return nil
	}
	return m.NamedLensByID(m.ActiveLensID)
}

// NamedLensByID returns the definition with the given id, or nil.
func (m *MachineState) NamedLensByID(id string) *NamedLens {
	if m == nil {
		return nil
	}
	for i := range m.NamedLenses {
		if m.NamedLenses[i].ID == id {
			return &m.NamedLenses[i]
		}
	}
	return nil
}

// RemoveNamedLens drops the definition with the given id and reports whether
// anything changed. Removing the last one leaves an empty, non-nil list so the
// deletion persists as [] rather than reseeding the starters. It is also the
// single place deletion consistency is enforced: deleting the live lens clears
// the active pointer in the same call, so no caller can leave one dangling.
func (m *MachineState) RemoveNamedLens(id string) bool {
	if m == nil {
		return false
	}
	kept := make(NamedLensList, 0, len(m.NamedLenses))
	for _, lens := range m.NamedLenses {
		if lens.ID == id {
			continue
		}
		kept = append(kept, lens)
	}
	if len(kept) == len(m.NamedLenses) {
		return false
	}
	m.NamedLenses = kept
	if m.ActiveLensID == id {
		m.ActiveLensID = ""
	}
	return true
}

// Clone returns a deep copy of the machine snapshot. The lens definitions carry
// nested state, tag, and project-tag slices, so a shallow copy would let an edit
// to the clone rewrite the snapshot before the commit that is supposed to
// publish it.
func (m *MachineState) Clone() *MachineState {
	cp := *m
	cp.NamedLenses = m.NamedLenses.Clone()
	cp.SearchHistory = slices.Clone(m.SearchHistory)
	cp.LearnedFooterKeys = maps.Clone(m.LearnedFooterKeys)
	return &cp
}
