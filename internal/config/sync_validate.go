package config

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ValidateSyncedDomainArtifact reports whether data is a workspace catalog,
// planning, or lens-definition file ("workspace", "planning", "lenses") the
// live loader accepts as it stands: the current envelope schema, the named
// kind, a valid workspace identity, a payload with no unknown fields, and a
// domain that validates. Sync holds an external change it refuses rather than
// publishing a file no device could load.
func ValidateSyncedDomainArtifact(kindName string, data []byte) error {
	var (
		kind    domainKind
		payload any
	)
	switch kindName {
	case domainCatalog.name():
		kind, payload = domainCatalog, &WorkspaceCatalog{}
	case domainPlanning.name():
		kind, payload = domainPlanning, &PlanningState{}
	case domainLenses.name():
		kind, payload = domainLenses, NewWorkspaceLensDefinitions()
	default:
		return fmt.Errorf("%q is not a synced workspace domain", kindName)
	}
	env, err := decodeEnvelope(data)
	if err != nil {
		return fmt.Errorf("decoding %s: %w", kind.name(), err)
	}
	if env.SchemaVersion != CurrentDomainSchemaVersion {
		return fmt.Errorf("%s envelope schema version %d is not supported; want %d", kind.name(), env.SchemaVersion, CurrentDomainSchemaVersion)
	}
	if env.Kind != kind.name() {
		return fmt.Errorf("%s file contains %q domain data", kind.name(), env.Kind)
	}
	if err := validateWorkspaceID(env.WorkspaceID); err != nil {
		return fmt.Errorf("invalid workspace identity in %s: %w", kind.name(), err)
	}
	decoder := json.NewDecoder(bytes.NewReader(env.Data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(payload); err != nil {
		return fmt.Errorf("decoding %s payload: %w", kind.name(), err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return fmt.Errorf("decoding %s payload: %w", kind.name(), err)
	}
	switch typed := payload.(type) {
	case *WorkspaceCatalog:
		if typed.WorkspaceID != env.WorkspaceID {
			return fmt.Errorf("workspace catalog payload names workspace %q, its envelope %q", typed.WorkspaceID, env.WorkspaceID)
		}
		return typed.Validate()
	case *PlanningState:
		return typed.Validate()
	case *WorkspaceLensDefinitions:
		return typed.Validate()
	}
	return nil
}
