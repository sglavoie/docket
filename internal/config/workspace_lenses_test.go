package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// workspaceLensPayload renders the domain payload exactly as the envelope's
// Data field carries it, which is the surface the strict codec governs.
func workspaceLensPayload(t *testing.T, defs *WorkspaceLensDefinitions) string {
	t.Helper()
	data, err := json.Marshal(defs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(data)
}

func TestWorkspaceLensDefinitionsKeepTheStrictCodec(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantLen int
		defined bool
		wantErr bool
	}{
		{name: "explicit empty stays empty and defined", payload: `{"lenses":[]}`, wantLen: 0, defined: true},
		{name: "null is rejected", payload: `{"lenses":null}`, wantErr: true},
		{
			name:    "unknown nested field is rejected",
			payload: `{"lenses":[{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all","nope":1}]}`,
			wantErr: true,
		},
		{name: "omitted key leaves the collection undefined", payload: `{}`, wantLen: 0, defined: false},
		{
			name:    "one lens decodes",
			payload: `{"lenses":[{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all","tasks":{},"projects":{},"sort":{"ascending":true}}]}`,
			wantLen: 1,
			defined: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defs := NewWorkspaceLensDefinitions()
			err := json.Unmarshal([]byte(tc.payload), defs)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("payload %s should not decode", tc.payload)
				}
				return
			}
			if err != nil {
				t.Fatalf("unmarshal %s: %v", tc.payload, err)
			}
			if len(defs.Lenses) != tc.wantLen {
				t.Fatalf("lens count = %d, want %d", len(defs.Lenses), tc.wantLen)
			}
			if defs.Defined() != tc.defined {
				t.Fatalf("Defined() = %v, want %v", defs.Defined(), tc.defined)
			}
		})
	}
}

func TestWorkspaceLensDefinitionsRoundTripPreservesDetail(t *testing.T) {
	defs := &WorkspaceLensDefinitions{Lenses: NamedLensList{validLens()}}
	reloaded := NewWorkspaceLensDefinitions()
	if err := json.Unmarshal([]byte(workspaceLensPayload(t, defs)), reloaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got, want := mustJSON(t, reloaded.Lenses[0]), mustJSON(t, validLens()); got != want {
		t.Fatalf("round trip lost detail:\n got %s\nwant %s", got, want)
	}
	if err := reloaded.Validate(); err != nil {
		t.Fatalf("round-tripped definitions should validate: %v", err)
	}
}

func TestWorkspaceLensDefinitionsRenderAnEmptySetAsAnArray(t *testing.T) {
	// A nil collection is the absent record, not an empty one, so it is never
	// published; an empty-but-defined one must persist as [] rather than null,
	// which loading would reject.
	if got := workspaceLensPayload(t, &WorkspaceLensDefinitions{Lenses: NamedLensList{}}); !strings.Contains(got, `"lenses":[]`) {
		t.Fatalf("empty definitions should render as [], got %s", got)
	}
}

func TestWorkspaceLensDefinitionsValidateNamesTheWorkspaceHome(t *testing.T) {
	if err := NewWorkspaceLensDefinitions().Validate(); err != nil {
		t.Fatalf("an undefined collection is the absence of a record, not a malformed one: %v", err)
	}
	duplicate := validLens()
	defs := &WorkspaceLensDefinitions{Lenses: NamedLensList{validLens(), duplicate}}
	err := defs.Validate()
	if err == nil {
		t.Fatal("a repeated lens id must be refused")
	}
	if !strings.HasPrefix(err.Error(), "workspace: ") {
		t.Fatalf("workspace validation error = %q, want a workspace-named prefix", err)
	}
	machine := NewMachineState()
	machine.NamedLenses = NamedLensList{validLens(), duplicate}
	machineErr := machine.Validate()
	if machineErr == nil || !strings.HasPrefix(machineErr.Error(), "machine: named lens ") {
		t.Fatalf("machine validation error = %v, want the unchanged machine-named prefix", machineErr)
	}
}

func TestDefinedLensListNormalizesNilAndDetaches(t *testing.T) {
	empty := DefinedLensList(nil)
	if empty == nil || len(empty) != 0 {
		t.Fatalf("nil should normalize to the empty-but-defined list, got %#v", empty)
	}
	source := NamedLensList{validLens()}
	detached := DefinedLensList(source)
	detached[0].Name = "Renamed"
	if source[0].Name == "Renamed" {
		t.Fatal("DefinedLensList must detach the caller's collection")
	}
}

func TestSaveLensDefinitionsRefusesAnUndefinedCollection(t *testing.T) {
	s := newIsolatedStore(t)
	if err := s.SaveLensDefinitionsChecked(nil, ""); err == nil {
		t.Fatal("nil definitions must be refused")
	}
	if err := s.SaveLensDefinitionsChecked(NewWorkspaceLensDefinitions(), ""); err == nil {
		t.Fatal("an undefined collection must be refused rather than published as []")
	}
	if _, err := os.Stat(LensDefinitionsPath(s.DataDir())); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused publication wrote an artifact, stat err = %v", err)
	}
}

func TestSaveLensDefinitionsRejectsAnInvalidCollection(t *testing.T) {
	s := newIsolatedStore(t)
	broken := validLens()
	broken.Mode = "gallery"
	if err := s.SaveLensDefinitionsChecked(&WorkspaceLensDefinitions{Lenses: NamedLensList{broken}}, ""); err == nil {
		t.Fatal("an unknown Outliner mode must be refused")
	}
	if _, err := os.Stat(LensDefinitionsPath(s.DataDir())); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a rejected save wrote an artifact, stat err = %v", err)
	}
}

func TestWorkspaceLensDefinitionsSurviveAsAnEmptySet(t *testing.T) {
	s := newIsolatedStore(t)
	if err := s.SaveLensDefinitionsChecked(&WorkspaceLensDefinitions{Lenses: NamedLensList{}}, ""); err != nil {
		t.Fatalf("SaveLensDefinitionsChecked: %v", err)
	}
	reloaded, rev, err := NewDomainFileStore(s.DataDir()).LoadLensDefinitionsChecked()
	if err != nil {
		t.Fatalf("LoadLensDefinitionsChecked: %v", err)
	}
	if rev == "" {
		t.Fatal("a published collection must report a revision")
	}
	if !reloaded.Defined() || len(reloaded.Lenses) != 0 {
		t.Fatalf("deleting every lens must stay durable, got defined=%v len=%d", reloaded.Defined(), len(reloaded.Lenses))
	}
	// The starters may not creep back in through the machine-local seeding path.
	machine, err := NewDomainFileStore(s.DataDir()).LoadMachine()
	if err != nil {
		t.Fatalf("LoadMachine: %v", err)
	}
	if len(machine.NamedLenses) != 0 {
		t.Fatalf("machine read reseeded %d lenses over an empty workspace set", len(machine.NamedLenses))
	}
}

func TestLoadMachinePrefersWorkspaceLensDefinitions(t *testing.T) {
	s := newIsolatedStore(t)
	machine := NewMachineState()
	machine.Theme = "light"
	machine.ActiveLensID = machine.NamedLenses[0].ID
	if err := s.SaveMachine(machine); err != nil {
		t.Fatalf("SaveMachine: %v", err)
	}
	if err := s.SaveLensDefinitionsChecked(&WorkspaceLensDefinitions{Lenses: NamedLensList{validLens()}}, ""); err != nil {
		t.Fatalf("SaveLensDefinitionsChecked: %v", err)
	}

	reloaded, err := NewDomainFileStore(s.DataDir()).LoadMachine()
	if err != nil {
		t.Fatalf("LoadMachine: %v", err)
	}
	if len(reloaded.NamedLenses) != 1 || reloaded.NamedLenses[0].ID != validLens().ID {
		t.Fatalf("workspace definitions did not win the read: %+v", reloaded.NamedLenses)
	}
	if reloaded.Theme != "light" {
		t.Errorf("theme = %q, want the machine-local value", reloaded.Theme)
	}
	if reloaded.ActiveLensID != machine.ActiveLensID {
		t.Errorf("active lens = %q, want the machine-local pointer %q", reloaded.ActiveLensID, machine.ActiveLensID)
	}
}

func TestLoadMachineFallsBackToMachineLensesWithoutAWorkspaceSet(t *testing.T) {
	s := newIsolatedStore(t)
	machine := NewMachineState()
	machine.NamedLenses = append(machine.NamedLenses, validLens())
	if err := s.SaveMachine(machine); err != nil {
		t.Fatalf("SaveMachine: %v", err)
	}
	reloaded, err := NewDomainFileStore(s.DataDir()).LoadMachine()
	if err != nil {
		t.Fatalf("LoadMachine: %v", err)
	}
	if len(reloaded.NamedLenses) != len(machine.NamedLenses) {
		t.Fatalf("lens count = %d, want the machine-local %d", len(reloaded.NamedLenses), len(machine.NamedLenses))
	}
	if _, err := os.Stat(LensDefinitionsPath(s.DataDir())); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reading the fallback minted a workspace artifact, stat err = %v", err)
	}
}

func TestCommitLensDefinitionsOpensNoTransactionAndNoUndoEntry(t *testing.T) {
	s := newIsolatedStore(t)
	dataDir := s.DataDir()
	coordinator := NewDomainCoordinator(dataDir)
	if err := coordinator.CommitLensDefinitions(&WorkspaceLensDefinitions{Lenses: NamedLensList{validLens()}}); err != nil {
		t.Fatalf("CommitLensDefinitions: %v", err)
	}
	if _, err := os.Stat(LensDefinitionsPath(dataDir)); err != nil {
		t.Fatalf("lens definitions not written: %v", err)
	}
	for _, dir := range []string{TransactionsDir(dataDir), UndoLedgerDir(dataDir)} {
		if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("a lens commit touched %s, stat err = %v", filepath.Base(dir), err)
		}
	}
	if _, err := os.Stat(PlanningStatePath(dataDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a lens commit wrote the planning domain, stat err = %v", err)
	}
}

func TestCommitLensDefinitionsIsRevisionChecked(t *testing.T) {
	s := newIsolatedStore(t)
	coordinator := NewDomainCoordinator(s.DataDir())
	if err := coordinator.CommitLensDefinitions(&WorkspaceLensDefinitions{Lenses: NamedLensList{}}); err != nil {
		t.Fatalf("seed publication: %v", err)
	}
	_, rev, err := s.LoadLensDefinitionsChecked()
	if err != nil {
		t.Fatalf("LoadLensDefinitionsChecked: %v", err)
	}
	if err := s.SaveLensDefinitionsChecked(&WorkspaceLensDefinitions{Lenses: NamedLensList{validLens()}}, rev); err != nil {
		t.Fatalf("checked save against the observed revision: %v", err)
	}
	stale := s.SaveLensDefinitionsChecked(&WorkspaceLensDefinitions{Lenses: NamedLensList{}}, rev)
	if !errors.Is(stale, ErrStaleObject) {
		t.Fatalf("stale save error = %v, want ErrStaleObject", stale)
	}
}

func TestCommitLensDefinitionsRequiresAnExistingCatalog(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := t.TempDir()
	coordinator := NewDomainCoordinator(dataDir)
	err := coordinator.CommitLensDefinitions(&WorkspaceLensDefinitions{Lenses: NamedLensList{}})
	if !errors.Is(err, ErrWorkspaceIDRequired) {
		t.Fatalf("CommitLensDefinitions without a catalog error = %v, want ErrWorkspaceIDRequired", err)
	}
	if _, err := os.Stat(LensDefinitionsPath(dataDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused commit wrote an artifact, stat err = %v", err)
	}
	if err := coordinator.CommitLensDefinitions(nil); err == nil {
		t.Fatal("nil definitions must be refused")
	}
}

func TestCommitMachineLeavesTheWorkspaceLensHomeUnpublished(t *testing.T) {
	s := newIsolatedStore(t)
	dataDir := s.DataDir()
	machine := NewMachineState()
	machine.NamedLenses = append(machine.NamedLenses, validLens())
	if err := NewDomainCoordinator(dataDir).CommitMachine(machine); err != nil {
		t.Fatalf("CommitMachine: %v", err)
	}
	if _, err := os.Stat(LensDefinitionsPath(dataDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a machine commit minted the workspace lens home, stat err = %v", err)
	}
	reloaded, err := NewDomainFileStore(dataDir).LoadMachine()
	if err != nil {
		t.Fatalf("LoadMachine: %v", err)
	}
	if len(reloaded.NamedLenses) != len(machine.NamedLenses) {
		t.Fatalf("lens count = %d, want the machine-local %d", len(reloaded.NamedLenses), len(machine.NamedLenses))
	}
}

func TestCommitMachineFollowsThePublishedLensHome(t *testing.T) {
	s := newIsolatedStore(t)
	dataDir := s.DataDir()
	coordinator := NewDomainCoordinator(dataDir)
	if err := coordinator.CommitLensDefinitions(&WorkspaceLensDefinitions{Lenses: NamedLensList{}}); err != nil {
		t.Fatalf("publish the workspace home: %v", err)
	}

	machine := NewMachineState()
	machine.NamedLenses = NamedLensList{validLens()}
	if err := coordinator.CommitMachine(machine); err != nil {
		t.Fatalf("CommitMachine: %v", err)
	}

	published, _, err := s.LoadLensDefinitionsChecked()
	if err != nil {
		t.Fatalf("LoadLensDefinitionsChecked: %v", err)
	}
	if len(published.Lenses) != 1 || published.Lenses[0].ID != validLens().ID {
		t.Fatalf("the lens half of the machine commit did not reach the workspace home: %+v", published.Lenses)
	}
	// The machine copy stays current beside it, so a reader that has not adopted
	// the portable home still sees the same set.
	raw, err := os.ReadFile(mustMachineStatePath(t, dataDir))
	if err != nil {
		t.Fatalf("read machine.json: %v", err)
	}
	if !strings.Contains(string(raw), validLens().ID) {
		t.Fatalf("machine.json lost its fallback copy: %s", raw)
	}
}

// mustMachineStatePath resolves the machine-local file for the workspace rooted
// at dataDir, which is keyed by the catalog's workspace identity.
func mustMachineStatePath(t *testing.T, dataDir string) string {
	t.Helper()
	catalog, _, err := NewDomainFileStore(dataDir).LoadCatalogChecked()
	if err != nil {
		t.Fatalf("LoadCatalogChecked: %v", err)
	}
	path, err := MachineStatePath(catalog.WorkspaceID)
	if err != nil {
		t.Fatalf("MachineStatePath: %v", err)
	}
	return path
}
