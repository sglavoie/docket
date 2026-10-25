package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/sglavoie/docket/internal/storage"
)

//nolint:govet // Scoped errors keep each snapshot validation and transaction stage local.
func createPreConsolidationV6Snapshot(
	options PreConsolidationV6SnapshotOptions,
	callbacks preConsolidationV6SnapshotCallbacks,
) (*PreConsolidationV6Snapshot, error) {
	root, err := validateV5DryRunRoot(options.DataDir)
	if err != nil {
		return nil, err
	}
	options.DataDir = root
	dryRunOptions := V6WorkspaceDryRunOptions{
		DataDir: root, MachineStatePath: options.MachineStatePath,
	}

	// This validation happens before the commit service can create a lock or
	// transaction artifact. Invalid candidates therefore leave no snapshot state.
	validatedPlan, err := DryRunV6WorkspaceConversion(dryRunOptions)
	if err != nil {
		return nil, fmt.Errorf("pre-consolidation v6 candidate validation failed; workspace was not changed: %w", err)
	}
	if err := ValidateV6WorkspaceConversionPlan(validatedPlan); err != nil {
		return nil, fmt.Errorf("pre-consolidation v6 candidate validation failed; workspace was not changed: %w", err)
	}

	createdAt := time.Now().UTC().Truncate(time.Second)
	if callbacks != nil {
		if override := callbacks.snapshotCreatedAt(); !override.IsZero() {
			createdAt = override.UTC().Truncate(time.Second)
		}
	}
	stem := preConsolidationV6SnapshotPrefix + createdAt.Format(preConsolidationV6Timestamp)
	archivePath := filepath.Join(PreConsolidationV6BackupsDir(root), stem+preConsolidationV6ArchiveSuffix)
	auditPath := filepath.Join(PreConsolidationV6BackupsDir(root), stem+preConsolidationV6AuditSuffix)
	if err := precheckPreConsolidationV6Targets(archivePath, auditPath); err != nil {
		return nil, err
	}
	machinePath, err := preConsolidationV6MachineSource(validatedPlan)
	if err != nil {
		return nil, err
	}
	// Validate the complete snapshot topology before taking the workspace lock.
	// The locked inventory below remains authoritative.
	if _, err := inventoryPreConsolidationV6(root, machinePath, false); err != nil {
		return nil, err
	}

	var result *PreConsolidationV6Snapshot
	commit := storage.NewCommitService(TransactionsDir(root), []string{root})
	_, err = commit.CommitPlannedArtifacts(func() ([]storage.ArtifactChange, error) {
		if err := precheckPreConsolidationV6Targets(archivePath, auditPath); err != nil {
			return nil, err
		}
		// Recovery, if any, has completed under the workspace lock. Rebuild the
		// candidate from that exact coherent source state before capturing bytes.
		plan, err := DryRunV6WorkspaceConversion(dryRunOptions)
		if err != nil {
			return nil, fmt.Errorf("pre-consolidation v6 candidate changed before snapshot publication: %w", err)
		}
		if err := ValidateV6WorkspaceConversionPlan(plan); err != nil {
			return nil, err
		}
		machinePath, err := preConsolidationV6MachineSource(plan)
		if err != nil {
			return nil, err
		}
		inventory, err := inventoryPreConsolidationV6(root, machinePath, true)
		if err != nil {
			return nil, err
		}
		if callbacks != nil {
			callbacks.didInventorySnapshot()
		}
		confirmedPlan, err := DryRunV6WorkspaceConversion(dryRunOptions)
		if err != nil {
			return nil, fmt.Errorf("pre-consolidation v6 candidate changed while snapshotting: %w", err)
		}
		if !reflect.DeepEqual(plan, confirmedPlan) {
			return nil, fmt.Errorf("pre-consolidation v6 candidate changed while snapshotting")
		}
		current, err := inventoryPreConsolidationV6(root, machinePath, false)
		if err != nil {
			return nil, err
		}
		if err := comparePreConsolidationV6Inventories(inventory, current); err != nil {
			return nil, err
		}
		facts, err := collectPreConsolidationV6TaskFacts(plan, inventory)
		if err != nil {
			return nil, err
		}
		audit := renderPreConsolidationV6Audit(createdAt, plan, inventory, facts)
		archive, err := buildPreConsolidationV6Archive(inventory.files, audit)
		if err != nil {
			return nil, err
		}
		if callbacks != nil {
			if err := callbacks.willPublishSnapshot(archivePath, auditPath); err != nil {
				return nil, err
			}
		}
		archiveSum := sha256.Sum256(archive)
		absent := ""
		result = &PreConsolidationV6Snapshot{
			CreatedAt: createdAt, ArchivePath: archivePath, AuditPath: auditPath,
			ArchiveSHA256: hex.EncodeToString(archiveSum[:]),
			Files:         publicPreConsolidationV6Files(inventory.files),
		}
		return []storage.ArtifactChange{
			{Path: archivePath, Data: archive, Mode: preConsolidationV6Permissions, ExpectedRevision: &absent},
			{Path: auditPath, Data: audit, Mode: preConsolidationV6Permissions, ExpectedRevision: &absent},
		}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("pre-consolidation v6 snapshot failed: %w", err)
	}
	return result, nil
}

func precheckPreConsolidationV6Targets(paths ...string) error {
	for _, path := range paths {
		info, err := os.Lstat(path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			return fmt.Errorf("inspect pre-consolidation v6 snapshot destination %s: %w", path, err)
		case info.Mode()&os.ModeSymlink != 0:
			return fmt.Errorf("pre-consolidation v6 snapshot destination is a symlink, refusing to replace: %s", path)
		default:
			return fmt.Errorf("pre-consolidation v6 snapshot destination already exists, refusing to overwrite: %s", path)
		}
	}
	return nil
}

func preConsolidationV6MachineSource(plan V6WorkspaceConversionPlan) (string, error) {
	for _, artifact := range plan.Artifacts {
		if artifact.Kind == "machine" {
			return filepath.Clean(artifact.SourcePath), nil
		}
	}
	return "", fmt.Errorf("validated schema-6 candidate has no machine-state artifact")
}
