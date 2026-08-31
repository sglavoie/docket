package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/sglavoie/tt/internal/models"
)

// Named lenses are portable, workspace-scoped Outliner projections: their
// definitions live in the synced .tt/lenses.json, and only the active-lens
// pointer stays machine-local. The config layer owns their durable shape so the
// TUI never persists its own integer enums: every axis below is a stable string
// token whose spelling is the durable identity of that choice.

// MaxNamedLensNameRunes bounds a lens name at the same 200-rune boundary the
// TUI text prompt already enforces, so a name that can be typed is a name that
// can be stored.
const MaxNamedLensNameRunes = 200

// MaxNamedLensQueryRunes bounds the one persisted narrowing string. The query is
// now the whole predicate — every facet a lens once spelled in a struct field is
// a token in it — so it has no natural length and a hand-edited or synced file
// could otherwise carry an unbounded blob into every load. The bound sits far
// above any query the pickers can mint and is enforced on load, which is what
// makes it durable rather than advisory.
const MaxNamedLensQueryRunes = 2000

// Outliner mode tokens. These mirror tui.OutlinerModeID by value, deliberately
// duplicated rather than imported: config cannot depend on the TUI, and the
// persisted spelling must survive any TUI refactor.
const (
	LensModeAll     = "all"
	LensModeInbox   = "inbox"
	LensModeToday   = "today"
	LensModeNext    = "next"
	LensModeBacklog = "backlog"
	LensModeDue     = "due"
	LensModeRecent  = "recent"
)

// Manual sort field tokens. Sorting by lifecycle state retired with the
// carriers; v16_named_lens.go rewrites a persisted lens that still names it.
const (
	LensSortDefault  = "default"
	LensSortPriority = "priority"
	LensSortDue      = "due"
	LensSortCreated  = "created"
	LensSortUpdated  = "updated"
	LensSortTitle    = "title"
)

// lensTokenDefaultSentinel is what omitempty writes for an axis left at its
// default, so loading must read it back as that default rather than as garbage.
const lensTokenDefaultSentinel = ""

// Fixed identities for the built-in template catalog. They are frozen so a
// seeded workspace and a reseeded one name the same lenses, so a user edit
// lands on the record it opened, and so a gallery can tell an already-added
// template from a fresh one by stable ID alone. The first three are also the
// records seeded automatically; the rest are added on demand.
const (
	lensWaitingStarterName  = "Waiting"
	lensHighPriorityStarter = "High Priority"
	lensStaleStarterName    = "Stale"
	lensOverdueTemplateName = "Overdue"
	lensActiveWorkTmplName  = "Active Work"
	lensReviewQueueTmplName = "Review Queue"
	lensDeferredTmplName    = "Deferred"
	lensCleanup90dTmplName  = "Cleanup 90d"
	lensParkedWorkTmplName  = "Parked Work"

	lensWaitingStarterID    = "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c01"
	lensHighPriorityStartID = "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c02"
	lensStaleStarterID      = "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c03"
	lensOverdueTemplateID   = "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c04"
	lensActiveWorkTmplID    = "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c05"
	lensReviewQueueTmplID   = "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c06"
	lensDeferredTmplID      = "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c07"
	lensCleanup90dTmplID    = "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c08"
	lensParkedWorkTmplID    = "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c09"
)

// starterNamedLensCount is how many leading catalog entries a never-seeded
// machine state receives. The catalog grew past the starters; automatic seeding
// deliberately did not.
const starterNamedLensCount = 3

// LensSort is the persisted manual sort axis.
type LensSort struct {
	Field     string `json:"field,omitempty"`
	Ascending bool   `json:"ascending"`
}

// NamedLens is one durable Outliner projection on exactly six axes: identity,
// name, mode, scope, query, and sort.
//
// Query is the whole narrowing predicate. Schema 16 split it between a free-text
// query and two filter carriers, one struct field per facet, which made the same
// narrowing expressible two ways and left the durable meaning of a lens spread
// across twenty-odd fields no reader could compare. Schema 17 collapsed them:
// the canonical clause string internal/models renders and parses carries the
// facets, and whatever richer spelling the user-facing grammar owns follows it
// verbatim in the same string.
type NamedLens struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Mode string `json:"mode"`
	// Scope is the project the lens reopens into, named by project ID. An
	// empty Scope means the lens does not own the scope axis and leaves the
	// current view's scope alone; it is deliberately not a way to force the
	// global population, which v1 cannot express. omitempty keeps a machine
	// file written before the axis existed byte-identical.
	Scope string   `json:"scope,omitempty"`
	Query string   `json:"query,omitempty"`
	Sort  LensSort `json:"sort"`
}

// namedLensPayload is the decoding shape: the six live axes plus the two retired
// carriers, so a record persisted before schema 17 is lifted rather than
// refused. It exists because NamedLens carries its own codec and therefore
// cannot inherit the enclosing decoder's unknown-field strictness; the strict
// decode happens here instead, against the union of both shapes.
type namedLensPayload struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Mode     string                 `json:"mode"`
	Scope    string                 `json:"scope,omitempty"`
	Query    string                 `json:"query,omitempty"`
	Sort     LensSort               `json:"sort"`
	Tasks    *v16LensTaskFilters    `json:"tasks,omitempty"`
	Projects *v16LensProjectFilters `json:"projects,omitempty"`
}

// UnmarshalJSON decodes one lens, lifting a schema-16 record onto the canonical
// query as it reads it.
//
// The lift is here rather than in the migration alone because the two homes
// upgrade at different times: the portable .tt/lenses.json is rewritten by the
// guarded v17 transaction, while a machine-local named_lenses list written by an
// older build is only rewritten the next time that machine commits its own
// state. Refusing the old shape would make such a machine unable to load at all,
// so it is translated on the way in and self-heals on the next write.
func (n *NamedLens) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var payload namedLensPayload
	if err := decoder.Decode(&payload); err != nil {
		return err
	}
	lens := NamedLens{
		ID:    payload.ID,
		Name:  payload.Name,
		Mode:  payload.Mode,
		Scope: payload.Scope,
		Query: payload.Query,
		Sort:  LensSort{Field: v16LensSortField(payload.Sort.Field), Ascending: payload.Sort.Ascending},
	}
	if payload.Tasks != nil || payload.Projects != nil {
		carriers := v16NamedLensCarriers{}
		if payload.Tasks != nil {
			carriers.tasks = *payload.Tasks
		}
		if payload.Projects != nil {
			carriers.projects = *payload.Projects
		}
		query, err := carriers.canonicalQuery(payload.Query)
		if err != nil {
			return fmt.Errorf("lens %q: %w", payload.Name, err)
		}
		lens.Query = query
	}
	*n = lens
	return nil
}

// NamedLensList is the persisted collection. It carries its own JSON codec for
// two reasons the plain slice cannot serve: an explicit `null` must fail rather
// than silently read as "omitted", and an empty list must round-trip as `[]` so
// deleting every lens is durable instead of reseeding the starters on reload.
type NamedLensList []NamedLens

// UnmarshalJSON rejects null, keeps an explicit empty array empty, and holds
// nested lens payloads to the same unknown-field strictness the domain store
// applies to the enclosing machine payload.
func (l *NamedLensList) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if string(trimmed) == "null" {
		return fmt.Errorf("machine: named_lenses must be an array, not null")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	var decoded []NamedLens
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	if decoded == nil {
		decoded = []NamedLens{}
	}
	*l = decoded
	return nil
}

// MarshalJSON renders a nil list as an empty array so a machine payload never
// persists `null`, which loading would reject.
func (l NamedLensList) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]NamedLens(l))
}

// builtinNamedLensDefinitions is the single source of truth for every built-in
// template, in catalog order. It builds the whole list from literals on each
// call, so every returned lens and every nested slice is already detached from
// any earlier caller's copy; the exported wrappers below inherit that.
// Every template is written as the canonical query string its facets render to,
// in canonical field order, so the built-in catalog and a user lens saved from
// the same view are byte-identical records.
func builtinNamedLensDefinitions() NamedLensList {
	return NamedLensList{
		{
			ID:    lensWaitingStarterID,
			Name:  lensWaitingStarterName,
			Mode:  LensModeAll,
			Query: "$" + string(models.StateWaiting),
			Sort:  LensSort{Field: LensSortDefault, Ascending: true},
		},
		{
			ID:    lensHighPriorityStartID,
			Name:  lensHighPriorityStarter,
			Mode:  LensModeAll,
			Query: fmt.Sprintf("!%d+", models.PriorityHigh),
			Sort:  LensSort{Field: LensSortPriority, Ascending: true},
		},
		{
			ID:    lensStaleStarterID,
			Name:  lensStaleStarterName,
			Mode:  LensModeAll,
			Query: "?stale",
			Sort:  LensSort{Field: LensSortUpdated, Ascending: true},
		},
		{
			ID:    lensOverdueTemplateID,
			Name:  lensOverdueTemplateName,
			Mode:  LensModeDue,
			Query: "@" + models.QueryDueOverdue,
			Sort:  LensSort{Field: LensSortDefault, Ascending: true},
		},
		{
			ID:    lensActiveWorkTmplID,
			Name:  lensActiveWorkTmplName,
			Mode:  LensModeAll,
			Query: "$" + string(models.StateDoing),
			Sort:  LensSort{Field: LensSortPriority, Ascending: true},
		},
		{
			ID:    lensReviewQueueTmplID,
			Name:  lensReviewQueueTmplName,
			Mode:  LensModeAll,
			Query: "=review",
			Sort:  LensSort{Field: LensSortUpdated, Ascending: true},
		},
		{
			ID:    lensDeferredTmplID,
			Name:  lensDeferredTmplName,
			Mode:  LensModeAll,
			Query: "?deferred",
			Sort:  LensSort{Field: LensSortUpdated, Ascending: true},
		},
		{
			ID:   lensCleanup90dTmplID,
			Name: lensCleanup90dTmplName,
			Mode: LensModeAll,
			// Canonical order sorts one field's values, so canceled precedes done.
			Query: fmt.Sprintf("$%s $%s ?completed:90d", models.StateCanceled, models.StateDone),
			Sort:  LensSort{Field: LensSortUpdated, Ascending: true},
		},
		{
			ID:    lensParkedWorkTmplID,
			Name:  lensParkedWorkTmplName,
			Mode:  LensModeAll,
			Query: "+" + string(models.QueryFieldParked) + ":" + models.QueryPopulationOnly,
			Sort:  LensSort{Field: LensSortDefault, Ascending: true},
		},
	}
}

// BuiltinNamedLensTemplates returns the whole built-in catalog in catalog
// order, fully detached on every call. A template is only a definition: adding
// one persists an ordinary lens, and nothing downstream may treat the result as
// special.
func BuiltinNamedLensTemplates() NamedLensList {
	return builtinNamedLensDefinitions()
}

// StarterNamedLenses returns the three ordinary starter records seeded into a
// machine state that has never persisted a lens list. They are the first three
// catalog templates and nothing more: the remaining six are added on demand and
// never seeded. They are editable and removable like any other lens.
func StarterNamedLenses() NamedLensList {
	catalog := builtinNamedLensDefinitions()
	return append(NamedLensList(nil), catalog[:starterNamedLensCount]...)
}

// Clone returns a deep, independently mutable copy of the list. Every nested
// slice is detached so a runtime snapshot and a committed copy can never alias.
func (l NamedLensList) Clone() NamedLensList {
	if l == nil {
		return nil
	}
	out := make(NamedLensList, len(l))
	for i, lens := range l {
		out[i] = lens.Clone()
	}
	return out
}

// Clone returns a copy of one lens. Every axis is a value, so the copy is
// already independent; the method stays because the list clone reads better for
// it and because a caller should not have to know that.
func (n NamedLens) Clone() NamedLens {
	return n
}

// NormalizeNamedLensName trims a raw name and enforces the durable name rules:
// non-empty, within the text-prompt rune boundary, and free of control or
// newline characters.
func NormalizeNamedLensName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", fmt.Errorf("lens name cannot be empty")
	}
	if utf8.RuneCountInString(name) > MaxNamedLensNameRunes {
		return "", fmt.Errorf("lens name exceeds maximum length of %d characters", MaxNamedLensNameRunes)
	}
	for _, r := range name {
		if r == '\n' || r == '\r' || unicode.IsControl(r) {
			return "", fmt.Errorf("lens name %q contains a control character", name)
		}
	}
	return name, nil
}

// validateCanonicalUUID accepts only the canonical lowercase hyphenated UUID
// form. The urn: and braced spellings uuid.Parse tolerates are rejected so one
// identity has exactly one persisted rendering. label names the field being
// checked so the caller does not have to rewrap the error. It is shared by
// every persisted UUID reference in this layer: a lens identity and the project
// ID a lens scope names are minted the same way (uuid.New().String()), so they
// are held to the same spelling.
func validateCanonicalUUID(label, id string) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("%s %q is not a UUID: %w", label, id, err)
	}
	if parsed.String() != id {
		return fmt.Errorf("%s %q is not in canonical UUID form", label, id)
	}
	return nil
}

// ValidateNamedLensID accepts only the canonical UUID form and, unlike a lens
// scope, refuses the empty string: a lens must have an identity.
func ValidateNamedLensID(id string) error {
	if id == "" {
		return fmt.Errorf("lens id cannot be empty")
	}
	return validateCanonicalUUID("lens id", id)
}

// Validate reports problems with one lens definition. Token axes accept the
// empty string as "the default choice" because that is what omitempty writes;
// any other unrecognized spelling is malformed.
func (n NamedLens) Validate() error {
	if err := ValidateNamedLensID(n.ID); err != nil {
		return err
	}
	name, err := NormalizeNamedLensName(n.Name)
	if err != nil {
		return err
	}
	if name != n.Name {
		return fmt.Errorf("lens name %q is not trimmed", n.Name)
	}
	if !validLensToken(n.Mode, lensModeTokens(), false) {
		return fmt.Errorf("lens %q mode %q is not a known Outliner mode", n.Name, n.Mode)
	}
	// An empty scope is the unowned axis, not a malformed reference. A named
	// one is checked for shape only: whether that project still exists is a
	// question for the runtime, never for loading a machine file.
	if n.Scope != "" {
		if err := validateCanonicalUUID(fmt.Sprintf("lens %q scope project id", n.Name), n.Scope); err != nil {
			return err
		}
	}
	if err := validateNamedLensQuery(n.Name, n.Query); err != nil {
		return err
	}
	return n.Sort.validate(n.Name)
}

// validateNamedLensQuery holds the one narrowing string to the durable bound and
// nothing else. The query is deliberately not parsed here: this layer persists
// the string, the grammar that compiles it lives above, and an unreadable one
// keeps the ordinary invalid-query guidance at projection time rather than making
// a whole workspace file unloadable. What must hold on load is the length, since
// only a bound checked here is a bound at all, and the absence of a control
// character, since a lens query is one line.
func validateNamedLensQuery(lensName, query string) error {
	if utf8.RuneCountInString(query) > MaxNamedLensQueryRunes {
		return fmt.Errorf(
			"lens %q query exceeds maximum length of %d characters",
			lensName, MaxNamedLensQueryRunes,
		)
	}
	for _, r := range query {
		if r == '\n' || r == '\r' || unicode.IsControl(r) {
			return fmt.Errorf("lens %q query contains a control character", lensName)
		}
	}
	return nil
}

func (s LensSort) validate(lensName string) error {
	if !validLensToken(s.Field, lensSortTokens(), true) {
		return fmt.Errorf("lens %q sort field %q is not a known value", lensName, s.Field)
	}
	return nil
}

// Validate reports problems with the whole collection, including the identity
// and name uniqueness rules a single definition cannot see. The messages name no
// home: the collection is persisted both machine-locally and workspace-side, and
// each home's own Validate supplies the prefix.
func (l NamedLensList) Validate() error {
	seenID := make(map[string]bool, len(l))
	seenName := make(map[string]bool, len(l))
	for i, lens := range l {
		if err := lens.Validate(); err != nil {
			return fmt.Errorf("named lens %d: %w", i, err)
		}
		if seenID[lens.ID] {
			return fmt.Errorf("named lens %d repeats id %q", i, lens.ID)
		}
		seenID[lens.ID] = true
		folded := strings.ToLower(lens.Name)
		if seenName[folded] {
			return fmt.Errorf("named lens %d repeats name %q", i, lens.Name)
		}
		seenName[folded] = true
	}
	return nil
}

// validLensToken reports whether a token is known. When allowDefault is set the
// empty string is accepted as the omitted default rendering of that axis.
func validLensToken(token string, allowed []string, allowDefault bool) bool {
	if token == lensTokenDefaultSentinel {
		return allowDefault
	}
	return slices.Contains(allowed, token)
}

func lensModeTokens() []string {
	return []string{LensModeAll, LensModeInbox, LensModeToday, LensModeNext, LensModeBacklog, LensModeDue, LensModeRecent}
}

func lensSortTokens() []string {
	return []string{LensSortDefault, LensSortPriority, LensSortDue, LensSortCreated, LensSortUpdated, LensSortTitle}
}
