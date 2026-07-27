// Package textgrammar owns the inline metadata language shared by capture,
// quick edit, and structured search.
package textgrammar

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// Surface identifies a consumer of the shared inline grammar.
type Surface uint8

const (
	SurfaceCapture Surface = 1 << iota
	SurfaceEdit
	SurfaceSearch
)

// Field identifies one family of inline metadata tokens.
type Field string

const (
	FieldPriority   Field = "priority"
	FieldTag        Field = "tag"
	FieldRemoveTag  Field = "remove_tag"
	FieldDue        Field = "due"
	FieldRecurrence Field = "recurrence"
	FieldTaskType   Field = "type"
	FieldParent     Field = "parent"
	FieldProject    Field = "project"
	FieldState      Field = "state"
	FieldHas        Field = "has"
	FieldNo         Field = "no"
	FieldIs         Field = "is"
)

// Definition supplies parser-adjacent UI metadata. Help, completion, and
// previews consume these definitions instead of maintaining their own value
// lists.
type Definition struct {
	Field       Field
	Group       string
	Forms       []string
	Label       string
	Description string
	Values      []string
	Examples    []string
	Repeatable  bool
	Legacy      bool
	Surfaces    Surface
}

var priorityNames = []string{
	"1", "2", "3", "4", "5",
	"critical", "urgent", "high", "normal", "medium", "low", "lowest",
	"c", "cr", "u", "ur", "h", "hi", "n", "no", "m", "me", "med", "l", "lo",
}

var captureDueValues = []string{
	"today", "tomorrow", "+1d", "+3d", "+1w", "+2w", "+1m",
	"monday", "friday", "next monday", "next friday", "in 3 days", "in 1 week", "in 2 weeks",
}

var searchDueValues = []string{"today", "tomorrow", "week", "overdue", "none"}

var recurrenceValues = []string{
	"daily", "weekly", "biweekly", "monthly", "yearly", "2d", "3d", "2w", "3w",
}

var hasValues = []string{
	"description", "comment", "checklist", "attachment", "issue", "due",
	"dependency", "relation", "recurrence", "url", "parent", "children", "tag",
}

var isValues = []string{
	"note", "pinned", "dimmed", "blocked", "overdue", "snoozed", "recurring", "leaf", "root",
}

// Definitions returns defensive copies of the definitions available on a
// surface. Legacy definitions are included so callers can decide whether to
// advertise compatibility forms.
func Definitions(surface Surface) []Definition {
	typeNames := TaskTypeNames()
	stateNames := StateNames()
	dueValues := captureDueValues
	if surface == SurfaceSearch {
		dueValues = searchDueValues
		typeNames = append(typeNames, "note")
	}

	defs := []Definition{
		{
			Field: FieldTaskType, Group: "Basics", Forms: []string{"^type", "type:"},
			Label: "Type", Description: "Task type", Values: typeNames,
			Examples: prefixed(typeNames[:min(3, len(typeNames))], "^"),
			Surfaces: SurfaceCapture | SurfaceSearch,
		},
		{
			Field: FieldPriority, Group: "Priority & Tags", Forms: []string{"!", "pri:", "priority:"},
			Label: "Priority", Description: "Priority (1-5)", Values: slices.Clone(priorityNames),
			Examples: []string{"!1", "!urgent", "!normal", "!low", "pri:2", "priority:high"},
			Surfaces: SurfaceCapture | SurfaceEdit | SurfaceSearch,
		},
		{
			Field: FieldTag, Group: "Priority & Tags", Forms: []string{"#", "tag:"},
			Label: "Tag", Description: "Add tag", Examples: []string{"#backend", "#urgent", "tag:api"},
			Repeatable: true, Surfaces: SurfaceCapture | SurfaceEdit | SurfaceSearch,
		},
		{
			Field: FieldRemoveTag, Group: "Priority & Tags", Forms: []string{"-#tag"},
			Label: "Remove tag", Description: "Remove tag", Examples: []string{"-#blocked"},
			Repeatable: true, Surfaces: SurfaceEdit,
		},
		{
			Field: FieldDue, Group: "Schedule", Forms: []string{"@", "due:"},
			Label: "Due", Description: "Due date", Values: slices.Clone(dueValues),
			Examples: []string{"@tomorrow", "@friday", "@+3d", "@next monday", "due:2w"},
			Surfaces: SurfaceCapture | SurfaceEdit | SurfaceSearch,
		},
		{
			Field: FieldRecurrence, Group: "Schedule", Forms: []string{"~", "recur:"},
			Label: "Recurrence", Description: "Recurrence", Values: slices.Clone(recurrenceValues),
			Examples: []string{"~daily", "~weekly", "~2w", "recur:monthly", "~weekly:mon,fri"},
			Surfaces: SurfaceCapture,
		},
		{
			Field: FieldParent, Group: "Placement", Forms: []string{">", "parent:"},
			Label: "Parent", Description: "Parent task", Examples: []string{">1", ">Login", "parent:2"},
			Surfaces: SurfaceCapture,
		},
		{
			Field: FieldProject, Group: "Project Routing", Forms: []string{"%", "p:", "proj:"},
			Label: "Project", Description: "Target project", Examples: []string{"%slug", "p:slug", "proj:name"},
			Surfaces: SurfaceCapture | SurfaceSearch,
		},
		{
			Field: FieldState, Group: "State", Forms: []string{"$", "state:"},
			Label: "State", Description: "Canonical workflow state", Values: stateNames,
			Examples: prefixed(stateNames, "$"), Surfaces: SurfaceCapture | SurfaceEdit | SurfaceSearch,
		},
		{
			Field: FieldHas, Group: "Properties", Forms: []string{"has:X"},
			Label: "Has", Description: "Task has property", Values: slices.Clone(hasValues),
			Surfaces: SurfaceSearch,
		},
		{
			Field: FieldNo, Group: "Properties", Forms: []string{"no:X"},
			Label: "No", Description: "Task lacks property", Values: slices.Clone(hasValues),
			Surfaces: SurfaceSearch,
		},
		{
			Field: FieldIs, Group: "Properties", Forms: []string{"is:X"},
			Label: "Is", Description: "Task matches derived property", Values: slices.Clone(isValues),
			Surfaces: SurfaceSearch,
		},
	}

	result := make([]Definition, 0, len(defs))
	for _, def := range defs {
		if def.Surfaces&surface == 0 {
			continue
		}
		def.Forms = slices.Clone(def.Forms)
		def.Values = slices.Clone(def.Values)
		def.Examples = slices.Clone(def.Examples)
		result = append(result, def)
	}
	return result
}

// DefinitionFor returns the definition for a field on a surface.
func DefinitionFor(surface Surface, field Field) (Definition, bool) {
	for _, def := range Definitions(surface) {
		if def.Field == field {
			return def, true
		}
	}
	return Definition{}, false
}

// StateNames returns all ten canonical state names in model order.
func StateNames() []string {
	states := models.AllWorkflowStates()
	result := make([]string, len(states))
	for i, state := range states {
		result[i] = state.String()
	}
	return result
}

// TaskTypeNames returns every valid task type in model order.
func TaskTypeNames() []string {
	types := models.AllTaskTypes()
	result := make([]string, len(types))
	for i, taskType := range types {
		result[i] = taskType.String()
	}
	return result
}

// StateDescription returns concise presentation text for a canonical state.
func StateDescription(state models.WorkflowState) string {
	switch state {
	case models.StateInbox:
		return "unprocessed inbox"
	case models.StateReady:
		return "ready for planning"
	case models.StateNext:
		return "next up"
	case models.StateInProgress:
		return "in progress"
	case models.StateWaiting:
		return "waiting externally"
	case models.StatePaused:
		return "paused"
	case models.StateReview:
		return "awaiting review"
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

// StateMatch is the canonical interpretation of a state token.
type StateMatch struct {
	States []models.WorkflowState
	Legacy bool
}

// ParseState resolves exact canonical state names on every text surface.
func ParseState(_ Surface, value string) (StateMatch, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if state, ok := models.ParseWorkflowState(value); ok {
		return StateMatch{States: []models.WorkflowState{state}}, true
	}
	return StateMatch{}, false
}

// InvalidStateError formats the shared canonical-state validation error.
func InvalidStateError(raw string) error {
	if replacement, ok := retiredStateReplacement(raw); ok {
		return fmt.Errorf("state $%s was removed (use %s)", raw, replacement)
	}
	return fmt.Errorf(
		"invalid state: $%s (use %s)",
		raw,
		strings.Join(prefixed(StateNames(), "$"), ", "),
	)
}

func retiredStateReplacement(raw string) (string, bool) {
	replacements := map[string]string{
		"todo":      "$inbox, $ready, or $next",
		"active":    "$ready",
		"start":     "$in_progress",
		"started":   "$in_progress",
		"progress":  "$in_progress",
		"blocked":   "$waiting",
		"standby":   "$paused",
		"hold":      "$paused",
		"maybe":     "$someday",
		"complete":  "$done",
		"completed": "$done",
		"cancel":    "$canceled",
		"cancelled": "$canceled", //nolint:misspell // Intentional rejected-input alias.
	}
	replacement, ok := replacements[strings.ToLower(strings.TrimSpace(raw))]
	return replacement, ok
}

// ParsePriority resolves a numeric or named priority. Search may opt into an
// inclusive N-M range.
func ParsePriority(value string, allowRange bool) ([]int, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if allowRange {
		if parts := strings.Split(value, "-"); len(parts) == 2 {
			start, startErr := strconv.Atoi(parts[0])
			end, endErr := strconv.Atoi(parts[1])
			if startErr == nil && endErr == nil && start >= 1 && start <= 5 && end >= 1 && end <= 5 {
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

	aliases := map[string]int{
		"critical": 1, "urgent": 1, "high": 1,
		"c": 1, "cr": 1, "u": 1, "ur": 1, "h": 1, "hi": 1,
		"normal": 3, "medium": 3,
		"n": 3, "no": 3, "m": 3, "me": 3, "med": 3,
		"low": 5, "lowest": 5, "l": 5, "lo": 5,
	}
	if priority, ok := aliases[value]; ok {
		return []int{priority}, true
	}
	priority, err := strconv.Atoi(value)
	if err != nil || priority < 1 || priority > 5 {
		return nil, false
	}
	return []int{priority}, true
}

// ContainsDefinitionValue reports whether value is advertised for a field.
func ContainsDefinitionValue(surface Surface, field Field, value string) bool {
	def, ok := DefinitionFor(surface, field)
	return ok && slices.Contains(def.Values, strings.ToLower(value))
}

func prefixed(values []string, prefix string) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = prefix + value
	}
	return result
}
