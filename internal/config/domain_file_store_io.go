package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sglavoie/docket/internal/util"
)

// This file owns the bytes: the locked, atomic, backed-up read and write paths
// every domain artifact goes through, including the optimistic concurrency
// check and the backup recovery a corrupted primary falls back to. The envelope
// those bytes carry lives in domain_file_store_envelope.go, and the per-domain
// entry points in domain_file_store_domains.go.

// load reads an artifact into out and returns its content revision. A missing
// file yields ("", nil) so the caller keeps its seeded defaults; corruption
// falls back to the .bak sidecar before failing.
func (s *DomainFileStore) load(kind domainKind, out any) (string, error) {
	return s.loadArtifact(kind, out, true)
}

// loadArtifact is load with the backup recovery optional: a read-only caller
// passes recoverFromBackup false, so a corrupted primary fails without the
// sidecar being read or the primary rewritten.
func (s *DomainFileStore) loadArtifact(kind domainKind, out any, recoverFromBackup bool) (string, error) {
	path, err := s.pathFor(kind)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", filepath.Base(path), err)
	}

	rev := revisionOf(data)
	env, decErr := decodeEnvelope(data)
	if decErr != nil {
		if !recoverFromBackup {
			return rev, fmt.Errorf("failed to parse %s: %w; %s", absoluteArtifactPath(path), decErr, s.corruptArtifactNextStep(path))
		}
		recovered, recoveredData, recErr := s.recoverEnvelopeFromBackup(path, decErr)
		if recErr != nil {
			return rev, recErr
		}
		env = recovered
		rev = revisionOf(recoveredData)
	}
	if env.SchemaVersion != CurrentDomainSchemaVersion {
		return rev, fmt.Errorf("%s envelope schema version %d is not supported; want %d", filepath.Base(path), env.SchemaVersion, CurrentDomainSchemaVersion)
	}
	if env.Kind != kind.name() {
		return rev, fmt.Errorf("%s contains %q domain data; want %q", filepath.Base(path), env.Kind, kind.name())
	}
	if err := s.BindWorkspaceID(env.WorkspaceID); err != nil {
		return rev, fmt.Errorf("invalid workspace identity in %s: %w", filepath.Base(path), err)
	}
	if len(env.Data) == 0 {
		return rev, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(env.Data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return rev, fmt.Errorf("failed to parse %s payload: %w; %s", absoluteArtifactPath(path), err, s.corruptArtifactNextStep(path))
	}
	if err := requireJSONEOF(decoder); err != nil {
		return rev, fmt.Errorf("failed to parse %s payload: %w; %s", absoluteArtifactPath(path), err, s.corruptArtifactNextStep(path))
	}
	if catalog, ok := out.(*WorkspaceCatalog); ok {
		if catalog.WorkspaceID == "" {
			return rev, fmt.Errorf("invalid workspace identity in %s payload: %w", filepath.Base(path), ErrWorkspaceIDRequired)
		}
		if err := s.BindWorkspaceID(catalog.WorkspaceID); err != nil {
			return rev, fmt.Errorf("invalid workspace identity in %s payload: %w", filepath.Base(path), err)
		}
	}
	return rev, nil
}

// corruptArtifactNextStep is the recovery step every parse failure of a domain
// artifact ends with, so the operator who cannot open the workspace learns what
// to do next rather than only which file broke. A synced artifact sits beside
// the migration archives in the internal .tt directory, so those are named as
// one source of a copy; machine-local state has no archive to point at.
func (s *DomainFileStore) corruptArtifactNextStep(path string) string {
	if s.dataDir != "" && filepath.Dir(path) == InternalDir(s.dataDir) {
		return fmt.Sprintf("fix the JSON in %s by hand, or restore it from a copy of the workspace "+
			"(migration archives are kept in %s), then retry",
			absoluteArtifactPath(path), absoluteArtifactPath(PreScopeV7BackupsDir(s.dataDir)))
	}
	return fmt.Sprintf("fix the JSON in %s by hand, then retry", absoluteArtifactPath(path))
}

// absoluteArtifactPath is path made absolute for an error message, so an
// operator who passed a relative --data-dir is still told exactly which file.
func absoluteArtifactPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// recoverEnvelopeFromBackup restores an artifact from its .bak sidecar when the
// primary is corrupted, re-persisting the recovered bytes so later loads do not
// hit the corruption again.
func (s *DomainFileStore) recoverEnvelopeFromBackup(path string, primaryErr error) (domainEnvelope, []byte, error) {
	backupData, err := os.ReadFile(path + ".bak")
	if err != nil {
		return domainEnvelope{}, nil, fmt.Errorf("failed to parse %s (no backup available): %w; %s", absoluteArtifactPath(path), primaryErr, s.corruptArtifactNextStep(path))
	}
	env, err := decodeEnvelope(backupData)
	if err != nil {
		return domainEnvelope{}, nil, fmt.Errorf("failed to parse %s and its backup is also corrupted: %w; %s", absoluteArtifactPath(path), primaryErr, s.corruptArtifactNextStep(path))
	}
	if writeErr := util.AtomicWriteFile(path, backupData, FilePermissions); writeErr != nil {
		return domainEnvelope{}, nil, fmt.Errorf("recovered %s from backup but failed to persist it: %w", filepath.Base(path), writeErr)
	}
	return env, backupData, nil
}

// saveObserved writes an artifact atomically under a per-file lock with a
// backup and private permissions. When expectedRev is non-empty the on-disk
// revision is verified inside the lock and a mismatch (including a vanished
// file) returns ErrStaleObject without writing.
//
// When baselineObserved is true an empty expectedRev stops meaning "skip the
// check" and starts meaning "the caller observed no artifact", which is a
// require-absent contract: a file that appeared since that observation returns
// ErrStaleObject instead of being overwritten. It also returns the revision of
// the bytes it wrote, so the caller can advance its baseline without re-reading
// the file or re-deriving the encoding.
func (s *DomainFileStore) saveObserved(kind domainKind, payload any, expectedRev string, baselineObserved bool) (string, error) {
	if kind != domainMachine {
		if err := s.requireDataDir(); err != nil {
			return "", err
		}
	}
	if catalog, ok := payload.(*WorkspaceCatalog); ok {
		if _, err := s.bindCatalogPayload(catalog); err != nil {
			return "", err
		}
	} else if _, err := s.validatedWorkspaceID(); err != nil {
		return "", err
	}

	path, err := s.pathFor(kind)
	if err != nil {
		return "", err
	}
	if mkErr := os.MkdirAll(filepath.Dir(path), DirPermissions); mkErr != nil {
		return "", fmt.Errorf("failed to create config directory: %w", mkErr)
	}

	written := ""
	lockPath := path + ".lock"
	err = util.WithFileLock(lockPath, true, func() error {
		// Workspace identity is immutable even for last-writer-wins saves. Check
		// the current envelope while holding the same lock as the write so a
		// concurrent catalog create cannot be replaced with another identity.
		current, currentErr := os.ReadFile(path)
		if currentErr == nil {
			env, decErr := decodeEnvelope(current)
			if decErr != nil {
				return fmt.Errorf("failed to validate workspace identity in %s: %w", filepath.Base(path), decErr)
			}
			if bindErr := s.BindWorkspaceID(env.WorkspaceID); bindErr != nil {
				return fmt.Errorf("invalid workspace identity in %s: %w", filepath.Base(path), bindErr)
			}
		} else if !errors.Is(currentErr, os.ErrNotExist) {
			return fmt.Errorf("failed to read %s for workspace identity check: %w", filepath.Base(path), currentErr)
		}

		if expectedRev != "" || baselineObserved {
			switch {
			case errors.Is(currentErr, os.ErrNotExist):
				// An observed-absent baseline is satisfied by a still-absent file;
				// every other caller reaching here expected a revision and finds none.
				if expectedRev != "" {
					return ErrStaleObject
				}
			case currentErr != nil:
				return fmt.Errorf("failed to re-read %s for concurrency check: %w", filepath.Base(path), currentErr)
			default:
				if revisionOf(current) != expectedRev {
					return ErrStaleObject
				}
			}
		}

		backupDomainArtifact(path)

		out, encErr := encodeDomainEnvelope(kind, s.workspaceID, payload)
		if encErr != nil {
			return encErr
		}
		if writeErr := util.AtomicWriteFile(path, out, FilePermissions); writeErr != nil {
			return fmt.Errorf("failed to write %s: %w", filepath.Base(path), writeErr)
		}
		written = revisionOf(out)
		return nil
	})
	if err != nil {
		return "", err
	}
	return written, nil
}

// backupDomainArtifact best-effort copies an existing artifact to its .bak
// sidecar before it is overwritten, mirroring the single-file manager's backup.
func backupDomainArtifact(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return // nothing to back up (first write) or unreadable; overwrite proceeds
	}
	if err := util.AtomicWriteFile(path+".bak", content, FilePermissions); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to back up %s: %v\n", filepath.Base(path), err)
	}
}
