package coordinator

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// The three helpers here are the catalog half of a project mutation, and they
// sit below every surface that performs one. The shell commits a rename from a
// handler, the Project Hub's editor and creator commit their own, and the
// Outliner's cross-scope load stamps freshly parsed projects with the catalog
// metadata their tasks.md never carried. All three readings need the same
// snapshot discipline, so the functions live once, below all of them.

// CloneWorkspaceCatalog returns a defensive copy of a catalog snapshot whose
// project slice can be rewritten without mutating the runtime's own view.
func CloneWorkspaceCatalog(c *config.WorkspaceCatalog) *config.WorkspaceCatalog {
	return app.CloneCatalog(c)
}

// CatalogDomainArtifact renders only workspace.json for a project metadata
// mutation that must commit atomically with project-directory and tasks.md
// changes. It never reconstructs the Config facade or rewrites unrelated
// configuration domains.
//
// The candidate was derived from a runtime snapshot, so it is stamped with the
// revision that snapshot observed. Reloading the domains here to obtain a
// fresher baseline would silently re-parent stale in-memory intent onto a
// concurrent writer's state and overwrite it; the snapshot's own revision is
// what makes an intervening write fail closed.
func CatalogDomainArtifact(runtime *config.RuntimeState, catalog *config.WorkspaceCatalog) ([]storage.ArtifactChange, error) {
	artifact, err := runtime.CheckedCatalogArtifact(catalog)
	if err != nil {
		return nil, err
	}
	return []storage.ArtifactChange{artifact.StorageChange()}, nil
}

// CopyCatalogMetadata overwrites the catalog-owned fields of dst with src's.
// A project parsed from disk carries its tasks and nothing else, so the record
// the catalog holds is the only source for identity, lifecycle and presentation.
func CopyCatalogMetadata(dst, src *models.Project) {
	if dst == nil || src == nil {
		return
	}
	dst.Slug = src.Slug
	dst.Name = src.Name
	dst.Description = src.Description
	dst.Path = src.Path
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
	dst.Archived = src.Archived
	dst.ArchivedAt = src.ArchivedAt
	dst.Pinned = src.Pinned
	dst.PinnedAt = src.PinnedAt
	dst.ColorSlot = src.ColorSlot
	dst.Tags = src.Tags
}
