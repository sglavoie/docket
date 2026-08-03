package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

// quickEditResult holds the parsed result of inline task editing.
type quickEditResult struct {
	title    string
	titleSet bool

	priority    int
	prioritySet bool

	tagsToAdd    []string
	tagsToRemove []string

	dueDate      *time.Time
	dueDateSet   bool
	dueDateClear bool

	state    models.WorkflowState
	stateSet bool
}

// parseQuickEdit parses inline task edits through the shared metadata grammar.
func parseQuickEdit(input string, _ *models.Task, project *models.Project) (*quickEditResult, error) {
	result, errMsg := parseQuickEditShared(input, project, true)
	if errMsg != "" {
		return nil, fmt.Errorf("%s", errMsg)
	}
	return result, nil
}

// parseQuickEditForPreview parses input without failing, for live preview.
func parseQuickEditForPreview(input string, _ *models.Task, project *models.Project) (*quickEditResult, string) {
	return parseQuickEditShared(input, project, false)
}

func parseQuickEditShared(input string, _ *models.Project, strict bool) (*quickEditResult, string) {
	parsed := textgrammar.Parse(input, textgrammar.SurfaceEdit)
	result := &quickEditResult{}
	var errMsg string
	setError := func(message string) bool {
		if errMsg == "" {
			errMsg = message
		}
		return strict
	}

	if tokens := parsed.TokensFor(textgrammar.FieldPriority); len(tokens) > 0 {
		if tokens[0].Value == "0" {
			result.priority = models.PriorityUnset
			result.prioritySet = true
		} else {
			values, ok := textgrammar.ParsePriority(tokens[0].Value, false)
			if !ok {
				if setError(fmt.Sprintf("invalid priority: %s (use 0-3)", tokens[0].Value)) {
					return result, errMsg
				}
			} else {
				result.priority = values[0]
				result.prioritySet = true
			}
		}
	}

	for _, token := range parsed.TokensFor(textgrammar.FieldRemoveTag) {
		result.tagsToRemove = append(result.tagsToRemove, token.Value)
	}
	for _, token := range parsed.TokensFor(textgrammar.FieldTag) {
		result.tagsToAdd = append(result.tagsToAdd, token.Value)
	}

	if tokens := aliasFirst(parsed.TokensFor(textgrammar.FieldDue), "@"); len(tokens) > 0 {
		token := tokens[len(tokens)-1]
		if token.Value == "0" {
			result.dueDateClear = true
			result.dueDateSet = true
		} else {
			date, err := ParseDate(token.Value)
			if err != nil {
				if setError(fmt.Sprintf("invalid date: %s", token.Value)) {
					return result, errMsg
				}
			} else {
				result.dueDate = &date
				result.dueDateSet = true
			}
		}
	}

	for _, token := range aliasFirst(parsed.TokensFor(textgrammar.FieldState), "$") {
		match, ok := textgrammar.ParseState(textgrammar.SurfaceEdit, token.Value)
		if !ok {
			if setError(textgrammar.InvalidStateError(token.Value).Error()) {
				return result, errMsg
			}
			continue
		}
		result.state = match.States[0]
		result.stateSet = true
	}

	result.title = parsed.Text
	result.titleSet = true
	if result.title == "" && errMsg == "" {
		errMsg = "task title cannot be empty"
	}
	return result, errMsg
}

// applyQuickEdit applies parsed changes atomically in memory after validating
// the exact canonical state transition.
func applyQuickEdit(task *models.Task, project *models.Project, result *quickEditResult) string {
	targetState, hasTargetState := result.state, result.stateSet
	if hasTargetState {
		if msg := models.CanTransitionState(task.State(), targetState); msg != "" {
			return msg
		}
	}

	project.MutateTask(task.ID, func(t *models.Task) {
		if result.titleSet && result.title != "" && models.ValidateTaskTitle(result.title) == nil {
			t.Title = result.title
		}
		if result.prioritySet {
			t.Priority = result.priority
		}
		for _, tag := range result.tagsToRemove {
			t.RemoveTag(tag)
		}
		for _, tag := range result.tagsToAdd {
			t.AddTag(tag)
		}
		if result.dueDateSet {
			if result.dueDateClear {
				t.DueDate = nil
			} else {
				t.DueDate = result.dueDate
			}
		}
		if hasTargetState {
			_ = t.ApplyStateTransition(targetState)
		}
	})
	return ""
}

// renderQuickEditPreview renders canonical metadata chips for changed fields.
func renderQuickEditPreview(result *quickEditResult, original *quickEditResult) string {
	if result == nil {
		return ""
	}

	var parts []string
	if result.prioritySet && result.priority != original.priority {
		parts = append(parts, fmt.Sprintf("!%d", result.priority))
	}
	for _, tag := range result.tagsToAdd {
		parts = append(parts, "+#"+tag)
	}
	for _, tag := range result.tagsToRemove {
		parts = append(parts, "-#"+tag)
	}
	if result.dueDateSet {
		if result.dueDateClear {
			parts = append(parts, "@clear")
		} else if result.dueDate != nil {
			parts = append(parts, "@"+result.dueDate.Format("Jan 2"))
		}
	}
	if result.stateSet && result.state != original.state {
		parts = append(parts, "$"+result.state.String())
	}

	if len(parts) == 0 {
		if result.titleSet && result.title != original.title {
			return "(title changed)"
		}
		return "(no changes)"
	}
	return "-> " + strings.Join(parts, " ")
}
