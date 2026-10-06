package cmd

import (
	"fmt"
	"os"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

func checkWorkspaceHealth(workspace *CLIWorkspace, results *doctorResults) {
	if workspace == nil || workspace.Runtime == nil {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
			project: "Workspace",
			detail:  "runtime state is unavailable",
		})
		return
	}

	runtime := workspace.Runtime
	catalog := runtime.Catalog()
	if catalog == nil {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
			project: "Workspace",
			detail:  "workspace catalog is unavailable",
		})
		return
	}

	schema := catalog.SchemaVersion
	switch {
	case schema == config.CurrentConfigVersion:
		results.workspaceNotes = append(results.workspaceNotes, doctorIssue{
			project: "Migration",
			detail:  fmt.Sprintf("workspace schema %d is current", schema),
		})
	case schema == config.SyncEnrolledConfigVersion && catalog.Enrolled():
		results.workspaceNotes = append(results.workspaceNotes, doctorIssue{
			project: "Migration",
			detail:  fmt.Sprintf("workspace schema %d is current for a workspace enrolled in sync", schema),
		})
	case schema < config.CurrentConfigVersion:
		results.workspaceNotes = append(results.workspaceNotes, doctorIssue{
			project: "Migration",
			detail:  fmt.Sprintf("workspace schema %d is pre-v%d; startup migration remains pending", schema, config.CurrentConfigVersion),
		})
	default:
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
			project: "Migration",
			detail:  fmt.Sprintf("workspace schema %d is newer than supported schema %d", schema, config.SupportedConfigVersion),
		})
	}

	checkRetainedDomainArtifacts(runtime.DataDir(), schema, results)
	checkProtectedUnfiled(catalog, results)
	checkWorkspaceUndoLedger(runtime.DataDir(), results)
	// The synced-state buckets run in one fixed order: the portable lens home,
	// the transaction journal and its quarantine, conflict bundles, lock files,
	// and last the migration snapshots. Each one reads a different durable
	// artifact of the same synced tree, and the order is the order a human should
	// read the report in — what refuses a view, what refuses a mutation, what
	// needs resolving, what is merely present, then the rollback history.
	checkLensDefinitions(runtime, schema, results)
	checkTransactionJournal(runtime, results)
	checkConflictBundles(runtime, results)
	checkLockFiles(runtime, runtime.AllProjects(), results)
	checkMigrationSnapshots(runtime.DataDir(), schema, results)
}

func checkRetainedDomainArtifacts(dataDir string, schema int, results *doctorResults) {
	required := []struct {
		name string
		path string
	}{
		{name: "workspace", path: config.WorkspaceCatalogPath(dataDir)},
		{name: "planning", path: config.PlanningStatePath(dataDir)},
	}
	if schema < config.V6ConfigSchemaVersion {
		// A legacy v4 workspace owns both retired domains. During the staged
		// runtime cutover, newly bootstrapped workspaces own neither even though
		// activation has not yet raised their catalog schema. Treat one surviving
		// retired domain as a partial legacy bundle and require its peer.
		settingsPath := config.SettingsPath(dataDir)
		automationPath := config.AutomationPath(dataDir)
		_, settingsErr := os.Lstat(settingsPath)
		_, automationErr := os.Lstat(automationPath)
		if !os.IsNotExist(settingsErr) || !os.IsNotExist(automationErr) {
			required = append(required,
				struct {
					name string
					path string
				}{name: "settings", path: settingsPath},
				struct {
					name string
					path string
				}{name: "automation", path: automationPath},
			)
		}
	}
	for _, domain := range required {
		info, err := os.Lstat(domain.path)
		switch {
		case os.IsNotExist(err):
			results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
				project: "Domains",
				detail:  fmt.Sprintf("retained %s domain is missing: %s", domain.name, domain.path),
			})
		case err != nil:
			results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
				project: "Domains",
				detail:  fmt.Sprintf("cannot inspect retained %s domain %s: %v", domain.name, domain.path, err),
			})
		case !info.Mode().IsRegular():
			results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
				project: "Domains",
				detail:  fmt.Sprintf("retained %s domain is not a regular file: %s", domain.name, domain.path),
			})
		}
	}

	if schema >= config.V6ConfigSchemaVersion {
		for _, retired := range []struct {
			name string
			path string
		}{
			{name: "settings", path: config.SettingsPath(dataDir)},
			{name: "automation", path: config.AutomationPath(dataDir)},
		} {
			if _, err := os.Lstat(retired.path); err == nil {
				results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
					project: "Domains",
					detail:  fmt.Sprintf("retired %s domain still exists: %s", retired.name, retired.path),
				})
			} else if !os.IsNotExist(err) {
				results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
					project: "Domains",
					detail:  fmt.Sprintf("cannot inspect retired %s domain %s: %v", retired.name, retired.path, err),
				})
			}
		}
	}
}

func checkProtectedUnfiled(catalog *config.WorkspaceCatalog, results *doctorResults) {
	unfiledID := catalog.SystemUnfiledProjectID
	if unfiledID == "" {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
			project: "Unfiled",
			detail:  "protected system Unfiled ID is missing",
		})
		return
	}
	for _, project := range catalog.Projects {
		if project.ID != unfiledID {
			continue
		}
		if project.Name != config.SystemUnfiledName || project.Slug != config.SystemUnfiledSlug {
			results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
				project: "Unfiled",
				detail: fmt.Sprintf(
					"protected project %q has identity %q/%q; want %q/%q",
					unfiledID,
					project.Name,
					project.Slug,
					config.SystemUnfiledName,
					config.SystemUnfiledSlug,
				),
			})
		}
		if project.Archived || project.ArchivedAt != nil {
			results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
				project: "Unfiled",
				detail:  fmt.Sprintf("protected project %q is archived", unfiledID),
			})
		}
		return
	}
	results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
		project: "Unfiled",
		detail:  fmt.Sprintf("protected system Unfiled references unknown project %q", unfiledID),
	})
}

func checkWorkspaceUndoLedger(dataDir string, results *doctorResults) {
	ledger := storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), []string{dataDir})
	status, err := ledger.Validate()
	if err != nil {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
			project: "Undo",
			detail:  err.Error(),
		})
		return
	}
	results.workspaceNotes = append(results.workspaceNotes, doctorIssue{
		project: "Undo",
		detail: fmt.Sprintf(
			"workspace ledger is valid (%s, %s)",
			Pluralize(status.UndoEntries, "undo entry", "undo entries"),
			Pluralize(status.RedoEntries, "redo entry", "redo entries"),
		),
	})
}
