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

// Named lenses are machine-local, workspace-scoped Outliner projections. The
// config layer owns their durable shape so the TUI never persists its own
// integer enums: every axis below is a stable string token whose spelling is
// the durable identity of that choice.

// MaxNamedLensNameRunes bounds a lens name at the same 200-rune boundary the
// TUI text prompt already enforces, so a name that can be typed is a name that
// can be stored.
const MaxNamedLensNameRunes = 200

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

// Project population tokens for the parked and archived axes.
const (
	LensPopulationExclude = "exclude"
	LensPopulationInclude = "include"
	LensPopulationOnly    = "only"
)

// Manual sort field tokens.
const (
	LensSortDefault  = "default"
	LensSortPriority = "priority"
	LensSortDue      = "due"
	LensSortCreated  = "created"
	LensSortStatus   = "status"
	LensSortUpdated  = "updated"
	LensSortTitle    = "title"
)

// Due-bucket tokens.
const (
	LensDueAll     = "all"
	LensDueOverdue = "overdue"
	LensDueToday   = "today"
	LensDueWeek    = "week"
	LensDueNone    = "none"
)

// Hierarchy-position tokens.
const (
	LensHierarchyAll    = "all"
	LensHierarchyParent = "parent"
	LensHierarchyChild  = "child"
	LensHierarchyRoot   = "root"
	LensHierarchyLeaf   = "leaf"
)

// Completion-age tokens. The activation flag lives beside them because the
// "all" threshold is an active cleanup facet, not an absent one.
const (
	LensCompletionAgeAll    = "all"
	LensCompletionAge7Days  = "7d"
	LensCompletionAge30Days = "30d"
	LensCompletionAge90Days = "90d"
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

// lensReviewQueueTag is the task tag the Review Queue template selects on.
const lensReviewQueueTag = "review"

// LensTaskFilters is the persisted form of the Outliner's shared task facets.
// StatesSet and CompletionAgeSet are retained explicitly because each
// distinguishes an absent predicate from an active one that happens to admit
// everything.
type LensTaskFilters struct {
	States           []string `json:"states,omitempty"`
	StatesSet        bool     `json:"states_set,omitempty"`
	Priority         int      `json:"priority,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	DueDate          string   `json:"due_date,omitempty"`
	CompletionAge    string   `json:"completion_age,omitempty"`
	CompletionAgeSet bool     `json:"completion_age_set,omitempty"`
	Hierarchy        string   `json:"hierarchy,omitempty"`
	Stale            bool     `json:"stale,omitempty"`
	Deferred         bool     `json:"deferred,omitempty"`
	Pinned           bool     `json:"pinned,omitempty"`
}

// LensProjectFilters is the persisted form of the Outliner's population facet.
type LensProjectFilters struct {
	Tags         []string `json:"tags,omitempty"`
	Untagged     bool     `json:"untagged,omitempty"`
	ParkedMode   string   `json:"parked,omitempty"`
	ArchivedMode string   `json:"archived,omitempty"`
}

// LensSort is the persisted manual sort axis.
type LensSort struct {
	Field     string `json:"field,omitempty"`
	Ascending bool   `json:"ascending"`
}

// NamedLens is one durable Outliner projection: identity, mode, accepted query,
// both filter families, and the manual sort it restores.
type NamedLens struct {
	ID       string             `json:"id"`
	Name     string             `json:"name"`
	Mode     string             `json:"mode"`
	Query    string             `json:"query,omitempty"`
	Tasks    LensTaskFilters    `json:"tasks"`
	Projects LensProjectFilters `json:"projects"`
	Sort     LensSort           `json:"sort"`
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
func builtinNamedLensDefinitions() NamedLensList {
	return NamedLensList{
		{
			ID:   lensWaitingStarterID,
			Name: lensWaitingStarterName,
			Mode: LensModeAll,
			Tasks: LensTaskFilters{
				States:    []string{string(models.StateWaiting)},
				StatesSet: true,
			},
			Sort: LensSort{Field: LensSortDefault, Ascending: true},
		},
		{
			ID:   lensHighPriorityStartID,
			Name: lensHighPriorityStarter,
			Mode: LensModeAll,
			Tasks: LensTaskFilters{
				Priority: models.PriorityHigh,
			},
			Sort: LensSort{Field: LensSortPriority, Ascending: true},
		},
		{
			ID:   lensStaleStarterID,
			Name: lensStaleStarterName,
			Mode: LensModeAll,
			Tasks: LensTaskFilters{
				Stale: true,
			},
			Sort: LensSort{Field: LensSortUpdated, Ascending: true},
		},
		{
			ID:   lensOverdueTemplateID,
			Name: lensOverdueTemplateName,
			Mode: LensModeDue,
			Tasks: LensTaskFilters{
				DueDate: LensDueOverdue,
			},
			Sort: LensSort{Field: LensSortDefault, Ascending: true},
		},
		{
			ID:   lensActiveWorkTmplID,
			Name: lensActiveWorkTmplName,
			Mode: LensModeAll,
			Tasks: LensTaskFilters{
				States:    []string{string(models.StateDoing)},
				StatesSet: true,
			},
			Sort: LensSort{Field: LensSortPriority, Ascending: true},
		},
		{
			ID:   lensReviewQueueTmplID,
			Name: lensReviewQueueTmplName,
			Mode: LensModeAll,
			Tasks: LensTaskFilters{
				Tags: []string{lensReviewQueueTag},
			},
			Sort: LensSort{Field: LensSortUpdated, Ascending: true},
		},
		{
			ID:   lensDeferredTmplID,
			Name: lensDeferredTmplName,
			Mode: LensModeAll,
			Tasks: LensTaskFilters{
				Deferred: true,
			},
			Sort: LensSort{Field: LensSortUpdated, Ascending: true},
		},
		{
			ID:   lensCleanup90dTmplID,
			Name: lensCleanup90dTmplName,
			Mode: LensModeAll,
			Tasks: LensTaskFilters{
				States:           []string{string(models.StateDone), string(models.StateCanceled)},
				StatesSet:        true,
				CompletionAge:    LensCompletionAge90Days,
				CompletionAgeSet: true,
			},
			Sort: LensSort{Field: LensSortUpdated, Ascending: true},
		},
		{
			ID:   lensParkedWorkTmplID,
			Name: lensParkedWorkTmplName,
			Mode: LensModeAll,
			Projects: LensProjectFilters{
				ParkedMode: LensPopulationOnly,
			},
			Sort: LensSort{Field: LensSortDefault, Ascending: true},
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

// Clone returns a deep copy of one lens.
func (n NamedLens) Clone() NamedLens {
	cp := n
	cp.Tasks.States = slices.Clone(n.Tasks.States)
	cp.Tasks.Tags = slices.Clone(n.Tasks.Tags)
	cp.Projects.Tags = slices.Clone(n.Projects.Tags)
	return cp
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

// ValidateNamedLensID accepts only the canonical lowercase hyphenated UUID
// form. The urn: and braced spellings uuid.Parse tolerates are rejected so one
// identity has exactly one persisted rendering.
func ValidateNamedLensID(id string) error {
	if id == "" {
		return fmt.Errorf("lens id cannot be empty")
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("lens id %q is not a UUID: %w", id, err)
	}
	if parsed.String() != id {
		return fmt.Errorf("lens id %q is not in canonical UUID form", id)
	}
	return nil
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
	if err := n.Tasks.validate(n.Name); err != nil {
		return err
	}
	if err := n.Projects.validate(n.Name); err != nil {
		return err
	}
	return n.Sort.validate(n.Name)
}

func (f LensTaskFilters) validate(lensName string) error {
	seenState := make(map[string]bool, len(f.States))
	for _, raw := range f.States {
		state := models.WorkflowState(raw)
		if !state.IsValid() {
			return fmt.Errorf("lens %q references unknown workflow state %q", lensName, raw)
		}
		if seenState[raw] {
			return fmt.Errorf("lens %q repeats workflow state %q", lensName, raw)
		}
		seenState[raw] = true
	}
	if f.Priority != models.PriorityUnset {
		if err := models.ValidatePriority(f.Priority, false); err != nil {
			return fmt.Errorf("lens %q priority: %w", lensName, err)
		}
	}
	seenTag := make(map[string]bool, len(f.Tags))
	for _, tag := range f.Tags {
		if err := models.ValidateTag(tag); err != nil {
			return fmt.Errorf("lens %q task tag: %w", lensName, err)
		}
		if seenTag[tag] {
			return fmt.Errorf("lens %q repeats task tag %q", lensName, tag)
		}
		seenTag[tag] = true
	}
	if !validLensToken(f.DueDate, lensDueTokens(), true) {
		return fmt.Errorf("lens %q due bucket %q is not a known value", lensName, f.DueDate)
	}
	if !validLensToken(f.CompletionAge, lensCompletionAgeTokens(), true) {
		return fmt.Errorf("lens %q completion age %q is not a known value", lensName, f.CompletionAge)
	}
	if !validLensToken(f.Hierarchy, lensHierarchyTokens(), true) {
		return fmt.Errorf("lens %q hierarchy %q is not a known value", lensName, f.Hierarchy)
	}
	return nil
}

func (f LensProjectFilters) validate(lensName string) error {
	seen := make(map[string]bool, len(f.Tags))
	for _, tag := range f.Tags {
		if err := models.ValidateTag(tag); err != nil {
			return fmt.Errorf("lens %q project tag: %w", lensName, err)
		}
		if seen[tag] {
			return fmt.Errorf("lens %q repeats project tag %q", lensName, tag)
		}
		seen[tag] = true
	}
	if !validLensToken(f.ParkedMode, lensPopulationTokens(), true) {
		return fmt.Errorf("lens %q parked population %q is not a known value", lensName, f.ParkedMode)
	}
	if !validLensToken(f.ArchivedMode, lensPopulationTokens(), true) {
		return fmt.Errorf("lens %q archived population %q is not a known value", lensName, f.ArchivedMode)
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
// and name uniqueness rules a single definition cannot see.
func (l NamedLensList) Validate() error {
	seenID := make(map[string]bool, len(l))
	seenName := make(map[string]bool, len(l))
	for i, lens := range l {
		if err := lens.Validate(); err != nil {
			return fmt.Errorf("machine: named lens %d: %w", i, err)
		}
		if seenID[lens.ID] {
			return fmt.Errorf("machine: named lens %d repeats id %q", i, lens.ID)
		}
		seenID[lens.ID] = true
		folded := strings.ToLower(lens.Name)
		if seenName[folded] {
			return fmt.Errorf("machine: named lens %d repeats name %q", i, lens.Name)
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

func lensPopulationTokens() []string {
	return []string{LensPopulationExclude, LensPopulationInclude, LensPopulationOnly}
}

func lensSortTokens() []string {
	return []string{LensSortDefault, LensSortPriority, LensSortDue, LensSortCreated, LensSortStatus, LensSortUpdated, LensSortTitle}
}

func lensDueTokens() []string {
	return []string{LensDueAll, LensDueOverdue, LensDueToday, LensDueWeek, LensDueNone}
}

func lensHierarchyTokens() []string {
	return []string{LensHierarchyAll, LensHierarchyParent, LensHierarchyChild, LensHierarchyRoot, LensHierarchyLeaf}
}

func lensCompletionAgeTokens() []string {
	return []string{LensCompletionAgeAll, LensCompletionAge7Days, LensCompletionAge30Days, LensCompletionAge90Days}
}
