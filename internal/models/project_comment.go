package models

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// This file owns the project-comment value and the invariants every surface
// that stores one is held to. A project comment is not a task comment with a
// different owner: a task comment is positional and anonymous, while a project
// comment carries a stable identity of its own, because two machines adding
// comments concurrently have to be reconciled by identity rather than by index.

// projectCommentIDPattern is the canonical lowercase RFC 4122 spelling. The
// identity is minted as a UUID and stored verbatim, so a document naming the
// same value in another spelling would hash and sort differently while meaning
// the same comment; refusing the alternate spellings is what keeps one comment
// to one string.
var projectCommentIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ProjectComment is one timestamped comment on a project. Its collection lives
// in the project's own Markdown document rather than in the catalog or a note,
// and CreatedAt is immutable: editing a comment rewrites Text alone.
type ProjectComment struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// ValidateProjectComment reports whether one comment's parts can be persisted.
// It takes primitives rather than the value so a storage codec can validate the
// record shape it just decoded without first building a domain value, exactly
// as ValidateTaskReference does.
func ValidateProjectComment(id, text string, createdAt time.Time) error {
	if !projectCommentIDPattern.MatchString(id) {
		return fmt.Errorf("project comment id %q is not a canonical lowercase UUID", id)
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("project comment %s has no text", id)
	}
	if createdAt.IsZero() {
		return fmt.Errorf("project comment %s has no creation timestamp", id)
	}
	return nil
}

// ValidateProjectComments validates a whole collection: every comment on its
// own, plus the project-local uniqueness of the identity the merge reconciles
// by. A duplicate id would make two comments indistinguishable to every
// identity-keyed reader, so it is refused here rather than resolved.
func ValidateProjectComments(comments []*ProjectComment) error {
	seen := make(map[string]bool, len(comments))
	for index, comment := range comments {
		if comment == nil {
			return fmt.Errorf("project has nil comment at index %d", index)
		}
		if err := ValidateProjectComment(comment.ID, comment.Text, comment.CreatedAt); err != nil {
			return err
		}
		if seen[comment.ID] {
			return fmt.Errorf("duplicate project comment id %q", comment.ID)
		}
		seen[comment.ID] = true
	}
	return nil
}

// CloneProjectComments returns a detached copy. The elements are pointers, so a
// caller holding the source collection would otherwise see an edit made through
// the copy; a snapshot, a rollback document, and a live project must never
// share one comment object.
func CloneProjectComments(comments []*ProjectComment) []*ProjectComment {
	if len(comments) == 0 {
		return nil
	}
	cloned := make([]*ProjectComment, 0, len(comments))
	for _, comment := range comments {
		if comment == nil {
			continue
		}
		copied := *comment
		cloned = append(cloned, &copied)
	}
	return cloned
}

// SortProjectComments puts a collection in canonical order: oldest first, ties
// broken by identity. The order is total and derived only from stored values,
// so two machines that hold the same set write the same bytes.
func SortProjectComments(comments []*ProjectComment) {
	sort.SliceStable(comments, func(i, j int) bool {
		left, right := comments[i], comments[j]
		switch {
		case left == nil:
			return right != nil
		case right == nil:
			return false
		case !left.CreatedAt.Equal(right.CreatedAt):
			return left.CreatedAt.Before(right.CreatedAt)
		default:
			return left.ID < right.ID
		}
	})
}

// ProjectCommentsEqual reports whether two collections carry the same comments
// in the same order. It compares the instants rather than the time structs, so
// the same moment spelled in two locations is one value.
func ProjectCommentsEqual(left, right []*ProjectComment) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		a, b := left[i], right[i]
		if a == nil || b == nil {
			if a != b {
				return false
			}
			continue
		}
		if a.ID != b.ID || a.Text != b.Text || !a.CreatedAt.Equal(b.CreatedAt) {
			return false
		}
	}
	return true
}
