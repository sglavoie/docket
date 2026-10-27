package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMachineStateLensJSONOmissionSemantics(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantErr bool
		wantLen int
	}{
		// Three, literally: the catalog has nine templates and an omitted key
		// must still seed only the three starters.
		{name: "omitted seeds exactly three starters", payload: `{"theme":"light"}`, wantLen: 3},
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

func TestMachineStateMenuBarVisibleJSONRoundTrip(t *testing.T) {
	m := NewMachineState()
	if m.MenuBarVisible {
		t.Fatal("a fresh machine must open with the menu bar hidden")
	}
	m.MenuBarVisible = true
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"menu_bar_visible":true`) {
		t.Fatalf("a shown menu bar missing from payload: %s", data)
	}
	reloaded := NewMachineState()
	if err := json.Unmarshal(data, reloaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reloaded.MenuBarVisible {
		t.Fatalf("menu bar visibility did not survive the round trip: %s", data)
	}
	if err := reloaded.Validate(); err != nil {
		t.Fatalf("round-tripped machine state should validate: %v", err)
	}

	// The field is chrome, not a lens axis: it may not reach into the collection
	// the round trip above carries beside it.
	if len(reloaded.NamedLenses) != len(m.NamedLenses) {
		t.Fatalf("lens count = %d, want %d", len(reloaded.NamedLenses), len(m.NamedLenses))
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
	// Lens definitions own their home, machine-local or workspace-side: neither
	// the catalog nor the planning domain may carry them.
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
	if err = s.SaveMachine(reloaded); err != nil {
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
		{name: "legacy payload without the key seeds exactly three starters", payload: `{"theme":"light"}`, wantLen: 3},
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
	broken.Query = strings.Repeat("x", MaxNamedLensQueryRunes+1)
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

// lensScopeProjectID is shaped exactly like a real project ID, which
// internal/app mints with uuid.New().String(); that is why a lens scope is held
// to the same canonical spelling a lens identity is.
const lensScopeProjectID = "3c2a1f60-4e5d-4b8a-9c7e-2d1f0a3b4c5d"

func TestMachineStateMouseDisabledJSONRoundTripAndOmission(t *testing.T) {
	// machine.json decodes with DisallowUnknownFields, so a machine that never
	// turned the mouse off must write no key at all: older builds then read it.
	bare, err := json.Marshal(NewMachineState())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(bare), "mouse_disabled") {
		t.Fatalf("a mouse left on must omit the key entirely: %s", bare)
	}
	m := NewMachineState()
	m.MouseDisabled = true
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"mouse_disabled":true`) {
		t.Fatalf("a disabled mouse missing from payload: %s", data)
	}
	reloaded := NewMachineState()
	if err := json.Unmarshal(data, reloaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reloaded.MouseDisabled {
		t.Fatalf("the mouse setting did not survive the round trip: %s", data)
	}
	if err := reloaded.Validate(); err != nil {
		t.Fatalf("round-tripped machine state should validate: %v", err)
	}
}
