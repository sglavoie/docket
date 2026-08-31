package app

import (
	"slices"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// DependencyTargetState is what a dependency locator resolves to inside one
// population. The four failure shapes are kept apart because they are different
// facts about the workspace, not different wordings of one: a target in an
// unloaded project may well exist, a missing one does not, a terminal one exists
// and is simply closed, and a self-reference is the task itself.
type DependencyTargetState uint8

const (
	DependencyTargetOpen DependencyTargetState = iota
	DependencyTargetTerminal
	DependencyTargetMissing
	DependencyTargetUnavailable
	DependencyTargetSelf
)

// DependencyTargetStateFor is the one classifier, the workspace half of the
// reading: given whether the target's project was part of the population at all
// and whatever task that population holds for the locator, it says what the
// reference points at. Self-reference is deliberately not its business — that is
// a fact about a mutation's subject, and a population has no subject — so a
// patch tests for it before calling here and a rendered row never has to.
//
// Both readers reach it: referenceResolver.classify against one snapshot, and
// DependencyIndex.Status against one loaded population. There is no second
// definition of "already closed" or "not available" for the two to drift apart.
func DependencyTargetStateFor(projectLoaded bool, target *models.Task) DependencyTargetState {
	if !projectLoaded {
		return DependencyTargetUnavailable
	}
	if target == nil {
		return DependencyTargetMissing
	}
	if target.IsTerminalState() {
		return DependencyTargetTerminal
	}
	return DependencyTargetOpen
}

// DependencyIndex is one immutable reading of the soft-reference graph over one
// population of projects. It holds both directions — forward, what a locator
// resolves to, and reverse, which tasks name it — because dependents are never
// stored and computing them by rescanning the population once per row is the
// cost this index exists to remove.
//
// It is pure: it reads the projects handed to it, keeps pointers into them, and
// never loads, mutates, or commits anything. It is therefore only ever as
// current as the population it was built from, which is why its holder rebuilds
// it whenever that population is replaced rather than trying to patch it.
//
// One hop, never transitive. A closure would assert reachability through
// projects the population never loaded, would smear the honest unreadable
// reading over a whole path, and would answer "everything reaches everything"
// for every task inside a legal cycle.
type DependencyIndex struct {
	// projects records which project IDs the population covered, which is the
	// whole of what separates a target that is gone from one that cannot be read.
	projects map[string]*models.Project
	// forward resolves a stored locator to the task it names.
	forward map[models.TaskReference]*models.Task
	// reverse lists, per target, the tasks naming it, in population order.
	reverse map[models.TaskReference][]models.TaskReference
}

// NewDependencyIndex builds the index over one population. A nil project, a
// project with no ID, and a repeated project ID are all skipped, so a caller
// handing over a partially built population gets an index that under-reports
// rather than one that double-counts a dependent.
func NewDependencyIndex(projects []*models.Project) *DependencyIndex {
	index := &DependencyIndex{
		projects: make(map[string]*models.Project, len(projects)),
		forward:  make(map[models.TaskReference]*models.Task),
		reverse:  make(map[models.TaskReference][]models.TaskReference),
	}
	for _, project := range projects {
		if project == nil || project.ID == "" {
			continue
		}
		if _, seen := index.projects[project.ID]; seen {
			continue
		}
		index.projects[project.ID] = project
		for _, task := range project.Tasks {
			if task == nil || task.ID == "" {
				continue
			}
			self := models.TaskReference{ProjectID: project.ID, TaskID: task.ID}
			index.forward[self] = task
			for _, reference := range task.Dependencies {
				index.reverse[reference] = append(index.reverse[reference], self)
			}
		}
	}
	return index
}

// Status reports what one stored locator resolves to. It never returns
// DependencyTargetSelf: an index has no subject task to compare against.
//
// A nil index reads every locator as unavailable, which is the honest answer for
// a caller holding no population at all.
func (index *DependencyIndex) Status(reference models.TaskReference) DependencyTargetState {
	if index == nil {
		return DependencyTargetUnavailable
	}
	_, loaded := index.projects[reference.ProjectID]
	return DependencyTargetStateFor(loaded, index.forward[reference])
}

// Lookup resolves one locator to the project and task it names. The third return
// says whether the locator's project was in the population at all, which is what
// lets a caller distinguish a target that is gone from one it cannot read
// without asking Status a second question.
func (index *DependencyIndex) Lookup(
	reference models.TaskReference,
) (*models.Project, *models.Task, bool) {
	if index == nil {
		return nil, nil, false
	}
	project, loaded := index.projects[reference.ProjectID]
	if !loaded {
		return nil, nil, false
	}
	return project, index.forward[reference], true
}

// Incoming lists the tasks naming one target, in population order. Dependents
// are never stored, so this list is the whole of what the product knows about
// them and it is honestly partial: a task in a project outside the population
// may name the subject without appearing here.
//
// The returned slice is the index's own. Callers read it and do not retain or
// sort it in place.
func (index *DependencyIndex) Incoming(reference models.TaskReference) []models.TaskReference {
	if index == nil {
		return nil
	}
	return index.reverse[reference]
}

// DependencyCycle is one closed ring of soft references, listed from its lowest
// member so the same ring is reported the same way whichever task is reached
// first.
type DependencyCycle []models.TaskReference

// Cycles enumerates every reference cycle wholly inside the population. A cycle
// is legal — soft references never gate readiness, ordering, ranking, or
// completion, so a ring cannot deadlock anything — which is why this is a census
// a reader asks for rather than a rule a mutation enforces.
//
// It reports each ring once and only rings every one of whose members the
// population holds: a ring passing through an unloaded project is not something
// this population can establish, and guessing at one would be the same
// overreach a transitive closure would be.
func (index *DependencyIndex) Cycles() []DependencyCycle {
	if index == nil {
		return nil
	}
	// Tarjan is overkill for a census: the graph is tiny and every edge is
	// already materialized, so a plain colored walk that records the stack
	// segment between a back edge and its target names each ring directly.
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[models.TaskReference]int, len(index.forward))
	var stack []models.TaskReference
	var found []DependencyCycle
	seen := make(map[string]bool)

	var walk func(node models.TaskReference)
	walk = func(node models.TaskReference) {
		color[node] = gray
		stack = append(stack, node)
		task := index.forward[node]
		if task != nil {
			for _, next := range task.Dependencies {
				if _, held := index.forward[next]; !held {
					continue
				}
				switch color[next] {
				case white:
					walk(next)
				case gray:
					if cycle := cycleFromStack(stack, next); cycle != nil {
						if key := cycle.key(); !seen[key] {
							seen[key] = true
							found = append(found, cycle)
						}
					}
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[node] = black
	}

	for _, node := range sortedReferences(index.forward) {
		if color[node] == white {
			walk(node)
		}
	}
	return found
}

// cycleFromStack lifts the ring out of the walk stack: everything from the back
// edge's target to the top, rotated so the lowest locator leads.
func cycleFromStack(stack []models.TaskReference, target models.TaskReference) DependencyCycle {
	start := -1
	for i, node := range stack {
		if node == target {
			start = i
			break
		}
	}
	if start < 0 {
		return nil
	}
	segment := stack[start:]
	lowest := 0
	for i, node := range segment {
		if node.Locator() < segment[lowest].Locator() {
			lowest = i
		}
	}
	ring := make(DependencyCycle, 0, len(segment))
	ring = append(ring, segment[lowest:]...)
	return append(ring, segment[:lowest]...)
}

// key is the ring's identity, which is what keeps one ring reached from three of
// its own members reported once.
func (c DependencyCycle) key() string {
	key := ""
	for _, node := range c {
		key += node.Locator() + ">"
	}
	return key
}

// Locators renders the ring as the durable locators a reader can act on.
func (c DependencyCycle) Locators() []string {
	locators := make([]string, 0, len(c))
	for _, node := range c {
		locators = append(locators, node.Locator())
	}
	return locators
}

// sortedReferences fixes the walk's entry order so a census is deterministic.
func sortedReferences(forward map[models.TaskReference]*models.Task) []models.TaskReference {
	refs := make([]models.TaskReference, 0, len(forward))
	for reference := range forward {
		refs = append(refs, reference)
	}
	slices.SortFunc(refs, func(a, b models.TaskReference) int {
		return strings.Compare(a.Locator(), b.Locator())
	})
	return refs
}
