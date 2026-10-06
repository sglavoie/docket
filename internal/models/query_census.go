package models

import "time"

// QueryCensus is the population-wide knowledge the census fields read and one
// task cannot supply: how many open direct children each task has, and which
// tasks some loaded task names as a dependency. It is built once per population
// and answers every task of it, so the census fields mean the same thing to
// every holder of a population rather than to whichever view built its own.
type QueryCensus struct {
	openChildren map[TaskReference]int
	dependedOn   map[TaskReference]bool
}

// NewQueryCensus censuses a loaded population. It reads the authoritative flat
// project.Tasks, which already holds every nested child, so a filter pipeline
// that rewrote a clone's Children cannot change what parent or leaf means.
func NewQueryCensus(projects []*Project) QueryCensus {
	return QueryCensus{
		openChildren: QueryOpenChildCounts(projects),
		dependedOn:   QueryDependencyTargets(projects),
	}
}

// QueryOpenChildCounts counts each task's open direct children across the
// population, keyed by the qualified reference. A task with none is absent.
func QueryOpenChildCounts(projects []*Project) map[TaskReference]int {
	counts := make(map[TaskReference]int)
	for _, project := range projects {
		if project == nil || project.ID == "" {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || task.ID == "" {
				continue
			}
			if open := len(task.IncompleteChildren()); open > 0 {
				counts[TaskReference{ProjectID: project.ID, TaskID: task.ID}] = open
			}
		}
	}
	return counts
}

// QueryDependencyTargets names every task some loaded task points at. It is
// honestly partial: dependents are computed, never stored, so a reference held
// by a project outside the population is not knowable from it.
func QueryDependencyTargets(projects []*Project) map[TaskReference]bool {
	targets := make(map[TaskReference]bool)
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			for _, reference := range task.Dependencies {
				targets[reference] = true
			}
		}
	}
	return targets
}

// For reads one task's census entry.
func (c QueryCensus) For(task *Task, project *Project) DerivedCensus {
	if task == nil || project == nil {
		return DerivedCensus{}
	}
	key := TaskReference{ProjectID: project.ID, TaskID: task.ID}
	return DerivedCensus{OpenChildren: c.openChildren[key], HasDependents: c.dependedOn[key]}
}

// AdmitsInPopulation is Admits for a holder of the population: the census
// fields QueryFieldRequiresCensus marks are lifted out of the row predicate and
// answered against the census, and every other clause is Admits' alone. The
// join law is unchanged — positives of one census field OR, negatives AND-not.
func (q Query) AdmitsInPopulation(task *Task, project *Project, census QueryCensus, now time.Time) bool {
	if task == nil || project == nil {
		return false
	}
	row := Query{Mode: q.Mode, Scope: q.Scope, Sort: q.Sort}
	grouped := make(map[QueryField][]QueryClause)
	for _, clause := range q.Clauses {
		if QueryFieldRequiresCensus(clause.Field) {
			grouped[clause.Field] = append(grouped[clause.Field], clause)
			continue
		}
		row.Clauses = append(row.Clauses, clause)
	}
	if !row.Admits(task, project, now) {
		return false
	}
	entry := census.For(task, project)
	for field, clauses := range grouped {
		matched := queryCensusFieldMatches(field, task, entry)
		positives, admitted := 0, false
		for _, clause := range clauses {
			if clause.Polarity == QueryPolarityNegative {
				if matched {
					return false
				}
				continue
			}
			positives++
			admitted = admitted || matched
		}
		if positives > 0 && !admitted {
			return false
		}
	}
	return true
}

// queryCensusFieldMatches answers one census field. Each takes no value, so a
// field's every clause asks the same question and one answer serves them all.
func queryCensusFieldMatches(field QueryField, task *Task, census DerivedCensus) bool {
	switch field {
	case QueryFieldParent:
		return census.OpenChildren > 0
	case QueryFieldLeaf:
		return census.OpenChildren == 0
	case QueryFieldChild:
		return task.ParentID != nil
	case QueryFieldBlocked:
		return len(task.Dependencies) > 0
	case QueryFieldBlocking:
		return census.HasDependents
	default:
		return false
	}
}
