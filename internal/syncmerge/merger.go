// Package syncmerge is the artifact-aware half of sync reconciliation. The
// sync history (internal/synclog) decides which versions of a file compete and
// against which base; this package merges those versions by what the file
// means: project documents task by task, the catalog project by project, and
// planning entry by entry. Every merge is a pure function of its inputs, so
// every device reaches the same bytes.
package syncmerge

import (
	"fmt"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/synclog"
)

// Merger reconciles every synced artifact kind. The zero value is ready.
type Merger struct{}

var (
	_ synclog.Merger    = Merger{}
	_ synclog.Validator = Merger{}
)

// Merge combines req.Sides against req.Base. A kind with no structure of its
// own — notes, lenses, attachments, other project files — and any artifact a
// side wrote that does not parse keep the primary side whole and report the
// competing versions, which stay retained in the history.
func (Merger) Merge(req synclog.MergeRequest) (synclog.MergeResult, error) {
	if len(req.Sides) < 2 {
		return synclog.MergeResult{}, fmt.Errorf("syncmerge: %s needs at least two versions to merge", req.Key)
	}
	var (
		result synclog.MergeResult
		err    error
	)
	switch req.Kind {
	case synclog.EffectProjectDocument:
		result, err = mergeProjectDocument(req)
	case synclog.EffectCatalog:
		result, err = mergeDomain(req, config.ReconcileCatalogArtifacts)
	case synclog.EffectPlanning:
		result, err = mergeDomain(req, config.ReconcilePlanningArtifacts)
	default:
		return keepPrimary(req, "the versions cannot be merged by content"), nil
	}
	if err != nil {
		return keepPrimary(req, fmt.Sprintf("a version could not be read (%v)", err)), nil
	}
	return result, nil
}

// RestoreProject names projectID in catalog again with its entry from from.
func (Merger) RestoreProject(catalog []byte, projectID string, from []byte) ([]byte, bool, error) {
	return config.RestoreCatalogProject(catalog, projectID, from)
}

// ValidateArtifact accepts exactly what the live readers load: a project
// document that parses, passes the project audit, and renders through the
// canonical writer, and a catalog, planning, or lens file the domain loader
// accepts. Other kinds are opaque bytes and always valid.
func (Merger) ValidateArtifact(kind synclog.EffectKind, data []byte) error {
	switch kind {
	case synclog.EffectProjectDocument:
		doc, err := storage.ParseProjectDocument(string(data))
		if err != nil {
			return err
		}
		if err = models.AuditProject(&models.Project{Name: doc.ProjectName, Tasks: doc.Tasks, Comments: doc.Comments}); err != nil {
			return err
		}
		_, err = doc.WriteCurrent()
		return err
	case synclog.EffectCatalog:
		return config.ValidateSyncedDomainArtifact("workspace", data)
	case synclog.EffectPlanning:
		return config.ValidateSyncedDomainArtifact("planning", data)
	case synclog.EffectLenses:
		return config.ValidateSyncedDomainArtifact("lenses", data)
	case synclog.EffectNote, synclog.EffectGlobalNote, synclog.EffectAttachment, synclog.EffectProjectTree:
		return nil
	}
	return nil
}

func keepPrimary(req synclog.MergeRequest, why string) synclog.MergeResult {
	return synclog.MergeResult{
		Data: req.Sides[0].Data,
		Conflicts: []synclog.ConflictDetail{{
			Field:  "content",
			Detail: fmt.Sprintf("%d competing versions; %s, so the primary version is kept and the others are retained", len(req.Sides), why),
		}},
	}
}

func mergeProjectDocument(req synclog.MergeRequest) (synclog.MergeResult, error) {
	var base *storage.ProjectDocument
	if req.Base.Exists {
		parsed, err := storage.ParseProjectDocument(string(req.Base.Data))
		if err != nil {
			return synclog.MergeResult{}, err
		}
		base = parsed
	}
	sides := make([]*storage.ProjectDocument, 0, len(req.Sides))
	for _, side := range req.Sides {
		parsed, err := storage.ParseProjectDocument(string(side.Data))
		if err != nil {
			return synclog.MergeResult{}, err
		}
		sides = append(sides, parsed)
	}
	merged, conflicts, err := storage.ReconcileProjectDocuments(base, sides)
	if err != nil {
		return synclog.MergeResult{}, err
	}
	content, err := merged.WriteCurrent()
	if err != nil {
		return synclog.MergeResult{}, err
	}
	result := synclog.MergeResult{Data: []byte(content)}
	for _, c := range conflicts {
		subject := c.TaskTitle
		if c.CommentID != "" {
			subject = "comment " + c.CommentID
		}
		detail := fmt.Sprintf("%s: %s", c.Kind, c.Detail)
		if c.Base != "" || c.Local != "" || c.Current != "" {
			detail = fmt.Sprintf("%s: base %q, versions %q and %q. %s", c.Kind, c.Base, c.Local, c.Current, c.Detail)
		}
		result.Conflicts = append(result.Conflicts, synclog.ConflictDetail{Subject: subject, Field: c.Field, Detail: detail})
	}
	return result, nil
}

type domainReconciler func(base []byte, sides [][]byte) ([]byte, []config.SyncMergeConflict, error)

func mergeDomain(req synclog.MergeRequest, reconcile domainReconciler) (synclog.MergeResult, error) {
	var base []byte
	if req.Base.Exists {
		base = req.Base.Data
	}
	sides := make([][]byte, len(req.Sides))
	for i, side := range req.Sides {
		sides[i] = side.Data
	}
	data, conflicts, err := reconcile(base, sides)
	if err != nil {
		return synclog.MergeResult{}, err
	}
	result := synclog.MergeResult{Data: data}
	for _, c := range conflicts {
		result.Conflicts = append(result.Conflicts, synclog.ConflictDetail{Subject: c.Subject, Field: c.Field, Detail: c.Detail})
	}
	return result, nil
}
