package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sglavoie/tt/internal/transaction"
	"github.com/sglavoie/tt/internal/util"
)

func lockPathsNotHeldBy(paths, held []string) []string {
	heldSet := make(map[string]bool, len(held))
	for _, path := range held {
		heldSet[filepath.Clean(path)] = true
	}
	remaining := make([]string, 0, len(paths))
	for _, path := range paths {
		if !heldSet[filepath.Clean(path)] {
			remaining = append(remaining, path)
		}
	}
	return remaining
}

// RawArtifactRevision returns the optimistic-concurrency revision of an artifact's
// raw on-disk bytes: the hex-encoded SHA-256 of data. It is the storage-side
// definition of ArtifactChange.ExpectedRevision, and it matches the hash the
// config domain store computes for the same bytes, so a revision observed at load
// time by a config coordinator validates against the same file here.
func RawArtifactRevision(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// validateArtifactRevisions enforces every artifact's ExpectedRevision contract
// against the current on-disk bytes. It must be called under the artifact locks so
// the observed state cannot change between the check and the staged write. A nil
// ExpectedRevision skips the artifact; an empty expected revision requires the
// file to be absent; a non-empty expected revision requires the current raw hash
// to match exactly. Any violation returns an *ArtifactConflictError.
func validateArtifactRevisions(artifacts []ArtifactChange) error {
	for i := range artifacts {
		a := artifacts[i]
		if a.ExpectedRevision == nil {
			continue
		}
		checkPath := artifactRevisionPath(a)
		data, err := os.ReadFile(checkPath)
		switch {
		case os.IsNotExist(err):
			if *a.ExpectedRevision != "" {
				return &ArtifactConflictError{Path: checkPath, Expected: *a.ExpectedRevision, Actual: ""}
			}
		case err != nil:
			return fmt.Errorf("reading artifact %s for concurrency check: %w", filepath.Base(checkPath), err)
		default:
			actual := RawArtifactRevision(data)
			if *a.ExpectedRevision == "" {
				return &ArtifactConflictError{Path: checkPath, Expected: "", Actual: actual}
			}
			if actual != *a.ExpectedRevision {
				return &ArtifactConflictError{Path: checkPath, Expected: *a.ExpectedRevision, Actual: actual}
			}
		}
	}
	return nil
}

// validateArtifactPreconditions checks read-only artifact guards under their
// held advisory locks. It deliberately shares ArtifactConflictError with
// ArtifactChange revision failures so callers can classify both as
// ErrStaleArtifact while inspecting the conflicting path and revisions.
func validateArtifactPreconditions(preconditions []ArtifactPrecondition) error {
	for i := range preconditions {
		precondition := preconditions[i]
		data, err := os.ReadFile(precondition.Path)
		switch {
		case os.IsNotExist(err):
			if precondition.ExpectedRevision != "" {
				return &ArtifactConflictError{Path: precondition.Path, Expected: precondition.ExpectedRevision, Actual: ""}
			}
		case err != nil:
			return fmt.Errorf("reading artifact %s for precondition check: %w", filepath.Base(precondition.Path), err)
		default:
			actual := RawArtifactRevision(data)
			if precondition.ExpectedRevision == "" || actual != precondition.ExpectedRevision {
				return &ArtifactConflictError{Path: precondition.Path, Expected: precondition.ExpectedRevision, Actual: actual}
			}
		}
	}
	return nil
}

func artifactRevisionPath(artifact ArtifactChange) string {
	if artifact.ExpectedRevisionPath != "" {
		return artifact.ExpectedRevisionPath
	}
	return artifact.Path
}

func artifactAdvisoryLockPath(path string) string {
	if filepath.Base(path) == tasksFileName {
		return filepath.Join(filepath.Dir(path), LockFileName)
	}
	return path + ".lock"
}

// validateProjectPaths checks every path CommitMulti reads, writes, or opens for
// a project before the workspace lock can create the transactions directory.
// PathWithinRoot resolves existing symlinks, so an escaping project directory,
// tasks.md, or advisory lock is rejected without filesystem effects.
func validateProjectPaths(changeset int, projectPath string, roots []string) error {
	if projectPath == "" {
		return fmt.Errorf("storage: changeset %d project requires a path", changeset)
	}
	checks := []artifactPathRole{
		{path: projectPath, role: "project directory"},
		{path: tasksFilePath(projectPath), role: "project tasks"},
		{path: filepath.Join(projectPath, LockFileName), role: "project lock"},
	}
	for _, check := range checks {
		if !pathWithinApprovedRoots(check.path, roots) {
			return fmt.Errorf("storage: changeset %d %s %q: %w", changeset, check.role, check.path, transaction.ErrTargetOutsideRoot)
		}
	}
	return nil
}

// validateProjectPreconditionPath checks every path a read-only project guard
// may inspect or open before the workspace lock can create transaction or lock
// files. A guard has no backup target because it never writes tasks.md.
func validateProjectPreconditionPath(changeset, precondition int, projectPath string, roots []string) error {
	if projectPath == "" {
		return fmt.Errorf("storage: changeset %d project precondition %d requires a path", changeset, precondition)
	}
	checks := []artifactPathRole{
		{path: projectPath, role: "project directory"},
		{path: tasksFilePath(projectPath), role: "project tasks"},
		{path: filepath.Join(projectPath, LockFileName), role: "project lock"},
	}
	for _, check := range checks {
		if !pathWithinApprovedRoots(check.path, roots) {
			return fmt.Errorf("storage: changeset %d project precondition %d %s %q: %w", changeset, precondition, check.role, check.path, transaction.ErrTargetOutsideRoot)
		}
	}
	return nil
}

// artifactLockPaths returns the deterministic lock-file path for every artifact
// target after validating both the write target and revision source against the
// approved roots. It is deliberately read-only: all artifact and lock paths,
// including existing symlink prefixes, must pass containment before the commit
// creates its workspace lock, target directories, staging files, or manifest.
// Domain artifacts use the adjacent ".lock" suffix; tasks.md uses the canonical
// project .tasks.md.lock so lifecycle rewrites serialize with ordinary commits.
func artifactLockPaths(artifacts []ArtifactChange, roots []string) ([]string, error) {
	if err := validateArtifactPaths(artifacts, roots); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(artifacts)*2)
	for i := range artifacts {
		artifact := artifacts[i]
		checkPath := artifactRevisionPath(artifact)
		paths = append(paths, artifactAdvisoryLockPath(checkPath))
		// A marked artifact reads Path to capture its backup. When revision
		// validation uses a different source, also hold the primary's advisory lock
		// so a checked single-file writer cannot change those bytes between capture
		// and journal preparation.
		if artifact.BackupPath != "" && artifactPathIdentity(checkPath) != artifactPathIdentity(artifact.Path) {
			paths = append(paths, artifactAdvisoryLockPath(artifact.Path))
		}
	}
	return paths, nil
}

type artifactPathRole struct {
	path string
	role string
}

// validateArtifactPaths checks every primary, revision source, backup target,
// and adjacent advisory lock before the commit boundary creates directories or
// lock files. Backup targets may not alias their own primary, another backup, an
// artifact target/revision source, or a lock file. Identity resolution follows
// existing symlink prefixes so an in-root symlink alias cannot evade collision
// checks while pathWithinApprovedRoots separately rejects symlink escapes.
func validateArtifactPaths(artifacts []ArtifactChange, roots []string) error {
	reserved := make([]artifactPathRole, 0, len(artifacts)*4)
	backups := make([]artifactPathRole, 0, len(artifacts))
	for i := range artifacts {
		artifact := artifacts[i]
		if artifact.Path == "" {
			return fmt.Errorf("storage: artifact change requires a path")
		}
		checkPath := artifactRevisionPath(artifact)
		checks := []artifactPathRole{
			{path: artifact.Path, role: fmt.Sprintf("artifact %d target", i)},
			{path: checkPath, role: fmt.Sprintf("artifact %d revision source", i)},
			{path: artifactAdvisoryLockPath(artifact.Path), role: fmt.Sprintf("artifact %d target lock", i)},
			{path: artifactAdvisoryLockPath(checkPath), role: fmt.Sprintf("artifact %d revision lock", i)},
		}
		for _, check := range checks {
			if !pathWithinApprovedRoots(check.path, roots) {
				return fmt.Errorf("storage: %s %q: %w", check.role, check.path, transaction.ErrTargetOutsideRoot)
			}
			reserved = append(reserved, check)
		}
		if artifact.BackupPath == "" {
			continue
		}
		if !pathWithinApprovedRoots(artifact.BackupPath, roots) {
			return fmt.Errorf("storage: artifact %d backup target %q: %w", i, artifact.BackupPath, transaction.ErrTargetOutsideRoot)
		}
		backupLock := artifactAdvisoryLockPath(artifact.BackupPath)
		if !pathWithinApprovedRoots(backupLock, roots) {
			return fmt.Errorf("storage: artifact %d backup lock %q: %w", i, backupLock, transaction.ErrTargetOutsideRoot)
		}
		backups = append(backups, artifactPathRole{path: artifact.BackupPath, role: fmt.Sprintf("artifact %d backup", i)})
	}

	for i, backup := range backups {
		backupID := artifactPathIdentity(backup.path)
		for _, path := range reserved {
			if backupID != artifactPathIdentity(path.path) {
				continue
			}
			if strings.Contains(path.role, " target") {
				return fmt.Errorf("storage: %s path %q is self-referential or collides with %s %q", backup.role, backup.path, path.role, path.path)
			}
			return fmt.Errorf("storage: %s path %q collides with %s %q", backup.role, backup.path, path.role, path.path)
		}
		for j := 0; j < i; j++ {
			if backupID == artifactPathIdentity(backups[j].path) {
				return fmt.Errorf("storage: %s path %q collides with %s path %q", backup.role, backup.path, backups[j].role, backups[j].path)
			}
		}
	}
	return nil
}

// artifactPathIdentity resolves symlinks on the longest existing prefix and
// rejoins any missing suffix. It is intentionally the collision-key counterpart
// to util.PathWithinRoot's containment resolution.
func artifactPathIdentity(path string) string {
	current := filepath.Clean(path)
	var rest []string
	for {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			for i := len(rest) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, rest[i])
			}
			return filepath.Clean(resolved)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(path)
		}
		rest = append(rest, filepath.Base(current))
		current = parent
	}
}

// expandArtifactChanges converts typed artifacts into ordered journal
// operations. Callers invoke it only after revision validation while holding the
// workspace and artifact locks. A marked artifact snapshots the current primary
// into a private backup write before its primary write/delete; an absent primary
// adds no backup op. The returned directories cover every generated write.
func expandArtifactChanges(artifacts []ArtifactChange) ([]transaction.Op, []string, error) {
	ops := make([]transaction.Op, 0, len(artifacts)*2)
	dirsToCreate := make([]string, 0, len(artifacts)*2)
	for _, artifact := range artifacts {
		if artifact.BackupPath != "" {
			current, err := os.ReadFile(artifact.Path)
			switch {
			case err == nil:
				ops = append(ops, transaction.Op{
					Kind:   transaction.OpWrite,
					Target: artifact.BackupPath,
					Data:   current,
					Mode:   FilePermissions,
				})
				dirsToCreate = append(dirsToCreate, filepath.Dir(artifact.BackupPath))
			case os.IsNotExist(err):
				// First save: there is no preceding committed value to preserve.
			default:
				return nil, nil, fmt.Errorf("reading artifact %s for backup: %w", filepath.Base(artifact.Path), err)
			}
		}

		if artifact.Delete {
			ops = append(ops, transaction.Op{Kind: transaction.OpDelete, Target: artifact.Path})
			continue
		}
		mode := artifact.Mode
		if mode == 0 {
			mode = FilePermissions
		}
		ops = append(ops, transaction.Op{Kind: transaction.OpWrite, Target: artifact.Path, Data: artifact.Data, Mode: mode})
		dirsToCreate = append(dirsToCreate, filepath.Dir(artifact.Path))
	}
	return ops, dirsToCreate, nil
}

// artifactPreconditionLockPaths validates each read-only guard and returns the
// same adjacent advisory lock path an ArtifactChange revision check would use.
func artifactPreconditionLockPaths(preconditions []ArtifactPrecondition, roots []string) ([]string, error) {
	paths := make([]string, 0, len(preconditions))
	for i := range preconditions {
		path := preconditions[i].Path
		if path == "" {
			return nil, fmt.Errorf("storage: artifact precondition requires a path")
		}
		if !pathWithinApprovedRoots(path, roots) {
			return nil, fmt.Errorf("storage: artifact precondition %q: %w", path, transaction.ErrTargetOutsideRoot)
		}
		lockPath := artifactAdvisoryLockPath(path)
		if !pathWithinApprovedRoots(lockPath, roots) {
			return nil, fmt.Errorf("storage: artifact precondition lock %q: %w", lockPath, transaction.ErrTargetOutsideRoot)
		}
		paths = append(paths, lockPath)
	}
	return paths, nil
}

// pathWithinApprovedRoots applies the journal's lexical and existing-symlink
// containment rule to a path without mutating the filesystem.
func pathWithinApprovedRoots(path string, roots []string) bool {
	for _, root := range roots {
		if util.PathWithinRoot(root, path) {
			return true
		}
	}
	return false
}

// createArtifactLockDirectories creates the already-validated parent directories
// needed by WithFileLock. Callers must invoke artifactLockPaths before entering any
// mutation-capable commit boundary and pass only its returned paths here.
func createArtifactLockDirectories(lockPaths []string) error {
	for _, lockPath := range lockPaths {
		if err := os.MkdirAll(filepath.Dir(lockPath), DirPermissions); err != nil {
			return fmt.Errorf("creating artifact lock directory: %w", err)
		}
	}
	return nil
}
