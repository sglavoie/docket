package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// handleFilterTypeKey applies a type filter based on the key pressed.
// Returns true if the key was handled.
func handleFilterTypeKey(key string, f *CommonFilterState) bool {
	switch key {
	case "e":
		f.Type = models.TaskTypeEpic
	case "s":
		f.Type = models.TaskTypeStory
	case "t":
		f.Type = models.TaskTypeTask
	case "b":
		f.Type = models.TaskTypeBug
	case "i":
		f.Type = models.TaskTypeIdea
	case "0":
		f.Type = ""
	default:
		return false
	}
	return true
}

// handleFilterStatusKey applies a status filter based on the key pressed.
// Returns true if the key was handled.
func handleFilterStatusKey(key string, f *CommonFilterState) bool {
	switch key {
	case "t":
		f.Status = models.StatusTodo
	case "s":
		f.Status = models.StatusInProgress
	case "r":
		f.Status = models.StatusReview
	case "S":
		f.Status = models.StatusStandby
	case "b":
		f.Status = models.StatusBlocked
	case "d":
		f.Status = models.StatusDone
	case "x":
		f.Status = models.StatusCanceled
	case "0":
		f.Status = ""
	default:
		return false
	}
	return true
}

// handleFilterDueDateKey applies a due date filter based on the key pressed.
// Returns true if the key was handled.
func handleFilterDueDateKey(key string, f *CommonFilterState) bool {
	switch key {
	case "o":
		f.DueDate = DueDateOverdue
	case "t":
		f.DueDate = DueDateToday
	case "w":
		f.DueDate = DueDateThisWeek
	case "n":
		f.DueDate = DueDateNone
	case "0":
		f.DueDate = DueDateAll
	default:
		return false
	}
	return true
}

// handleFilterAttrKey handles the attribute sub-prefix selection. Returns the
// new pendingPrefix if a sub-menu was opened, or "" if the key was handled
// directly (validated toggle, clear). Returns handled=false for unknown keys.
func handleFilterAttrKey(key string, f *CommonFilterState) (newPrefix string, handled bool) {
	switch key {
	case "s":
		return "Fas", true
	case "r":
		return "Far", true
	case "e":
		return "Fae", true
	case "c":
		return "Fac", true
	case "v":
		return "Fav", true
	case "w":
		return "Faw", true
	case "o":
		return "Fao", true
	case "p":
		return "Fap", true
	case "V":
		if f.Attrs["idea.validated"] == "true" {
			delete(f.Attrs, "idea.validated")
		} else {
			f.Attrs["idea.validated"] = "true"
			f.Type = models.TaskTypeIdea
		}
		return "", true
	case "0":
		f.Attrs = make(map[string]string)
		return "", true
	}
	return "", false
}

// handleFilterAttrSeverityKey applies bug severity filter.
func handleFilterAttrSeverityKey(key string, f *CommonFilterState) bool {
	switch key {
	case "c":
		f.Attrs["bug.severity"] = string(models.BugSeverityCritical)
		f.Type = models.TaskTypeBug
	case "j":
		f.Attrs["bug.severity"] = string(models.BugSeverityMajor)
		f.Type = models.TaskTypeBug
	case "m":
		f.Attrs["bug.severity"] = string(models.BugSeverityMinor)
		f.Type = models.TaskTypeBug
	case "o":
		f.Attrs["bug.severity"] = string(models.BugSeverityCosmetic)
		f.Type = models.TaskTypeBug
	case "0":
		delete(f.Attrs, "bug.severity")
	default:
		return false
	}
	return true
}

// handleFilterAttrReproducibleKey applies bug reproducible filter.
func handleFilterAttrReproducibleKey(key string, f *CommonFilterState) bool {
	switch key {
	case "y":
		f.Attrs["bug.reproducible"] = string(models.BugReproducibleYes)
		f.Type = models.TaskTypeBug
	case "n":
		f.Attrs["bug.reproducible"] = string(models.BugReproducibleNo)
		f.Type = models.TaskTypeBug
	case "s":
		f.Attrs["bug.reproducible"] = string(models.BugReproducibleSometimes)
		f.Type = models.TaskTypeBug
	case "0":
		delete(f.Attrs, "bug.reproducible")
	default:
		return false
	}
	return true
}

// handleFilterAttrEffortKey applies effort filter.
func handleFilterAttrEffortKey(key string, f *CommonFilterState) bool {
	switch key {
	case "s":
		f.Attrs["effort"] = string(models.EffortSmall)
	case "m":
		f.Attrs["effort"] = string(models.EffortMedium)
	case "l":
		f.Attrs["effort"] = string(models.EffortLarge)
	case "u":
		f.Attrs["effort"] = string(models.EffortUnknown)
	case "0":
		delete(f.Attrs, "effort")
	default:
		return false
	}
	return true
}

// handleFilterAttrCategoryKey applies idea category filter.
func handleFilterAttrCategoryKey(key string, f *CommonFilterState) bool {
	switch key {
	case "f":
		f.Attrs["idea.category"] = string(models.IdeaCategoryFeature)
		f.Type = models.TaskTypeIdea
	case "i":
		f.Attrs["idea.category"] = string(models.IdeaCategoryImprovement)
		f.Type = models.TaskTypeIdea
	case "r":
		f.Attrs["idea.category"] = string(models.IdeaCategoryRefactor)
		f.Type = models.TaskTypeIdea
	case "s":
		f.Attrs["idea.category"] = string(models.IdeaCategoryResearch)
		f.Type = models.TaskTypeIdea
	case "0":
		delete(f.Attrs, "idea.category")
	default:
		return false
	}
	return true
}

// handleFilterAttrValueKey applies idea value filter.
func handleFilterAttrValueKey(key string, f *CommonFilterState) bool {
	switch key {
	case "l":
		f.Attrs["idea.value"] = string(models.IdeaValueLow)
		f.Type = models.TaskTypeIdea
	case "m":
		f.Attrs["idea.value"] = string(models.IdeaValueMedium)
		f.Type = models.TaskTypeIdea
	case "h":
		f.Attrs["idea.value"] = string(models.IdeaValueHigh)
		f.Type = models.TaskTypeIdea
	case "0":
		delete(f.Attrs, "idea.value")
	default:
		return false
	}
	return true
}

// handleFilterAttrWorktypeKey applies task worktype filter.
func handleFilterAttrWorktypeKey(key string, f *CommonFilterState) bool {
	switch key {
	case "d":
		f.Attrs["task.worktype"] = string(models.WorkTypeDevelopment)
		f.Type = models.TaskTypeTask
	case "o":
		f.Attrs["task.worktype"] = string(models.WorkTypeDocumentation)
		f.Type = models.TaskTypeTask
	case "t":
		f.Attrs["task.worktype"] = string(models.WorkTypeTesting)
		f.Type = models.TaskTypeTask
	case "v":
		f.Attrs["task.worktype"] = string(models.WorkTypeDevOps)
		f.Type = models.TaskTypeTask
	case "e":
		f.Attrs["task.worktype"] = string(models.WorkTypeDesign)
		f.Type = models.TaskTypeTask
	case "0":
		delete(f.Attrs, "task.worktype")
	default:
		return false
	}
	return true
}

// handleFilterAttrScopeKey applies epic scope filter.
func handleFilterAttrScopeKey(key string, f *CommonFilterState) bool {
	switch key {
	case "d":
		f.Attrs["epic.scope"] = string(models.EpicScopeDefined)
		f.Type = models.TaskTypeEpic
	case "e":
		f.Attrs["epic.scope"] = string(models.EpicScopeEvolving)
		f.Type = models.TaskTypeEpic
	case "u":
		f.Attrs["epic.scope"] = string(models.EpicScopeUndefined)
		f.Type = models.TaskTypeEpic
	case "0":
		delete(f.Attrs, "epic.scope")
	default:
		return false
	}
	return true
}

// handleFilterAttrPointsKey applies story points filter.
func handleFilterAttrPointsKey(key string, f *CommonFilterState) bool {
	switch key {
	case "1":
		f.Attrs["story.points"] = "1"
		f.Type = models.TaskTypeStory
	case "2":
		f.Attrs["story.points"] = "2"
		f.Type = models.TaskTypeStory
	case "3":
		f.Attrs["story.points"] = "3"
		f.Type = models.TaskTypeStory
	case "5":
		f.Attrs["story.points"] = "5"
		f.Type = models.TaskTypeStory
	case "8":
		f.Attrs["story.points"] = "8"
		f.Type = models.TaskTypeStory
	case "0":
		delete(f.Attrs, "story.points")
	default:
		return false
	}
	return true
}

// handleFilterAttrSubKey dispatches attribute sub-prefix keys to the appropriate handler.
// Returns true if the key was handled.
func handleFilterAttrSubKey(prefix, key string, f *CommonFilterState) bool {
	switch prefix {
	case "Fas":
		return handleFilterAttrSeverityKey(key, f)
	case "Far":
		return handleFilterAttrReproducibleKey(key, f)
	case "Fae":
		return handleFilterAttrEffortKey(key, f)
	case "Fac":
		return handleFilterAttrCategoryKey(key, f)
	case "Fav":
		return handleFilterAttrValueKey(key, f)
	case "Faw":
		return handleFilterAttrWorktypeKey(key, f)
	case "Fao":
		return handleFilterAttrScopeKey(key, f)
	case "Fap":
		return handleFilterAttrPointsKey(key, f)
	}
	return false
}

// handleFilterScoreKey applies a score threshold filter based on the key pressed.
// Returns true if the key was handled.
func handleFilterScoreKey(key string, f *CommonFilterState, cfg *config.Config) bool {
	switch key {
	case "1":
		f.ScoreMin = 30
	case "2":
		f.ScoreMin = 60
	case "3":
		f.ScoreMin = 90
	case "4":
		f.ScoreMin = 120
	case "5":
		f.ScoreMin = 150
	case "6":
		f.ScoreMin = 180
	case "7":
		f.ScoreMin = 210
	case "8":
		f.ScoreMin = 240
	case "9":
		f.ScoreMin = 270
	case "c":
		if cfg != nil && cfg.ScoreFilterCustom != nil {
			f.ScoreMin = *cfg.ScoreFilterCustom
		}
	case "0":
		f.ScoreMin = 0
	default:
		return false
	}
	return true
}

// dispatchFilterSubPrefix handles a filter sub-prefix key press for views using
// CommonFilterState. It routes the key to the appropriate handler and returns
// true if the filter state changed (caller should refresh). When a new
// sub-prefix popup is needed (e.g., Fa → Fas), setPendingPrefix is called and
// false is returned (no refresh needed, just show the next popup).
func dispatchFilterSubPrefix(prefix, key string, f *CommonFilterState, cfg *config.Config, setPendingPrefix func(string)) bool {
	switch prefix {
	case "Ft":
		return handleFilterTypeKey(key, f)
	case "Fs":
		return handleFilterStatusKey(key, f)
	case "Fd":
		return handleFilterDueDateKey(key, f)
	case "Fa":
		newPrefix, handled := handleFilterAttrKey(key, f)
		if !handled {
			return false
		}
		if newPrefix != "" {
			setPendingPrefix(newPrefix)
			return false
		}
		return true
	case "Fas", "Far", "Fae", "Fac", "Fav", "Faw", "Fao", "Fap":
		return handleFilterAttrSubKey(prefix, key, f)
	case "FS":
		return handleFilterScoreKey(key, f, cfg)
	}
	return false
}

// FilterPrefixResult holds the result of a shared filter prefix key handler.
type FilterPrefixResult struct {
	Handled   bool    // true if the key was recognized
	NewPrefix string  // set pendingPrefix to this (e.g. "Ft")
	Changed   bool    // filter state mutated, caller should refresh
	TagCmd    tea.Cmd // non-nil when "#" opens tag selector
}

// handleCommonFilterPrefix handles the common first-level F-prefix keys shared
// across views: p (priority toggle), t/s (sub-prefix), # (tag selector),
// a (attribute sub-prefix), S (score sub-prefix), x (clear all).
// buildTagCmd should return the tea.Cmd that opens the tag filter selector.
// The caller handles view-specific keys before calling this function.
func handleCommonFilterPrefix(key string, f *CommonFilterState, buildTagCmd func() tea.Cmd) FilterPrefixResult {
	switch key {
	case "p":
		if f.Priority == 1 {
			f.Priority = 0
		} else {
			f.Priority = 1
		}
		return FilterPrefixResult{Handled: true, Changed: true}
	case "t":
		return FilterPrefixResult{Handled: true, NewPrefix: "Ft"}
	case "s":
		return FilterPrefixResult{Handled: true, NewPrefix: "Fs"}
	case "#":
		return FilterPrefixResult{Handled: true, TagCmd: buildTagCmd()}
	case "a":
		return FilterPrefixResult{Handled: true, NewPrefix: "Fa"}
	case "S":
		return FilterPrefixResult{Handled: true, NewPrefix: "FS"}
	case "x":
		clearCommonFilters(f)
		return FilterPrefixResult{Handled: true, Changed: true}
	}
	return FilterPrefixResult{}
}

// buildCommonFilterPrefixHints builds the dynamic F-prefix hint list for views
// that use CommonFilterState. Set includeDueDate and includeStatus to false
// to omit those categories for views that handle them differently.
func buildCommonFilterPrefixHints(f *CommonFilterState, includeDueDate, includeStatus bool) []PrefixHint {
	priorityDetail := "(all)"
	if f.Priority == 1 {
		priorityDetail = "!Hi"
	}

	typeDetail := "(all)"
	if f.Type != "" {
		typeDetail = string(f.Type)
	}

	tagDetail := "(none)"
	if len(f.Tags) > 0 {
		tagDetail = fmt.Sprintf("%d tags", len(f.Tags))
	}

	attrDetail := "(none)"
	if len(f.Attrs) > 0 {
		attrDetail = fmt.Sprintf("%d attrs", len(f.Attrs))
	}

	scoreDetail := "(off)"
	if f.ScoreMin > 0 {
		scoreDetail = fmt.Sprintf(">=%.0f", f.ScoreMin)
	}

	hints := []PrefixHint{
		{Key: "p", Description: "priority", Detail: priorityDetail, Active: f.Priority != 0},
		{Key: "t", Description: "type", Detail: typeDetail, Active: f.Type != ""},
	}

	if includeStatus {
		statusDetail := "(all)"
		if f.Status != "" {
			statusDetail = string(f.Status)
		}
		hints = append(hints, PrefixHint{Key: "s", Description: "status", Detail: statusDetail, Active: f.Status != ""})
	}

	hints = append(hints, PrefixHint{Key: "#", Description: "tags", Detail: tagDetail, Active: len(f.Tags) > 0})

	if includeDueDate {
		dueDetail := "(all)"
		switch f.DueDate {
		case DueDateOverdue:
			dueDetail = "overdue"
		case DueDateToday:
			dueDetail = "today"
		case DueDateThisWeek:
			dueDetail = "week"
		case DueDateNone:
			dueDetail = "no-date"
		}
		hints = append(hints, PrefixHint{Key: "d", Description: "due-date", Detail: dueDetail, Active: f.DueDate != DueDateAll})
	}

	hints = append(hints,
		PrefixHint{Key: "a", Description: "attribute", Detail: attrDetail, Active: len(f.Attrs) > 0},
		PrefixHint{Key: "S", Description: "score", Detail: scoreDetail, Active: f.ScoreMin > 0},
		PrefixHint{Key: "x", Description: "clear-all"},
	)

	return hints
}

// applyCommonFilterResult applies the result of handleCommonFilterPrefix to the
// view's state: sets pending prefix, refreshes if changed, returns tag command.
// Returns the tea.Cmd to return from the view's handler (nil if no command).
func applyCommonFilterResult(result FilterPrefixResult, setPendingPrefix func(string), refreshView func()) tea.Cmd {
	if result.NewPrefix != "" {
		setPendingPrefix(result.NewPrefix)
	}
	if result.Changed {
		refreshView()
	}
	return result.TagCmd
}

// applyFilterSubPrefix handles a filter sub-prefix key press, including the
// score filter custom-value special case. Returns true if the view should
// keep processing (score input was activated), false otherwise.
func applyFilterSubPrefix(prefix, key string, f *CommonFilterState, cfg *config.Config, setPendingPrefix func(string), enterScoreInput func(*float64), refreshView func()) bool {
	if prefix == "FS" && key == "C" {
		var custom *float64
		if cfg != nil {
			custom = cfg.ScoreFilterCustom
		}
		enterScoreInput(custom)
		return true
	}
	if dispatchFilterSubPrefix(prefix, key, f, cfg, setPendingPrefix) {
		refreshView()
	}
	return false
}

// buildCommonFilterPrefixPopupHints returns the full PrefixHints map for the F prefix
// and its sub-prefixes. Set includeDueDate and includeStatus to control which
// categories are available.
func buildCommonFilterPrefixPopupHints(f *CommonFilterState, cfg *config.Config, includeDueDate, includeStatus bool) PrefixHints {
	hints := PrefixHints{
		"F":   buildCommonFilterPrefixHints(f, includeDueDate, includeStatus),
		"Ft":  FilterTypePrefixHints,
		"Fa":  FilterAttrPrefixHints,
		"Fas": FilterAttrSeverityPrefixHints,
		"Far": FilterAttrReproduciblePrefixHints,
		"Fae": FilterAttrEffortPrefixHints,
		"Fac": FilterAttrCategoryPrefixHints,
		"Fav": FilterAttrValuePrefixHints,
		"Faw": FilterAttrWorktypePrefixHints,
		"Fao": FilterAttrScopePrefixHints,
		"Fap": FilterAttrPointsPrefixHints,
		"FS":  BuildFilterScorePrefixHints(cfg),
	}
	if includeStatus {
		hints["Fs"] = FilterStatusPrefixHints
	}
	if includeDueDate {
		hints["Fd"] = FilterDuePrefixHints
	}
	return hints
}
