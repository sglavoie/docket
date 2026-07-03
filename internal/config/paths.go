package config

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
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
	// machine-local runtime state keyed by data dir when the durable data dir
	// is not the local default.
	runtimeDirName = "runtime"
)

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

// LocalDataRoot returns the local (non-synced) default data directory, ~/.tt.
// When the home directory cannot be determined it falls back to a relative
// ".tt" so callers still receive a usable path.
func LocalDataRoot() string {
	home, ok := resolveHomeDir()
	if !ok {
		return dataDirName
	}
	return filepath.Join(home, dataDirName)
}

// iCloudDataRoot returns the tt data directory inside iCloud Drive and whether
// iCloud Drive is available on this machine. Availability is determined by the
// presence of the iCloud Drive container directory under the home directory.
func iCloudDataRoot() (string, bool) {
	home, ok := resolveHomeDir()
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

// ActiveDefaultDataDir returns the data directory tt uses by default when the
// user has not overridden it: the iCloud Drive root when iCloud Drive is
// available, otherwise the local ~/.tt root.
func ActiveDefaultDataDir() string {
	if root, ok := iCloudDataRoot(); ok {
		return root
	}
	return LocalDataRoot()
}

// DefaultConfigPath returns the config file path inside the active default data
// directory.
func DefaultConfigPath() string {
	return filepath.Join(ActiveDefaultDataDir(), configFileName)
}

// LegacyConfigPath returns the config file path inside the local ~/.tt data
// directory. This is where pre-iCloud installs stored their config; it is used
// to detect and migrate an existing local default install.
func LegacyConfigPath() string {
	return filepath.Join(LocalDataRoot(), configFileName)
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

// DefaultDataDirDisplay returns the active default data directory with the home
// prefix abbreviated to "~", suitable for help text and generated docs.
func DefaultDataDirDisplay() string {
	return DisplayPath(ActiveDefaultDataDir())
}

// LocalRuntimeDirForDataDir returns a machine-local directory for runtime state
// that must never sync (logs, TUI PID locks) even when the durable data
// directory lives on iCloud Drive. Runtime state always lives under the local
// ~/.tt root: when the data directory is that same local root the local root is
// returned directly (preserving the historical ~/.tt/logs layout), otherwise
// the path is keyed by a short hash of the cleaned data-directory path so
// distinct data directories get isolated runtime state.
func LocalRuntimeDirForDataDir(dataDir string) string {
	local := LocalDataRoot()
	cleanData := filepath.Clean(dataDir)
	if cleanData == filepath.Clean(local) {
		return local
	}
	sum := sha256.Sum256([]byte(cleanData))
	key := hex.EncodeToString(sum[:])[:12]
	return filepath.Join(local, runtimeDirName, key)
}
