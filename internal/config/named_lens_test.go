package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// validLens returns a definition that passes validation, so each test can make
// exactly one thing wrong.
func validLens() NamedLens {
	return NamedLens{
		ID:   "9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01",
		Name: "Review Queue",
		Mode: LensModeAll,
		Tasks: LensTaskFilters{
			States:           []string{string(models.StateWaiting)},
			StatesSet:        true,
			Priority:         models.PriorityMedium,
			Tags:             []string{"deep-work"},
			DueDate:          LensDueWeek,
			CompletionAge:    LensCompletionAge30Days,
			CompletionAgeSet: true,
			Hierarchy:        LensHierarchyRoot,
			Stale:            true,
			Deferred:         true,
			Pinned:           true,
		},
		Projects: LensProjectFilters{
			Tags:         []string{"client"},
			Untagged:     true,
			ParkedMode:   LensPopulationInclude,
			ArchivedMode: LensPopulationOnly,
		},
		Sort: LensSort{Field: LensSortUpdated, Ascending: true},
	}
}

func TestNewMachineStateSeedsStarterLenses(t *testing.T) {
	m := NewMachineState()
	if err := m.Validate(); err != nil {
		t.Fatalf("seeded machine state should validate: %v", err)
	}
	names := make([]string, len(m.NamedLenses))
	for i, lens := range m.NamedLenses {
		names[i] = lens.Name
	}
	want := []string{"Waiting", "High Priority", "Stale"}
	if len(names) != len(want) {
		t.Fatalf("starter lens names = %v, want %v", names, want)
	}
	for i, name := range want {
		if names[i] != name {
			t.Fatalf("starter lens names = %v, want %v", names, want)
		}
	}

	waiting := m.NamedLensByID(lensWaitingStarterID)
	if waiting == nil {
		t.Fatal("Waiting starter missing")
	}
	if waiting.Mode != LensModeAll || !waiting.Tasks.StatesSet ||
		len(waiting.Tasks.States) != 1 || waiting.Tasks.States[0] != string(models.StateWaiting) {
		t.Fatalf("Waiting starter = %+v, want All mode with the waiting state", waiting)
	}
	if waiting.Sort.Field != LensSortDefault || !waiting.Sort.Ascending {
		t.Fatalf("Waiting starter sort = %+v, want default ascending", waiting.Sort)
	}

	high := m.NamedLensByID(lensHighPriorityStartID)
	if high == nil {
		t.Fatal("High Priority starter missing")
	}
	if high.Mode != LensModeAll || high.Tasks.Priority != models.PriorityHigh {
		t.Fatalf("High Priority starter = %+v, want All mode at the High threshold", high)
	}
	if high.Sort.Field != LensSortPriority || !high.Sort.Ascending {
		t.Fatalf("High Priority starter sort = %+v, want priority ascending", high.Sort)
	}

	stale := m.NamedLensByID(lensStaleStarterID)
	if stale == nil {
		t.Fatal("Stale starter missing")
	}
	if stale.Mode != LensModeAll || !stale.Tasks.Stale {
		t.Fatalf("Stale starter = %+v, want All mode with the stale facet", stale)
	}
	if stale.Sort.Field != LensSortUpdated || !stale.Sort.Ascending {
		t.Fatalf("Stale starter sort = %+v, want updated ascending", stale.Sort)
	}
}

func TestStarterLensIDsAreCanonicalUUIDs(t *testing.T) {
	for _, lens := range StarterNamedLenses() {
		if err := ValidateNamedLensID(lens.ID); err != nil {
			t.Errorf("starter %q id: %v", lens.Name, err)
		}
	}
}

func TestValidateNamedLensID(t *testing.T) {
	cases := map[string]bool{
		"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01":   true,
		"":                                       false,
		"not-a-uuid":                             false,
		"9F8B8E42-7C3A-4D61-8F2A-1B0C9D7E5A01":   false, // uppercase is not canonical
		"{9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01}": false,
		"urn:uuid:9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01":   false,
		"9f8b8e427c3a4d618f2a1b0c9d7e5a01":                false, // unhyphenated
		"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01-9f8b8e4200": false,
	}
	for id, wantValid := range cases {
		err := ValidateNamedLensID(id)
		if wantValid && err != nil {
			t.Errorf("ValidateNamedLensID(%q) = %v, want nil", id, err)
		}
		if !wantValid && err == nil {
			t.Errorf("ValidateNamedLensID(%q) = nil, want error", id)
		}
	}
}

func TestNormalizeNamedLensName(t *testing.T) {
	got, err := NormalizeNamedLensName("  Review Queue  ")
	if err != nil {
		t.Fatalf("NormalizeNamedLensName: %v", err)
	}
	if got != "Review Queue" {
		t.Errorf("NormalizeNamedLensName trimmed = %q, want %q", got, "Review Queue")
	}

	for _, raw := range []string{"", "   ", "line\nbreak", "carriage\rreturn", "bell\a", strings.Repeat("x", MaxNamedLensNameRunes+1)} {
		if _, err := NormalizeNamedLensName(raw); err == nil {
			t.Errorf("NormalizeNamedLensName(%q) = nil error, want rejection", raw)
		}
	}
	if _, err := NormalizeNamedLensName(strings.Repeat("é", MaxNamedLensNameRunes)); err != nil {
		t.Errorf("a %d-rune multibyte name should fit: %v", MaxNamedLensNameRunes, err)
	}
}

func TestNamedLensValidateRejectsMalformedTokens(t *testing.T) {
	cases := []struct {
		name  string
		mutas func(*NamedLens)
	}{
		{"mode", func(l *NamedLens) { l.Mode = "everything" }},
		{"empty mode", func(l *NamedLens) { l.Mode = "" }},
		{"due bucket", func(l *NamedLens) { l.Tasks.DueDate = "fortnight" }},
		{"completion age", func(l *NamedLens) { l.Tasks.CompletionAge = "14d" }},
		{"hierarchy", func(l *NamedLens) { l.Tasks.Hierarchy = "sibling" }},
		{"sort field", func(l *NamedLens) { l.Sort.Field = "color" }},
		{"parked population", func(l *NamedLens) { l.Projects.ParkedMode = "maybe" }},
		{"archived population", func(l *NamedLens) { l.Projects.ArchivedMode = "maybe" }},
		{"workflow state", func(l *NamedLens) { l.Tasks.States = []string{"ready"} }},
		{"repeated workflow state", func(l *NamedLens) { l.Tasks.States = []string{"doing", "doing"} }},
		{"priority", func(l *NamedLens) { l.Tasks.Priority = 4 }},
		{"task tag case", func(l *NamedLens) { l.Tasks.Tags = []string{"DeepWork"} }},
		{"repeated task tag", func(l *NamedLens) { l.Tasks.Tags = []string{"deep-work", "deep-work"} }},
		{"project tag case", func(l *NamedLens) { l.Projects.Tags = []string{"Client"} }},
		{"repeated project tag", func(l *NamedLens) { l.Projects.Tags = []string{"client", "client"} }},
		{"untrimmed name", func(l *NamedLens) { l.Name = " Review Queue" }},
		{"id", func(l *NamedLens) { l.ID = "lens-1" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lens := validLens()
			tc.mutas(&lens)
			if err := lens.Validate(); err == nil {
				t.Fatalf("%s should be rejected", tc.name)
			}
		})
	}
}

func TestNamedLensValidateAcceptsCanonicalDefinition(t *testing.T) {
	if err := validLens().Validate(); err != nil {
		t.Fatalf("canonical lens should validate: %v", err)
	}
}

func TestNamedLensValidateAcceptsOmittedTokenDefaults(t *testing.T) {
	// omitempty writes "" for every axis left at its default, so loading one
	// back must read as the default rather than as a malformed token.
	lens := NamedLens{
		ID:   "9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a02",
		Name: "Bare",
		Mode: LensModeToday,
	}
	if err := lens.Validate(); err != nil {
		t.Fatalf("bare lens should validate: %v", err)
	}
}

func TestNamedLensValidateAcceptsCanonicalUnusedTags(t *testing.T) {
	// A canonical tag no task carries is valid; it simply matches nothing.
	lens := validLens()
	lens.Tasks.Tags = []string{"nobody-uses-this"}
	lens.Projects.Tags = []string{"nor-this-one"}
	if err := lens.Validate(); err != nil {
		t.Fatalf("canonical unused tags should validate: %v", err)
	}
}

func TestNamedLensListValidateRejectsDuplicates(t *testing.T) {
	base := validLens()

	dupID := base
	dupID.Name = "Other Name"
	if err := (NamedLensList{base, dupID}).Validate(); err == nil {
		t.Error("duplicate lens id should be rejected")
	}

	dupName := base
	dupName.ID = "9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a03"
	dupName.Name = strings.ToUpper(base.Name)
	if err := (NamedLensList{base, dupName}).Validate(); err == nil {
		t.Error("case-insensitively duplicate lens name should be rejected")
	}

	distinct := base
	distinct.ID = "9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a04"
	distinct.Name = "Something Else"
	if err := (NamedLensList{base, distinct}).Validate(); err != nil {
		t.Errorf("distinct lenses should validate: %v", err)
	}
}

func TestMachineStateValidateRejectsInvalidLens(t *testing.T) {
	m := NewMachineState()
	broken := validLens()
	broken.Mode = "everything"
	m.NamedLenses = append(m.NamedLenses, broken)
	if err := m.Validate(); err == nil {
		t.Fatal("machine state carrying a malformed lens should not validate")
	}
}

func TestMachineStateRemoveNamedLens(t *testing.T) {
	m := NewMachineState()
	if !m.RemoveNamedLens(lensStaleStarterID) {
		t.Fatal("removing a present lens should report a change")
	}
	if m.NamedLensByID(lensStaleStarterID) != nil {
		t.Fatal("removed lens is still reachable")
	}
	if m.RemoveNamedLens(lensStaleStarterID) {
		t.Fatal("removing an absent lens should report no change")
	}
	for _, lens := range StarterNamedLenses() {
		m.RemoveNamedLens(lens.ID)
	}
	if len(m.NamedLenses) != 0 {
		t.Fatalf("expected an empty list, got %+v", m.NamedLenses)
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"named_lenses":[]`) {
		t.Fatalf("emptied lens list must persist as [], got %s", data)
	}
}

func TestNamedLensCloneDetachesNestedSlices(t *testing.T) {
	original := validLens()
	clone := original.Clone()
	clone.Tasks.States[0] = string(models.StateDone)
	clone.Tasks.Tags[0] = "mutated"
	clone.Projects.Tags[0] = "mutated"
	if original.Tasks.States[0] != string(models.StateWaiting) {
		t.Error("clone aliases the states slice")
	}
	if original.Tasks.Tags[0] != "deep-work" {
		t.Error("clone aliases the task tag slice")
	}
	if original.Projects.Tags[0] != "client" {
		t.Error("clone aliases the project tag slice")
	}
}

func TestNamedLensListCloneDetachesFromSnapshot(t *testing.T) {
	snapshot := NewMachineState()
	clone := snapshot.NamedLenses.Clone()
	clone[0].Name = "Renamed"
	clone[0].Tasks.States[0] = string(models.StateDone)
	clone = append(clone, validLens())

	if snapshot.NamedLenses[0].Name != "Waiting" {
		t.Error("clone aliases the lens name")
	}
	if snapshot.NamedLenses[0].Tasks.States[0] != string(models.StateWaiting) {
		t.Error("clone aliases a nested state slice")
	}
	if len(snapshot.NamedLenses) != 3 {
		t.Errorf("appending to the clone changed the snapshot: %d lenses", len(snapshot.NamedLenses))
	}
	if NamedLensList(nil).Clone() != nil {
		t.Error("cloning a nil list should stay nil")
	}
}

func TestMachineStateLensJSONOmissionSemantics(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantErr bool
		wantLen int
	}{
		{name: "omitted seeds starters", payload: `{"theme":"light"}`, wantLen: len(StarterNamedLenses())},
		{name: "explicit empty stays empty", payload: `{"theme":"light","named_lenses":[]}`, wantLen: 0},
		{name: "null is invalid", payload: `{"theme":"light","named_lenses":null}`, wantErr: true},
		{name: "unknown nested field is invalid", payload: `{"theme":"light","named_lenses":[{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all","nope":1}]}`, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := NewMachineState()
			err := json.Unmarshal([]byte(tc.payload), out)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("payload %s should fail to decode", tc.payload)
				}
				return
			}
			if err != nil {
				t.Fatalf("decode %s: %v", tc.payload, err)
			}
			if len(out.NamedLenses) != tc.wantLen {
				t.Fatalf("lens count = %d, want %d", len(out.NamedLenses), tc.wantLen)
			}
		})
	}
}

func TestMachineStateLensJSONRoundTrip(t *testing.T) {
	m := NewMachineState()
	m.NamedLenses = append(m.NamedLenses, validLens())
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	reloaded := NewMachineState()
	if err := json.Unmarshal(data, reloaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(reloaded.NamedLenses) != len(m.NamedLenses) {
		t.Fatalf("lens count = %d, want %d", len(reloaded.NamedLenses), len(m.NamedLenses))
	}
	got, want := reloaded.NamedLenses[len(reloaded.NamedLenses)-1], validLens()
	if gotJSON, wantJSON := mustJSON(t, got), mustJSON(t, want); gotJSON != wantJSON {
		t.Fatalf("round trip lost detail:\n got %s\nwant %s", gotJSON, wantJSON)
	}
	if err := reloaded.Validate(); err != nil {
		t.Fatalf("round-tripped machine state should validate: %v", err)
	}
}

func TestNilNamedLensListMarshalsAsEmptyArray(t *testing.T) {
	// A MachineState assembled without the constructor must never persist null,
	// which loading would reject.
	data, err := json.Marshal(&MachineState{Theme: "dark"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"named_lenses":[]`) {
		t.Fatalf("nil lens list should render as [], got %s", data)
	}
}

func TestNamedLensesStayOutOfSyncedDomains(t *testing.T) {
	// Lens definitions are machine-local: neither synced domain may carry them.
	catalog := NewWorkspaceCatalog()
	catalog.WorkspaceID = testWorkspaceID
	planning := NewPlanningState()
	planning.AddToToday("t1", "p1")
	for _, payload := range []any{catalog, planning} {
		data := mustJSON(t, payload)
		if strings.Contains(data, "named_lenses") || strings.Contains(data, "lens") {
			t.Fatalf("synced domain leaked lens data: %s", data)
		}
	}
}

func TestDomainFileStoreMachineLensRoundTrip(t *testing.T) {
	s := newIsolatedStore(t)

	machine := NewMachineState()
	machine.Theme = "light"
	machine.NamedLenses = append(machine.NamedLenses, validLens())
	if err := s.SaveMachine(machine); err != nil {
		t.Fatalf("SaveMachine: %v", err)
	}

	reloaded, err := NewDomainFileStore(s.DataDir()).LoadMachine()
	if err != nil {
		t.Fatalf("LoadMachine: %v", err)
	}
	if len(reloaded.NamedLenses) != 4 {
		t.Fatalf("lens count = %d, want 4", len(reloaded.NamedLenses))
	}
	if got, want := mustJSON(t, reloaded.NamedLenses), mustJSON(t, machine.NamedLenses); got != want {
		t.Fatalf("persisted lenses changed:\n got %s\nwant %s", got, want)
	}

	// Deleting every lens is durable: reload must not reseed the starters.
	reloaded.NamedLenses = NamedLensList{}
	if err := s.SaveMachine(reloaded); err != nil {
		t.Fatalf("SaveMachine emptied: %v", err)
	}
	emptied, err := NewDomainFileStore(s.DataDir()).LoadMachine()
	if err != nil {
		t.Fatalf("LoadMachine emptied: %v", err)
	}
	if len(emptied.NamedLenses) != 0 {
		t.Fatalf("emptied lens list reseeded: %+v", emptied.NamedLenses)
	}
}

func TestDomainFileStoreMachineLensLoadSemantics(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantErr bool
		wantLen int
	}{
		{name: "legacy payload without the key seeds starters", payload: `{"theme":"light"}`, wantLen: len(StarterNamedLenses())},
		{name: "explicit empty stays empty", payload: `{"theme":"light","named_lenses":[]}`, wantLen: 0},
		{name: "null is rejected", payload: `{"theme":"light","named_lenses":null}`, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newIsolatedStore(t)
			writeRawMachinePayload(t, tc.payload)

			loaded, err := NewDomainFileStore(s.DataDir()).LoadMachine()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("payload %s should fail to load", tc.payload)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadMachine: %v", err)
			}
			if len(loaded.NamedLenses) != tc.wantLen {
				t.Fatalf("lens count = %d, want %d", len(loaded.NamedLenses), tc.wantLen)
			}
			if err := loaded.Validate(); err != nil {
				t.Fatalf("loaded machine state should validate: %v", err)
			}
		})
	}
}

// A malformed token survives decoding — the JSON shape is legal — and is caught
// by validation, which is what every persisted write runs.
func TestMachineLensMalformedTokenFailsValidationNotDecoding(t *testing.T) {
	s := newIsolatedStore(t)
	writeRawMachinePayload(t, `{"theme":"light","named_lenses":[{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"everything"}]}`)

	loaded, err := NewDomainFileStore(s.DataDir()).LoadMachine()
	if err != nil {
		t.Fatalf("LoadMachine: %v", err)
	}
	if err := loaded.Validate(); err == nil {
		t.Fatal("a malformed mode token should fail validation")
	}
	if err := s.SaveMachine(loaded); err == nil {
		t.Fatal("a malformed mode token should never be persisted")
	}
}

func TestSaveMachineRejectsInvalidLens(t *testing.T) {
	s := newIsolatedStore(t)
	machine := NewMachineState()
	broken := validLens()
	broken.Tasks.Hierarchy = "sibling"
	machine.NamedLenses = append(machine.NamedLenses, broken)
	if err := s.SaveMachine(machine); err == nil {
		t.Fatal("saving a machine state with a malformed lens should fail")
	}
}

// writeRawMachinePayload replaces machine.json with a hand-authored payload
// wrapped in the ordinary domain envelope.
func writeRawMachinePayload(t *testing.T, payload string) {
	t.Helper()
	data, err := marshalDomainEnvelope(domainMachine, testWorkspaceID, json.RawMessage(payload))
	if err != nil {
		t.Fatalf("marshalDomainEnvelope: %v", err)
	}
	path, err := MachineStatePath(testWorkspaceID)
	if err != nil {
		t.Fatalf("MachineStatePath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, FilePermissions); err != nil {
		t.Fatalf("write machine.json: %v", err)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(data)
}
