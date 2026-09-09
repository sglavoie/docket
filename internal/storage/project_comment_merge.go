package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/sglavoie/tt/internal/models"
)

// This file merges a project's own comment collection by comment identity. It is
// the counterpart of the task merge in project_merge.go and follows the same
// rules: independent writes merge, identical concurrent writes coalesce, and
// every remaining divergence becomes structured evidence rather than a value
// neither machine wrote.
//
// Position is never consulted. A comment the other side added shifts every index
// below it, so an index-keyed merge would reconcile two unrelated comments; the
// stable UUID is the only thing that survives a concurrent insertion, which is
// why the value carries one.
//
// Nothing here writes, and nothing here returns a partial collection: a conflict
// aborts the whole merge in MergeProjectDocuments exactly as a task conflict does.

// The fields a project-comment conflict reports. They locate the divergence
// rather than render it: projectCommentField is the whole value (a presence
// race), and the other two are the only stored parts a merge can diverge on.
const (
	projectCommentField          = "comment"
	projectCommentTextField      = "comment.text"
	projectCommentCreatedAtField = "comment.created_at"
)

// mergeProjectComments three-way merges a project's comments keyed by their
// stable ids, returning the merged collection in canonical order plus one
// conflict per diverging identity.
//
// Resolution rules:
//   - Additions on one side, and additions of different comments on both sides,
//     are kept: independent writes never collide, because they carry different
//     identities.
//   - An edit on one side is adopted; identical concurrent edits coalesce.
//   - Divergent text edits to one comment conflict.
//   - Deletion versus an untouched comment resolves as deletion; deletion on
//     both sides removes it; deletion versus an edit conflicts.
//   - Two different comments added under one identity conflict.
//   - A creation timestamp that moved conflicts, because it is identity metadata
//     rather than content.
//
// The merged collection is sorted into the canonical creation-time-then-identity
// order and fully detached from the three inputs, so committing it can never
// edit a document the merge read from.
func mergeProjectComments(base, local, current []*models.ProjectComment) ([]*models.ProjectComment, []MergeConflict) {
	baseByID := indexProjectComments(base)
	localByID := indexProjectComments(local)
	currentByID := indexProjectComments(current)

	merged := make([]*models.ProjectComment, 0, len(localByID)+len(currentByID))
	var conflicts []MergeConflict

	for _, id := range orderedProjectCommentIDs(base, local, current) {
		b, inB := baseByID[id]
		l, inL := localByID[id]
		c, inC := currentByID[id]

		switch {
		case inB && inL && inC:
			comment, fieldConflicts := mergeProjectCommentFields(b, l, c)
			if len(fieldConflicts) > 0 {
				conflicts = append(conflicts, fieldConflicts...)
				continue
			}
			merged = append(merged, comment)
		case inB && inL && !inC:
			// current deleted the comment; keep it only if local left it alone.
			if models.ProjectCommentEqual(b, l) {
				continue
			}
			conflicts = append(conflicts, projectCommentDeleteEditConflict(id, b, l, nil,
				"local edited a project comment deleted on current"))
		case inB && !inL && inC:
			if models.ProjectCommentEqual(b, c) {
				continue
			}
			conflicts = append(conflicts, projectCommentDeleteEditConflict(id, b, nil, c,
				"current edited a project comment deleted on local"))
		case inB && !inL && !inC:
			// Deleted on both sides: drop it.
			continue
		case !inB && inL && inC:
			// Two independently minted UUIDs do not collide, so this is either the
			// same comment reaching the merge twice — which coalesces — or a
			// collection that already lost the one-identity-one-comment invariant.
			// Adopting either side would publish a body the other machine never
			// agreed to store under that id.
			if models.ProjectCommentEqual(l, c) {
				merged = append(merged, l)
				continue
			}
			conflicts = append(conflicts, MergeConflict{
				Kind:      MergeConflictProjectMetadata,
				CommentID: id,
				Field:     projectCommentField,
				Base:      projectCommentSideSummary(nil),
				Local:     projectCommentSideSummary(l),
				Current:   projectCommentSideSummary(c),
				Detail:    "both sides added a different project comment under one identity",
			})
		case !inB && inL && !inC:
			merged = append(merged, l)
		case !inB && !inL && inC:
			merged = append(merged, c)
		}
	}

	models.SortProjectComments(merged)
	// One detachment point for every adopted value, whichever branch produced it.
	// The merged collection becomes live state, so a later comment edit must not
	// reach back into the base, local, or current document.
	return models.CloneProjectComments(merged), conflicts
}

// mergeProjectCommentFields three-way merges one comment that all three sides
// carry. Text is the only content field; the identity and the creation instant
// are immutable, so a side that moved either is reported rather than adopted.
func mergeProjectCommentFields(base, local, current *models.ProjectComment) (*models.ProjectComment, []MergeConflict) {
	var conflicts []MergeConflict

	// CreatedAt is identity metadata, not content: it is half of the canonical
	// order, it is what the stored header spells beside the id, and no mutation
	// edits it — editing a comment rewrites Text alone. A moved instant is
	// therefore not an edit to reconcile, and adopting it silently would let one
	// machine restate when the other machine's comment was written. Even a
	// matching move on both sides is reported, because "both machines agree" is
	// not the same as "the value may change".
	if !local.CreatedAt.Equal(base.CreatedAt) || !current.CreatedAt.Equal(base.CreatedAt) {
		conflicts = append(conflicts, MergeConflict{
			Kind:      MergeConflictProjectMetadata,
			CommentID: base.ID,
			Field:     projectCommentCreatedAtField,
			Base:      projectCommentSideSummary(base),
			Local:     projectCommentSideSummary(local),
			Current:   projectCommentSideSummary(current),
			Detail:    "a project comment's creation timestamp is immutable",
		})
	}

	text, textConflicted := scalarThreeWay(base.Text, local.Text, current.Text)
	if textConflicted {
		conflicts = append(conflicts, MergeConflict{
			Kind:      MergeConflictProjectMetadata,
			CommentID: base.ID,
			Field:     projectCommentTextField,
			Base:      projectCommentSideSummary(base),
			Local:     projectCommentSideSummary(local),
			Current:   projectCommentSideSummary(current),
			Detail:    "project comment text changed on both sides",
		})
	}

	if len(conflicts) > 0 {
		return nil, conflicts
	}
	return &models.ProjectComment{ID: base.ID, Text: text, CreatedAt: base.CreatedAt}, nil
}

// projectCommentDeleteEditConflict reports one side removing a comment the other
// side rewrote. Both are legitimate intents and neither contains the other, so
// the record names the identity and leaves the choice to the operator.
func projectCommentDeleteEditConflict(id string, base, local, current *models.ProjectComment, detail string) MergeConflict {
	return MergeConflict{
		Kind:      MergeConflictProjectMetadata,
		CommentID: id,
		Field:     projectCommentField,
		Base:      projectCommentSideSummary(base),
		Local:     projectCommentSideSummary(local),
		Current:   projectCommentSideSummary(current),
		Detail:    detail,
	}
}

// projectCommentSideSummary describes one side of a comment conflict without
// reproducing the prose. The bundle already stores all three documents, so a
// summary that inlined bodies would be the only place a comment's text was
// duplicated. What the record has to answer is which comment diverged, whether a
// side holds it at all, and whether two sides hold the same body — which the
// digest settles exactly, and a bare length would not.
func projectCommentSideSummary(comment *models.ProjectComment) string {
	if comment == nil {
		return "absent"
	}
	return fmt.Sprintf("created %s, text %s (%d rune(s))",
		comment.CreatedAt.UTC().Format(time.RFC3339Nano),
		projectCommentTextDigest(comment.Text),
		utf8.RuneCountInString(comment.Text))
}

// projectCommentTextDigest fingerprints a body so two sides can be compared in a
// conflict record without either being quoted there.
func projectCommentTextDigest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])[:12]
}

func indexProjectComments(comments []*models.ProjectComment) map[string]*models.ProjectComment {
	indexed := make(map[string]*models.ProjectComment, len(comments))
	for _, comment := range comments {
		if comment == nil {
			continue
		}
		indexed[comment.ID] = comment
	}
	return indexed
}

// orderedProjectCommentIDs returns every distinct comment identity across the
// three sides in the canonical creation-time-then-identity order, so the merged
// collection and the conflict list read in the order the documents are written
// in and do not depend on which side the caller passed as local.
//
// The representative instant is the earliest any side spells for that identity,
// which is what keeps the order symmetric when a side moved a timestamp the
// merge is about to refuse.
func orderedProjectCommentIDs(base, local, current []*models.ProjectComment) []string {
	created := make(map[string]time.Time)
	note := func(comments []*models.ProjectComment) {
		for _, comment := range comments {
			if comment == nil {
				continue
			}
			if existing, seen := created[comment.ID]; seen && !comment.CreatedAt.Before(existing) {
				continue
			}
			created[comment.ID] = comment.CreatedAt
		}
	}
	note(base)
	note(local)
	note(current)

	ids := make([]string, 0, len(created))
	for id := range created {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		left, right := created[ids[i]], created[ids[j]]
		if !left.Equal(right) {
			return left.Before(right)
		}
		return ids[i] < ids[j]
	})
	return ids
}
