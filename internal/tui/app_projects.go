package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/transaction"
)

// syncProjectCache reconciles catalog-owned metadata from RuntimeState while
// preserving each already-loaded project's task tree and other storage-owned
// fields. RuntimeState catalog reads intentionally return fresh pointers, but
// long-lived views need stable project identities within one TUI session.
func (m *AppModel) syncProjectCache() {
	if m == nil || m.runtime == nil {
		return
	}
	if m.projects == nil {
		m.projects = make(map[string]*models.Project)
	}

	seen := make(map[string]bool)
	for _, fresh := range m.runtime.AllProjects() {
		if fresh == nil {
			continue
		}
		seen[fresh.ID] = true
		if cached := m.projects[fresh.ID]; cached != nil {
			copyCatalogMetadata(cached, fresh)
			continue
		}
		m.projects[fresh.ID] = fresh
	}
	for id := range m.projects {
		if !seen[id] {
			delete(m.projects, id)
		}
	}

	if m.currentProject != nil {
		if cached := m.projects[m.currentProject.ID]; cached != nil {
			m.currentProject = cached
		} else {
			m.currentProject = nil
		}
	}
}

// adoptCrossProjectSnapshot installs the task trees loaded by the global
// Outliner into the shell's stable project cache and rewires the snapshot to
// those stable pointers. Catalog snapshots intentionally contain metadata
// only, so operations resolved through projectByID must adopt the disk-loaded
// task trees before the Outliner exposes their rows.
func (m *AppModel) adoptCrossProjectSnapshot(projects []*models.Project) {
	if m == nil {
		return
	}
	if m.projects == nil {
		m.projects = make(map[string]*models.Project)
	}
	for index, loaded := range projects {
		if loaded == nil || loaded.ID == "" {
			continue
		}
		cached := m.projects[loaded.ID]
		if cached == nil {
			m.projects[loaded.ID] = loaded
			cached = loaded
		} else if cached != loaded {
			*cached = *loaded
		}
		projects[index] = cached
		if m.currentProject != nil && m.currentProject.ID == cached.ID {
			m.currentProject = cached
		}
	}
}

func copyCatalogMetadata(dst, src *models.Project) {
	if dst == nil || src == nil {
		return
	}
	dst.Slug = src.Slug
	dst.Name = src.Name
	dst.Description = src.Description
	dst.Context = src.Context
	dst.Path = src.Path
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
	dst.Archived = src.Archived
	dst.ArchivedAt = src.ArchivedAt
	dst.Pinned = src.Pinned
	dst.PinnedAt = src.PinnedAt
	dst.ColorSlot = src.ColorSlot
}

// ensureProjectColorSlots persists legacy catalog backfill once during startup.
// A failed checked commit leaves the runtime snapshot untouched so startup can
// continue with the muted fallback and retry on the next catalog mutation.
func (m *AppModel) ensureProjectColorSlots() error {
	if m == nil || m.runtime == nil {
		return nil
	}
	catalog := cloneWorkspaceCatalog(m.runtime.Catalog())
	if !catalog.EnsureProjectColorSlots() {
		return nil
	}
	if m.runtime.DataDir() == "" {
		m.runtime.SetCatalogInMemory(catalog)
		m.syncProjectCache()
		return nil
	}
	if err := m.runtime.CommitCatalog(catalog); err != nil {
		return err
	}
	m.syncProjectCache()
	return nil
}

func (m *AppModel) projectByID(idOrSlug string) *models.Project {
	if m == nil || idOrSlug == "" {
		return nil
	}
	for _, project := range m.projects {
		if project != nil && (project.ID == idOrSlug || project.Slug == idOrSlug) {
			return project
		}
	}
	return nil
}

func (m *AppModel) activeProjectByID(projectID string) *models.Project {
	if m == nil || projectID == "" {
		return nil
	}
	project := m.projects[projectID]
	if project == nil || project.Archived {
		return nil
	}
	return project
}

func (m *AppModel) projectsInRuntimeOrder(projects []*models.Project) []*models.Project {
	result := make([]*models.Project, 0, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		if cached := m.projects[project.ID]; cached != nil {
			result = append(result, cached)
		}
	}
	return result
}

func (m *AppModel) activeProjects() []*models.Project {
	if m == nil || m.runtime == nil {
		return nil
	}
	return m.projectsInRuntimeOrder(m.runtime.ActiveProjects())
}

func (m *AppModel) allLoadableProjects() []*models.Project {
	if m == nil || m.runtime == nil {
		return nil
	}
	return m.projectsInRuntimeOrder(m.runtime.AllLoadableProjects())
}

// ensureSystemInboxProject installs (or upgrades) the workspace-owned Inbox.
// It is a regular visible project for every read surface; SystemInboxProjectID
// is the stable identity that distinguishes it from user-created projects.
//
//nolint:gocyclo // Bootstrap keeps catalog and filesystem recovery decisions in one transaction flow.
func (m *AppModel) ensureSystemInboxProject() error {
	if m == nil || m.runtime == nil || m.runtime.DataDir() == "" {
		return nil
	}

	project := m.runtime.SystemInboxProject()
	catalogChanged := project == nil
	if project == nil {
		for _, candidate := range m.runtime.AllProjects() {
			if candidate != nil && candidate.Slug == config.LegacyActionInboxSlug {
				project = candidate
				break
			}
		}
	}
	if project == nil {
		now := time.Now()
		project = &models.Project{
			ID:          "system-inbox-" + uuid.New().String(),
			Slug:        config.SystemInboxSlug,
			Name:        config.SystemInboxName,
			Context:     m.runtime.DefaultContext(),
			Path:        m.runtime.ProjectPath(config.SystemInboxSlug),
			CreatedAt:   now,
			UpdatedAt:   now,
			Tasks:       []*models.Task{},
			NextShortID: 1,
		}
		if project.Context == "" {
			project.Context = "personal"
		}
	}

	if project.Name != config.SystemInboxName || project.Archived || project.ArchivedAt != nil ||
		m.runtime.Catalog().SystemInboxProjectID != project.ID {
		catalogChanged = true
		project.Name = config.SystemInboxName
		project.Archived = false
		project.ArchivedAt = nil
		project.UpdatedAt = time.Now()
	}

	tasksMissing := false
	if _, err := os.Stat(storage.TasksFilePath(project.Path)); os.IsNotExist(err) {
		tasksMissing = true
	} else if err != nil {
		return fmt.Errorf("inspect system Inbox tasks: %w", err)
	}
	var ops []transaction.Op
	legacyPath := filepath.Join(config.InternalDir(m.runtime.DataDir()), config.LegacyActionInboxSlug)
	if tasksMissing && project.Slug == config.LegacyActionInboxSlug {
		if _, err := os.Stat(legacyPath); err == nil {
			ops = append(ops, transaction.Op{Kind: transaction.OpMove, Source: legacyPath, Target: project.Path})
			tasksMissing = false
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect legacy Action Inbox: %w", err)
		}
	}
	if !catalogChanged && !tasksMissing && len(ops) == 0 {
		return nil
	}

	// A catalog commit refreshes every runtime domain. Preserve the caller's
	// machine-local in-memory state as well: the headless fresh-install harness
	// intentionally does not persist machine.json, while a real workspace simply
	// restores the same loaded snapshot.
	machine := cloneMachineState(m.runtime.Machine())
	artifacts := make([]storage.ArtifactChange, 0, 2)
	if catalogChanged {
		catalog := cloneWorkspaceCatalog(m.runtime.Catalog())
		if err := catalog.RepairSystemInboxProject(project); err != nil {
			return err
		}
		artifact, err := m.runtime.CheckedCatalogArtifact(catalog)
		if err != nil {
			return err
		}
		artifacts = append(artifacts, artifact.StorageChange())
	}
	if tasksMissing {
		doc, err := storage.CanonicalProjectDocument(project, nil)
		if err != nil {
			return err
		}
		content, err := doc.WriteCurrent()
		if err != nil {
			return err
		}
		requireAbsent := ""
		artifacts = append(artifacts, storage.ArtifactChange{
			Path: storage.TasksFilePath(project.Path), Data: []byte(content), ExpectedRevision: &requireAbsent,
		})
	}
	if err := m.appService.CommitWorkspaceBootstrap(ops, artifacts...); err != nil {
		return err
	}
	if err := m.runtime.RefreshAfterCommit(); err != nil {
		return err
	}
	m.runtime.SetMachineInMemory(machine)
	m.syncProjectCache()
	return nil
}
