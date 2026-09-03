package operation

// ValueProviderID identifies the picker or other value source an operation
// needs before execution. The empty value means no provider is required.
type ValueProviderID string

const (
	ValueProviderState     ValueProviderID = "state"
	ValueProviderPriority  ValueProviderID = "priority"
	ValueProviderTags      ValueProviderID = "tags"
	ValueProviderNoteTitle ValueProviderID = "note-title"
	ValueProviderTagName   ValueProviderID = "tag-name"

	// ValueProviderDate is the one date picker a task operation reaches. It
	// sections due presets above defer presets so a single key answers both
	// fields, and the selected row carries which field it writes.
	ValueProviderDate ValueProviderID = "date"

	// ValueProviderDueDate is the due-only projection of the same presets. Only
	// the waiting check-back chain opens it, because that step is asking for a
	// deadline specifically and a defer section would be noise there.
	ValueProviderDueDate ValueProviderID = "due-date"

	// ValueProviderChecklistItem is the typed text of one checklist item. It is
	// a separate provider from the note title because the prompt prefills the
	// item under the cursor rather than a note's own name.
	ValueProviderChecklistItem ValueProviderID = "checklist-item"

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
		id == ValueProviderLensName
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

func WithValueProvider(descriptor OperationDescriptor, provider ValueProviderID) OperationDescriptor {
	descriptor.ValueProviderID = provider
	return descriptor
}
