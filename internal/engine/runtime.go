package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/syncmerge"
)

// Runtime is one open workspace: the startup-loaded configuration domains, the
// manager rooted at the same workspace, the shared undo ledger, and the
// application service every mutation commits through. It owns the durable side
// and nothing a frontend draws; a frontend keeps its own presentation state and
// reads and writes the workspace through this.
type Runtime struct {
	manager *config.Manager
	state   *config.RuntimeState
	ledger  *storage.WorkspaceLedger
	service *app.Service
	repo    *Repository
	clock   app.Clock
}

// Open opens the workspace Config names, as a desktop launch or a headless
// caller does: it completes interrupted transactions and pending schema
// activations, bootstraps a fresh workspace when CreateIfMissing allows it,
// loads the configuration domains, and composes the application service. A
// frontend reconciles planning once at startup through ReconcilePlanning, after
// any presentation state of its own exists.
func Open(cfg Config) (*Runtime, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	dataDir := filepath.Clean(cfg.DataDir)
	if info, err := os.Stat(dataDir); err == nil && !info.IsDir() {
		return nil, fmt.Errorf("data directory %s is not a directory", dataDir)
	}
	manager := newManager(dataDir, cfg.Machine)
	exists, err := workspaceExists(manager)
	if err != nil {
		return nil, err
	}
	if !exists && !cfg.CreateIfMissing {
		return nil, fmt.Errorf("%w: %s", ErrWorkspaceNotFound, dataDir)
	}
	if exists && cfg.RefuseMigration {
		if schemaErr := requireCurrentSchema(dataDir); schemaErr != nil {
			return nil, schemaErr
		}
	}
	// Loading the domains is what recovers: the manager completes this
	// machine's interrupted transactions and any pending activation before a
	// domain is read, so nothing below observes a half-applied commit.
	if _, err = manager.LoadOrCreateDomains(); err != nil {
		return nil, fmt.Errorf("failed to load workspace: %w", err)
	}
	state, err := manager.LoadRuntimeState()
	if err != nil {
		return nil, fmt.Errorf("failed to load runtime state: %w", err)
	}
	// A second recovery pass under the loaded snapshot is the one a frontend
	// has always taken before consuming domains: it is a no-op unless a
	// transaction landed between the two reads, and then the snapshot reloads.
	if state, err = Recover(state, manager); err != nil {
		return nil, fmt.Errorf("recover workspace: %w", err)
	}
	return Attach(state, manager, cfg.Clock, cfg.IDs), nil
}

// requireCurrentSchema refuses a workspace this engine would have to migrate
// or cannot read, judged from the catalog alone so nothing is recovered first.
// A workspace with no catalog yet predates the split domains and is older.
func requireCurrentSchema(dataDir string) error {
	schema, exists, err := config.WorkspaceCatalogSchema(dataDir)
	if err != nil {
		return err
	}
	if !exists || schema < config.CurrentConfigVersion {
		return fmt.Errorf("%w: %s holds schema %d, this engine writes %d", ErrMigrationRequired, dataDir, schema, config.CurrentConfigVersion)
	}
	return nil
}

// Attach composes a runtime over a domain snapshot and manager already loaded
// and recovered. Open ends here, and so does a test that built its own
// snapshot, so both run through one composition. A nil state is an empty,
// unrooted workspace and a nil manager a workspace that cannot commit domains;
// both are what isolated frontend tests model. A nil clock or identity
// generator is the system clock or random UUIDs.
func Attach(state *config.RuntimeState, manager *config.Manager, clock app.Clock, ids app.IDGenerator) *Runtime {
	if state == nil {
		state = config.EmptyRuntimeState("")
	}
	if clock == nil {
		clock = app.SystemClock()
	}
	dataDir := state.DataDir()
	var deps *config.RuntimeDependencies
	owner := ""
	if manager != nil {
		deps = manager.RuntimeDependencies()
		owner = manager.TransactionOwner()
	}
	ledger := storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), []string{dataDir})
	// Today/Next exclusivity folds into every workflow-state transition this
	// service commits, whichever frontend raised it.
	repo := NewRepository(dataDir, deps, ledger, owner)
	repo.svc.WithSyncHistory(syncHistory(dataDir, owner, clock)).WithSyncMerger(syncmerge.Merger{})
	service := NewService(repo, clock, ids).WithPlanningArtifacts(PlanningArtifacts(manager))
	return &Runtime{manager: manager, state: state, ledger: ledger, service: service, repo: repo, clock: clock}
}

// Recover completes the machine's interrupted transactions under a loaded
// snapshot's workspace and returns the snapshot to read afterwards: the same
// one when there was nothing to finish, or a fresh load when recovery moved
// state underneath it. It reads the manager's machine when one is given and
// the process machine otherwise.
func Recover(state *config.RuntimeState, manager *config.Manager) (*config.RuntimeState, error) {
	if state == nil {
		return config.EmptyRuntimeState(""), nil
	}
	dataDir := state.DataDir()
	if dataDir == "" {
		return state, nil
	}
	localRoot, owner := config.LocalDataRoot(), ""
	if manager != nil {
		localRoot, owner = manager.LocalDataRoot(), manager.TransactionOwner()
	}
	report, err := storage.NewCommitService(config.TransactionsDir(dataDir), []string{dataDir, localRoot}).
		OnMachine(owner).
		RecoverWorkspace()
	if err != nil {
		return nil, err
	}
	if len(report.RolledForward) == 0 && len(report.CleanedUp) == 0 {
		return state, nil
	}
	var recovered *config.RuntimeState
	if manager != nil {
		recovered, err = config.LoadRuntimeStateOnMachine(dataDir, manager.MachineHome())
	} else {
		recovered, err = config.LoadRuntimeState(dataDir)
	}
	if err != nil {
		return nil, fmt.Errorf("reload recovered workspace domains: %w", err)
	}
	return recovered, nil
}

// NewService builds the application service over a storage-backed repository,
// without a default planning builder. Attach adds one; a caller composing a
// narrower service picks its own.
func NewService(repo *Repository, clock app.Clock, ids app.IDGenerator) *app.Service {
	return app.NewServiceWithArtifacts(repo, clock, ids, app.NewStorageWorkspaceNotePort(repo.dataDir))
}

func newManager(dataDir string, machine Machine) *config.Manager {
	configPath := config.ConfigPathForDataDir(dataDir)
	if machine.followProcess {
		return config.NewManager(configPath)
	}
	return config.NewManagerForDevice(configPath, machine.Home, machine.DeviceID)
}

// workspaceExists reports whether the data directory holds a workspace: its
// internal .tt directory, which every schema since v4 keeps, or the legacy
// aggregate config an older workspace still needs migrating from.
func workspaceExists(manager *config.Manager) (bool, error) {
	configPath := manager.GetConfigPath()
	dataDir := config.WorkspaceRootFromConfigPath(configPath)
	for _, marker := range []string{config.InternalDir(dataDir), configPath} {
		_, err := os.Stat(marker)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf("inspect workspace at %s: %w", dataDir, err)
		}
	}
	return false, nil
}

// Manager is the configuration manager rooted at this workspace.
func (r *Runtime) Manager() *config.Manager { return r.manager }

// State is the loaded configuration-domain snapshot. Commits through Service
// do not refresh it; Refresh does.
func (r *Runtime) State() *config.RuntimeState { return r.state }

// Ledger is the shared workspace undo ledger every commit records into.
func (r *Runtime) Ledger() *storage.WorkspaceLedger { return r.ledger }

// Service is the application service every task, planning, note, and project
// mutation commits through.
func (r *Runtime) Service() *app.Service { return r.service }

// Snapshot reads the revision-consistent model a mutation would start from:
// every available project's tasks and the canonical revision each was loaded
// at, which is what a frontend passes back as its next precondition.
func (r *Runtime) Snapshot() (app.Snapshot, error) { return r.repo.Snapshot() }

// DataDir is the workspace root.
func (r *Runtime) DataDir() string { return r.state.DataDir() }

// Now reads the runtime's clock.
func (r *Runtime) Now() time.Time { return r.clock.Now() }

// Refresh re-reads the configuration domains from disk, so the snapshot sees
// what a commit, another process, or another machine wrote.
func (r *Runtime) Refresh() error { return r.state.Reload() }

// ReconcilePlanning drops Today and Next references the workspace no longer
// supports and refreshes a rooted snapshot when it changed anything. A
// frontend calls it once at startup.
func (r *Runtime) ReconcilePlanning() (bool, error) {
	changed, err := r.service.ReconcilePlanning()
	if err != nil || !changed || r.state.DataDir() == "" {
		return changed, err
	}
	return true, r.state.RefreshAfterCommit()
}
