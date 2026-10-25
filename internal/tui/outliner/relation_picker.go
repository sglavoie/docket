package outliner

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/fuzzy"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// The relationship picker is the one surface `g~` opens: the cursor task's
// stored dependencies and the loaded tasks naming it, in one list, so the
// reference graph can be walked a hop at a time.
//
// It is an Outliner-local gated sub-state on the menu bar's template rather than
// a shell overlay, because the package graph keeps the shared value pickers out
// of reach from this layer and because everything the picker does — resolving
// through the population Peek reads, revealing a target, and handing the
// previous view back — is the Outliner's own state.
//
// Two rules shape it. Direction is part of a row's identity, so two tasks that
// name each other list twice rather than collapsing onto one key. And the picker
// filters its own rows instead of setting the surface's query, because the
// surface ranks a query by score: Dependencies are shown in the order the task
// stores them and Dependents in the order the index holds them, and a relevance
// sort would silently replace both.

const relationPickerItemPrefix = "relation-picker:"

// The two sections. Dependents names its own limit in the header rather than in
// a footnote: the list is computed from the loaded population and can never be
// the whole truth, so the one place a reader looks is where it says so.
const (
	relationSectionOutgoing listsurface.ListCategoryID = "relation:dependencies"
	relationSectionIncoming listsurface.ListCategoryID = "relation:dependents"

	relationSectionOutgoingLabel = "Dependencies"
	relationSectionIncomingLabel = "Dependents (loaded projects only)"
)

// The refusals. An unavailable outgoing locator is retained as a disabled row
// rather than dropped, because the locator is what storage holds; the wording
// says what this view could not read and never that the target is gone.
const (
	relationUnavailableReason  = "This view has not loaded that project"
	relationStaleTargetRefusal = "That task is no longer in the loaded projects"
	relationEmptyTitle         = "No relationships"
	relationEmptyDetail        = "This task names no dependencies and nothing loaded names it."
	relationNoMatchTitle       = "No matching relationships"
	relationNoMatchDetail      = "Esc clears the search."
)

// relationDirection is which half of the relationship a row belongs to. It is
// part of the row identity rather than decoration: a reciprocal pair resolves to
// the same locator in both directions, and two rows sharing one ID would let the
// surface's cursor retention land on the wrong half.
type relationDirection uint8

const (
	relationOutgoing relationDirection = iota
	relationIncoming
)

func (direction relationDirection) prefix() string {
	if direction == relationIncoming {
		return "in:"
	}
	return "out:"
}

// relationCandidate is one listed relationship. The project-qualified identity
// travels beside the display text rather than inside it: the label is prose
// about a task and the identity is what a jump resolves, and a picker that read
// its target back out of its own rendering would navigate by a rename.
type relationCandidate struct {
	Direction relationDirection
	Reference models.TaskReference
	// Target is the loaded identity this row resolved to, or the zero value when
	// nothing could be read. A disabled row keeps its reference and nothing else.
	Target   taskref.TaskIdentity
	Detail   DependencyDetail
	Disabled string
}

// relationPickerState is the whole picker: the subject it was opened over, the
// candidates in their two authoritative orders, the shared list surface, and the
// picker's own search. The zero value is closed.
type relationPickerState struct {
	Open bool
	// Subject is the task the list was built for. It is the identity the return
	// trail parks the cursor back on, so it outlives every row below it.
	Subject      taskref.TaskIdentity
	SubjectTitle string
	Surface      *listsurface.ListSurfaceModel
	Candidates   []relationCandidate
	Query        string
	Searching    bool
	SearchInput  listsurface.PickerSearchInput
	Notice       string
}

// RelationPickerOpen reports that the picker owns the input.
func (m *OutlinerModel) RelationPickerOpen() bool {
	return m != nil && m.RelationPicker.Open && m.RelationPicker.Surface != nil
}

// OpenRelationPicker lists one task's relationships. It opens over an empty set
// too: the empty state is what says the task has none, and refusing the key
// would leave that fact unreachable from the key that asks it.
func (m *OutlinerModel) OpenRelationPicker(task *models.Task, project *models.Project) error {
	if m == nil {
		return fmt.Errorf("the Outliner is unavailable")
	}
	if task == nil || project == nil {
		return fmt.Errorf("there is no task under the cursor")
	}
	subject := taskref.For(task, project)
	if !subject.Valid() {
		return fmt.Errorf("that task has no stable identity")
	}
	state := &relationPickerState{
		Open:         true,
		Subject:      subject,
		SubjectTitle: task.Title,
		Candidates:   m.relationCandidatesFor(task, subject),
	}
	empty := relationPickerEmptyState(state.Query)
	surface, err := listsurface.NewListSurfaceModel(
		state.items(),
		relationPickerCategories(),
		listsurface.ListSurfaceConfig{
			SelectionMode: listsurface.ListSelectionSingle,
			EmptyState:    empty,
			NoMatchState:  empty,
		},
	)
	if err != nil {
		return err
	}
	state.Surface = surface
	m.RelationPicker = *state
	return nil
}

// closeRelationPicker dismisses the picker outright. There is no state worth
// retaining across an opening: the list is read from the index every time, so a
// stale query or cursor would only be a reading of a graph that has since moved.
func (m *OutlinerModel) closeRelationPicker() {
	if m == nil {
		return
	}
	m.RelationPicker = relationPickerState{}
}

func relationPickerCategories() []listsurface.ListCategory {
	return []listsurface.ListCategory{
		{ID: relationSectionOutgoing, Label: relationSectionOutgoingLabel},
		{ID: relationSectionIncoming, Label: relationSectionIncomingLabel},
	}
}

// relationCandidatesFor reads both halves in the order each is authoritative in:
// outgoing references in the order the task stores them, which is the order the
// grammar round-trips against, and incoming ones in the index's own order.
// index.Incoming hands back its own slice, so it is read and never retained or
// sorted in place.
func (m *OutlinerModel) relationCandidatesFor(
	task *models.Task,
	subject taskref.TaskIdentity,
) []relationCandidate {
	if m == nil || task == nil || !subject.Valid() {
		return nil
	}
	index := m.dependencyIndex()
	candidates := make([]relationCandidate, 0, len(task.Dependencies))
	for _, reference := range task.Dependencies {
		candidates = append(candidates, relationCandidateFor(relationOutgoing, reference, index))
	}
	incoming := index.Incoming(models.TaskReference{ProjectID: subject.ProjectID, TaskID: subject.TaskID})
	for _, reference := range incoming {
		candidate := relationCandidateFor(relationIncoming, reference, index)
		if candidate.Disabled != "" {
			// A dependent is computed out of the loaded population, so one that
			// cannot be read is not a retained locator the task holds — it is a row
			// about nothing, and the section header already says the list is partial.
			continue
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func relationCandidateFor(
	direction relationDirection,
	reference models.TaskReference,
	index *app.DependencyIndex,
) relationCandidate {
	candidate := relationCandidate{Direction: direction, Reference: reference}
	project, task, _ := index.Lookup(reference)
	if task == nil || project == nil {
		candidate.Detail = DependencyDetailUnresolved(reference)
		candidate.Disabled = relationUnavailableReason
		return candidate
	}
	candidate.Detail = dependencyDetailFor(project, task)
	candidate.Target = taskref.For(task, project)
	return candidate
}

func relationPickerItemID(candidate relationCandidate) listsurface.ListItemID {
	return listsurface.ListItemID(
		relationPickerItemPrefix + candidate.Direction.prefix() + candidate.Reference.Locator(),
	)
}

// items renders every candidate the picker's own query admits, in source order.
func (state *relationPickerState) items() []listsurface.ListItem {
	items := make([]listsurface.ListItem, 0, len(state.Candidates))
	for _, candidate := range state.Candidates {
		if !relationCandidateMatches(candidate, state.Query) {
			continue
		}
		items = append(items, relationPickerItem(candidate))
	}
	return items
}

func relationPickerItem(candidate relationCandidate) listsurface.ListItem {
	category := relationSectionOutgoing
	if candidate.Direction == relationIncoming {
		category = relationSectionIncoming
	}
	return listsurface.ListItem{
		ID:             relationPickerItemID(candidate),
		CategoryID:     category,
		PrimaryText:    candidate.Detail.Label,
		SecondaryText:  candidate.Detail.State,
		DisabledReason: candidate.Disabled,
		Data:           candidate,
	}
}

// relationCandidateMatches scores one row against the picker's query. The state
// participates through the rendered detail, so searching `done` narrows to the
// settled half of either section.
func relationCandidateMatches(candidate relationCandidate, raw string) bool {
	normalized := strings.Join(strings.Fields(raw), " ")
	if normalized == "" {
		return true
	}
	return fuzzy.ScoreTerms(candidate.Detail.String(), normalized) != 0
}

// relationPickerEmptyState is the copy shown when nothing is listed. The picker
// filters its own items, so a query matching nothing hands the surface an empty
// source list and the surface can no longer tell "this task has no
// relationships" from "the search found none" — the query is what tells them
// apart, and the picker is the one holding it. Both slots take the same answer
// because only one of the two situations can be true at a time.
func relationPickerEmptyState(raw string) listsurface.ListEmptyState {
	if strings.TrimSpace(raw) != "" {
		return listsurface.ListEmptyState{Title: relationNoMatchTitle, Detail: relationNoMatchDetail}
	}
	return listsurface.ListEmptyState{Title: relationEmptyTitle, Detail: relationEmptyDetail}
}

// setQuery re-narrows the rows. It is the picker's own filter, so the surface
// never sees a query and neither section is ever reordered by relevance.
func (state *relationPickerState) setQuery(raw string) {
	if state == nil || state.Surface == nil || state.Query == raw {
		return
	}
	state.Query = raw
	empty := relationPickerEmptyState(raw)
	state.Surface.SetEmptyStates(empty, empty)
	_ = state.Surface.SetItems(state.items())
}

// handleRelationPickerKey is the picker's gated keymap. It follows the named
// lens picker's conventions exactly — one Esc clears a typed query, the next
// leaves search, and closing belongs to browse mode alone — so the two
// searchable pickers in this product are one grammar.
//
//nolint:gocyclo // Key dispatch stays in one switch so every picker state has one unwind path.
func (m *OutlinerModel) handleRelationPickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	state := &m.RelationPicker
	if state.Surface == nil {
		m.closeRelationPicker()
		return m, nil
	}
	key := msg.String()
	if state.Searching {
		switch key {
		case "esc":
			if state.Query != "" {
				state.setQuery("")
				return m, nil
			}
			state.Searching = false
		case "enter":
			state.Searching = false
		case "up":
			state.Surface.MoveCursor(-1)
		case "down":
			state.Surface.MoveCursor(1)
		default:
			state.setQuery(state.SearchInput.HandleKey(state.Query, msg))
		}
		return m, nil
	}

	switch key {
	case "esc":
		m.closeRelationPicker()
	case "/":
		state.Searching = true
	case "up", "k", "ctrl+p":
		state.Surface.MoveCursor(-1)
	case "down", "j", "ctrl+n":
		state.Surface.MoveCursor(1)
	case "home", "g":
		state.Surface.MoveCursorToStart()
	case "end", "G":
		state.Surface.MoveCursorToEnd()
	case "pgup":
		state.Surface.PageCursor(-1)
	case "pgdown":
		state.Surface.PageCursor(1)
	case "enter":
		// Spelled out rather than reached through a default arm, for the same
		// reason the lens picker spells its digits: a default would silently
		// swallow every key this switch has not learned yet.
		return m, m.activateRelationCursor()
	}
	return m, nil
}

// activateRelationCursor is Enter. It revalidates the row against the live index
// before anything moves, so a target that went away while the picker was open
// refuses in place: the view and the return trail are left exactly as they were
// and the picker stays open carrying the reason.
func (m *OutlinerModel) activateRelationCursor() tea.Cmd {
	state := &m.RelationPicker
	item, ok := state.Surface.CurrentItem()
	if !ok {
		return nil
	}
	if item.DisabledReason != "" {
		state.Notice = item.DisabledReason
		return nil
	}
	candidate, isCandidate := item.Data.(relationCandidate)
	if !isCandidate {
		state.Notice = "That row names no task"
		return nil
	}
	project, task, _ := m.dependencyIndex().Lookup(candidate.Reference)
	if task == nil || project == nil || taskref.For(task, project) != candidate.Target {
		state.Notice = relationStaleTargetRefusal
		return nil
	}
	previous := m.Cursor
	if refusal := m.jumpToRelation(task, project); refusal != "" {
		state.Notice = refusal
		return nil
	}
	m.closeRelationPicker()
	m.SetTransientStatus("Went to " + candidate.Detail.Label)
	return listsurface.FlashIfJumped(previous, m.Cursor, &m.CursorFlash)
}
