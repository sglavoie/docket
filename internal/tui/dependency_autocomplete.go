package tui

import (
	"fmt"
	"sort"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

// dependencyCandidate is one row a ~ or -~ completion may offer. A removal
// candidate whose target this workspace never loaded carries the locator alone,
// because a stored reference must stay droppable even when nothing can be said
// about what it points at.
type dependencyCandidate struct {
	project *models.Project
	task    *models.Task
	locator app.TaskLocator
}

// dependencyAdditionCandidates offers every open task in every loaded project.
// An addition records a soft reference, so the only requirements are the ones
// the application layer enforces: a real, loaded, still-open target that is not
// the source task and is not already referenced.
func dependencyAdditionCandidates(projects []*models.Project, sourceTaskID string, excluded map[app.TaskLocator]bool) []dependencyCandidate {
	candidates := make([]dependencyCandidate, 0, len(projects))
	for _, project := range projects {
		if project == nil || project.Archived {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || task.ID == sourceTaskID || task.IsTerminalState() {
				continue
			}
			locator := app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}
			if excluded[locator] {
				continue
			}
			candidates = append(candidates, dependencyCandidate{project: project, task: task, locator: locator})
		}
	}
	return candidates
}

// dependencyRemovalCandidates offers exactly the references a task carries, in
// stored order. Terminal and unloaded targets are deliberately included: they
// are the references a user is most likely to be dropping, and withholding them
// would leave the set editable only by hand.
func dependencyRemovalCandidates(task *models.Task, projects []*models.Project, excluded map[app.TaskLocator]bool) []dependencyCandidate {
	if task == nil {
		return nil
	}
	candidates := make([]dependencyCandidate, 0, len(task.Dependencies))
	for _, reference := range task.Dependencies {
		locator := app.LocatorForReference(reference)
		if excluded[locator] {
			continue
		}
		candidate := dependencyCandidate{locator: locator}
		if project := projectByStableID(reference.ProjectID, projects); project != nil {
			candidate.project = project
			candidate.task = project.GetTaskByID(reference.TaskID)
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

// dependencySuggestions ranks candidates by project name and slug, short ID, and
// title, all scored as one search text so a query may name any of them. The
// inserted value is the local alias when the target already lives in the
// effective project and a qualified one otherwise, which is what keeps a
// same-project reference short without ever making it ambiguous.
func dependencySuggestions(candidates []dependencyCandidate, partial string, effective *models.Project, trigger string, titleWidth int) []ACSuggestion {
	type scored struct {
		candidate dependencyCandidate
		score     int
	}
	items := make([]scored, 0, len(candidates))
	for _, candidate := range candidates {
		score := fuzzy.Score(dependencyCandidateSearchText(candidate), partial)
		if partial == "" || score > 0 {
			items = append(items, scored{candidate, score})
		}
	}
	// Stable, because an empty partial scores every candidate identically and an
	// unordered list of ties would reshuffle itself between keystrokes.
	sort.SliceStable(items, func(i, j int) bool { return items[i].score > items[j].score })

	result := make([]ACSuggestion, 0, len(items))
	for _, item := range items {
		value, display := dependencyCandidateRow(item.candidate, effective, trigger, titleWidth)
		result = append(result, ACSuggestion{Display: display, Value: value})
	}
	return result
}

func dependencyCandidateSearchText(candidate dependencyCandidate) string {
	if candidate.task == nil {
		return candidate.locator.Reference().Locator()
	}
	name, slug := "", ""
	if candidate.project != nil {
		name, slug = candidate.project.Name, candidate.project.Slug
	}
	return fmt.Sprintf("%s %s #%d %s", name, slug, candidate.task.ShortID, candidate.task.Title)
}

// dependencyCandidateRow renders one candidate as the token it inserts followed
// by the project, state, short ID, and title that identify it.
func dependencyCandidateRow(candidate dependencyCandidate, effective *models.Project, trigger string, titleWidth int) (value, display string) {
	if candidate.task == nil {
		value = candidate.locator.Reference().Locator()
		return value, fmt.Sprintf("%s%s (unavailable)", trigger, value)
	}
	value = dependencyAlias(candidate.project, candidate.task, effective)
	title := truncateToVisualWidth(candidate.task.Title, max(titleWidth, 10))
	name := ""
	if candidate.project != nil {
		name = candidate.project.Name
	}
	return value, fmt.Sprintf("%s%s %s %s #%d %s", trigger, value, candidate.task.State().Icon(), name, candidate.task.ShortID, title)
}

// draftedDependencyLocators resolves the dependency tokens already accepted on a
// drafted line, so a completion never re-offers a target the line names. Tokens
// are resolved before comparison, so a short ID and a qualified alias pointing at
// one task exclude it once rather than letting the other spelling reappear.
func draftedDependencyLocators(parsed textgrammar.Result, field textgrammar.Field, effective *models.Project, allProjects []*models.Project) map[app.TaskLocator]bool {
	tokens := parsed.TokensFor(field)
	if len(tokens) == 0 {
		return nil
	}
	used := make(map[app.TaskLocator]bool, len(tokens))
	for _, token := range tokens {
		ref, err := resolveDependencyReference(token.Value, effective, allProjects)
		if err != nil || ref.locator.TaskID == "" {
			continue
		}
		used[ref.locator] = true
	}
	return used
}
