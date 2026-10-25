package valuepicker

import (
	"strings"

	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/query"
)

// The two categories every tag list seats its rows in. Behavioral tags are
// pinned above the descriptive vocabulary because they are the only ones that
// change what the project does; the shell's own tag pickers seat their rows in
// the same two, so both spellings live here beside the picker that reads them.
const (
	ProjectTagBehavioralCategory listsurface.ListCategoryID = "project-tag:behavioral"
	TagDescriptiveCategory       listsurface.ListCategoryID = "tag:other"
)

// ProjectTagVocabularyActionsMsg opens the vocabulary sheet for one tag row.
type ProjectTagVocabularyActionsMsg struct{ Tag string }

// projectTagVocabularySelectableTag reports the tag a picker row names, or false
// for the Untagged pseudo-row, which is a facet rather than a tag the catalog
// could rename or remove.
func projectTagVocabularySelectableTag(item listsurface.ListItem) (string, bool) {
	tag, ok := item.Data.(string)
	if !ok || tag == "" || tag == query.UntaggedProjectFilterKey {
		return "", false
	}
	return strings.ToLower(tag), true
}
