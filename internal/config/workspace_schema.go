package config

import "fmt"

// SchemaTooNewError reports a workspace artifact written by a newer tt than
// this build. A frontend that must not guess at an unknown format matches it
// with errors.As rather than reading the message.
type SchemaTooNewError struct {
	// Artifact names what carried the schema, as the message spells it.
	Artifact string
	// Found is the schema the artifact declares.
	Found int
	// Supported is the newest schema this build reads.
	Supported int
}

func (e *SchemaTooNewError) Error() string {
	return fmt.Sprintf("%s schema %d is newer than supported schema %d; upgrade tt", e.Artifact, e.Found, e.Supported)
}

// WorkspaceCatalogSchema reports the payload schema of the workspace catalog at
// dataDir without loading, recovering, or migrating anything, and whether a
// catalog exists there at all. An envelope or payload newer than this build is
// a *SchemaTooNewError; comparing an older payload against
// CurrentConfigVersion is the caller's question. An enrolled workspace reports
// SyncEnrolledConfigVersion, which this build reads.
func WorkspaceCatalogSchema(dataDir string) (int, bool, error) {
	schema, exists, err := workspaceCatalogPayloadSchema(dataDir)
	if err != nil || !exists {
		return 0, exists, err
	}
	if schema > SupportedConfigVersion {
		return schema, true, &SchemaTooNewError{Artifact: "workspace catalog", Found: schema, Supported: SupportedConfigVersion}
	}
	return schema, true, nil
}
