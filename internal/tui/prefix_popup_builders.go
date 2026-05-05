package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/config"
)

// BuildPriorityPrefixHints returns priority hints with colored icons and
// the current priority marked as active.
func BuildPriorityPrefixHints(currentPriority int) []PrefixHint {
	hints := make([]PrefixHint, len(PriorityPrefixHintsBase))
	copy(hints, PriorityPrefixHintsBase)
	for i := range hints {
		priority := i + 1
		hints[i].RenderedIcon = GetPriorityStyle(priority).Render(PriorityIcon(priority))
		hints[i].Active = currentPriority == priority
	}
	return hints
}

// CommonTaskActionPrefixHints returns the prefix hints shared across most
// task-operating views: status (,), priority (!), stage (.), due date (d),
// snooze (s), create-linked (c), and move (m). Views extend the returned
// map with their view-specific entries (e.g. "g" navigation, "z" folds).
func CommonTaskActionPrefixHints(stages []string, currentStage string, currentPriority int) PrefixHints {
	return PrefixHints{
		",": StatusPrefixHints,
		"!": BuildPriorityPrefixHints(currentPriority),
		".": BuildStagePrefixHints(stages, currentStage),
		"d": DueDatePrefixHints,
		"s": SnoozePrefixHints,
		"c": CreateLinkedPrefixHints,
		"m": SecondaryViewMovePrefixHints,
		"t": TaskPrefixHints,
		"b": BlockerPrefixHints,
	}
}

// FullPrefixHints builds the complete prefix hint map for a view.
// Base includes all task action hints. Extra hints (g, z, F-sub) are view-specific.
func FullPrefixHints(stages []string, currentStage string, currentPriority int, extras PrefixHints) PrefixHints {
	base := CommonTaskActionPrefixHints(stages, currentStage, currentPriority)
	for k, v := range extras {
		base[k] = v
	}
	return base
}

// overrideHintDetail returns a shallow copy of hints with the Detail field of
// the entry matching key replaced. If key is not found, the original slice is returned.
func overrideHintDetail(hints []PrefixHint, key, detail string) []PrefixHint {
	result := make([]PrefixHint, len(hints))
	copy(result, hints)
	for i, h := range result {
		if h.Key == key {
			result[i].Detail = detail
			return result
		}
	}
	return result
}

// BuildFilterScorePrefixHints builds dynamic filter score hints showing the
// current custom value from config. Used by project tree and outliner views.
func BuildFilterScorePrefixHints(cfg *config.Config) []PrefixHint {
	customDetail := "(not set)"
	if cfg != nil && cfg.ScoreFilterCustom != nil {
		customDetail = fmt.Sprintf(">=%.0f", *cfg.ScoreFilterCustom)
	}

	return []PrefixHint{
		{Key: "1", Description: ">=30"},
		{Key: "2", Description: ">=60"},
		{Key: "3", Description: ">=90"},
		{Key: "4", Description: ">=120"},
		{Key: "5", Description: ">=150"},
		{Key: "6", Description: ">=180"},
		{Key: "7", Description: ">=210"},
		{Key: "8", Description: ">=240"},
		{Key: "9", Description: ">=270"},
		{Key: "c", Description: "custom", Detail: customDetail},
		{Key: "C", Description: "set value", Detail: "inline input"},
		{Key: "0", Description: "clear"},
	}
}

// BuildStagePrefixHints builds dynamic stage hints for the "." (stage) prefix.
// Keys 1-9 map to the first 9 configured stages, 0 clears.
func BuildStagePrefixHints(stages []string, currentStage string) []PrefixHint {
	hints := make([]PrefixHint, 0, len(stages)+1)
	for i, stage := range stages {
		hints = append(hints, PrefixHint{
			Key:         stageKeyForIndex(i),
			Description: stage,
			Active:      stage == currentStage,
		})
	}
	hints = append(hints, PrefixHint{Key: "0", Description: "clear"})
	return hints
}

// BuildFilterStagePrefixHints builds dynamic filter stage hints for the "F." prefix.
// Keys 1-9 map to the first 9 configured stages, 0 clears.
func BuildFilterStagePrefixHints(stages []string, currentFilter string) []PrefixHint {
	hints := make([]PrefixHint, 0, len(stages)+1)
	for i, stage := range stages {
		hints = append(hints, PrefixHint{
			Key:         stageKeyForIndex(i),
			Description: stage,
			Active:      stage == currentFilter,
		})
	}
	hints = append(hints, PrefixHint{Key: "0", Description: "clear"})
	return hints
}
