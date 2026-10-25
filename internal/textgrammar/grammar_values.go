// Package textgrammar owns the inline metadata language shared by capture,
// inline editing, and outline editing.
package textgrammar

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/sglavoie/docket/internal/models"
)

func StateDescription(state models.WorkflowState) string {
	switch state {
	case models.StateInbox:
		return "unprocessed inbox"
	case models.StateBacklog:
		return "backlog"
	case models.StateDoing:
		return "doing"
	case models.StateWaiting:
		return "waiting externally"
	case models.StatePaused:
		return "paused"
	case models.StateSomeday:
		return "deferred someday"
	case models.StateDone:
		return "completed"
	case models.StateCanceled:
		return "canceled"
	default:
		return ""
	}
}

// ParsePriority resolves a canonical numeric priority. Callers may opt into an
// inclusive N-M range for legacy input inspection.
func ParsePriority(value string, allowRange bool) ([]int, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	definition, ok := registrySpecFor(FieldPriority)
	if !ok {
		return nil, false
	}
	if allowRange {
		if parts := strings.Split(value, "-"); len(parts) == 2 {
			start, startErr := strconv.Atoi(parts[0])
			end, endErr := strconv.Atoi(parts[1])
			if startErr == nil && endErr == nil && slices.Contains(definition.Values, parts[0]) && slices.Contains(definition.Values, parts[1]) {
				if start > end {
					start, end = end, start
				}
				result := make([]int, 0, end-start+1)
				for priority := start; priority <= end; priority++ {
					result = append(result, priority)
				}
				return result, true
			}
		}
	}

	priority, err := strconv.Atoi(value)
	if err != nil || !slices.Contains(definition.Values, value) {
		return nil, false
	}
	return []int{priority}, true
}

// searchPriorityThresholdSuffix widens a priority predicate to everything at
// least that urgent. It is search-only for the obvious reason: a threshold is a
// question, and there is no widened value to assign to a task.
const searchPriorityThresholdSuffix = "+"

// ParseSearchPriority resolves one search-surface priority token into the
// priorities it admits. A bare !N admits that priority alone; !N+ admits N and
// everything more urgent, which is every smaller number, because !1 is the most
// urgent. It is the one reader of the "+" suffix, so the gate that filters and
// the diagnostic that refuses cannot disagree about whether !2+ is a legal query.
//
// !0 admits the tasks carrying no priority, spelled as the effective priority an
// unset value reads as, so the gate needs no second case. It takes no threshold:
// nothing ranks below unset, so !0+ would be every task under a narrower name.
func ParseSearchPriority(value string) ([]int, bool) {
	body, threshold := strings.CutSuffix(strings.TrimSpace(value), searchPriorityThresholdSuffix)
	if body == clearValue {
		if threshold {
			return nil, false
		}
		return []int{models.EffectivePriority(models.PriorityUnset)}, true
	}
	priorities, ok := ParsePriority(body, false)
	if !ok || !threshold {
		return priorities, ok
	}
	admitted := make([]int, 0, priorities[0])
	for priority := 1; priority <= priorities[0]; priority++ {
		admitted = append(admitted, priority)
	}
	return admitted, true
}

// InvalidPriorityError formats capture guidance from the canonical values.
func InvalidPriorityError(raw string) error {
	definition, ok := registrySpecFor(FieldPriority)
	if !ok {
		return fmt.Errorf("invalid priority: !%s", raw)
	}
	values := prefixed(definition.Values, definition.Trigger)
	if slices.Contains(definition.RetiredValues, raw) {
		return fmt.Errorf("priority !%s was removed (use %s)", raw, joinOptions(values))
	}
	return fmt.Errorf("invalid priority: !%s (use %s)", raw, joinOptions(values))
}

// InvalidScheduleError includes short, parseable examples from the same
// registry that supplies capture help and completion.
func InvalidScheduleError(field Field, raw string) error {
	name := "date"
	if field == FieldDefer {
		name = "defer"
	}
	definition, ok := registrySpecFor(field)
	if !ok || len(definition.Examples) == 0 {
		return fmt.Errorf("invalid %s: %s", name, raw)
	}
	examples := definition.Examples[:min(3, len(definition.Examples))]
	return fmt.Errorf("invalid %s: %s (use %s)", name, raw, joinOptions(examples))
}

func joinOptions(values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return values[0]
	case 2:
		return values[0] + " or " + values[1]
	default:
		return strings.Join(values[:len(values)-1], ", ") + ", or " + values[len(values)-1]
	}
}

func prefixed(values []string, prefix string) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = prefix + value
	}
	return result
}
