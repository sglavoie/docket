package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

// getTagSuggestions returns matching tags for the partial query.
func (q *quickCaptureInput) getTagSuggestions(partial string) []ACSuggestion {
	type scoredTag struct {
		tag   string
		score int
	}

	var scored []scoredTag
	for _, tag := range q.tagCache {
		score := fuzzy.Score(tag, partial)
		if score > 0 {
			scored = append(scored, scoredTag{tag: tag, score: score})
		}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return top matches (no hard limit, scrollable)
	suggestions := make([]ACSuggestion, 0, len(scored))
	for _, s := range scored {
		suggestions = append(suggestions, ACSuggestion{
			Display: "#" + s.tag,
			Value:   s.tag,
		})
	}
	return suggestions
}

// getPrioritySuggestions returns matching priority options for the partial query.
func (q *quickCaptureInput) getPrioritySuggestions(partial string) []ACSuggestion {
	descriptions := map[string]string{
		"1": "High",
		"2": "Medium",
		"3": "Low",
	}
	var suggestions []ACSuggestion
	def, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldPriority)
	for _, value := range def.Values {
		desc, canonical := descriptions[value]
		if !canonical {
			continue
		}
		if partial == "" || strings.HasPrefix(value, partial) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("!%s (%s)", value, desc),
				Value:   value,
			})
		}
	}
	return suggestions
}

// getDateSuggestions returns common due date options for the partial query.
func (q *quickCaptureInput) getDateSuggestions(partial string) []ACSuggestion {
	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)
	def, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldDue)
	for _, value := range def.Values {
		if partial == "" || strings.Contains(value, partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: "@" + value,
				Value:   value,
			})
		}
	}
	return suggestions
}

// getDestinationSuggestions returns lifecycle and planning destinations from
// the shared capture grammar.
func (q *quickCaptureInput) getDestinationSuggestions(partial string) []ACSuggestion {
	def, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldDestination)
	suggestions := make([]ACSuggestion, 0, len(def.Values))
	partialLower := strings.ToLower(partial)
	for _, name := range def.Values {
		if partial != "" && !strings.Contains(name, partialLower) {
			continue
		}
		destination, _ := app.ParseCaptureDestination(name)
		description := textgrammar.StateDescription(destination.WorkflowState())
		switch destination {
		case app.CaptureDestinationToday:
			description = "add to Today"
		case app.CaptureDestinationNext:
			description = "add to Next"
		}
		display := fmt.Sprintf("$%s %s (%s)", name, destination.WorkflowState().Icon(), description)
		suggestions = append(suggestions, ACSuggestion{Display: display, Value: name})
	}
	return suggestions
}

// getParentSuggestions returns matching parent tasks for the partial query.
func (q *quickCaptureInput) getParentSuggestions(partial string) []ACSuggestion {
	type scoredParent struct {
		task  *models.Task
		score int
	}

	var scored []scoredParent
	for _, task := range q.parentCache {
		searchText := fmt.Sprintf("#%d %s", task.ShortID, task.Title)
		score := fuzzy.Score(searchText, partial)
		if score > 0 {
			scored = append(scored, scoredParent{task: task, score: score})
		}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return all matches (scrollable)
	suggestions := make([]ACSuggestion, 0, len(scored))
	for _, s := range scored {
		task := s.task
		// Truncate title if too long (width-aware).
		titleLimit := max(q.width-15, 10)
		title := truncateToVisualWidth(task.Title, titleLimit)
		titleRendered := TaskTitleStyleFor(task, TaskTitleStyleOptions{}).Render(title)
		suggestions = append(suggestions, ACSuggestion{
			Display: fmt.Sprintf("#%d %s", task.ShortID, titleRendered),
			Value:   strconv.Itoa(task.ShortID),
		})
	}
	return suggestions
}

// getProjectSuggestions returns matching projects for the partial query.
func (q *quickCaptureInput) getProjectSuggestions(partial string) []ACSuggestion {
	type scoredProject struct {
		project *models.Project
		score   int
	}

	var scored []scoredProject
	for _, project := range q.projectCache {
		// Build searchable text: "Name [context]"
		searchText := fmt.Sprintf("%s %s", project.Name, project.Context)
		score := fuzzy.Score(searchText, partial)
		if score > 0 {
			scored = append(scored, scoredProject{project: project, score: score})
		}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return all matches (scrollable)
	suggestions := make([]ACSuggestion, 0, len(scored))
	for _, s := range scored {
		project := s.project
		// Truncate name if too long (width-aware: ~20 chars overhead for context+brackets+spaces)
		nameLimit := max(q.width-20, 10)
		name := truncateToVisualWidth(project.Name, nameLimit)
		suggestions = append(suggestions, ACSuggestion{
			Display: fmt.Sprintf("%%%s [%s]", name, project.Context),
			Value:   project.Slug,
		})
	}
	return suggestions
}

// acceptSuggestion inserts the selected suggestion into the input.
func (q *quickCaptureInput) acceptSuggestion() {
	if !q.ac.IsActive() {
		return
	}

	triggerChar := TriggerCharForMode(q.ac.Mode, q.triggers)
	newInput := q.ac.BuildAcceptedInput(q.input.Value(), triggerChar)

	q.input.SetValue(newInput)
	// Move cursor to end
	q.input.CursorEnd()

	// Reset autocomplete state
	q.ac.Clear()
}
