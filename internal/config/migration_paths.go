package config

import "path/filepath"

// projectsDirName is the durable project-data subdirectory.
const projectsDirName = "projects"

// migrationRootExcludedNames are machine-local entries excluded from durable
// workspace snapshots and moves, together with the schema-24 tombstone and the
// stages its activation assembles: none of them is workspace data, and a
// workspace opened without its tombstone gets a fresh one.
var migrationRootExcludedNames = map[string]bool{
	"logs":                   true,
	"pid.lock":               true,
	machineStateFileName:     true,
	"runtime":                true,
	".runtime":               true,
	legacyInternalDirName:    true,
	v24StageDirName:          true,
	v24TombstoneStageDirName: true,
	v24RetiredDirName:        true,
}

func isMigrationRootExcluded(rel string) bool {
	clean := filepath.Clean(rel)
	return filepath.Dir(clean) == "." && migrationRootExcludedNames[clean]
}
