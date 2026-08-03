package config

import "path/filepath"

// projectsDirName is the durable project-data subdirectory.
const projectsDirName = "projects"

// migrationRootExcludedNames are machine-local entries excluded from durable
// workspace snapshots and moves.
var migrationRootExcludedNames = map[string]bool{
	"logs":               true,
	"pid.lock":           true,
	machineStateFileName: true,
	"runtime":            true,
	".runtime":           true,
}

func isMigrationRootExcluded(rel string) bool {
	clean := filepath.Clean(rel)
	return filepath.Dir(clean) == "." && migrationRootExcludedNames[clean]
}
