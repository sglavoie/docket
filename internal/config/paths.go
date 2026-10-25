package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sglavoie/docket/internal/util"
)

// Path resolution constants. All default data-dir and config-path decisions
// route through the helpers in this file so there is a single source of truth
// for where tt keeps durable data, its config file, and machine-local runtime
// state.
const (
	// dataDirName is the local (home-relative) tt data directory name.
	dataDirName = ".tt"

	// iCloudDataDirName is the tt data directory name inside iCloud Drive.
	// iCloud Drive folders are user-visible in Finder, so the synced root drops
	// the leading dot used by the local root.
	iCloudDataDirName = "tt"

	// configFileName is the JSON config file stored inside the data directory.
	configFileName = ".tt.json"

	// iCloudDriveRelPath is the iCloud Drive container path relative to the
	// user's home directory on macOS.
	iCloudDriveRelPath = "Library/Mobile Documents/com~apple~CloudDocs"

	// runtimeDirName is the subdirectory under the local data root that holds
	// machine-local state. machine.json is keyed by workspace ID beneath this
	// directory; path-local ephemeral files use LocalRuntimeDirForDataDir.
	runtimeDirName = "runtime"

	// activeWorkspacePointerName is the machine-local file that records which
	// data directory this machine last opened. It lives under the local runtime
	// root (never synced) at a fixed location so it can select the data dir
	// before any data dir is known.
	activeWorkspacePointerName = "active-workspace.json"

	// machineStateFileName is the machine-local decomposed-config file that holds
	// editors, navigation, histories, and layout state. It lives under the local
	// runtime dir keyed by the durable workspace ID so moving a workspace does
	// not lose its machine-local state and distinct identities never share it.
	machineStateFileName = "machine.json"
)

// InternalDir returns the internal ".tt" directory inside a data directory.
// It contains the portable catalog and planning files plus transaction and
// migration artifacts. The helper is available without a loaded Config so all
// path derivation shares one root.
func InternalDir(dataDir string) string {
	return filepath.Join(dataDir, dataDirName)
}

// ConfigPathForDataDir returns the legacy single-file config path inside a data
// directory. It is the inverse of WorkspaceRootFromConfigPath and lets callers
// derive the config path once a data directory has been resolved.
func ConfigPathForDataDir(dataDir string) string {
	return filepath.Join(dataDir, configFileName)
}

// WorkspaceRootFromConfigPath returns the workspace root (data directory) that
// owns a config file. The legacy --config flag is now an isolation-compatible
// alias: whatever directory holds the named config file becomes the workspace
// root, so pointing --config at a temp dir isolates an entire workspace there.
func WorkspaceRootFromConfigPath(configPath string) string {
	return filepath.Dir(configPath)
}

// WorkspaceCatalogPath returns the synced workspace.json path for a data dir.
func WorkspaceCatalogPath(dataDir string) string {
	return filepath.Join(InternalDir(dataDir), "workspace.json")
}

// PlanningStatePath returns the synced planning.json path for a data dir.
func PlanningStatePath(dataDir string) string {
	return filepath.Join(InternalDir(dataDir), "planning.json")
}

// LensDefinitionsPath returns the synced lenses.json path for a data dir. Lens
// definitions are portable content: they name projects by workspace-scoped ID
// and nothing else, so they travel with the workspace rather than staying on
// the machine that authored them. Which lens is open, and the theme, remain
// machine-local.
func LensDefinitionsPath(dataDir string) string {
	return filepath.Join(InternalDir(dataDir), "lenses.json")
}

// SettingsPath returns the retired v4 settings.json path for migration and
// doctor inspection. Live runtime code does not load this domain.
func SettingsPath(dataDir string) string {
	return filepath.Join(InternalDir(dataDir), "settings.json")
}

// AutomationPath returns the retired v4 automation.json path for migration and
// doctor inspection. Live runtime code does not load this domain.
func AutomationPath(dataDir string) string {
	return filepath.Join(InternalDir(dataDir), "automation.json")
}

// TransactionsDir returns the durable directory that holds prepared and
// recovering transaction manifests for a data dir. It lives under the synced
// internal ".tt" directory so any machine that opens the workspace can inspect
// in-flight transactions and drive crash recovery.
func TransactionsDir(dataDir string) string {
	return filepath.Join(InternalDir(dataDir), "transactions")
}

// UndoLedgerDir returns the durable directory that holds the workspace undo
// ledger and its retained before/after artifact blobs for a data dir. It lives
// under the synced internal ".tt" directory so the append-only global
// transaction ledger travels with the workspace, mirroring TransactionsDir.
func UndoLedgerDir(dataDir string) string {
	return filepath.Join(InternalDir(dataDir), "undo")
}

// ConflictsDir returns the durable directory that holds persisted three-way
// merge conflict bundles (base/local/remote documents plus structured conflict
// metadata) for a data dir. It lives under the synced internal ".tt" directory
// so a recorded conflict travels with the workspace and can be inspected and
// resolved from any machine that opens it, mirroring TransactionsDir.
func ConflictsDir(dataDir string) string {
	return filepath.Join(InternalDir(dataDir), "conflicts")
}

// MachineStatePath returns the canonical machine-local machine.json path for a
// workspace ID. Every workspace, including the default local workspace, is
// isolated beneath ~/.tt/runtime/<workspace-id>. The ID must be a canonical
// UUID before it is used as a path component.
func MachineStatePath(workspaceID string) (string, error) {
	return machineStatePathOn("", workspaceID)
}

// machineStatePathOn is MachineStatePath resolved against an explicit machine
// home. It is the one path a second machine identity actually moves, which is
// why the injectable home stops here rather than reaching every path helper.
func machineStatePathOn(machineHome, workspaceID string) (string, error) {
	if err := validateWorkspaceID(workspaceID); err != nil {
		return "", err
	}
	return filepath.Join(localDataRootOn(machineHome), runtimeDirName, workspaceID, machineStateFileName), nil
}

// legacyMachineStatePaths returns the two locations used before machine state
// was keyed by workspace identity. The path derived from the workspace's
// current data directory wins; the historical default-local path is consulted
// only when that path is absent. Duplicate paths are removed for ~/.tt itself.
func legacyMachineStatePaths(dataDir string) []string {
	return legacyMachineStatePathsOn("", dataDir)
}

// legacyMachineStatePathsOn is legacyMachineStatePaths resolved against an
// explicit machine home.
func legacyMachineStatePathsOn(machineHome, dataDir string) []string {
	candidates := []string{
		filepath.Join(localRuntimeDirForDataDirOn(machineHome, dataDir), machineStateFileName),
		filepath.Join(localDataRootOn(machineHome), machineStateFileName),
	}
	paths := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		paths = append(paths, candidate)
	}
	return paths
}

// activeWorkspacePointerPath returns the fixed machine-local path of the
// active-workspace pointer. It lives under the local runtime dir (not keyed by
// any data dir) because it is consulted before a data dir has been selected.
func activeWorkspacePointerPath() string {
	return filepath.Join(LocalDataRoot(), runtimeDirName, activeWorkspacePointerName)
}

// activeWorkspacePointer is the machine-local record of the last-opened data
// directory. It carries a schema version and the workspace ID so a future
// migration can reason about it, but its authoritative field is DataDir.
type activeWorkspacePointer struct {
	SchemaVersion int    `json:"schema_version"`
	WorkspaceID   string `json:"workspace_id,omitempty"`
	DataDir       string `json:"data_dir"`
}

// ReadActiveWorkspacePointer returns the data directory recorded by the
// machine-local active-workspace pointer and whether one was found. A missing,
// unreadable, or malformed pointer reports (\"\", false) so resolution falls
// through to the next precedence tier rather than failing.
func ReadActiveWorkspacePointer() (string, bool) {
	data, err := os.ReadFile(activeWorkspacePointerPath())
	if err != nil {
		return "", false
	}
	var ptr activeWorkspacePointer
	if err := json.Unmarshal(data, &ptr); err != nil {
		return "", false
	}
	if ptr.DataDir == "" {
		return "", false
	}
	return filepath.Clean(ptr.DataDir), true
}

// WriteActiveWorkspacePointer records the active data directory (and optional
// workspace ID) in the machine-local pointer, written atomically with private
// permissions. It is the machine-local half of workspace selection; the synced
// workspace catalog remains the source of truth for workspace identity.
func WriteActiveWorkspacePointer(dataDir, workspaceID string) error {
	if dataDir == "" {
		return fmt.Errorf("config: active-workspace pointer requires a data dir")
	}
	path := activeWorkspacePointerPath()
	if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
		return fmt.Errorf("failed to create active-workspace pointer dir: %w", err)
	}
	ptr := activeWorkspacePointer{
		SchemaVersion: CurrentDomainSchemaVersion,
		WorkspaceID:   workspaceID,
		DataDir:       filepath.Clean(dataDir),
	}
	data, err := json.MarshalIndent(ptr, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode active-workspace pointer: %w", err)
	}
	if err := util.AtomicWriteFile(path, data, FilePermissions); err != nil {
		return fmt.Errorf("failed to write active-workspace pointer: %w", err)
	}
	return nil
}

// resolveHomeDir returns the user's home directory and whether it could be
// determined. It is the single wrapper over os.UserHomeDir so every path helper
// shares one source of truth (and tests can drive it via $HOME).
func resolveHomeDir() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", false
	}
	return home, true
}

// HomeDir is resolveHomeDir for callers outside this package, so a surface that
// abbreviates a home path asks the same one source of truth every path helper
// reads rather than calling os.UserHomeDir itself.
func HomeDir() (string, bool) {
	return resolveHomeDir()
}

// resolveMachineHome is the injectable machine-identity seam over
// resolveHomeDir. Machine identity is exactly the home directory — machine.json
// lives at <home>/.tt/runtime/<workspace-id>/machine.json — so naming a home is
// how a caller says "resolve this as a different machine". The empty string is
// the default and means this process's own $HOME, which keeps every existing
// caller on the environment; only a caller that explicitly carries a machine
// home departs from it. That is what lets two managers with distinct homes run
// concurrently over one data directory, which process-global $HOME cannot
// express.
func resolveMachineHome(machineHome string) (string, bool) {
	if machineHome != "" {
		return machineHome, true
	}
	return resolveHomeDir()
}

// LocalDataRoot returns the local (non-synced) default data directory, ~/.tt.
// When the home directory cannot be determined it falls back to a relative
// ".tt" so callers still receive a usable path.
func LocalDataRoot() string {
	return localDataRootOn("")
}

// localDataRootOn is LocalDataRoot resolved against an explicit machine home.
func localDataRootOn(machineHome string) string {
	home, ok := resolveMachineHome(machineHome)
	if !ok {
		return dataDirName
	}
	return filepath.Join(home, dataDirName)
}

// iCloudDataRoot returns the tt data directory inside iCloud Drive and whether
// iCloud Drive is available on this machine. Availability is determined by the
// presence of the iCloud Drive container directory under the home directory.
func iCloudDataRoot() (string, bool) {
	return iCloudDataRootOn("")
}

// iCloudDataRootOn is iCloudDataRoot resolved against an explicit machine home.
func iCloudDataRootOn(machineHome string) (string, bool) {
	home, ok := resolveMachineHome(machineHome)
	if !ok {
		return "", false
	}
	drive := filepath.Join(home, iCloudDriveRelPath)
	info, err := os.Stat(drive)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return filepath.Join(drive, iCloudDataDirName), true
}

// hasSplitDomainMarker reports whether a data directory contains any synced
// split-domain artifact. A single marker is enough: selecting a partial iCloud
// workspace lets normal loading surface the corruption instead of silently
// falling back to an unrelated local workspace.
func hasSplitDomainMarker(dataDir string) bool {
	for _, path := range []string{
		WorkspaceCatalogPath(dataDir),
		PlanningStatePath(dataDir),
		SettingsPath(dataDir),
		AutomationPath(dataDir),
	} {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// activeDefaultDataDirOn returns the data directory tt uses by default when the
// user has made no explicit workspace selection, resolved against an explicit
// machine home so a named machine selects its own default workspace rather than
// the process home's. The empty machine home is exactly the process home.
//
// New installations are local-first: the local ~/.tt root is used unless an
// iCloud Drive workspace already exists (an existing active iCloud install), in
// which case that workspace is preserved. Automatic migration of a local
// install into iCloud has been removed; relocating a workspace to (or from)
// iCloud is now an explicit action — see MoveWorkspace and `tt workspace move`.
func activeDefaultDataDirOn(machineHome string) string {
	if root, ok := iCloudDataRootOn(machineHome); ok {
		if hasSplitDomainMarker(root) {
			return root
		}
		if _, err := os.Stat(filepath.Join(root, configFileName)); err == nil {
			return root
		}
	}
	return localDataRootOn(machineHome)
}

// defaultConfigPathOn returns the config file path inside the active default
// data directory of an explicit machine home.
func defaultConfigPathOn(machineHome string) string {
	return filepath.Join(activeDefaultDataDirOn(machineHome), configFileName)
}

// DisplayPath abbreviates the user's home-directory prefix to "~" for
// human-facing output. Paths outside the home directory (or when the home
// directory is unknown) are returned unchanged.
func DisplayPath(path string) string {
	home, ok := resolveHomeDir()
	if !ok {
		return path
	}
	if path == home {
		return "~"
	}
	prefix := home + string(os.PathSeparator)
	if strings.HasPrefix(path, prefix) {
		return "~" + string(os.PathSeparator) + path[len(prefix):]
	}
	return path
}

// LocalRuntimeDirForDataDir returns a machine-local directory for runtime state
// that must never sync (logs, TUI PID locks) even when the durable data
// directory lives on iCloud Drive. Runtime state always lives under the local
// ~/.tt root: when the data directory is that same local root the local root is
// returned directly (preserving the historical ~/.tt/logs layout), otherwise
// the path is keyed by a short hash of the cleaned data-directory path so
// distinct data directories get isolated runtime state.
func LocalRuntimeDirForDataDir(dataDir string) string {
	return localRuntimeDirForDataDirOn("", dataDir)
}

// localRuntimeDirForDataDirOn is LocalRuntimeDirForDataDir resolved against an
// explicit machine home, so a named machine keeps its own logs and PID lock
// even while it shares the data directory with another machine.
func localRuntimeDirForDataDirOn(machineHome, dataDir string) string {
	local := localDataRootOn(machineHome)
	cleanData := filepath.Clean(dataDir)
	if cleanData == filepath.Clean(local) {
		return local
	}
	sum := sha256.Sum256([]byte(cleanData))
	key := hex.EncodeToString(sum[:])[:12]
	return filepath.Join(local, runtimeDirName, key)
}

// TUIProcessLockPath returns the machine-local PID lock path that excludes a
// second TUI process from a workspace. Keeping this derivation in config lets
// TUI startup and workspace relocation coordinate on the exact same lock while
// ensuring it never lives in a synced data directory.
func TUIProcessLockPath(dataDir string) string {
	return filepath.Join(LocalRuntimeDirForDataDir(dataDir), "pid.lock")
}
