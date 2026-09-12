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
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// syncProjectCache reconciles catalog-owned metadata from RuntimeState while
// preserving each already-loaded project's task tree and other storage-owned
// fields. RuntimeState catalog reads intentionally return fresh pointers, but
// long-lived views need stable project identities within one TUI session.
func (m *AppModel) syncProjectCache() {
	if m == nil || m.domain.runtime == nil {
		return
	}
	if m.domain.projects == nil {
		m.domain.projects = make(map[string]*models.Project)
	}

	seen := make(map[string]bool)
	for _, fresh := range m.domain.runtime.AllProjects() {
		if fresh == nil {
			continue
		}
		seen[fresh.ID] = true
		if cached := m.domain.projects[fresh.ID]; cached != nil {
			coordinator.CopyCatalogMetadata(cached, fresh)
			continue
		}
		m.domain.projects[fresh.ID] = fresh
	}
	for id := range m.domain.projects {
		if !seen[id] {
			delete(m.domain.projects, id)
		}
	}

	if m.currentProject != nil {
		if cached := m.domain.projects[m.currentProject.ID]; cached != nil {
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
//
// This is the one place an externally loaded task population becomes what the
// shell reads, for both the cross-project load and a published lens, so it is
// also where remembered deadline clocks are reconciled against it: a deadline
// another machine wrote arrives through no committed effect of ours, and the
// clock it superseded has to retire here rather than wait for a local gesture
// to spend it. Every caller is already gated on the snapshot being accepted, so
// a superseded load observes nothing.
func (m *AppModel) adoptCrossProjectSnapshot(projects []*models.Project) {
	if m == nil {
		return
	}
	if m.domain.projects == nil {
		m.domain.projects = make(map[string]*models.Project)
	}
	for index, loaded := range projects {
		if loaded == nil || loaded.ID == "" {
			continue
		}
		cached := m.domain.projects[loaded.ID]
		if cached == nil {
			m.domain.projects[loaded.ID] = loaded
			cached = loaded
		} else if cached != loaded {
			*cached = *loaded
		}
		projects[index] = cached
		if m.currentProject != nil && m.currentProject.ID == cached.ID {
			m.currentProject = cached
		}
	}
	// Reconcile after the rewiring, never before it: the clocks are checked
	// against the tasks projectByID now answers with, which is the snapshot that
	// was just adopted rather than the one it replaced.
	m.reconcileRememberedDueClocks()
}

// parkedOwnerLabel names a project as parked when it carries the reserved tag,
// and returns "" otherwise. A task written to a parked project is committed and
// durable but will not appear in any base projection, so a status that named
// only its section would send the user looking for it in a place it can never
// render. The label matches the Outliner's focus chrome, so the same words
// identify the project when the user goes there to find it.
func (m *AppModel) parkedOwnerLabel(project *models.Project) string {
	if m == nil || project == nil || !models.ProjectTagsParked(project.Tags) {
		return ""
	}
	return project.Name + outliner.ScopeChromeSeparator + models.ParkedProjectTag
}

// ensureProjectColorSlots persists legacy catalog backfill once during startup.
// A failed checked commit leaves the runtime snapshot untouched so startup can
// continue with the muted fallback and retry on the next catalog mutation.
func (m *AppModel) ensureProjectColorSlots() error {
	if m == nil || m.domain.runtime == nil {
		return nil
	}
	catalog := coordinator.CloneWorkspaceCatalog(m.domain.runtime.Catalog())
	if !catalog.EnsureProjectColorSlots() {
		return nil
	}
	if m.domain.runtime.DataDir() == "" {
		m.domain.runtime.SetCatalogInMemory(catalog)
		m.syncProjectCache()
		return nil
	}
	if err := m.domain.runtime.CommitCatalog(catalog); err != nil {
		return err
	}
	m.syncProjectCache()
	return nil
}

func (m *AppModel) projectByID(idOrSlug string) *models.Project {
	if m == nil || idOrSlug == "" {
		return nil
	}
	for _, project := range m.domain.projects {
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
	project := m.domain.projects[projectID]
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
		if cached := m.domain.projects[project.ID]; cached != nil {
			result = append(result, cached)
		}
	}
	return result
}

// taskWorkspaceProjects is the base task workspace: the non-archived projects
// that do not carry the reserved parked tag, in runtime order. Every task
// projection and context scope
// reads it. A surface that must keep admitting a parked project — Notes owners,
// capture destinations, move targets, project administration — reads
// availableProjects instead.
func (m *AppModel) taskWorkspaceProjects() []*models.Project {
	if m == nil || m.domain.runtime == nil {
		return nil
	}
	return m.projectsInRuntimeOrder(m.domain.runtime.TaskWorkspaceProjects())
}

func (m *AppModel) availableProjects() []*models.Project {
	if m == nil || m.domain.runtime == nil {
		return nil
	}
	return m.projectsInRuntimeOrder(m.domain.runtime.AvailableProjects())
}

// ensureSystemUnfiledProject installs the workspace-owned Unfiled project.
// It is a regular visible project for every read surface; SystemUnfiledProjectID
// is the stable identity that distinguishes it from user-created projects.
//
//nolint:gocyclo // Bootstrap keeps catalog and filesystem recovery decisions in one transaction flow.
func (m *AppModel) ensureSystemUnfiledProject() error {
	if m == nil || m.domain.runtime == nil || m.domain.runtime.DataDir() == "" {
		return nil
	}

	project := m.domain.runtime.SystemUnfiledProject()
	catalogChanged := project == nil
	if project == nil {
		for _, candidate := range m.domain.runtime.AllProjects() {
			if candidate != nil && candidate.Slug == config.LegacyActionInboxSlug {
				project = candidate
				break
			}
		}
	}
	if project == nil {
		now := time.Now()
		project = &models.Project{
			ID:          "system-unfiled-" + uuid.New().String(),
			Slug:        config.SystemUnfiledSlug,
			Name:        config.SystemUnfiledName,
			Path:        m.domain.runtime.ProjectPath(config.SystemUnfiledSlug),
			CreatedAt:   now,
			UpdatedAt:   now,
			Tasks:       []*models.Task{},
			NextShortID: 1,
		}
	}

	if project.Name != config.SystemUnfiledName || project.Slug != config.SystemUnfiledSlug ||
		project.Archived || project.ArchivedAt != nil ||
		m.domain.runtime.Catalog().SystemUnfiledProjectID != project.ID {
		catalogChanged = true
		project.Name = config.SystemUnfiledName
		project.Slug = config.SystemUnfiledSlug
		project.Path = m.domain.runtime.ProjectPath(config.SystemUnfiledSlug)
		project.Archived = false
		project.ArchivedAt = nil
		project.UpdatedAt = time.Now()
	}

	tasksMissing := false
	if _, err := os.Stat(storage.TasksFilePath(project.Path)); os.IsNotExist(err) {
		tasksMissing = true
	} else if err != nil {
		return fmt.Errorf("inspect system Unfiled tasks: %w", err)
	}
	var ops []transaction.Op
	legacyPath := filepath.Join(config.InternalDir(m.domain.runtime.DataDir()), config.LegacyActionInboxSlug)
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
	machine := m.domain.runtime.Machine().Clone()
	artifacts := make([]storage.ArtifactChange, 0, 2)
	if catalogChanged {
		catalog := coordinator.CloneWorkspaceCatalog(m.domain.runtime.Catalog())
		if err := catalog.RepairSystemUnfiledProject(project); err != nil {
			return err
		}
		artifact, err := m.domain.runtime.CheckedCatalogArtifact(catalog)
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
	if err := m.domain.service.CommitWorkspaceBootstrap(ops, artifacts...); err != nil {
		return err
	}
	if err := m.domain.runtime.RefreshAfterCommit(); err != nil {
		return err
	}
	m.domain.runtime.SetMachineInMemory(machine)
	m.syncProjectCache()
	return nil
}
