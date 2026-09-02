package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// This file owns the on-disk envelope every domain artifact is wrapped in: its
// shape, the single encode point shared by the store and the transactional
// artifact encoder, and the content revision that makes the optimistic
// concurrency check possible. Reading and writing the files themselves lives in
// domain_file_store_io.go.

// domainEnvelope wraps a domain payload on disk with the schema version and
// workspace ID required of every artifact. The payload itself is stored under
// Data so the envelope metadata stays uniform across the three runtime files.
type domainEnvelope struct {
	SchemaVersion int             `json:"schema_version"`
	WorkspaceID   string          `json:"workspace_id,omitempty"`
	Kind          string          `json:"kind"`
	Data          json.RawMessage `json:"data"`
}

// revisionOf returns the content revision (hex SHA-256) of raw artifact bytes.
// An empty revision denotes "no artifact observed" and disables the concurrency
// check on save.
func revisionOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// encodeDomainEnvelope renders a domain payload into its on-disk envelope bytes,
// exactly as DomainFileStore.save writes them. It is the single encode point
// shared by the store's own writes and the transactional artifact encoder so a
// domain file produced by a transaction is byte-compatible with one produced by
// Manager.Save, and a reload observes the same content either way.
func encodeDomainEnvelope(kind domainKind, workspaceID string, payload any) ([]byte, error) {
	if err := validateWorkspaceID(workspaceID); err != nil {
		return nil, err
	}
	if catalog, ok := payload.(*WorkspaceCatalog); ok {
		if err := catalog.Validate(); err != nil {
			return nil, fmt.Errorf("invalid workspace catalog: %w", err)
		}
	}
	if planning, ok := payload.(*PlanningState); ok {
		if err := planning.Validate(); err != nil {
			return nil, fmt.Errorf("invalid planning state: %w", err)
		}
	}
	if machine, ok := payload.(*MachineState); ok {
		if err := machine.Validate(); err != nil {
			return nil, fmt.Errorf("invalid machine state: %w", err)
		}
	}
	if lenses, ok := payload.(*WorkspaceLensDefinitions); ok {
		if err := lenses.Validate(); err != nil {
			return nil, fmt.Errorf("invalid workspace lens definitions: %w", err)
		}
	}
	return marshalDomainEnvelope(kind, workspaceID, payload)
}

// encodeMigrationCatalogEnvelope renders the catalog a schema migration is about
// to activate. It validates against the schema the migration targets rather than
// the schema this build loads, which is the one thing encodeDomainEnvelope
// cannot do: an activation publishes its target schema before that target is
// loadable. It shares marshalDomainEnvelope so the bytes stay identical to an
// ordinary save.
func encodeMigrationCatalogEnvelope(catalog *WorkspaceCatalog, targetSchema int) ([]byte, error) {
	if err := validateWorkspaceID(catalog.WorkspaceID); err != nil {
		return nil, err
	}
	if err := catalog.validateAtSchema(targetSchema); err != nil {
		return nil, fmt.Errorf("invalid workspace catalog: %w", err)
	}
	return marshalDomainEnvelope(domainCatalog, catalog.WorkspaceID, catalog)
}

func marshalDomainEnvelope(kind domainKind, workspaceID string, payload any) ([]byte, error) {
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s payload: %w", kind.name(), err)
	}
	env := domainEnvelope{
		SchemaVersion: CurrentDomainSchemaVersion,
		WorkspaceID:   workspaceID,
		Kind:          kind.name(),
		Data:          rawPayload,
	}
	out, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s: %w", kind.name(), err)
	}
	return out, nil
}

func decodeEnvelope(data []byte) (domainEnvelope, error) {
	var env domainEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return domainEnvelope{}, err
	}
	return env, nil
}
