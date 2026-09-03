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
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// getTagSuggestions returns matching tags for the partial query.
func (q *quickCaptureInput) getTagSuggestions(partial string) []ACSuggestion {
	def, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldTag)
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
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return top matches (no hard limit, scrollable)
	suggestions := make([]ACSuggestion, 0, len(scored))
	for _, s := range scored {
		suggestions = append(suggestions, ACSuggestion{
			Display: def.Trigger + s.tag,
			Value:   s.tag,
		})
	}
	return suggestions
}

// getContextSuggestions returns matching task contexts for the partial query.
func (q *quickCaptureInput) getContextSuggestions(partial string) []ACSuggestion {
	def, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldContext)
	type scoredContext struct {
		context string
		score   int
	}

	var scored []scoredContext
	for _, context := range q.contextCache {
		score := fuzzy.Score(context, partial)
		if score > 0 {
			scored = append(scored, scoredContext{context: context, score: score})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	suggestions := make([]ACSuggestion, 0, len(scored))
	for _, item := range scored {
		suggestions = append(suggestions, ACSuggestion{
			Display: def.Trigger + item.context,
			Value:   item.context,
		})
	}
	return suggestions
}

// getPrioritySuggestions returns matching priority options for the partial query.
func (q *quickCaptureInput) getPrioritySuggestions(partial string) []ACSuggestion {
	var suggestions []ACSuggestion
	def, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldPriority)
	for _, value := range def.Values {
		priorities, ok := textgrammar.ParsePriority(value, false)
		if !ok || len(priorities) != 1 {
			continue
		}
		if partial == "" || strings.HasPrefix(value, partial) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("%s%s (%s)", def.Trigger, value, models.PriorityName(priorities[0])),
				Value:   value,
			})
		}
	}
	return suggestions
}

// getPinSuggestions returns the pin options for the partial query.
func (q *quickCaptureInput) getPinSuggestions(partial string) []ACSuggestion {
	def, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldPinned)
	suggestions := make([]ACSuggestion, 0, len(def.Values))
	partialLower := strings.ToLower(partial)
	for _, value := range def.Values {
		if partial != "" && !strings.HasPrefix(value, partialLower) {
			continue
		}
		suggestions = append(suggestions, ACSuggestion{
			Display: def.Trigger + value,
			Value:   value,
		})
	}
	return suggestions
}

// getReviewSuggestions returns the review options for the partial query. It
// reads the capture definition for the same reason the pin one does: the
// advertised vocabulary belongs to the registry, not to this list.
func (q *quickCaptureInput) getReviewSuggestions(partial string) []ACSuggestion {
	def, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldReview)
	suggestions := make([]ACSuggestion, 0, len(def.Values))
	partialLower := strings.ToLower(partial)
	for _, value := range def.Values {
		if partial != "" && !strings.HasPrefix(value, partialLower) {
			continue
		}
		suggestions = append(suggestions, ACSuggestion{
			Display: def.Trigger + value,
			Value:   value,
		})
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
				Display: def.Trigger + value,
				Value:   value,
			})
		}
	}
	return suggestions
}

// getDeferSuggestions returns common deferral targets for the partial query.
func (q *quickCaptureInput) getDeferSuggestions(partial string) []ACSuggestion {
	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)
	def, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldDefer)
	for _, value := range def.Values {
		if partial == "" || strings.Contains(value, partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: def.Trigger + value,
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
		default:
			// Lifecycle destinations use the shared state description above.
		}
		display := fmt.Sprintf("%s%s %s (%s)", def.Trigger, name, destination.WorkflowState().Icon(), description)
		suggestions = append(suggestions, ACSuggestion{Display: display, Value: name})
	}
	return suggestions
}

// getParentSuggestions returns matching parent tasks for the partial query.
func (q *quickCaptureInput) getParentSuggestions(partial string, candidates []*models.Task) []ACSuggestion {
	return q.structuralSuggestions(partial, candidates, "#")
}

// getChildSuggestions returns matching adoptable tasks for the partial query,
// rendered with the child sigil so a suggestion reads as the token it inserts.
func (q *quickCaptureInput) getChildSuggestions(partial string, candidates []*models.Task) []ACSuggestion {
	definition, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldChild)
	return q.structuralSuggestions(partial, candidates, definition.Trigger)
}

// structuralSuggestions ranks candidate tasks for a >parent or <child reference.
// Both families score one search text, so the two relationships can never rank
// the same project differently; only the rendered prefix distinguishes them.
func (q *quickCaptureInput) structuralSuggestions(partial string, candidates []*models.Task, displayPrefix string) []ACSuggestion {
	type scoredTask struct {
		task  *models.Task
		score int
	}

	var scored []scoredTask
	for _, task := range candidates {
		searchText := fmt.Sprintf("#%d %s", task.ShortID, task.Title)
		score := fuzzy.Score(searchText, partial)
		if score > 0 {
			scored = append(scored, scoredTask{task: task, score: score})
		}
	}

	// Sort by score descending. The sort is stable because an empty partial
	// scores every candidate identically, and an unordered list of tied tasks
	// would reshuffle itself between keystrokes.
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return all matches (scrollable)
	suggestions := make([]ACSuggestion, 0, len(scored))
	for _, s := range scored {
		task := s.task
		// Truncate title if too long (width-aware).
		titleLimit := max(q.width-15, 10)
		title := textfit.TruncateToVisualWidth(task.Title, titleLimit)
		titleRendered := theme.TaskTitleStyleFor(task, theme.TaskTitleStyleOptions{}).Render(title)
		suggestions = append(suggestions, ACSuggestion{
			Display: fmt.Sprintf("%s%d %s", displayPrefix, task.ShortID, titleRendered),
			Value:   strconv.Itoa(task.ShortID),
		})
	}
	return suggestions
}

// getDependencySuggestions offers every open task the workspace loaded, not just
// the drafted line's project: a dependency is the one capture reference that may
// cross a project boundary, so withholding other projects would hide exactly
// what the qualified alias exists for. Capture has no stored set, so there is
// nothing to remove and no ~0 row.
func (q *quickCaptureInput) getDependencySuggestions(partial string, parsedLine textgrammar.Result) []ACSuggestion {
	definition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldDependency)
	if !ok {
		return nil
	}
	effective := q.effectiveProject(parsedLine)
	used := draftedDependencyLocators(parsedLine, textgrammar.FieldDependency, effective, q.allProjects)
	candidates := dependencyAdditionCandidates(q.dependencyProjects(), "", used)
	return dependencySuggestions(candidates, partial, effective, definition.Trigger, q.width-25)
}

// dependencyProjects is the loaded, non-archived population a dependency
// completion searches. It falls back to the composer's own project so a host
// launched without a project list still completes locally.
func (q *quickCaptureInput) dependencyProjects() []*models.Project {
	if len(q.projectCache) > 0 {
		return q.projectCache
	}
	if q.project != nil {
		return []*models.Project{q.project}
	}
	return nil
}

// getProjectSuggestions returns matching projects for the partial query.
func (q *quickCaptureInput) getProjectSuggestions(partial string) []ACSuggestion {
	def, _ := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldProject)
	type scoredProject struct {
		project *models.Project
		score   int
	}

	var scored []scoredProject
	for _, project := range q.projectCache {
		score := fuzzy.Score(project.Name, partial)
		if score > 0 {
			scored = append(scored, scoredProject{project: project, score: score})
		}
	}

	// Sort by score descending
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return all matches (scrollable)
	suggestions := make([]ACSuggestion, 0, len(scored))
	for _, s := range scored {
		project := s.project
		nameLimit := max(q.width-20, 10)
		name := textfit.TruncateToVisualWidth(project.Name, nameLimit)
		suggestions = append(suggestions, ACSuggestion{
			Display: def.Trigger + name,
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

	triggerText := TriggerCharForMode(q.ac.Mode, q.triggers)
	newInput := q.ac.BuildAcceptedInput(q.input.Value(), triggerText)

	q.input.SetValue(newInput)
	// Move cursor to end
	q.input.CursorEnd()

	// Reset autocomplete state
	q.ac.Clear()
}
