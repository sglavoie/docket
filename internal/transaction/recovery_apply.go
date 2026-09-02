package transaction

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// driveCommit is the single apply path shared by crash recovery (rollForward)
// and the online commit boundary (Journal.Commit). It advances a prepared or
// committing manifest through committing to committed — writing the committing
// state as a durable intent marker so a crash mid-apply is re-driven to
// completion on the next Recover pass — applies every file operation
// idempotently, and reclaims the transaction's staged artifacts and manifest.
// Keeping exactly one apply implementation guarantees recovery and commit can
// never diverge on rename/fsync ordering.
func (j *Journal) driveCommit(m *Manifest, path string) error {
	if m.State != StateCommitting {
		m.State = StateCommitting
		if err := j.writeManifest(m); err != nil {
			return fmt.Errorf("marking transaction committing: %w", err)
		}
	}
	for i, op := range m.Ops {
		if err := j.applyOp(op); err != nil {
			return fmt.Errorf("applying op %d (%s %q): %w", i, op.Kind, op.Target, err)
		}
	}
	m.State = StateCommitted
	if err := j.writeManifest(m); err != nil {
		return fmt.Errorf("marking transaction committed: %w", err)
	}
	return j.reclaim(m, path)
}

// applyOp applies a single file operation idempotently.
func (j *Journal) applyOp(op FileOp) error {
	switch op.Kind {
	case OpWrite:
		return j.applyWrite(op)
	case OpMove:
		return j.applyMove(op)
	case OpDelete:
		return j.applyDelete(op)
	default:
		return fmt.Errorf("%q: %w", op.Kind, ErrUnknownOpKind)
	}
}

// applyWrite renames a verified staging file onto its target. If the staging file
// is already gone the write was applied on an earlier pass, so a present target
// is accepted as success; a target that is also missing means the staged bytes
// were lost and the write is irrecoverable.
func (j *Journal) applyWrite(op FileOp) error {
	_, statErr := os.Stat(op.Staged)
	switch {
	case statErr == nil:
		if op.Hash != "" {
			got, herr := hashFile(op.Staged)
			if herr != nil {
				return fmt.Errorf("hashing staged file %q: %w", op.Staged, herr)
			}
			if got != op.Hash {
				return fmt.Errorf("staged file %q hash %s does not match expected %s", op.Staged, got, op.Hash)
			}
		}
		if renameErr := j.rename(op.Staged, op.Target); renameErr != nil {
			return fmt.Errorf("renaming staged file %q onto target: %w", op.Staged, renameErr)
		}
		if syncErr := j.syncDir(filepath.Dir(op.Target)); syncErr != nil {
			return fmt.Errorf("syncing target directory: %w", syncErr)
		}
		return nil
	case os.IsNotExist(statErr):
		return confirmApplied(op.Target, "write")
	default:
		return fmt.Errorf("stat staged file %q: %w", op.Staged, statErr)
	}
}

// applyMove renames the source onto the target. An already-missing source with a
// present target means the move completed on an earlier pass.
func (j *Journal) applyMove(op FileOp) error {
	_, statErr := os.Stat(op.Source)
	switch {
	case statErr == nil:
		if renameErr := j.rename(op.Source, op.Target); renameErr != nil {
			return fmt.Errorf("renaming %q onto %q: %w", op.Source, op.Target, renameErr)
		}
		if syncErr := j.syncDir(filepath.Dir(op.Target)); syncErr != nil {
			return fmt.Errorf("syncing target directory: %w", syncErr)
		}
		if srcDir := filepath.Dir(op.Source); srcDir != filepath.Dir(op.Target) {
			if syncErr := j.syncDir(srcDir); syncErr != nil {
				return fmt.Errorf("syncing source directory: %w", syncErr)
			}
		}
		return nil
	case os.IsNotExist(statErr):
		return confirmApplied(op.Target, "move")
	default:
		return fmt.Errorf("stat move source %q: %w", op.Source, statErr)
	}
}

// applyDelete removes the target; an already-absent target is success.
func (j *Journal) applyDelete(op FileOp) error {
	if err := j.remove(op.Target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %q: %w", op.Target, err)
	}
	if err := j.syncDir(filepath.Dir(op.Target)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("syncing directory after delete: %w", err)
	}
	return nil
}

// confirmApplied treats a present target as a completed operation and a missing
// one as irrecoverable, used when the source/staged input is already gone.
func confirmApplied(target, kind string) error {
	_, err := os.Stat(target)
	switch {
	case err == nil:
		return nil
	case os.IsNotExist(err):
		return fmt.Errorf("%s source and target %q both missing: irrecoverable", kind, target)
	default:
		return fmt.Errorf("stat target %q: %w", target, err)
	}
}

// reclaim removes a transaction's staged artifacts and then its manifest. Both
// removals tolerate already-absent files so reclaim is idempotent.
func (j *Journal) reclaim(m *Manifest, path string) error {
	for _, op := range m.Ops {
		if op.Kind == OpWrite && op.Staged != "" {
			if err := j.remove(op.Staged); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing staged file %q: %w", op.Staged, err)
			}
		}
	}
	if err := j.remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing manifest %q: %w", path, err)
	}
	return nil
}

// hashBytes returns the hex SHA-256 of data, used to record a write's expected
// content in its manifest.
func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// hashFile returns the hex SHA-256 of a file's contents.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
