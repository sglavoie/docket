package storage

import "github.com/sglavoie/docket/internal/models"

// rebaseProjectCommentsRestore applies only the original comment deltas. A
// touched identity must still have its complete expected value (or be absent
// for an insertion); even an identical independently applied change refuses.
// Untouched comments remain current, and creation instants never move on edit.
// All documents are private preflight values: nothing is staged on a refusal.
func rebaseProjectCommentsRestore(expected, desired, current *ProjectDocument) bool {
	expectedByID := indexProjectComments(expected.Comments)
	desiredByID := indexProjectComments(desired.Comments)
	currentByID := indexProjectComments(current.Comments)
	for _, id := range orderedProjectCommentIDs(expected.Comments, desired.Comments, nil) {
		before, after := expectedByID[id], desiredByID[id]
		if models.ProjectCommentEqual(before, after) {
			continue
		}
		if !models.ProjectCommentEqual(before, currentByID[id]) ||
			(before != nil && after != nil && !before.CreatedAt.Equal(after.CreatedAt)) {
			return false
		}
		if after == nil {
			delete(currentByID, id)
		} else {
			restored := *after
			currentByID[id] = &restored
		}
	}
	comments := make([]*models.ProjectComment, 0, len(currentByID))
	for _, comment := range currentByID {
		comments = append(comments, comment)
	}
	models.SortProjectComments(comments)
	current.Comments = comments
	return true
}
