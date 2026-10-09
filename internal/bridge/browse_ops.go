package bridge

import (
	"slices"
	"strings"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

// The browsing half of the protocol: saved lenses, a search box, and the
// relations a task detail navigates. Every answer is the shared runtime's;
// nothing here re-implements a predicate the desktop evaluates.

type lensView struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Mode  string   `json:"mode"`
	Scope string   `json:"scope,omitempty"`
	Query string   `json:"query"`
	Sort  lensSort `json:"sort"`
}

// lensSort always spells both halves, so a descending lens is not read back
// as one with no direction.
type lensSort struct {
	Field     string `json:"field"`
	Ascending bool   `json:"ascending"`
}

type lensesResult struct {
	Lenses []lensView `json:"lenses"`
}

// queryLenses lists the saved lenses in their stored order: the workspace's
// portable definitions when it defines them, else this device's own set.
func (b *Bridge) queryLenses(request Request) (any, error) {
	if err := decodePayload(request.Payload, &struct{}{}); err != nil {
		return nil, err
	}
	runtime := b.workspace.runtime
	if err := runtime.Refresh(); err != nil {
		return nil, err
	}
	result := lensesResult{Lenses: []lensView{}}
	if machine := runtime.State().Machine(); machine != nil {
		for _, lens := range machine.NamedLenses {
			result.Lenses = append(result.Lenses, viewLens(lens))
		}
	}
	return result, nil
}

func viewLens(lens config.NamedLens) lensView {
	field := lens.Sort.Field
	if field == config.LensSortDefault {
		field = ""
	}
	return lensView{
		ID:    lens.ID,
		Name:  lens.Name,
		Mode:  lens.Mode,
		Scope: lens.Scope,
		Query: lens.Query,
		Sort:  lensSort{Field: field, Ascending: lens.Sort.Ascending},
	}
}

// findLens resolves a lens by id, refusing one this workspace does not define.
func (b *Bridge) findLens(id string) (config.NamedLens, error) {
	runtime := b.workspace.runtime
	if err := runtime.Refresh(); err != nil {
		return config.NamedLens{}, err
	}
	if machine := runtime.State().Machine(); machine != nil {
		for _, lens := range machine.NamedLenses {
			if lens.ID == id {
				return lens, nil
			}
		}
	}
	return config.NamedLens{}, newError(CodeNotFound, "lens %s is not defined", id).with("lens", id)
}

// compileSearch reads one search-box value the way the desktop's slash search
// does: a recognized value the grammar cannot honor refuses the whole query by
// name, every canonical token becomes its clause, and the rest is one free-text
// clause over the task's own prose.
func compileSearch(search string) ([]models.QueryClause, *Error) {
	search = strings.TrimSpace(search)
	if search == "" {
		return nil, nil
	}
	if guidance := textgrammar.SearchGuidance(textgrammar.ParseSearch(search)); guidance != "" {
		return nil, newError(CodeValidation, "search: %s", guidance).with("search", search)
	}
	parsed, remainder := models.PartitionQueryString(search)
	clauses := slices.Clone(parsed.Clauses)
	// The durable parser requires canonical tags, while the search box accepts
	// the same spelling as capture (including keyboard capitalization). Quoted
	// text has already been lifted into clauses, so normalize only whole tag or
	// context tokens in the remainder before asking the durable parser again.
	// Presence and clear aliases are lowered to their durable predicates too.
	words := strings.Fields(remainder)
	for i, word := range words {
		if canonical := searchAlias(word); canonical != word {
			words[i] = canonical
			continue
		}
		prefix := strings.HasSuffix(word, "*")
		tokenized := textgrammar.ParseSearch(strings.TrimSuffix(word, "*"))
		if tokenized.Text != "" || len(tokenized.Tokens) != 1 {
			continue
		}
		token := tokenized.Tokens[0]
		switch token.Field { //nolint:exhaustive // Only tag and context spellings need normalization here.
		case textgrammar.FieldTag, textgrammar.FieldRemoveTag, textgrammar.FieldContext, textgrammar.FieldRemoveContext:
			value, err := models.NormalizeTaskTag(token.Value)
			if err != nil {
				continue
			}
			words[i] = token.Form + value
			if prefix {
				words[i] += "*"
			}
		}
	}
	normalized, remainder := models.PartitionQueryString(strings.Join(words, " "))
	clauses = append(clauses, normalized.Clauses...)
	if remainder = strings.TrimSpace(remainder); remainder != "" {
		text, err := models.NewQueryClause(models.QueryFieldText, models.QueryPolarityPositive, models.QueryOperatorContains, remainder)
		if err != nil {
			return nil, newError(CodeValidation, "search: %v", err).with("search", search)
		}
		clauses = append(clauses, text)
	}
	return clauses, nil
}

// searchAlias maps the search grammar's presence and clear spellings to the
// durable predicates. Negation inverts the same predicate as the positive form.
func searchAlias(word string) string {
	value, negative := strings.CutPrefix(word, "-")
	if negative && (value == "*unpin" || value == "=unreview") {
		return word
	}
	var canonical string
	switch value {
	case "@none":
		canonical = "@0"
	case "@@any":
		canonical = "?deferred"
	case "@@none":
		canonical = "-?deferred"
	case "*unpin":
		canonical = "-*pin"
	case "=unreview":
		canonical = "-=review"
	default:
		return word
	}
	if negative {
		if positive, ok := strings.CutPrefix(canonical, "-"); ok {
			return positive
		}
		return "-" + canonical
	}
	return canonical
}

// lensQuery is the query a lens reopens into, its stored string read through
// the same search reading so a richer spelling the canonical grammar does not
// own still narrows rather than being dropped.
func lensQuery(lens config.NamedLens) (models.Query, error) {
	clauses, refusal := compileSearch(lens.Query)
	if refusal != nil {
		return models.Query{}, refusal
	}
	sort := models.QuerySort{Ascending: lens.Sort.Ascending}
	if lens.Sort.Field != config.LensSortDefault {
		sort.Field = models.QuerySortField(lens.Sort.Field)
	}
	query, err := models.NewQuery(models.QueryMode(lens.Mode), lens.Scope, sort, clauses...)
	if err != nil {
		return models.Query{}, newError(CodeValidation, "lens %s: %v", lens.Name, err)
	}
	return query, nil
}

// relationView names one related task. A reference the workspace cannot
// resolve keeps its stored locator and says so rather than disappearing.
type relationView struct {
	ProjectID   string `json:"project_id"`
	TaskID      string `json:"task_id"`
	Available   bool   `json:"available"`
	Title       string `json:"title,omitempty"`
	State       string `json:"state,omitempty"`
	ProjectName string `json:"project_name,omitempty"`
}

// relations are what a task detail navigates: the ancestor chain root first,
// the direct children in stored order, the tasks this one depends on, and the
// computed dependents.
type relations struct {
	Ancestors    []relationView `json:"ancestors"`
	Children     []relationView `json:"children"`
	Dependencies []relationView `json:"dependencies"`
	Dependents   []relationView `json:"dependents"`
}

func relate(snapshot app.Snapshot, target app.TaskLocator) relationView {
	view := relationView{ProjectID: target.ProjectID, TaskID: target.TaskID}
	ref, ok := snapshot.LookupLocator(target)
	if !ok {
		return view
	}
	view.Available = true
	view.Title = ref.Task.Title
	view.State = string(ref.Task.WorkflowState)
	view.ProjectName = ref.Project.Name
	return view
}

func taskRelations(snapshot app.Snapshot, project *models.Project, task *models.Task) relations {
	result := relations{
		Ancestors:    []relationView{},
		Children:     []relationView{},
		Dependencies: []relationView{},
		Dependents:   []relationView{},
	}
	seen := map[string]bool{task.ID: true}
	for parent := task.ParentID; parent != nil && !seen[*parent]; {
		seen[*parent] = true
		view := relate(snapshot, app.TaskLocator{ProjectID: project.ID, TaskID: *parent})
		result.Ancestors = append([]relationView{view}, result.Ancestors...)
		ref, ok := snapshot.LookupLocator(app.TaskLocator{ProjectID: project.ID, TaskID: *parent})
		if !ok {
			break
		}
		parent = ref.Task.ParentID
	}
	for _, child := range project.Tasks {
		if child != nil && child.ParentID != nil && *child.ParentID == task.ID {
			result.Children = append(result.Children, relate(snapshot, app.TaskLocator{ProjectID: project.ID, TaskID: child.ID}))
		}
	}
	for _, dependency := range task.Dependencies {
		result.Dependencies = append(result.Dependencies, relate(snapshot, app.TaskLocator{ProjectID: dependency.ProjectID, TaskID: dependency.TaskID}))
	}
	self := models.TaskReference{ProjectID: project.ID, TaskID: task.ID}
	for _, other := range snapshot.Projects() {
		for _, candidate := range other.Tasks {
			if candidate != nil && slices.Contains(candidate.Dependencies, self) {
				result.Dependents = append(result.Dependents, relate(snapshot, app.TaskLocator{ProjectID: other.ID, TaskID: candidate.ID}))
			}
		}
	}
	return result
}
