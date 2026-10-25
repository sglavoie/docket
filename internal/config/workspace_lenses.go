package config

import "fmt"

// WorkspaceLensDefinitions is the portable, workspace-side home for named lens
// definitions. It maps to <data>/.docket/lenses.json and holds nothing but the
// collection: a lens definition references a project by workspace-scoped ID and
// carries no other external reference, so it is content that belongs to the
// workspace rather than to the machine that authored it.
//
// The two genuinely per-machine halves stay in MachineState: the dark/light
// theme, and ActiveLensID, which records which lens *this* machine has open.
//
// Lenses definition state remains outside the undo ledger and outside every
// workspace transaction even though it is now portable. Moving the home does not
// move the write policy: see CommitLensDefinitions.
type WorkspaceLensDefinitions struct {
	// Lenses distinguishes three states, which is why it carries neither
	// omitempty nor a plain slice type. A nil list means this workspace does not
	// define lenses, so a reader falls back to the machine-local set. A non-nil
	// empty list is the durable record of "every lens was deleted" and must
	// round-trip as [] rather than reseeding the starters. Anything else is the
	// defined set, decoded through NamedLensList's strict codec, which rejects an
	// explicit null and holds each nested lens to DisallowUnknownFields.
	Lenses NamedLensList `json:"lenses"`
}

// NewWorkspaceLensDefinitions returns the undefined collection, which is what a
// workspace with no lenses.json decodes into. It deliberately seeds no starter
// records: seeding is a machine-local bootstrap concern (NewMachineState), and a
// workspace-side default would make "this workspace defines no lenses"
// indistinguishable from "this workspace defines the starters".
func NewWorkspaceLensDefinitions() *WorkspaceLensDefinitions {
	return &WorkspaceLensDefinitions{}
}

// Defined reports whether the workspace owns the lens collection. It is the read
// precedence test: a defined collection wins over the machine-local set, and an
// undefined one leaves that set alone. An empty but defined collection is still
// defined, which is what keeps "I deleted every lens" durable.
func (d *WorkspaceLensDefinitions) Defined() bool {
	return d != nil && d.Lenses != nil
}

// Validate reports problems with the persisted collection, deferring entirely to
// NamedLensList so one lens is held to the same rules in both homes. An
// undefined collection is valid: it is the absence of a record, not a malformed
// one.
func (d *WorkspaceLensDefinitions) Validate() error {
	if !d.Defined() {
		return nil
	}
	if err := d.Lenses.Validate(); err != nil {
		return fmt.Errorf("workspace: %w", err)
	}
	return nil
}

// DefinedLensList normalizes a collection for publication: a nil list becomes
// the empty-but-defined one, because committing a machine's lens set that
// happens to be nil means "this workspace has no lenses", which is a record, not
// the absence of one. Callers pair it with SaveLensDefinitionsChecked, which
// refuses an undefined collection outright.
func DefinedLensList(lenses NamedLensList) NamedLensList {
	if lenses == nil {
		return NamedLensList{}
	}
	return lenses.Clone()
}
