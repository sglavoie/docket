package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/ttenv"
	"github.com/sglavoie/tt/internal/tterrors"
)

// CLIWorkspace is the facade-free composition root shared by CLI commands. A
// command constructs it once, reads the cached domain snapshot, and uses the
// manager only to build revision-checked artifacts or domain commits.
type CLIWorkspace struct {
	Manager *config.Manager
	Runtime *config.RuntimeState
}

// LoadCLIWorkspace ensures fresh workspaces have a complete split-domain set,
// then builds the cached RuntimeState and RuntimeDependencies composition once
// for the command.
func LoadCLIWorkspace() (*CLIWorkspace, error) {
	return loadCLIWorkspace()
}

func loadCLIWorkspace() (*CLIWorkspace, error) {
	if err := checkDataDirOverride(); err != nil {
		return nil, err
	}
	manager := config.NewManager(resolveConfigPath())
	_, err := manager.LoadOrCreateDomains()
	if err != nil {
		return nil, fmt.Errorf("failed to load workspace: %w", err)
	}
	runtime, err := manager.LoadRuntimeState()
	if err != nil {
		return nil, fmt.Errorf("failed to load runtime state: %w", err)
	}
	return &CLIWorkspace{
		Manager: manager,
		Runtime: runtime,
	}, nil
}

// openDesktopRuntime opens the workspace the desktop launch resolves — an
// explicit --data-dir or TT_DATA_DIR, the active-workspace pointer, or the
// default — through the shared runtime, as the process machine. The resolution
// policy is the desktop's and stays here; the runtime receives its answer as
// explicit configuration.
func openDesktopRuntime() (*engine.Runtime, error) {
	if err := checkDataDirOverride(); err != nil {
		return nil, err
	}
	configPath := config.NewManager(resolveConfigPath()).GetConfigPath()
	return engine.Open(engine.Config{
		DataDir:         config.WorkspaceRootFromConfigPath(configPath),
		Machine:         engine.ProcessMachine(),
		CreateIfMissing: true,
	})
}

// LoadRuntimeProject resolves one project by slug or ID from the cached runtime
// domains.
func LoadRuntimeProject(ref string) (*models.Project, *CLIWorkspace, error) {
	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return nil, nil, err
	}
	project := workspace.Runtime.GetProject(ref)
	if project == nil {
		return nil, nil, tterrors.NewProjectNotFoundError(ref)
	}
	return project, workspace, nil
}

// dataDirOverrideSource names where the explicit workspace override came from,
// the flag winning over the environment exactly as GetDataDir resolves it.
func dataDirOverrideSource() string {
	if dataDir != "" {
		return "--data-dir"
	}
	return ttenv.DataDirEnvVar
}

// checkDataDirOverride refuses an explicit --data-dir or TT_DATA_DIR that names
// something other than a directory, naming the override itself, before any
// path beneath it is derived and fails with ENOTDIR. A path that does not
// exist yet is not refused here: creating a workspace there is legitimate.
func checkDataDirOverride() error {
	dir := GetDataDir()
	if dir == "" {
		return nil
	}
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		if dataDir == "" {
			return fmt.Errorf("%s=%s is not a directory", ttenv.DataDirEnvVar, dir)
		}
		return fmt.Errorf("--data-dir %s is not a directory", dir)
	}
	return nil
}

// requireExistingWorkspace refuses to inspect a workspace that is not there.
// Loading a workspace creates one, which a command reading an existing one must
// never do, so doctor, the conflict commands, and project export ask this first and name the path
// they would otherwise have created.
// The data directory alone proves nothing — the default ~/.tt also holds the
// machine-local logs every command opens — so a workspace is its internal .tt
// directory, which every schema since v4 keeps, or the legacy aggregate config
// file an older workspace still needs migrating from.
func requireExistingWorkspace() error {
	if err := checkDataDirOverride(); err != nil {
		return err
	}
	configPath := config.NewManager(resolveConfigPath()).GetConfigPath()
	dir := config.WorkspaceRootFromConfigPath(configPath)
	for _, marker := range []string{config.InternalDir(dir), configPath} {
		_, err := os.Stat(marker)
		if err == nil {
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to inspect workspace at %s: %w", absolutePath(dir), err)
		}
	}
	return fmt.Errorf("no tt workspace at %s; this command never creates one: "+
		"pass --data-dir with an existing workspace, or run 'tt' to create one there", absolutePath(dir))
}

// absolutePath is path made absolute for an error message, or path unchanged
// when it cannot be.
func absolutePath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}
