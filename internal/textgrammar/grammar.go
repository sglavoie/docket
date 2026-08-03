// Package textgrammar owns the inline metadata language shared by capture and
// quick edit.
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
)

// Field identifies one family of inline metadata tokens.
type Field string

const (
	FieldPriority    Field = "priority"
	FieldTag         Field = "tag"
	FieldRemoveTag   Field = "remove_tag"
	FieldDue         Field = "due"
	FieldRecurrence  Field = "recurrence"
	FieldTaskType    Field = "type"
	FieldParent      Field = "parent"
	FieldProject     Field = "project"
	FieldDestination Field = "destination"
	FieldState       Field = "state"
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

var priorityNames = []string{"1", "2", "3"}

var captureDueValues = []string{
	"today", "tomorrow", "+1d", "+3d", "+1w", "+2w", "+1m",
	"monday", "friday", "next monday", "next friday", "in 3 days", "in 1 week", "in 2 weeks",
}

var captureDestinationNames = []string{
	"inbox", "backlog", "today", "next", "doing", "waiting", "paused", "someday", "done", "canceled",
}

// Definitions returns defensive copies of the canonical definitions available
// on a surface.
func Definitions(surface Surface) []Definition {
	stateNames := StateNames()

	defs := []Definition{
		{
			Field: FieldPriority, Group: "Priority & Tags", Forms: []string{"!1-3"},
			Label: "Priority", Description: "Priority (1-3)", Values: slices.Clone(priorityNames),
			Examples: []string{"!1", "!2", "!3"},
			Surfaces: SurfaceCapture | SurfaceEdit,
		},
		{
			Field: FieldTag, Group: "Priority & Tags", Forms: []string{"#tag"},
			Label: "Tag", Description: "Add tag", Examples: []string{"#backend", "#bug", "#source:github"},
			Repeatable: true, Surfaces: SurfaceCapture | SurfaceEdit,
		},
		{
			Field: FieldDue, Group: "Schedule", Forms: []string{"@date"},
			Label: "Due", Description: "Due date", Values: slices.Clone(captureDueValues),
			Examples: []string{"@tomorrow", "@friday", "@+3d", "@next monday"},
			Surfaces: SurfaceCapture | SurfaceEdit,
		},
		{
			Field: FieldDestination, Group: "Destination", Forms: []string{"$destination"},
			Label: "Destination", Description: "Lifecycle or planning destination", Values: slices.Clone(captureDestinationNames),
			Examples: prefixed(captureDestinationNames, "$"), Surfaces: SurfaceCapture,
		},
		{
			Field: FieldProject, Group: "Project Routing", Forms: []string{"%project"},
			Label: "Project", Description: "Target project", Examples: []string{"%project-slug"},
			Surfaces: SurfaceCapture,
		},
		{
			Field: FieldParent, Group: "Placement", Forms: []string{">parent"},
			Label: "Parent", Description: "Parent task", Examples: []string{">1", ">Login"},
			Surfaces: SurfaceCapture,
		},
		{
			Field: FieldState, Group: "State", Forms: []string{"$state"},
			Label: "State", Description: "Canonical workflow state", Values: stateNames,
			Examples: prefixed(stateNames, "$"), Surfaces: SurfaceEdit,
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

// StateNames returns all eight canonical state names in model order.
func StateNames() []string {
	states := models.AllWorkflowStates()
	result := make([]string, len(states))
	for i, state := range states {
		result[i] = state.String()
	}
	return result
}

// StateDescription returns concise presentation text for a canonical state.
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

// StateMatch is the canonical interpretation of a state token.
type StateMatch struct {
	States []models.WorkflowState
	Legacy bool
}

// ParseState resolves exact canonical state names on every text surface.
func ParseState(surface Surface, value string) (StateMatch, bool) {
	if surface&SurfaceEdit == 0 {
		return StateMatch{}, false
	}
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
		"todo":      "$inbox or $backlog",
		"active":    "$backlog",
		"start":     "$doing",
		"started":   "$doing",
		"progress":  "$doing",
		"blocked":   "$waiting",
		"standby":   "$paused",
		"hold":      "$paused",
		"review":    "$backlog",
		"maybe":     "$someday",
		"complete":  "$done",
		"completed": "$done",
		"cancel":    "$canceled",
		"cancelled": "$canceled", //nolint:misspell // Intentional rejected-input alias.
	}
	replacement, ok := replacements[strings.ToLower(strings.TrimSpace(raw))]
	return replacement, ok
}

// ParsePriority resolves a canonical numeric priority. Callers may opt into an
// inclusive N-M range for legacy input inspection.
func ParsePriority(value string, allowRange bool) ([]int, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if allowRange {
		if parts := strings.Split(value, "-"); len(parts) == 2 {
			start, startErr := strconv.Atoi(parts[0])
			end, endErr := strconv.Atoi(parts[1])
			if startErr == nil && endErr == nil && start >= 1 && start <= 3 && end >= 1 && end <= 3 {
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
	if err != nil || priority < 1 || priority > 3 {
		return nil, false
	}
	return []int{priority}, true
}

func prefixed(values []string, prefix string) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = prefix + value
	}
	return result
}
