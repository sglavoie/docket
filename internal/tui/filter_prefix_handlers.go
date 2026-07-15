package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

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

// handleFilterStateKey applies a workflow-state filter based on the key
// pressed. It backs the Fs sub-prefix (the sole state/status filter after the
// legacy TaskStatus cycle was retired). Returns true if the key was handled.
func handleFilterStateKey(key string, f *CommonFilterState) bool {
	switch key {
	case "i":
		f.State = models.StateInbox
	case "r":
		f.State = models.StateReady
	case "n":
		f.State = models.StateNext
	case "s":
		f.State = models.StateInProgress
	case "w":
		f.State = models.StateWaiting
	case "p":
		f.State = models.StatePaused
	case "v":
		f.State = models.StateReview
	case "m":
		f.State = models.StateSomeday
	case "d":
		f.State = models.StateDone
	case "x":
		f.State = models.StateCanceled
	case "0":
		f.State = ""
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
type filterAttrSpec struct {
	attrName   string
	typeFilter models.TaskType // zero value = don't set type filter
	keyMap     map[string]string
}

var filterAttrSpecs = map[string]filterAttrSpec{
	"Fas": {"bug.severity", models.TaskTypeBug, map[string]string{
		"c": string(models.BugSeverityCritical), "j": string(models.BugSeverityMajor),
		"m": string(models.BugSeverityMinor), "o": string(models.BugSeverityCosmetic),
	}},
	"Far": {"bug.reproducible", models.TaskTypeBug, map[string]string{
		"y": string(models.BugReproducibleYes), "n": string(models.BugReproducibleNo),
		"s": string(models.BugReproducibleSometimes),
	}},
	"Fae": {"effort", "", map[string]string{
		"s": string(models.EffortSmall), "m": string(models.EffortMedium),
		"l": string(models.EffortLarge), "u": string(models.EffortUnknown),
	}},
	"Fac": {"idea.category", models.TaskTypeIdea, map[string]string{
		"f": string(models.IdeaCategoryFeature), "i": string(models.IdeaCategoryImprovement),
		"r": string(models.IdeaCategoryRefactor), "s": string(models.IdeaCategoryResearch),
	}},
	"Fav": {"idea.value", models.TaskTypeIdea, map[string]string{
		"l": string(models.IdeaValueLow), "m": string(models.IdeaValueMedium),
		"h": string(models.IdeaValueHigh),
	}},
	"Faw": {"task.worktype", models.TaskTypeTask, map[string]string{
		"d": string(models.WorkTypeDevelopment), "o": string(models.WorkTypeDocumentation),
		"t": string(models.WorkTypeTesting), "v": string(models.WorkTypeDevOps),
		"e": string(models.WorkTypeDesign),
	}},
	"Fao": {"epic.scope", models.TaskTypeEpic, map[string]string{
		"d": string(models.EpicScopeDefined), "e": string(models.EpicScopeEvolving),
		"u": string(models.EpicScopeUndefined),
	}},
	"Fap": {"story.points", models.TaskTypeStory, map[string]string{
		"1": "1", "2": "2", "3": "3", "5": "5", "8": "8",
	}},
}

func handleFilterAttrSubKey(prefix, key string, f *CommonFilterState) bool {
	spec, ok := filterAttrSpecs[prefix]
	if !ok {
		return false
	}
	if key == "0" {
		delete(f.Attrs, spec.attrName)
		return true
	}
	val, ok := spec.keyMap[key]
	if !ok {
		return false
	}
	f.Attrs[spec.attrName] = val
	if spec.typeFilter != "" {
		f.Type = spec.typeFilter
	}
	return true
}

// handleFilterScoreKey applies a score threshold filter based on the key pressed.
// Returns true if the key was handled.
func handleFilterScoreKey(key string, f *CommonFilterState, machine *config.MachineState) bool {
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
		if machine != nil && machine.ScoreFilterCustom != nil {
			f.ScoreMin = *machine.ScoreFilterCustom
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
func dispatchFilterSubPrefix(prefix, key string, f *CommonFilterState, machine *config.MachineState, setPendingPrefix func(string)) bool {
	switch prefix {
	case "Ft":
		return handleFilterTypeKey(key, f)
	case "Fs":
		return handleFilterStateKey(key, f)
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
		return handleFilterScoreKey(key, f, machine)
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
// across views: p/k (boolean toggles), t/s (sub-prefix), # (tag selector),
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
	case "h":
		f.HideDimmed = !f.HideDimmed
		return FilterPrefixResult{Handled: true, Changed: true}
	case "i":
		f.GitHubIssue = !f.GitHubIssue
		return FilterPrefixResult{Handled: true, Changed: true}
	case "k":
		f.Parked = !f.Parked
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
		stateDetail := "(all)"
		if f.State != "" {
			stateDetail = string(f.State)
		}
		hints = append(hints, PrefixHint{Key: "s", Description: "state", Detail: stateDetail, Active: f.State != ""})
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
		default:
		}
		hints = append(hints, PrefixHint{Key: "d", Description: "due-date", Detail: dueDetail, Active: f.DueDate != DueDateAll})
	}

	dimDetail := "off"
	if f.HideDimmed {
		dimDetail = "on"
	}

	issueDetail := "off"
	if f.GitHubIssue {
		issueDetail = "on"
	}

	parkedDetail := "off"
	if f.Parked {
		parkedDetail = "on"
	}

	hints = append(hints,
		PrefixHint{Key: "a", Description: "attribute", Detail: attrDetail, Active: len(f.Attrs) > 0},
		PrefixHint{Key: "S", Description: "score", Detail: scoreDetail, Active: f.ScoreMin > 0},
		PrefixHint{Key: "h", Description: "dimmed", Detail: dimDetail, Active: f.HideDimmed},
		PrefixHint{Key: "i", Description: "issues", Detail: issueDetail, Active: f.GitHubIssue},
		PrefixHint{Key: "k", Description: "parked", Detail: parkedDetail, Active: f.Parked},
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

// applyFilterSubPrefix handles a filter sub-prefix key press for views whose
// custom-score input is wired through keymap actions. Returns true when the
// sub-prefix was handled (caller already refreshed via refreshView).
func applyFilterSubPrefix(prefix, key string, f *CommonFilterState, machine *config.MachineState, setPendingPrefix func(string), refreshView func()) bool {
	if dispatchFilterSubPrefix(prefix, key, f, machine, setPendingPrefix) {
		refreshView()
		return true
	}
	return false
}

// buildCommonFilterPrefixPopupHints returns the full PrefixHints map for the F prefix
// and its sub-prefixes. Set includeDueDate and includeStatus to control which
// categories are available; set supportsCustomScoreInput true only for views
// that wire FS -> C to a score-input component (project tree and outliner today).
func buildCommonFilterPrefixPopupHints(f *CommonFilterState, machine *config.MachineState, includeDueDate, includeStatus, supportsCustomScoreInput bool) PrefixHints {
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
		"FS":  BuildFilterScorePrefixHints(machine, supportsCustomScoreInput),
	}
	if includeStatus {
		hints["Fs"] = FilterStatePrefixHints
	}
	if includeDueDate {
		hints["Fd"] = FilterDuePrefixHints
	}
	return hints
}
