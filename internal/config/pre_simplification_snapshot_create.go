package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/storage"
)

func createPreSimplificationV5SnapshotWithMigrationAudit(
	dataDir string,
	migrationAudit []byte,
) (*PreSimplificationV5Snapshot, error) {
	return createPreSimplificationV5SnapshotWithAudit(dataDir, nil, migrationAudit)
}

func createPreSimplificationV5SnapshotWithAudit(
	dataDir string,
	callbacks preSimplificationV5SnapshotCallbacks,
	migrationAudit []byte,
) (*PreSimplificationV5Snapshot, error) {
	root, err := validatePreSimplificationV5Root(dataDir)
	if err != nil {
		return nil, err
	}

	createdAt := time.Now().UTC().Truncate(time.Second)
	if callbacks != nil {
		if override := callbacks.snapshotCreatedAt(); !override.IsZero() {
			createdAt = override.UTC().Truncate(time.Second)
		}
	}
	stem := preSimplificationV5SnapshotPrefix + createdAt.Format(preSimplificationV5Timestamp)
	archivePath := filepath.Join(PreSimplificationV5BackupsDir(root), stem+preSimplificationV5ArchiveSuffix)
	auditPath := filepath.Join(PreSimplificationV5BackupsDir(root), stem+preSimplificationV5AuditSuffix)

	if precheckErr := precheckPreSimplificationV5Targets(archivePath, auditPath); precheckErr != nil {
		return nil, precheckErr
	}
	// Validate the complete source shape before taking the workspace lock. Common
	// failures such as an unsafe symlink therefore leave neither a backup
	// artifact nor a transaction marker behind. The locked planner inventories
	// again and remains authoritative.
	if _, inventoryErr := inventoryPreSimplificationV5(root, false); inventoryErr != nil {
		return nil, inventoryErr
	}

	var result *PreSimplificationV5Snapshot
	commit := storage.NewCommitService(TransactionsDir(root), []string{root})
	_, err = commit.CommitPlannedArtifacts(func() ([]storage.ArtifactChange, error) {
		if precheckErr := precheckPreSimplificationV5Targets(archivePath, auditPath); precheckErr != nil {
			return nil, precheckErr
		}
		inventory, inventoryErr := inventoryPreSimplificationV5(root, true)
		if inventoryErr != nil {
			return nil, inventoryErr
		}
		if callbacks != nil {
			callbacks.didInventorySnapshot()
		}
		current, inventoryErr := inventoryPreSimplificationV5(root, false)
		if inventoryErr != nil {
			return nil, inventoryErr
		}
		if compareErr := comparePreSimplificationV5Inventories(inventory, current); compareErr != nil {
			return nil, compareErr
		}

		audit := renderPreSimplificationV5Audit(createdAt, inventory)
		audit = append(audit, migrationAudit...)
		archive, archiveErr := buildPreSimplificationV5Archive(inventory.archived, audit)
		if archiveErr != nil {
			return nil, archiveErr
		}
		archiveSum := sha256.Sum256(archive)
		absent := ""
		result = &PreSimplificationV5Snapshot{
			CreatedAt:     createdAt,
			ArchivePath:   archivePath,
			AuditPath:     auditPath,
			ArchiveSHA256: hex.EncodeToString(archiveSum[:]),
			ArchivedFiles: publicPreSimplificationV5Files(inventory.archived),
			Attachments:   publicPreSimplificationV5Files(inventory.attachments),
		}
		if callbacks != nil {
			callbacks.willPublishSnapshot(archivePath, auditPath)
		}
		return []storage.ArtifactChange{
			{
				Path:             archivePath,
				Data:             archive,
				Mode:             preSimplificationV5Permissions,
				ExpectedRevision: &absent,
			},
			{
				Path:             auditPath,
				Data:             audit,
				Mode:             preSimplificationV5Permissions,
				ExpectedRevision: &absent,
			},
		}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("pre-simplification v5 snapshot failed: %w", err)
	}
	return result, nil
}

func validatePreSimplificationV5Root(dataDir string) (string, error) {
	if strings.TrimSpace(dataDir) == "" {
		return "", fmt.Errorf("pre-simplification v5 snapshot requires a workspace data dir")
	}
	root, err := filepath.Abs(filepath.Clean(dataDir))
	if err != nil {
		return "", fmt.Errorf("resolve workspace data dir: %w", err)
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", fmt.Errorf("inspect workspace data dir %s: %w", root, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("pre-simplification v5 snapshot refuses a symlink workspace root: %s", root)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("pre-simplification v5 snapshot workspace root is not a directory: %s", root)
	}
	return root, nil
}

func precheckPreSimplificationV5Targets(paths ...string) error {
	for _, path := range paths {
		info, err := os.Lstat(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			continue
		case err != nil:
			return fmt.Errorf("inspect snapshot destination %s: %w", path, err)
		case info.Mode()&os.ModeSymlink != 0:
			return fmt.Errorf("pre-simplification v5 snapshot destination is a symlink, refusing to replace: %s", path)
		default:
			return fmt.Errorf("pre-simplification v5 snapshot destination already exists, refusing to overwrite: %s", path)
		}
	}
	return nil
}
