package grammar

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

func (q *QuickEditComponent) cursorByteOffset() int {
	runes := []rune(q.Input.Value())
	position := max(0, min(q.Input.Position(), len(runes)))
	return len(string(runes[:position]))
}

func (q *QuickEditComponent) refreshAutocomplete() {
	value := q.Input.Value()
	cursorByte := q.cursorByteOffset()
	mode, pos, partial, triggerText, removeTag := detectQuickEditTrigger(value[:cursorByte], q.triggers)
	if mode == ACModeNone {
		q.AC.Clear()
		q.triggerText = ""
		q.RemoveTag = false
		return
	}
	q.triggerText = triggerText
	q.RemoveTag = removeTag
	parsedPrefix := textgrammar.Parse(value[:pos], textgrammar.SurfaceEdit)
	if field, singleton := autocompleteSingletonField(mode); singleton && len(parsedPrefix.TokensFor(field)) > 0 {
		q.AC.Clear()
		return
	}
	q.AC.UpdateSuggestions(mode, pos, q.suggestions(mode, pos, partial, parsedPrefix))
}

func (q *QuickEditComponent) suggestions(mode, triggerPos int, partial string, parsedPrefix textgrammar.Result) []ACSuggestion {
	switch mode {
	case ACModeTag:
		project := q.effectiveProject()
		field := textgrammar.FieldTag
		values := CollectTagNamesForProject(project)
		prefix := "#"
		if q.RemoveTag {
			field = textgrammar.FieldRemoveTag
			values = append([]string(nil), q.task.Tags...)
			prefix = "-#"
		}
		used := parsedPrefix.TokensFor(field)
		return FilterUsedTagSuggestions(quickEditTagSuggestions(values, partial, prefix), used)
	case acModeParent:
		parents, _ := QuickEditParentCandidates(q.Input.Value(), q.task, q.project, q.projects)
		items := quickEditClearSuggestions(textgrammar.FieldParent, partial)
		return append(items, quickEditStructuralSuggestions(parents, partial, textgrammar.FieldParent)...)
	case ACModeChild:
		// The token under the cursor is removed before the draft is read, so an
		// in-progress alias cannot withhold the very task it is naming. Child
		// declares no clear operation, so no <0 row is offered.
		draft := quickEditDraftWithoutToken(q.Input.Value(), triggerPos)
		children, _ := QuickEditChildCandidates(draft, q.task, q.project, q.projects)
		return quickEditStructuralSuggestions(children, partial, textgrammar.FieldChild)
	case ACModeDependency, ACModeRemoveDependency:
		return q.dependencySuggestions(mode, triggerPos, partial)
	case acModeProject:
		return quickEditProjectSuggestions(q.projects, partial)
	case acModePriority, acModeDate, ACModeDestination, acModePin:
		return quickEditStaticSuggestions(mode, partial)
	default:
		return nil
	}
}

// dependencySuggestions offers open tasks from every loaded project for a ~task
// addition, and exactly the references the task already carries for a -~task
// removal. The two candidate sets are deliberately disjoint in kind: an addition
// asks the workspace for something still open, while a removal must reach the
// closed, moved, and unloaded targets an addition would refuse.
func (q *QuickEditComponent) dependencySuggestions(mode, triggerPos int, partial string) []ACSuggestion {
	field := textgrammar.FieldDependency
	if mode == ACModeRemoveDependency {
		field = textgrammar.FieldRemoveDependency
	}
	definition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceEdit, field)
	if !ok {
		return nil
	}
	effective := q.effectiveProject()
	// The token under the cursor is removed before the draft is read, so an
	// in-progress alias cannot withhold the very task it is naming.
	draft := textgrammar.Parse(quickEditDraftWithoutToken(q.Input.Value(), triggerPos), textgrammar.SurfaceEdit)
	used := draftedDependencyLocators(draft, field, effective, q.projects)

	sourceID := ""
	if q.task != nil {
		sourceID = q.task.ID
	}
	if mode == ACModeRemoveDependency {
		candidates := DependencyRemovalCandidates(q.task, q.projects, used)
		return DependencySuggestions(candidates, partial, effective, definition.Trigger, q.Input.Width()-25)
	}
	// An addition never re-offers a reference the task already stores, for the
	// same reason it never re-offers one the draft already names.
	for _, reference := range q.dependencyReferences() {
		if used == nil {
			used = map[app.TaskLocator]bool{}
		}
		used[app.LocatorForReference(reference)] = true
	}
	candidates := DependencyAdditionCandidates(q.projects, sourceID, used)
	items := quickEditClearSuggestions(field, partial)
	return append(items, DependencySuggestions(candidates, partial, effective, definition.Trigger, q.Input.Width()-25)...)
}

// dependencyReferences reads the stored set the draft starts from.
func (q *QuickEditComponent) dependencyReferences() []models.TaskReference {
	if q.task == nil {
		return nil
	}
	return q.task.Dependencies
}

func (q *QuickEditComponent) effectiveProject() *models.Project {
	parsed := textgrammar.Parse(q.Input.Value(), textgrammar.SurfaceEdit)
	if tokens := parsed.TokensFor(textgrammar.FieldProject); len(tokens) > 0 {
		if project := resolveProject(tokens[0].Value, q.projects); project != nil {
			return project
		}
	}
	return q.project
}

func (q *QuickEditComponent) AcceptSuggestion() {
	if !q.AC.IsActive() {
		return
	}
	cursorByte := q.cursorByteOffset()
	triggerText := q.triggerText
	if triggerText == "" {
		triggerText = TriggerCharForMode(q.AC.Mode, q.triggers)
	}
	selected := q.AC.Selected()
	value := q.AC.BuildAcceptedInputAt(q.Input.Value(), cursorByte, triggerText)
	insertedPrefix := q.Input.Value()[:q.AC.TriggerPos] + triggerText + selected.Value + " "
	q.Input.SetValue(value)
	q.Input.SetCursor(utf8.RuneCountInString(insertedPrefix))
	q.AC.Clear()
	q.triggerText = ""
	q.RemoveTag = false
}

func detectQuickEditTrigger(input string, triggers []ACTrigger) (mode, pos int, partial, triggerText string, removeTag bool) {
	tokenStart := strings.LastIndexAny(input, " \n\t") + 1
	if tokenStart < len(input) && strings.HasPrefix(input[tokenStart:], "-#") {
		return ACModeTag, tokenStart, input[tokenStart+2:], "-#", true
	}
	// -~ is peeled here for the same reason -# is: DetectTrigger takes the longest
	// registered trigger, and quickEditTriggers carries single-rune triggers only,
	// so a two-rune removal form would otherwise complete as its own base sigil.
	if tokenStart < len(input) && strings.HasPrefix(input[tokenStart:], "-~") {
		return ACModeRemoveDependency, tokenStart, input[tokenStart+2:], "-~", false
	}
	mode, pos, partial = DetectTrigger(input, triggers)
	if mode == ACModeNone {
		return mode, pos, partial, "", false
	}
	return mode, pos, partial, TriggerCharForMode(mode, triggers), false
}

func quickEditTagSuggestions(values []string, partial, prefix string) []ACSuggestion {
	return scoredQuickEditSuggestions(values, partial, func(value string) ACSuggestion {
		return ACSuggestion{Display: prefix + value, Value: value}
	})
}

// quickEditStructuralSuggestions ranks candidates for a >parent or <child
// reference. Both families score one search text, so the two relationships can
// never rank the same task differently; the field decides only which sigil the
// row is rendered with, read from the registry rather than spelled here.
func quickEditStructuralSuggestions(tasks []*models.Task, partial string, field textgrammar.Field) []ACSuggestion {
	definition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceEdit, field)
	if !ok {
		return nil
	}
	type candidate struct {
		task  *models.Task
		score int
	}
	items := make([]candidate, 0, len(tasks))
	for _, task := range tasks {
		search := fmt.Sprintf("%d %s", task.ShortID, task.Title)
		score := fuzzy.Score(search, partial)
		if partial == "" || score > 0 {
			items = append(items, candidate{task, score})
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].score > items[j].score })
	result := make([]ACSuggestion, 0, len(items))
	for _, item := range items {
		result = append(result, ACSuggestion{
			Display: fmt.Sprintf("%s%d %s", definition.Trigger, item.task.ShortID, item.task.Title),
			Value:   strconv.Itoa(item.task.ShortID),
		})
	}
	return result
}

// quickEditDraftWithoutToken removes the token the cursor is completing so the
// rest of the line reads as settled facts. A repeatable field needs every other
// token of its own family visible, and needs its own in-progress token hidden.
func quickEditDraftWithoutToken(value string, triggerPos int) string {
	if triggerPos < 0 || triggerPos >= len(value) {
		return value
	}
	end := strings.IndexAny(value[triggerPos:], " \t\n")
	if end < 0 {
		return value[:triggerPos]
	}
	return value[:triggerPos] + value[triggerPos+end:]
}

func quickEditProjectSuggestions(projects []*models.Project, partial string) []ACSuggestion {
	type candidate struct {
		project *models.Project
		score   int
	}
	items := make([]candidate, 0, len(projects))
	for _, project := range projects {
		// Callers provide the Outliner's already-admitted project population,
		// which includes parked destinations but excludes archived projects.
		if project == nil {
			continue
		}
		score := fuzzy.Score(project.Slug+" "+project.Name, partial)
		if partial == "" || score > 0 {
			items = append(items, candidate{project, score})
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].score > items[j].score })
	result := make([]ACSuggestion, 0, len(items))
	for _, item := range items {
		result = append(result, ACSuggestion{Display: "%" + item.project.Slug + " " + item.project.Name, Value: item.project.Slug})
	}
	return result
}

func quickEditStaticSuggestions(mode int, partial string) []ACSuggestion {
	field, ok := autocompleteSingletonField(mode)
	if !ok {
		return nil
	}
	definition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceEdit, field)
	if !ok {
		return nil
	}
	result := make([]ACSuggestion, 0, len(definition.Values))
	partial = strings.ToLower(partial)
	result = append(result, quickEditClearSuggestions(field, partial)...)
	for _, value := range definition.Values {
		if partial != "" && !strings.Contains(strings.ToLower(value), partial) {
			continue
		}
		result = append(result, ACSuggestion{Display: definition.Trigger + value, Value: value})
	}
	return result
}

func quickEditClearSuggestions(field textgrammar.Field, partial string) []ACSuggestion {
	definition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceEdit, field)
	if !ok {
		return nil
	}
	partial = strings.ToLower(partial)
	result := make([]ACSuggestion, 0, len(definition.Clears))
	for _, clear := range definition.Clears {
		if clear.Surfaces&textgrammar.SurfaceEdit == 0 || (partial != "" && !strings.Contains(strings.ToLower(clear.Value), partial)) {
			continue
		}
		display := definition.Trigger + clear.Value
		if clear.Description != "" {
			display += " (" + clear.Description + ")"
		}
		result = append(result, ACSuggestion{Display: display, Value: clear.Value})
	}
	return result
}

func scoredQuickEditSuggestions(values []string, partial string, render func(string) ACSuggestion) []ACSuggestion {
	type candidate struct {
		value string
		score int
	}
	items := make([]candidate, 0, len(values))
	for _, value := range values {
		score := fuzzy.Score(value, partial)
		if partial == "" || score > 0 {
			items = append(items, candidate{value, score})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score == items[j].score {
			return items[i].value < items[j].value
		}
		return items[i].score > items[j].score
	})
	result := make([]ACSuggestion, 0, len(items))
	for _, item := range items {
		result = append(result, render(item.value))
	}
	return result
}
