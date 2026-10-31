package engine

import (
	"path/filepath"
	"slices"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/transaction"
)

func guardPlanningRestore(ledger *storage.WorkspaceLedger, dataDir string) {
	ledger.WithPlanningRestore(config.PlanningStatePath(dataDir), config.RebasePlanningRestore)
	ledger.WithPlanningRestoreGuard(func(expected, desired []byte) ([]string, func([]transaction.Op) error, error) {
		return preparePlanningRestoreGuard(dataDir, expected, desired)
	})
	if dataDir == "" {
		return
	}
	path := config.PlanningStatePath(dataDir)
	ledger.WithTerminalTaskRestoreGuard(path, func(projectIDs, taskIDs []string, proposed []byte) error {
		var planning *config.PlanningState
		var err error
		if proposed != nil {
			planning, err = config.PlanningRestoreState(proposed)
		} else {
			planning, err = config.NewDomainFileStore(dataDir).LoadPlanningReadOnly()
		}
		if err != nil {
			return err
		}
		for _, refs := range [][]config.ProjectTaskLocator{planning.Today, planning.Next} {
			for _, ref := range refs {
				if slices.Contains(projectIDs, ref.ProjectID) && slices.Contains(taskIDs, ref.TaskID) {
					return &storage.ArtifactConflictError{Path: path, Expected: "terminal task absent from Today and Next", Actual: "task has a planning membership"}
				}
			}
		}
		return nil
	})
}

// preparePlanningRestoreGuard freezes only the projects a restore would add
// memberships in. Storage locks their documents and the catalog, then calls
// the returned validator against the proposed writes, not just today's files.
func preparePlanningRestoreGuard(dataDir string, expected, desired []byte) ([]string, func([]transaction.Op) error, error) {
	additions, workspaceID, err := config.PlanningRestoreAdditions(expected, desired)
	if err != nil || len(additions.Today)+len(additions.Next) == 0 {
		return nil, nil, err
	}
	store := config.NewDomainFileStore(dataDir)
	catalog, err := store.LoadCatalogReadOnly()
	if err != nil {
		return nil, nil, err
	}
	catalogPath := config.WorkspaceCatalogPath(dataDir)
	if catalog.WorkspaceID != workspaceID {
		return nil, nil, &storage.ArtifactConflictError{Path: catalogPath, Expected: workspaceID, Actual: catalog.WorkspaceID}
	}
	refs := append(additions.Today, additions.Next...)
	projects := make(map[string]config.CatalogProject)
	paths := []string{catalogPath}
	for _, ref := range refs {
		if _, exists := projects[ref.ProjectID]; exists {
			continue
		}
		project, ok := planningRestoreProject(catalog, ref.ProjectID)
		if !ok {
			return nil, nil, &storage.ArtifactConflictError{Path: catalogPath, Expected: "planning target project present", Actual: "project missing"}
		}
		projects[ref.ProjectID] = project
		paths = append(paths, storage.TasksFilePath(project.CatalogPath(dataDir)))
	}
	validate := func(ops []transaction.Op) error {
		current, err := store.LoadCatalogReadOnly()
		if err != nil {
			return err
		}
		for id, project := range projects {
			live, exists := planningRestoreProject(current, id)
			if current.WorkspaceID != workspaceID || !exists || live.Slug != project.Slug {
				return &storage.ArtifactConflictError{Path: catalogPath, Expected: "original planning target project", Actual: "project changed"}
			}
		}
		for id, project := range projects {
			doc, err := planningRestoreDocument(project, dataDir, ops)
			if err != nil {
				return err
			}
			for _, ref := range refs {
				if ref.ProjectID != id {
					continue
				}
				eligible := false
				for _, task := range doc.Tasks {
					if task.ID == ref.TaskID {
						eligible = app.PlanningTodayEligible(task.State())
						break
					}
				}
				if !eligible {
					return &storage.ArtifactConflictError{Path: storage.TasksFilePath(project.CatalogPath(dataDir)), Expected: "planning target present and nonterminal", Actual: "task missing or terminal"}
				}
			}
		}
		return nil
	}
	return paths, validate, nil
}

func planningRestoreProject(catalog *config.WorkspaceCatalog, id string) (config.CatalogProject, bool) {
	for _, project := range catalog.Projects {
		if project.ID == id {
			return project, true
		}
	}
	return config.CatalogProject{}, false
}

func planningRestoreDocument(project config.CatalogProject, dataDir string, ops []transaction.Op) (*storage.ProjectDocument, error) {
	path := storage.TasksFilePath(project.CatalogPath(dataDir))
	for _, op := range ops {
		if filepath.Clean(op.Target) != filepath.Clean(path) {
			continue
		}
		if op.Kind == transaction.OpWrite {
			return storage.ParseProjectDocument(string(op.Data))
		}
		return &storage.ProjectDocument{}, nil
	}
	return storage.LoadProjectDocument(project.ToModel(dataDir))
}
