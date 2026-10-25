package config

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/storage/format"
)

//nolint:gocyclo // The activation plan validates and assembles one indivisible migration transaction.
func buildV5MigrationActivationPlan(dataDir string, location *time.Location) (v5MigrationActivationPlan, error) {
	if location == nil {
		return v5MigrationActivationPlan{}, fmt.Errorf("v5 migration requires a local timezone")
	}
	domainPlan, err := DryRunV5WorkspaceConversion(V5WorkspaceDryRunOptions{DataDir: dataDir})
	if err != nil {
		return v5MigrationActivationPlan{}, err
	}

	reintegrationInput := storage.V5TaskReintegrationInput{
		Projects: make([]storage.V5ProjectTaskReintegrationInput, 0, len(domainPlan.Domains.Domains.Catalog.Projects)),
	}
	var blockerPaths []string
	var taskAudit []v5MigrationTaskAuditRow
	for _, project := range domainPlan.Domains.Domains.Catalog.Projects {
		projectDir := filepath.Join(dataDir, "projects", project.Slug)
		activePath := storage.TasksFilePath(projectDir)
		active, conversionErr := convertLegacyV4TaskFile(activePath, location)
		if conversionErr != nil {
			return v5MigrationActivationPlan{}, fmt.Errorf("convert active tasks for project %q: %w", project.ID, conversionErr)
		}
		if active.ProjectName != project.Name {
			return v5MigrationActivationPlan{}, fmt.Errorf(
				"project %q task document name %q does not match catalog name %q",
				project.ID,
				active.ProjectName,
				project.Name,
			)
		}
		activeSourceID, relErr := filepath.Rel(dataDir, activePath)
		if relErr != nil {
			return v5MigrationActivationPlan{}, relErr
		}
		taskAudit = appendV5MigrationTaskAudit(
			taskAudit,
			project.ID,
			filepath.ToSlash(activeSourceID),
			active.Audit,
		)

		blockersPath := filepath.Join(projectDir, legacyV4BlockersFileName)
		if blockersData, found, readErr := readOptionalV5MigrationFile(blockersPath); readErr != nil {
			return v5MigrationActivationPlan{}, readErr
		} else if found {
			blockers, decodeErr := storage.DecodeLegacyV4Blockers(string(blockersData))
			if decodeErr != nil {
				return v5MigrationActivationPlan{}, fmt.Errorf("decode project %q blockers: %w", project.ID, decodeErr)
			}
			active.Tasks = append(active.Tasks, storage.ConvertLegacyV4Blockers(blockers)...)
			blockerSourceID, relErr := filepath.Rel(dataDir, blockersPath)
			if relErr != nil {
				return v5MigrationActivationPlan{}, relErr
			}
			for _, blocker := range blockers {
				entry := format.V5TaskMigrationAuditEntry{
					TaskID: blocker.ID,
					Field:  "standalone_blocker",
				}
				if blocker.ResolvedAt == nil {
					entry.Action = "converted"
					entry.From = "unresolved blocker"
					entry.To = "Inbox task + tag:blocker"
					entry.Reason = "v5 represents unresolved blockers as ordinary tasks"
				} else {
					entry.Action = "snapshot-only"
					entry.From = "resolved blocker"
					entry.Reason = "v5 has no standalone blocker storage"
				}
				taskAudit = append(taskAudit, v5MigrationTaskAuditRow{
					ProjectID: project.ID,
					SourceID:  filepath.ToSlash(blockerSourceID),
					Entry:     entry,
				})
			}
			blockerPaths = append(blockerPaths, blockersPath)
		}

		archives, indexPath, archiveErr := legacyV4ProjectArchives(dataDir, projectDir, location)
		if archiveErr != nil {
			return v5MigrationActivationPlan{}, fmt.Errorf("read project %q archives: %w", project.ID, archiveErr)
		}
		for _, archive := range archives {
			taskAudit = appendV5MigrationTaskAudit(
				taskAudit,
				project.ID,
				archive.SourceID,
				archive.Conversion.Audit,
			)
		}
		reintegrationInput.Projects = append(reintegrationInput.Projects, storage.V5ProjectTaskReintegrationInput{
			ProjectID:        project.ID,
			Active:           active,
			Archives:         archives,
			ArchiveIndexPath: indexPath,
		})
	}

	reintegration, err := storage.PlanV5ArchiveReintegration(reintegrationInput)
	if err != nil {
		return v5MigrationActivationPlan{}, err
	}
	if reconcileErr := reconcileV5MigrationPlanning(&domainPlan.Domains.Domains, reintegration.Projects); reconcileErr != nil {
		return v5MigrationActivationPlan{}, reconcileErr
	}

	planningBytes, err := encodeV5DomainEnvelope(
		domainPlanning.name(),
		domainPlan.Domains.Domains.Catalog.WorkspaceID,
		domainPlan.Domains.Domains.Planning,
	)
	if err != nil {
		return v5MigrationActivationPlan{}, err
	}

	builder := newV5ActivationBuilder(dataDir)
	for _, candidate := range domainPlan.Artifacts {
		data := candidate.Data
		if candidate.Kind == domainPlanning.name() {
			data = planningBytes
		}
		var addErr error
		if candidate.Kind == domainMachine.name() {
			addErr = builder.addMachineWrite(candidate.Path, data)
		} else {
			addErr = builder.addWrite(candidate.Path, data)
		}
		if addErr != nil {
			return v5MigrationActivationPlan{}, addErr
		}
	}

	projectByID := make(map[string]storage.V5ProjectTaskCandidate, len(reintegration.Projects))
	for _, project := range reintegration.Projects {
		projectByID[project.ProjectID] = project
	}
	for _, project := range domainPlan.Domains.Domains.Catalog.Projects {
		candidate, ok := projectByID[project.ID]
		if !ok {
			return v5MigrationActivationPlan{}, fmt.Errorf("v5 migration produced no task document for project %q", project.ID)
		}
		content, encodeErr := storage.WriteV5ProjectDocument(storage.V5ProjectDocument{
			ProjectName: candidate.ProjectName,
			Tasks:       candidate.Tasks,
		})
		if encodeErr != nil {
			return v5MigrationActivationPlan{}, encodeErr
		}
		if _, parseErr := storage.ParseV5ProjectDocument(content); parseErr != nil {
			return v5MigrationActivationPlan{}, fmt.Errorf("validate v5 project %q output: %w", project.ID, parseErr)
		}
		if addErr := builder.addWrite(
			storage.TasksFilePath(filepath.Join(dataDir, "projects", project.Slug)),
			[]byte(content),
		); addErr != nil {
			return v5MigrationActivationPlan{}, addErr
		}
	}

	for _, cleanup := range reintegration.Cleanup {
		if deleteErr := builder.addFileDelete(cleanup.Path); deleteErr != nil {
			return v5MigrationActivationPlan{}, deleteErr
		}
	}
	for _, path := range blockerPaths {
		if deleteErr := builder.addFileDelete(path); deleteErr != nil {
			return v5MigrationActivationPlan{}, deleteErr
		}
	}
	for _, path := range domainPlan.RetiredDomainPaths {
		if deleteErr := builder.addOptionalFileDelete(path); deleteErr != nil {
			return v5MigrationActivationPlan{}, deleteErr
		}
		if deleteErr := builder.addOptionalFileDelete(path + ".bak"); deleteErr != nil {
			return v5MigrationActivationPlan{}, deleteErr
		}
	}
	for _, dir := range domainPlan.SnapshotOnlyProjectDirs {
		if cleanupErr := builder.addCleanupTree(dir); cleanupErr != nil {
			return v5MigrationActivationPlan{}, cleanupErr
		}
	}
	if cleanupErr := builder.addCleanupTree(filepath.Join(InternalDir(dataDir), "trash")); cleanupErr != nil {
		return v5MigrationActivationPlan{}, cleanupErr
	}
	for _, dir := range []string{
		filepath.Join(InternalDir(dataDir), "themes"),
		filepath.Join(dataDir, "themes"),
	} {
		if cleanupErr := builder.addCleanupTree(dir); cleanupErr != nil {
			return v5MigrationActivationPlan{}, cleanupErr
		}
	}
	for _, project := range domainPlan.Domains.Domains.Catalog.Projects {
		if cleanupErr := builder.addCleanupTree(
			filepath.Join(dataDir, "projects", project.Slug, legacyV4ProjectUndoDirName),
		); cleanupErr != nil {
			return v5MigrationActivationPlan{}, cleanupErr
		}
		if deleteErr := builder.addOptionalFileDelete(
			filepath.Join(dataDir, "projects", project.Slug, legacyV4ProjectBackupName),
		); deleteErr != nil {
			return v5MigrationActivationPlan{}, deleteErr
		}
	}

	plan, err := builder.finish()
	if err != nil {
		return v5MigrationActivationPlan{}, err
	}
	plan.audit = renderV5MigrationConversionAudit(
		domainPlan.Domains.Audit,
		domainPlan.NoteAudit,
		taskAudit,
		reintegration.Report,
	)
	return plan, nil
}
