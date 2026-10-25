package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sglavoie/docket/internal/config"
)

// NewFixtureDriver builds an isolated workspace preloaded with the named
// built-in fixture and returns a driver over it, the owning workspace (call
// Cleanup when done), and any error. On error the workspace is already cleaned
// up and a nil workspace is returned.
func NewFixtureDriver(name string, width, height int) (*UITestDriver, *uitestWorkspace, error) {
	build, ok := builtinFixtures[name]
	if !ok {
		return nil, nil, fmt.Errorf("unknown fixture %q (available: %s)", name, strings.Join(BuiltinFixtureNames(), ", "))
	}

	ws, err := newUITestWorkspace()
	if err != nil {
		return nil, nil, err
	}
	if build == nil {
		buildFreshFixture(ws)
	} else if err = build(ws); err != nil {
		_ = ws.Cleanup()
		return nil, nil, err
	}
	driver, err := ws.launch(width, height)
	if err != nil {
		_ = ws.Cleanup()
		return nil, nil, err
	}
	return driver, ws, nil
}

// NewCopiedWorkspaceDriver loads a real user config, copies its data dir into a
// fresh temp workspace, rewrites DataDir and project paths onto the copy, and
// launches a driver from the copy. The source config and task files are read
// only — never mutated. Returns the driver, the owning temp workspace (call
// Cleanup when done), and any error.
func NewCopiedWorkspaceDriver(sourceConfigPath string, width, height int) (*UITestDriver, *uitestWorkspace, error) {
	if strings.TrimSpace(sourceConfigPath) == "" {
		return nil, nil, fmt.Errorf("source config path is required")
	}
	// Persistence is the decomposed domain files under the workspace root, so the
	// source "config" is the whole workspace directory (the .tt/ config domain
	// files plus the project data), not a single monolithic file. Copy that durable
	// tree into a fresh temporary workspace first, then load from the copy.
	srcRoot := config.WorkspaceRootFromConfigPath(sourceConfigPath)

	root, err := os.MkdirTemp("", "tt-uitest-copy-*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create copied workspace temp dir: %w", err)
	}
	ws := &uitestWorkspace{Root: root, ConfigPath: filepath.Join(root, filepath.Base(sourceConfigPath))}

	if err = os.CopyFS(root, os.DirFS(srcRoot)); err != nil {
		_ = ws.Cleanup()
		return nil, nil, fmt.Errorf("failed to copy source workspace: %w", err)
	}
	if err = forkCopiedWorkspaceIdentity(root); err != nil {
		_ = ws.Cleanup()
		return nil, nil, fmt.Errorf("failed to isolate copied workspace identity: %w", err)
	}

	ws.Manager = config.NewManager(ws.ConfigPath)
	if _, err = ws.Manager.LoadDomains(); err != nil {
		_ = ws.Cleanup()
		return nil, nil, fmt.Errorf("failed to load copied config: %w", err)
	}
	runtime, err := ws.Manager.LoadRuntimeState()
	if err != nil {
		_ = ws.Cleanup()
		return nil, nil, fmt.Errorf("failed to load copied runtime domains: %w", err)
	}
	ws.DataDir = root
	ws.Runtime = runtime

	driver, err := ws.launch(width, height)
	if err != nil {
		_ = ws.Cleanup()
		return nil, nil, err
	}
	return driver, ws, nil
}

// forkCopiedWorkspaceIdentity rewrites every copied synced-domain envelope with
// one fresh workspace ID, then creates canonical default machine state for that
// new identity. The source store is read before any copied artifact is removed;
// failures affect only the disposable temp copy. Removing stale .bak sidecars is
// essential because restoring one would otherwise resurrect the source ID.
func forkCopiedWorkspaceIdentity(dataDir string) error {
	source := config.NewDomainFileStore(dataDir)
	catalog, err := source.LoadCatalog()
	if err != nil {
		return err
	}
	planning, err := source.LoadPlanning()
	if err != nil {
		return err
	}

	paths := []string{
		config.WorkspaceCatalogPath(dataDir),
		config.PlanningStatePath(dataDir),
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove copied domain %s: %w", filepath.Base(path), err)
		}
		if err := os.Remove(path + ".bak"); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove copied domain backup %s: %w", filepath.Base(path), err)
		}
	}

	workspaceID := config.NewWorkspaceID()
	catalog.WorkspaceID = workspaceID
	destination := config.NewDomainFileStore(dataDir)
	if err := destination.BindWorkspaceID(workspaceID); err != nil {
		return err
	}
	if err := destination.SaveCatalog(catalog); err != nil {
		return err
	}
	if err := destination.SavePlanning(planning); err != nil {
		return err
	}
	return destination.SaveMachine(config.NewMachineState())
}

// --- Built-in fixture builders ---
