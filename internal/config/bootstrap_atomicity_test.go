package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
)

func TestCreateDomainBundle_ValidatesBeforeBootstrapEffects(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "absent", "workspace")
	domains := NewDomains()
	domains.Machine.Theme = "sepia"

	err := createDomainBundle(
		storage.NewCommitService(TransactionsDir(dataDir), []string{dataDir}),
		NewDomainFileStore(dataDir),
		domains,
	)
	if err == nil || !strings.Contains(err.Error(), "invalid machine domain") {
		t.Fatalf("createDomainBundle error = %v, want machine validation failure", err)
	}
	if _, statErr := os.Stat(dataDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("invalid bundle created workspace effects, stat error = %v", statErr)
	}
}

func TestManagerLegacyOnlyStateRequiresMigrationBeforeEffects(t *testing.T) {
	loaders := []struct {
		name string
		load func(*Manager) (*Domains, error)
	}{
		{name: "strict", load: (*Manager).LoadDomains},
		{name: "create-on-load", load: (*Manager).LoadOrCreateDomains},
	}
	for _, loader := range loaders {
		t.Run(loader.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			dataDir := filepath.Join(home, "legacy-workspace")
			if err := os.MkdirAll(dataDir, DirPermissions); err != nil {
				t.Fatalf("mkdir legacy workspace: %v", err)
			}
			legacyPath := ConfigPathForDataDir(dataDir)
			if err := os.WriteFile(legacyPath, []byte(`{"version": 1}`), FilePermissions); err != nil {
				t.Fatalf("write legacy config: %v", err)
			}

			domains, err := loader.load(NewManager(legacyPath))
			if domains != nil {
				t.Fatal("legacy-only load returned domains")
			}
			if !errors.Is(err, ErrLegacyConfigRequiresMigration) {
				t.Fatalf("legacy-only error = %v, want ErrLegacyConfigRequiresMigration", err)
			}
			var migrationErr *LegacyConfigRequiresMigrationError
			if !errors.As(err, &migrationErr) {
				t.Fatalf("legacy-only error type = %T, want *LegacyConfigRequiresMigrationError", err)
			}
			if migrationErr.Path != legacyPath {
				t.Fatalf("migration path = %q, want %q", migrationErr.Path, legacyPath)
			}

			entries, readErr := os.ReadDir(dataDir)
			if readErr != nil {
				t.Fatalf("read legacy workspace: %v", readErr)
			}
			if len(entries) != 1 || entries[0].Name() != configFileName {
				t.Fatalf("legacy-only load created workspace effects: %v", entries)
			}
			for _, path := range []string{
				InternalDir(dataDir),
				filepath.Join(LocalDataRoot(), runtimeDirName),
			} {
				if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("legacy-only load created %q, stat error = %v", path, statErr)
				}
			}
		})
	}
}

// TestManagerDataDirThatIsAFileIsNamed refuses a workspace root that is a
// regular file by naming it, rather than failing on the first path beneath it
// with ENOTDIR.
func TestManagerDataDirThatIsAFileIsNamed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	file := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(file, []byte("x"), FilePermissions); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := NewManager(ConfigPathForDataDir(file)).LoadOrCreateDomains()
	want := "workspace data directory " + file + " is not a directory"
	if err == nil || err.Error() != want {
		t.Fatalf("LoadOrCreateDomains error = %v, want %q", err, want)
	}
}

func TestManagerCompleteSplitStateWinsOverStaleLegacyConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dataDir := filepath.Join(home, "split-workspace")
	manager := NewManager(ConfigPathForDataDir(dataDir))
	want, err := manager.LoadOrCreateDomains()
	if err != nil {
		t.Fatalf("bootstrap split workspace: %v", err)
	}
	if err := os.WriteFile(manager.GetConfigPath(), []byte("not valid legacy JSON"), FilePermissions); err != nil {
		t.Fatalf("write stale legacy config: %v", err)
	}

	for _, loader := range []struct {
		name string
		load func() (*Domains, error)
	}{
		{name: "strict", load: manager.LoadDomains},
		{name: "create-on-load", load: manager.LoadOrCreateDomains},
	} {
		t.Run(loader.name, func(t *testing.T) {
			got, loadErr := loader.load()
			if loadErr != nil {
				t.Fatalf("complete split load with stale legacy config: %v", loadErr)
			}
			if got.Catalog.WorkspaceID != want.Catalog.WorkspaceID {
				t.Fatalf("workspace ID = %q, want %q", got.Catalog.WorkspaceID, want.Catalog.WorkspaceID)
			}
		})
	}
}

func TestManagerLoadOrCreateDomains_EmptyWorkspaceStillBootstraps(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "new-workspace")
	manager := NewManager(ConfigPathForDataDir(dataDir))
	domains, err := manager.LoadOrCreateDomains()
	if err != nil {
		t.Fatalf("bootstrap empty workspace: %v", err)
	}
	if domains == nil || domains.Catalog == nil {
		t.Fatal("bootstrap empty workspace returned incomplete domains")
	}
	inboxID := domains.Catalog.SystemUnfiledProjectID
	var inbox *CatalogProject
	for i := range domains.Catalog.Projects {
		if domains.Catalog.Projects[i].ID == inboxID {
			inbox = &domains.Catalog.Projects[i]
			break
		}
	}
	if inboxID == "" || inbox == nil || inbox.Name != SystemUnfiledName || inbox.Slug != SystemUnfiledSlug {
		t.Fatalf("fresh protected Inbox = %+v, id = %q", inbox, inboxID)
	}
	taskPath := storage.TasksFilePath(inbox.CatalogPath(dataDir))
	taskBytes, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read protected Inbox tasks: %v", err)
	}
	document, err := storage.ParseProjectDocument(string(taskBytes))
	if err != nil {
		t.Fatalf("parse protected Inbox tasks: %v", err)
	}
	if document.SchemaVersion != storage.CurrentTaskSchemaVersion {
		t.Fatalf("fresh protected Inbox schema = %d, want %d", document.SchemaVersion, storage.CurrentTaskSchemaVersion)
	}
	if len(document.Tasks) != 0 {
		t.Fatalf("fresh protected Inbox has %d tasks", len(document.Tasks))
	}
	assertCompleteSyncedBootstrap(t, dataDir, domains.Catalog.WorkspaceID)

	reloaded, err := manager.LoadOrCreateDomains()
	if err != nil {
		t.Fatalf("repeat bootstrap load: %v", err)
	}
	if reloaded.Catalog.SystemUnfiledProjectID != inboxID {
		t.Fatalf("repeat bootstrap changed protected Inbox from %q to %q", inboxID, reloaded.Catalog.SystemUnfiledProjectID)
	}
	after, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read protected Inbox tasks after repeat load: %v", err)
	}
	if string(after) != string(taskBytes) {
		t.Fatal("repeat bootstrap rewrote protected Inbox task document")
	}
}

func TestManagerLoadOrCreateDomains_RecoversInterruptedBootstrapBeforeClassification(t *testing.T) {
	const workspaceID = "44444444-4444-4444-8444-444444444444"
	for prefix := 0; prefix <= len(syncedDomainKinds); prefix++ {
		name := "prepared"
		if prefix > 0 {
			name = fmt.Sprintf("after-op-%d", prefix)
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			dataDir := filepath.Join(t.TempDir(), "workspace")
			journal := transaction.NewJournal(TransactionsDir(dataDir), []string{dataDir}, "")
			manifest, err := journal.Prepare(defaultBootstrapOps(t, dataDir, workspaceID))
			if err != nil {
				t.Fatalf("prepare bootstrap journal: %v", err)
			}
			if prefix > 0 {
				manifest.State = transaction.StateCommitting
				writeBootstrapManifest(t, journal, manifest)
				applyBootstrapPrefix(t, manifest, prefix)
			}

			domains, err := NewManager(ConfigPathForDataDir(dataDir)).LoadOrCreateDomains()
			if err != nil {
				t.Fatalf("LoadOrCreateDomains after interrupted bootstrap: %v", err)
			}
			if domains.Catalog.WorkspaceID != workspaceID {
				t.Fatalf("recovered workspace ID = %q, want %q", domains.Catalog.WorkspaceID, workspaceID)
			}
			if domains.Machine == nil {
				t.Fatal("recovered domains omitted machine state")
			}
			assertCompleteSyncedBootstrap(t, dataDir, workspaceID)
			assertNoBootstrapManifests(t, TransactionsDir(dataDir))
		})
	}
}

func TestManagerLoadOrCreateDomains_ConcurrentInitializersConverge(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")
	const initializerCount = 16
	type result struct {
		domains *Domains
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, initializerCount)
	var ready sync.WaitGroup
	ready.Add(initializerCount)
	for range initializerCount {
		go func() {
			ready.Done()
			<-start
			domains, err := NewManager(ConfigPathForDataDir(dataDir)).LoadOrCreateDomains()
			results <- result{domains: domains, err: err}
		}()
	}
	ready.Wait()
	close(start)

	// Drain every initializer before judging any, so a failure cannot leave
	// writers running into the TempDir cleanup.
	collected := make([]result, 0, initializerCount)
	for range initializerCount {
		collected = append(collected, <-results)
	}
	workspaceID := ""
	for _, got := range collected {
		if got.err != nil {
			t.Fatalf("concurrent LoadOrCreateDomains: %v", got.err)
		}
		if got.domains == nil || got.domains.Catalog == nil {
			t.Fatal("concurrent initializer returned an incomplete domain bundle")
		}
		if workspaceID == "" {
			workspaceID = got.domains.Catalog.WorkspaceID
		}
		if got.domains.Catalog.WorkspaceID != workspaceID {
			t.Fatalf("concurrent workspace ID = %q, want winner %q", got.domains.Catalog.WorkspaceID, workspaceID)
		}
	}
	if err := validateWorkspaceID(workspaceID); err != nil {
		t.Fatalf("winning workspace ID %q is invalid: %v", workspaceID, err)
	}
	assertCompleteSyncedBootstrap(t, dataDir, workspaceID)
}

func TestManagerLoadOrCreateDomains_MachineFailureLeavesCompleteSyncedWorkspace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dataDir := filepath.Join(t.TempDir(), "workspace")
	manager := NewManager(ConfigPathForDataDir(dataDir))
	if err := os.MkdirAll(LocalDataRoot(), DirPermissions); err != nil {
		t.Fatalf("mkdir local data root: %v", err)
	}
	runtimeBlocker := filepath.Join(LocalDataRoot(), runtimeDirName)
	if err := os.WriteFile(runtimeBlocker, []byte("block machine directory"), FilePermissions); err != nil {
		t.Fatalf("write machine-state blocker: %v", err)
	}

	_, err := manager.LoadOrCreateDomains()
	if err == nil || !strings.Contains(err.Error(), "machine-local configuration") {
		t.Fatalf("LoadOrCreateDomains error = %v, want wrapped machine-state failure", err)
	}
	store := manager.domainStore()
	state, partialErr := manager.classifyDomainState(store)
	if state != domainStateComplete || partialErr != nil {
		t.Fatalf("synced state after machine failure = %v partial=%v, want complete", state, partialErr)
	}
	catalog, err := store.LoadCatalog()
	if err != nil {
		t.Fatalf("load committed catalog: %v", err)
	}
	assertCompleteSyncedBootstrap(t, dataDir, catalog.WorkspaceID)

	// Remove the transient local obstruction while leaving machine.json absent.
	// A later load must accept the committed synced workspace and return defaults.
	if removeErr := os.Remove(runtimeBlocker); removeErr != nil {
		t.Fatalf("remove machine-state blocker: %v", removeErr)
	}
	reloaded, err := manager.LoadOrCreateDomains()
	if err != nil {
		t.Fatalf("reload after machine-state failure: %v", err)
	}
	if reloaded.Catalog.WorkspaceID != catalog.WorkspaceID {
		t.Fatalf("reloaded workspace ID = %q, want %q", reloaded.Catalog.WorkspaceID, catalog.WorkspaceID)
	}
	if reloaded.Machine == nil {
		t.Fatal("reloaded domains omitted default machine state")
	}
	// The reload does write machine.json, but only through lens adoption: a
	// workspace at schema 16 whose machine has no adoption marker merges its
	// lens set into the portable home and records the marker, which is
	// machine-local state and therefore machine.json. Nothing else about the
	// failed bootstrap is repaired implicitly.
	machinePath, err := MachineStatePath(catalog.WorkspaceID)
	if err != nil {
		t.Fatalf("MachineStatePath: %v", err)
	}
	if _, statErr := os.Stat(machinePath); statErr != nil {
		t.Fatalf("lens adoption should have recorded its marker in machine.json: %v", statErr)
	}
	if !reloaded.Machine.LensHomeAdopted {
		t.Fatal("reloaded machine state should carry the lens adoption marker")
	}
}

func defaultBootstrapOps(t *testing.T, dataDir, workspaceID string) []transaction.Op {
	t.Helper()
	domains := NewDomains()
	domains.Catalog.WorkspaceID = workspaceID
	specs := []struct {
		kind    domainKind
		path    string
		payload any
	}{
		{kind: domainCatalog, path: WorkspaceCatalogPath(dataDir), payload: domains.Catalog},
		{kind: domainPlanning, path: PlanningStatePath(dataDir), payload: domains.Planning},
	}
	ops := make([]transaction.Op, 0, len(specs))
	for _, spec := range specs {
		data, err := encodeDomainEnvelope(spec.kind, workspaceID, spec.payload)
		if err != nil {
			t.Fatalf("encode %s bootstrap envelope: %v", spec.kind.name(), err)
		}
		ops = append(ops, transaction.Op{Kind: transaction.OpWrite, Target: spec.path, Data: data, Mode: FilePermissions})
	}
	return ops
}

func writeBootstrapManifest(t *testing.T, journal *transaction.Journal, manifest *transaction.Manifest) {
	t.Helper()
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal bootstrap manifest: %v", err)
	}
	if writeErr := os.WriteFile(journal.ManifestPath(manifest.ID), data, FilePermissions); writeErr != nil {
		t.Fatalf("write bootstrap manifest: %v", writeErr)
	}
}

func applyBootstrapPrefix(t *testing.T, manifest *transaction.Manifest, prefix int) {
	t.Helper()
	for i := 0; i < prefix; i++ {
		op := manifest.Ops[i]
		if op.Kind != transaction.OpWrite {
			t.Fatalf("bootstrap op %d kind = %q, want write", i, op.Kind)
		}
		if err := os.Rename(op.Staged, op.Target); err != nil {
			t.Fatalf("apply bootstrap op %d: %v", i, err)
		}
	}
}

func assertCompleteSyncedBootstrap(t *testing.T, dataDir, workspaceID string) {
	t.Helper()
	paths := []string{
		WorkspaceCatalogPath(dataDir),
		PlanningStatePath(dataDir),
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read synced bootstrap artifact %s: %v", path, err)
		}
		var env domainEnvelope
		if decodeErr := json.Unmarshal(data, &env); decodeErr != nil {
			t.Fatalf("decode synced bootstrap artifact %s: %v", path, decodeErr)
		}
		if env.WorkspaceID != workspaceID {
			t.Fatalf("%s workspace ID = %q, want %q", filepath.Base(path), env.WorkspaceID, workspaceID)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat synced bootstrap artifact %s: %v", path, err)
		}
		if got := info.Mode().Perm(); got != os.FileMode(FilePermissions) {
			t.Fatalf("%s permissions = %o, want %o", filepath.Base(path), got, FilePermissions)
		}
	}
}

func assertNoBootstrapManifests(t *testing.T, transactionsDir string) {
	t.Helper()
	entries, err := os.ReadDir(transactionsDir)
	if err != nil {
		t.Fatalf("read transactions directory: %v", err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			t.Fatalf("bootstrap manifest %s remained after recovery", entry.Name())
		}
	}
}
