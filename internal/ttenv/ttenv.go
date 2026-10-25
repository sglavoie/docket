// Package ttenv declares environment-variable names shared by independent tt
// subsystems. It intentionally imports no tt packages so it remains safe for
// both configuration and process-adapter code to depend on.
package ttenv

const (
	// DataDirEnvVar selects the workspace data directory when no explicit
	// --data-dir flag is given.
	DataDirEnvVar = "DOCKET_DATA_DIR"
)
