package tui

import (
	"unicode/utf8"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// searchACOutliner builds a scoped Outliner already in search mode, which is the
// only state the suggestion pane can exist in.
func searchACOutliner(projects ...*models.Project) *outliner.OutlinerModel {
	outlinerView := scopeTestOutliner(projects...)
	outlinerView.SetSize(120, 40)
	outlinerView.ActivateSearch()
	return outlinerView
}

// typeSearchAC puts a whole query in the box with the caret at its end, which is
// the ordinary typing case.
func typeSearchAC(outlinerView *outliner.OutlinerModel, value string) {
	typeSearchACAt(outlinerView, value, utf8.RuneCountInString(value))
}

func searchACValues(outlinerView *outliner.OutlinerModel) []string {
	values := make([]string, 0, len(outlinerView.Search.Autocomplete.Ac.Items))
	for _, item := range outlinerView.Search.Autocomplete.Ac.Items {
		values = append(values, item.Value)
	}
	return values
}

// searchACTask seeds one tagged autocomplete candidate. The shell's own
// autocomplete tests exercise tag values only; the context axis is asserted by
// the tests living beside the outliner, whose copy of this helper takes one.
func searchACTask(id string, tags []string) *models.Task {
	task := primaryRowsTask(id, models.StateBacklog)
	task.Title = id
	task.Tags = tags
	return task
}

// typeSearchACAt is the mid-query case: the caret lands on a rune index inside a
// settled query rather than after it.
func typeSearchACAt(outlinerView *outliner.OutlinerModel, value string, runeCursor int) {
	outlinerView.SetSearchInputValue(value)
	outlinerView.SetSearchInputCursor(runeCursor)
	outlinerView.RefreshSearchAutocomplete()
}
