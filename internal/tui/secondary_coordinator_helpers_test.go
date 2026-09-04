package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// testAppRepo is a minimal app.Repository for isolated view tests, backed by
// storage.DefaultRepository (the real filesystem against a t.TempDir project,
// or a swapped-in mock). Snapshot mirrors the production tuiAppRepository's
// reload semantics: it loads each registered project's last-saved state back
// into the SHARED project pointer (replacing its task objects), so a view's
// optimistic in-memory mutation is discarded before the use case re-applies it
// against the loaded state — exactly what the production repository does.
type testAppRepo struct {
	projects []*models.Project
}

func newTestAppRepo(projects ...*models.Project) *testAppRepo {
	for _, p := range projects {
		// Seed the backing store so the first Snapshot load succeeds; projects
		// without a usable path simply skip persistence (their tests never
		// dispatch a coordinator mutation).
		_ = storage.SaveProject(p)
	}
	return &testAppRepo{projects: projects}
}

func (r *testAppRepo) Snapshot() (app.Snapshot, error) {
	revisions := make(map[string]string, len(r.projects))
	documents := make(map[string]*storage.ProjectDocument, len(r.projects))
	for _, p := range r.projects {
		if err := storage.LoadProject(p); err != nil {
			return nil, err
		}
		doc, err := storage.LoadProjectDocument(p)
		if err != nil {
			return nil, err
		}
		revisions[p.ID] = doc.Revision
		documents[p.ID] = doc
	}
	return app.NewSnapshot(r.projects, revisions, documents), nil
}

func (r *testAppRepo) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	results := make([]*storage.CommitResult, len(changesets))
	for i, cs := range changesets {
		if cs.Project != nil {
			if err := storage.SaveProject(cs.Project); err != nil {
				return nil, err
			}
		}
		results[i] = &storage.CommitResult{Revision: "test-revision", TransactionID: "test-transaction"}
	}
	return results, nil
}
