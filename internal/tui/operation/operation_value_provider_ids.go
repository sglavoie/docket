package operation

import "strings"

// ValueProviderID identifies the picker or other value source an operation
// needs before execution. The empty value means no provider is required.
type ValueProviderID string

const (
	ValueProviderState     ValueProviderID = "state"
	ValueProviderPriority  ValueProviderID = "priority"
	ValueProviderTags      ValueProviderID = "tags"
	ValueProviderNoteTitle ValueProviderID = "note-title"
	ValueProviderTagName   ValueProviderID = "tag-name"

	// ValueProviderDate is the combined scheduling picker a task operation
	// reaches. It sections due presets above defer presets and a typed Check-in
	// editor action; every selected row carries exactly which route it answers.
	ValueProviderDate ValueProviderID = "date"

	// ValueProviderCheckIn opens the reusable two-field check-in editor. It is
	// neither a text prompt nor a list picker: both the date and repeat cadence
	// stay in one draft until Ctrl+S submits the pair.
	ValueProviderCheckIn ValueProviderID = "check-in"

	// ValueProviderDueDate is the deadline-only projection of the same presets.
	// Optional active-work setup uses ValueProviderCheckIn.
	ValueProviderDueDate ValueProviderID = "due-date"

	// ValueProviderChecklistItem is the typed text of one checklist item. It is
	// a separate provider from the note title because the prompt prefills the
	// item under the cursor rather than a note's own name.
	ValueProviderChecklistItem ValueProviderID = "checklist-item"

	// ValueProviderResumeNote is the typed resume note of one task. It is its own
	// provider because it is the only text prompt that accepts the empty value:
	// clearing the note is what an emptied buffer means, where a name prompt has
	// nothing to express with one.
	ValueProviderResumeNote ValueProviderID = "resume-note"

	// ValueProviderComment is the typed text of one new comment. It is its own
	// provider because it is the only text prompt whose value is multiline: a
	// comment's stored form holds newlines, so Enter has to write one and the
	// save moves to Ctrl+S, which no single-line prompt would want.
	ValueProviderComment ValueProviderID = "comment"

	// ValueProviderLensName is the typed name of one saved lens. It is separate
	// from the note title because it prefills the lens under the cursor on a
	// rename and stays blank when the current view is being saved as a new lens.
	ValueProviderLensName ValueProviderID = "lens-name"

	// ValueProviderLensTemplate names one built-in view from the catalog. It is a
	// picked provider rather than a typed one: the vocabulary is closed, and the
	// value is the template's stable ID, never a definition the gallery captured.
	ValueProviderLensTemplate ValueProviderID = "lens-template"

	// ValueProviderParent lists the tasks the cursor task may hang under. It is a
	// plain list: the vocabulary is the project's own open tasks, so there is
	// nothing to mint and no inline entry mode.
	ValueProviderParent ValueProviderID = "parent"

	// ValueProviderChild lists the tasks the cursor task may adopt. It is
	// multi-select because the <child sigil is repeatable, and additive because
	// that sigil has no detaching form: rows the task already parents are absent,
	// so no selection state can express a detach.
	ValueProviderChild ValueProviderID = "child"

	// ValueProviderTaskReference lists the tasks the cursor task may name as soft
	// references. It is multi-select like ValueProviderChild but preselected where
	// that one is empty: `~` declares both `~0` and `-~task`, so unchecking a
	// stored row is a removal the grammar has a form for. Its population crosses
	// project boundaries, which no other relationship provider does.
	ValueProviderTaskReference ValueProviderID = "task-reference"

	// ValueProviderProject lists the projects the cursor task may move to. Its
	// population is AvailableProjects rather than the Outliner's own, so a parked
	// project stays a reachable destination, and the task's current project is
	// absent because a move to it is not a move.
	ValueProviderProject ValueProviderID = "project"

	// ValueProviderProjectTags edits a project's own tag set, which is a
	// different vocabulary and a different target from the task tags on
	// ValueProviderTags.
	ValueProviderProjectTags ValueProviderID = "project-tags"

	// ValueProviderProjectComment is the typed text of one new project comment.
	// It shares every rule with ValueProviderComment — multiline, bounded by the
	// same limit, refusing an empty buffer — and is still its own provider,
	// because the target it freezes is a project and the prompt built from a task
	// ref would have nothing to resolve on a project header.
	ValueProviderProjectComment ValueProviderID = "project-comment"

	// Filter providers feed the Outliner's view-state filter facets rather than
	// a task mutation; priority is preset-only while states, tags, due, and
	// completion age use pickers.
	ValueProviderFilterPriority      ValueProviderID = "filter-priority"
	ValueProviderFilterStates        ValueProviderID = "filter-states"
	ValueProviderFilterTags          ValueProviderID = "filter-tags"
	ValueProviderFilterDue           ValueProviderID = "filter-due"
	ValueProviderFilterCompletionAge ValueProviderID = "filter-completion-age"
	ValueProviderFilterHierarchy     ValueProviderID = "filter-hierarchy"
	ValueProviderFilterDependency    ValueProviderID = "filter-dependency"

	// ValueProviderFilterContexts feeds the context facet: a distinct axis from
	// ValueProviderFilterTags, whose vocabulary it may share spellings with but
	// whose question it does not ask. A task carries any number of tags and at
	// most one context, so this facet composes with OR where tags compose with
	// AND, and it offers a no-context row tags have no equivalent of.
	ValueProviderFilterContexts ValueProviderID = "filter-contexts"

	// ValueProviderFilterProjects feeds the project facet: which loaded
	// projects' rows show, by slug. It is a row facet and reloads nothing, unlike
	// ValueProviderFilterProjectTags, and it names several projects where the P
	// scope names one.
	ValueProviderFilterProjects ValueProviderID = "filter-projects"

	// ValueProviderFilterProjectTags feeds the population facet: a different
	// vocabulary from ValueProviderFilterTags, and a different question. It picks
	// which projects load rather than which of a loaded project's rows show.
	ValueProviderFilterProjectTags ValueProviderID = "filter-project-tags"

	// ValueProviderProjectTagDestination names the tag that inherits the members a
	// vocabulary-wide tag removal would otherwise strand.
	ValueProviderProjectTagDestination ValueProviderID = "project-tag-destination"
)

// TextEntry reports whether the provider's value is typed rather than picked
// from a list, so the value step opens the text prompt instead of the picker.
func (id ValueProviderID) TextEntry() bool {
	return id == ValueProviderNoteTitle ||
		id == ValueProviderTagName ||
		id == ValueProviderChecklistItem ||
		id == ValueProviderLensName ||
		id == ValueProviderResumeNote ||
		id == ValueProviderComment ||
		id == ValueProviderProjectComment
}

// TagEntry reports whether the picker offers "a" to mint a tag the vocabulary
// does not contain yet. Both tag namespaces need it because neither is a closed
// enumeration the way states and priorities are.
func (id ValueProviderID) TagEntry() bool {
	return id == ValueProviderTags || id == ValueProviderProjectTags
}

// DateEntry reports whether the picker offers "a" to type a date its relative
// presets cannot express. The calendar is not a closed enumeration either, so
// both date providers need the same escape hatch.
func (id ValueProviderID) DateEntry() bool {
	return id == ValueProviderDate || id == ValueProviderDueDate
}

// Prompt names what choosing an operation that needs this provider will ask
// for, in the words a menu row can show beside the operation's own label. The
// identifier is a registry key, not prose, so every provider declares its
// phrase here rather than having one spelled from its hyphens.
func (id ValueProviderID) Prompt() string {
	switch id {
	case ValueProviderState:
		return "choose a status"
	case ValueProviderPriority:
		return "choose a priority"
	case ValueProviderTags:
		return "choose tags and context"
	case ValueProviderNoteTitle:
		return "name the note"
	case ValueProviderTagName:
		return "type the new name"
	case ValueProviderDate:
		return "choose a date"
	case ValueProviderCheckIn:
		return "choose a day and repeat"
	case ValueProviderDueDate:
		return "choose a due date"
	case ValueProviderChecklistItem:
		return "type the item"
	case ValueProviderResumeNote:
		return "type the note"
	case ValueProviderComment, ValueProviderProjectComment:
		return "type the comment"
	case ValueProviderLensName:
		return "name the lens"
	case ValueProviderLensTemplate:
		return "choose a template"
	case ValueProviderParent:
		return "choose the parent"
	case ValueProviderChild:
		return "choose the children"
	case ValueProviderTaskReference:
		return "choose dependencies"
	case ValueProviderProject:
		return "choose a project"
	case ValueProviderProjectTags:
		return "choose the project's tags"
	case ValueProviderFilterPriority:
		return "choose priorities"
	case ValueProviderFilterStates:
		return "choose statuses"
	case ValueProviderFilterTags:
		return "choose tags"
	case ValueProviderFilterDue:
		return "choose a date range"
	case ValueProviderFilterCompletionAge:
		return "choose an age"
	case ValueProviderFilterHierarchy:
		return "choose parents or leaves"
	case ValueProviderFilterDependency:
		return "choose a reference filter"
	case ValueProviderFilterContexts:
		return "choose contexts"
	case ValueProviderFilterProjects:
		return "choose projects"
	case ValueProviderFilterProjectTags:
		return "choose which projects load"
	case ValueProviderProjectTagDestination:
		return "choose the tag that inherits them"
	}
	return "choose " + strings.ReplaceAll(string(id), "-", " ")
}

// CheckInEntry reports whether the operation needs the reusable check-in form.
func (id ValueProviderID) CheckInEntry() bool { return id == ValueProviderCheckIn }

func WithValueProvider(descriptor OperationDescriptor, provider ValueProviderID) OperationDescriptor {
	descriptor.ValueProviderID = provider
	return descriptor
}
