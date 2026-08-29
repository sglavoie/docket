package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Adoption fixtures. Machine identity is exactly the home directory: machine.json
// lives at <home>/.tt/runtime/<workspace-id>/machine.json, so two homes over one
// data directory are two machines sharing one workspace.
const (
	adoptionLensAlphaID  = "11111111-1111-4111-8111-111111111111"
	adoptionLensBetaID   = "22222222-2222-4222-8222-222222222222"
	adoptionLensGammaID  = "33333333-3333-4333-8333-333333333333"
	adoptionLensSharedID = "44444444-4444-4444-8444-444444444444"
)

func adoptionLens(id, name, query string) NamedLens {
	return NamedLens{
		ID:    id,
		Name:  name,
		Mode:  LensModeAll,
		Query: query,
		Sort:  LensSort{Field: LensSortDefault, Ascending: true},
	}
}

// useMachine repoints LocalDataRoot at one simulated machine. t.Setenv restores
// the previous value in LIFO order, so switching between machines inside one
// test is safe and the environment is whole again when the test ends.
func useMachine(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
}

// stageWorkspaceBeforeAdoption mints a workspace and then rewinds it to the
// state a real one is in when this feature ships: the catalog is at schema 16,
// but no machine has published the portable home yet. Bootstrap adopts on the
// way in, so the file it published has to be removed again.
func stageWorkspaceBeforeAdoption(t *testing.T) (string, *Manager) {
	t.Helper()
	dataDir := filepath.Join(t.TempDir(), "workspace")
	manager := NewManager(ConfigPathForDataDir(dataDir))
	if _, err := manager.LoadOrCreateDomains(); err != nil {
		t.Fatalf("bootstrap workspace: %v", err)
	}
	if err := os.Remove(LensDefinitionsPath(dataDir)); err != nil {
		t.Fatalf("rewind the portable lens home: %v", err)
	}
	return dataDir, manager
}

// stageUnadoptedMachine writes the machine-local state a machine carried before
// the portable home existed: a lens set of its own, its own theme and active
// pointer, and no adoption marker.
func stageUnadoptedMachine(t *testing.T, manager *Manager, lenses NamedLensList, theme, activeID string) {
	t.Helper()
	store := manager.domainStore()
	machine, rev, err := store.LoadMachineChecked()
	if err != nil {
		t.Fatalf("load machine state: %v", err)
	}
	machine.NamedLenses = lenses
	machine.Theme = theme
	machine.ActiveLensID = activeID
	machine.LensHomeAdopted = false
	if err := store.SaveMachineChecked(machine, rev); err != nil {
		t.Fatalf("stage unadopted machine: %v", err)
	}
}

// startupAdopt drives the real startup path rather than calling adoption
// directly, so every assertion below also covers the wiring in RecoverWorkspace.
func startupAdopt(t *testing.T, manager *Manager) *Domains {
	t.Helper()
	domains, err := manager.LoadOrCreateDomains()
	if err != nil {
		t.Fatalf("startup: %v", err)
	}
	return domains
}

func workspaceLenses(t *testing.T, manager *Manager) NamedLensList {
	t.Helper()
	defs, _, err := manager.domainStore().LoadLensDefinitionsChecked()
	if err != nil {
		t.Fatalf("load portable lens definitions: %v", err)
	}
	if !defs.Defined() {
		t.Fatal("the portable lens home should be defined after adoption")
	}
	return defs.Lenses
}

func lensNames(list NamedLensList) []string {
	names := make([]string, 0, len(list))
	for _, lens := range list {
		names = append(names, lens.Name)
	}
	return names
}

func requireLensNames(t *testing.T, list NamedLensList, want ...string) {
	t.Helper()
	got := lensNames(list)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("lens names = %v, want %v", got, want)
	}
}

func TestLensAdoptionKeepsBothMachinesDistinctSets(t *testing.T) {
	homeA, homeB := t.TempDir(), t.TempDir()
	useMachine(t, homeA)
	dataDir, manager := stageWorkspaceBeforeAdoption(t)
	stageUnadoptedMachine(t, manager, NamedLensList{
		adoptionLens(adoptionLensAlphaID, "Alpha", "#alpha"),
		adoptionLens(adoptionLensBetaID, "Beta", "#beta"),
	}, "dark", adoptionLensAlphaID)

	useMachine(t, homeB)
	machineB := NewManager(ConfigPathForDataDir(dataDir))
	stageUnadoptedMachine(t, machineB, NamedLensList{
		adoptionLens(adoptionLensGammaID, "Gamma", "#gamma"),
	}, "light", adoptionLensGammaID)

	// The first machine founds the portable home with exactly its own set.
	useMachine(t, homeA)
	adoptedA := startupAdopt(t, manager)
	requireLensNames(t, workspaceLenses(t, manager), "Alpha", "Beta")
	if !adoptedA.Machine.LensHomeAdopted {
		t.Fatal("machine A should carry the adoption marker")
	}
	if adoptedA.Machine.Theme != "dark" || adoptedA.Machine.ActiveLensID != adoptionLensAlphaID {
		t.Fatalf("machine A theme/active = %q/%q, want dark/%s", adoptedA.Machine.Theme, adoptedA.Machine.ActiveLensID, adoptionLensAlphaID)
	}

	// The second machine merges rather than replaces: neither side loses a lens.
	useMachine(t, homeB)
	adoptedB := startupAdopt(t, machineB)
	requireLensNames(t, workspaceLenses(t, machineB), "Alpha", "Beta", "Gamma")
	requireLensNames(t, adoptedB.Machine.NamedLenses, "Alpha", "Beta", "Gamma")
	if adoptedB.Machine.Theme != "light" || adoptedB.Machine.ActiveLensID != adoptionLensGammaID {
		t.Fatalf("machine B theme/active = %q/%q, want light/%s", adoptedB.Machine.Theme, adoptedB.Machine.ActiveLensID, adoptionLensGammaID)
	}

	// The first machine now reads the merged set while keeping its own theme and
	// active pointer: those two halves never travel.
	useMachine(t, homeA)
	reopenedA := startupAdopt(t, manager)
	requireLensNames(t, reopenedA.Machine.NamedLenses, "Alpha", "Beta", "Gamma")
	if reopenedA.Machine.Theme != "dark" || reopenedA.Machine.ActiveLensID != adoptionLensAlphaID {
		t.Fatalf("machine A theme/active drifted to %q/%q", reopenedA.Machine.Theme, reopenedA.Machine.ActiveLensID)
	}
}

func TestLensAdoptionKeepsBothSidesOfAnIDCollision(t *testing.T) {
	homeA, homeB := t.TempDir(), t.TempDir()
	useMachine(t, homeA)
	dataDir, manager := stageWorkspaceBeforeAdoption(t)
	stageUnadoptedMachine(t, manager, NamedLensList{
		adoptionLens(adoptionLensSharedID, "Shared", "#from-a"),
	}, "dark", "")

	useMachine(t, homeB)
	machineB := NewManager(ConfigPathForDataDir(dataDir))
	stageUnadoptedMachine(t, machineB, NamedLensList{
		adoptionLens(adoptionLensSharedID, "Shared", "#from-b"),
	}, "dark", "")

	useMachine(t, homeA)
	startupAdopt(t, manager)
	useMachine(t, homeB)
	startupAdopt(t, machineB)

	merged := workspaceLenses(t, machineB)
	requireLensNames(t, merged, "Shared", "Shared (2)")
	if merged[0].Query != "#from-a" || merged[1].Query != "#from-b" {
		t.Fatalf("both definitions should survive, got %q and %q", merged[0].Query, merged[1].Query)
	}
	if merged[0].ID != adoptionLensSharedID {
		t.Fatalf("the founding record should keep its identity, got %q", merged[0].ID)
	}
	if merged[1].ID == adoptionLensSharedID {
		t.Fatal("the adopted record needs an identity of its own")
	}
	if err := merged.Validate(); err != nil {
		t.Fatalf("a merged collection must satisfy the collection rules: %v", err)
	}
}

func TestLensAdoptionIsIdempotent(t *testing.T) {
	homeA, homeB := t.TempDir(), t.TempDir()
	useMachine(t, homeA)
	dataDir, manager := stageWorkspaceBeforeAdoption(t)
	stageUnadoptedMachine(t, manager, NamedLensList{
		adoptionLens(adoptionLensSharedID, "Shared", "#from-a"),
		adoptionLens(adoptionLensAlphaID, "Alpha", "#alpha"),
	}, "dark", "")

	useMachine(t, homeB)
	machineB := NewManager(ConfigPathForDataDir(dataDir))
	stageUnadoptedMachine(t, machineB, NamedLensList{
		adoptionLens(adoptionLensSharedID, "Shared", "#from-b"),
	}, "dark", "")

	useMachine(t, homeA)
	startupAdopt(t, manager)
	useMachine(t, homeB)
	startupAdopt(t, machineB)
	settled := mustJSON(t, workspaceLenses(t, machineB))

	// Repeat launches on both machines, in both orders, add nothing.
	for i := 0; i < 2; i++ {
		useMachine(t, homeA)
		startupAdopt(t, manager)
		useMachine(t, homeB)
		startupAdopt(t, machineB)
	}
	if got := mustJSON(t, workspaceLenses(t, machineB)); got != settled {
		t.Fatalf("adoption is not idempotent:\n got %s\nwant %s", got, settled)
	}

	// Re-adopting a machine whose marker was lost re-merges to the same set,
	// which is the property the derived identity of a split record buys.
	useMachine(t, homeB)
	stageUnadoptedMachine(t, machineB, NamedLensList{
		adoptionLens(adoptionLensSharedID, "Shared", "#from-b"),
	}, "dark", "")
	startupAdopt(t, machineB)
	if got := mustJSON(t, workspaceLenses(t, machineB)); got != settled {
		t.Fatalf("re-adoption changed the collection:\n got %s\nwant %s", got, settled)
	}
}

func TestLensAdoptionAdmitsAThirdMachineJoiningLater(t *testing.T) {
	homeA, homeB, homeC := t.TempDir(), t.TempDir(), t.TempDir()
	useMachine(t, homeA)
	dataDir, manager := stageWorkspaceBeforeAdoption(t)
	stageUnadoptedMachine(t, manager, NamedLensList{
		adoptionLens(adoptionLensAlphaID, "Alpha", "#alpha"),
	}, "dark", "")

	useMachine(t, homeB)
	machineB := NewManager(ConfigPathForDataDir(dataDir))
	stageUnadoptedMachine(t, machineB, NamedLensList{
		adoptionLens(adoptionLensBetaID, "Beta", "#beta"),
	}, "dark", "")

	useMachine(t, homeA)
	startupAdopt(t, manager)
	useMachine(t, homeB)
	startupAdopt(t, machineB)

	// Machine C has never written machine.json for this workspace, so it has no
	// lens record of its own and contributes nothing.
	useMachine(t, homeC)
	machineC := NewManager(ConfigPathForDataDir(dataDir))
	adoptedC := startupAdopt(t, machineC)
	requireLensNames(t, workspaceLenses(t, machineC), "Alpha", "Beta")
	requireLensNames(t, adoptedC.Machine.NamedLenses, "Alpha", "Beta")
	if !adoptedC.Machine.LensHomeAdopted {
		t.Fatal("machine C should carry the adoption marker")
	}
	if adoptedC.Machine.ActiveLensID != "" {
		t.Fatalf("machine C should open with no lens, got %q", adoptedC.Machine.ActiveLensID)
	}
}

func TestLensAdoptionDoesNotReseedStartersIntoAnEmptiedWorkspace(t *testing.T) {
	homeA, homeC := t.TempDir(), t.TempDir()
	useMachine(t, homeA)
	dataDir, manager := stageWorkspaceBeforeAdoption(t)
	// An explicit empty list is the durable record of "every lens was deleted".
	stageUnadoptedMachine(t, manager, NamedLensList{}, "dark", "")
	startupAdopt(t, manager)
	if got := workspaceLenses(t, manager); len(got) != 0 {
		t.Fatalf("the founding machine deleted every lens, got %v", lensNames(got))
	}

	useMachine(t, homeC)
	machineC := NewManager(ConfigPathForDataDir(dataDir))
	adoptedC := startupAdopt(t, machineC)
	if got := workspaceLenses(t, machineC); len(got) != 0 {
		t.Fatalf("a fresh machine reseeded the starters: %v", lensNames(got))
	}
	if len(adoptedC.Machine.NamedLenses) != 0 {
		t.Fatalf("machine C should read the emptied workspace set, got %v", lensNames(adoptedC.Machine.NamedLenses))
	}
}

func TestMachineStateStillLoadsAPayloadWrittenBeforeTheAdoptionMarker(t *testing.T) {
	useMachine(t, t.TempDir())
	_, manager := stageWorkspaceBeforeAdoption(t)
	store := manager.domainStore()
	catalog, err := store.LoadCatalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	machinePath, err := MachineStatePath(catalog.WorkspaceID)
	if err != nil {
		t.Fatalf("MachineStatePath: %v", err)
	}
	legacy := adoptionLens(adoptionLensAlphaID, "Alpha", "#alpha")
	payload := fmt.Sprintf(
		`{"theme":"light","named_lenses":%s,"active_lens_id":%q}`,
		mustJSON(t, NamedLensList{legacy}), legacy.ID,
	)
	envelope := fmt.Sprintf(
		`{"schema_version":%d,"workspace_id":%q,"kind":"machine","data":%s}`,
		CurrentDomainSchemaVersion, catalog.WorkspaceID, payload,
	)
	if err = os.WriteFile(machinePath, []byte(envelope), FilePermissions); err != nil {
		t.Fatalf("write legacy machine payload: %v", err)
	}

	// The strict decoder rejects unknown fields, so a machine.json written before
	// the marker existed has to keep loading on its own absence.
	loaded, _, err := store.LoadMachineChecked()
	if err != nil {
		t.Fatalf("a machine payload without the marker must still load: %v", err)
	}
	if loaded.LensHomeAdopted {
		t.Fatal("an absent marker must read as unadopted")
	}
	requireLensNames(t, loaded.NamedLenses, "Alpha")

	adopted := startupAdopt(t, manager)
	requireLensNames(t, workspaceLenses(t, manager), "Alpha")
	if !adopted.Machine.LensHomeAdopted || adopted.Machine.Theme != "light" {
		t.Fatalf("adoption should have marked the machine and kept its theme, got %+v", adopted.Machine)
	}
}

func TestAdoptedMachinePublishesLaterLensWritesToThePortableHome(t *testing.T) {
	useMachine(t, t.TempDir())
	dataDir, manager := stageWorkspaceBeforeAdoption(t)
	stageUnadoptedMachine(t, manager, NamedLensList{
		adoptionLens(adoptionLensAlphaID, "Alpha", "#alpha"),
	}, "dark", "")
	adopted := startupAdopt(t, manager)

	// Once a machine has adopted, an ordinary lens write follows the definitions
	// to the workspace instead of staying machine-local.
	adopted.Machine.NamedLenses = append(adopted.Machine.NamedLenses, adoptionLens(adoptionLensBetaID, "Beta", "#beta"))
	if err := manager.CommitMachine(adopted.Machine); err != nil {
		t.Fatalf("commit a later lens write: %v", err)
	}
	requireLensNames(t, workspaceLenses(t, manager), "Alpha", "Beta")

	// A hand-deleted portable home is recreated by the next write rather than
	// silently reverting an adopted machine to machine-local definitions.
	if err := os.Remove(LensDefinitionsPath(dataDir)); err != nil {
		t.Fatalf("remove the portable lens home: %v", err)
	}
	if err := manager.CommitMachine(adopted.Machine); err != nil {
		t.Fatalf("commit after the portable home was removed: %v", err)
	}
	requireLensNames(t, workspaceLenses(t, manager), "Alpha", "Beta")
}

func TestMergeNamedLensesNeverDropsAOneSidedLens(t *testing.T) {
	base := NamedLensList{adoptionLens(adoptionLensAlphaID, "Alpha", "#alpha")}
	incoming := NamedLensList{adoptionLens(adoptionLensBetaID, "Beta", "#beta")}
	merged := MergeNamedLenses(base, incoming)
	requireLensNames(t, merged, "Alpha", "Beta")
	if got := MergeNamedLenses(incoming, base); strings.Join(lensNames(got), "|") != "Beta|Alpha" {
		t.Fatalf("base order should lead, got %v", lensNames(got))
	}
	if len(base) != 1 || len(incoming) != 1 {
		t.Fatal("merging must not mutate its inputs")
	}
}

func TestMergeNamedLensesDedupesAnIdenticalRecord(t *testing.T) {
	lens := adoptionLens(adoptionLensAlphaID, "Alpha", "#alpha")
	merged := MergeNamedLenses(NamedLensList{lens}, NamedLensList{lens})
	requireLensNames(t, merged, "Alpha")
	if merged[0].ID != adoptionLensAlphaID {
		t.Fatalf("identity changed on a plain dedupe: %q", merged[0].ID)
	}
}

func TestMergeNamedLensesRenamesADistinctLensThatSharesAName(t *testing.T) {
	base := NamedLensList{adoptionLens(adoptionLensAlphaID, "Alpha", "#alpha")}
	incoming := NamedLensList{adoptionLens(adoptionLensBetaID, "alpha", "#beta")}
	merged := MergeNamedLenses(base, incoming)
	requireLensNames(t, merged, "Alpha", "alpha (2)")
	if err := merged.Validate(); err != nil {
		t.Fatalf("name uniqueness is case-folded, so the merge must resolve it: %v", err)
	}
	if merged[1].ID != adoptionLensBetaID {
		t.Fatal("a name collision alone must not change an identity")
	}
}

func TestMergeNamedLensesBoundsADisambiguatedName(t *testing.T) {
	long := strings.Repeat("n", MaxNamedLensNameRunes)
	base := NamedLensList{adoptionLens(adoptionLensAlphaID, long, "#alpha")}
	incoming := NamedLensList{adoptionLens(adoptionLensBetaID, long, "#beta")}
	merged := MergeNamedLenses(base, incoming)
	if err := merged.Validate(); err != nil {
		t.Fatalf("a disambiguated name must stay within the durable bound: %v", err)
	}
	if merged[1].Name == long {
		t.Fatal("the second record should have been renamed")
	}
}

func TestMergeNamedLensesIsStableAcrossRepeatedMerges(t *testing.T) {
	base := NamedLensList{adoptionLens(adoptionLensSharedID, "Shared", "#from-a")}
	incoming := NamedLensList{adoptionLens(adoptionLensSharedID, "Shared", "#from-b")}
	once := MergeNamedLenses(base, incoming)
	twice := MergeNamedLenses(once, incoming)
	if mustJSON(t, once) != mustJSON(t, twice) {
		t.Fatalf("re-merging changed the collection:\n once %s\ntwice %s", mustJSON(t, once), mustJSON(t, twice))
	}
}
