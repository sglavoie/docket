package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/valuepicker"
)

// The shell side of the filter pickers' rename/merge workflow. The picker owns
// the three steps and none of the effects: it cannot take a workspace census,
// cannot commit, and cannot read the saved lenses, so every sentence it shows
// was computed here and handed back through its three setters. That split is
// also what makes the workflow safe to walk away from — the frozen preview
// lives on the AppModel, and the picker's browse state lives on the picker.

// vocabularyRenameAxis binds one filter picker to everything a rename needs to
// know about it: the application namespace it renames in, the query field a
// saved lens spells that namespace with, the row-ID prefix its list is built
// from, and the sigil and noun its wording is written in.
type vocabularyRenameAxis struct {
	namespace app.TaskNameNamespace
	field     models.QueryField
	rowPrefix string
	sigil     string
	noun      string
}

// vocabularyRenameAxisFor is the whole set: the two filter facets that
// enumerate every spelling in use are the two surfaces a rename is reachable
// from, so a provider outside it has no axis and no workflow.
func vocabularyRenameAxisFor(provider operation.ValueProviderID) (vocabularyRenameAxis, bool) {
	switch provider {
	case operation.ValueProviderFilterTags:
		return vocabularyRenameAxis{
			namespace: app.TaskTagNamespace,
			field:     models.QueryFieldTag,
			rowPrefix: "tag:",
			sigil:     "#",
			noun:      "tag",
		}, true
	case operation.ValueProviderFilterContexts:
		return vocabularyRenameAxis{
			namespace: app.TaskContextNamespace,
			field:     models.QueryFieldContext,
			rowPrefix: "filter-context:",
			sigil:     "##",
			noun:      "context",
		}, true
	default:
		return vocabularyRenameAxis{}, false
	}
}

// vocabularyRenameSession is the plan the shell holds between the confirmation
// it drew and the acceptance that commits it. The preview pins a whole
// workspace census, so it is released on every exit from the confirmation —
// accepted, refused, or walked back.
type vocabularyRenameSession struct {
	preview app.RenameTaskNamePreview
	lenses  []vocabularyRenameLensImpact
}

// held reports whether a plan is still frozen. A preview with no affected task
// is never retained, so an empty task list is exactly "nothing is held".
func (session vocabularyRenameSession) held() bool { return len(session.preview.Tasks) > 0 }

// vocabularyRenameLensImpact is one saved lens whose predicate may select
// differently after a rename, named with the clauses that made it so.
type vocabularyRenameLensImpact struct {
	name    string
	clauses []string
}

// handleVocabularyRenameRequested answers a typed destination with a frozen
// plan. Everything the confirmation states — the counts, the scope, the merge,
// and the affected lenses — is computed once, here, from the same census the
// apply will be validated against. It issues no command: the plan is written
// straight onto the mounted picker, which the very next render reads.
func (m *AppModel) handleVocabularyRenameRequested(msg valuepicker.VocabularyRenameRequestedMsg) {
	picker := m.operations.ValuePicker
	axis, ok := vocabularyRenameAxisFor(msg.Provider)
	if picker == nil || !ok {
		return
	}
	// A superseded plan is released before a new one is computed, so a failed
	// preview cannot leave the previous confirmation's census behind it.
	m.vocabularyRename = vocabularyRenameSession{}
	if m.domain.service == nil {
		picker.RefuseVocabularyRename("This workspace cannot commit a rename")
		return
	}
	preview, err := m.domain.service.PreviewTaskNameRename(app.RenameTaskNameRequest{
		Namespace: axis.namespace,
		From:      msg.From,
		To:        msg.To,
	})
	if err != nil {
		picker.RefuseVocabularyRename(operationRefusalReason(err))
		return
	}
	if len(preview.Tasks) == 0 {
		// The picker lists the scope's vocabulary and the census reads the whole
		// workspace, so this is nearly unreachable; saying so beats confirming a
		// change that would write nothing.
		picker.RefuseVocabularyRename(fmt.Sprintf("No task carries %s%s", axis.sigil, preview.From))
		return
	}
	impacts := m.vocabularyRenameLensImpacts(axis.field, preview.From, preview.To)
	m.vocabularyRename = vocabularyRenameSession{preview: preview, lenses: impacts}
	picker.ShowVocabularyRenamePlan(valuepicker.VocabularyRenamePlan{
		Heading: vocabularyRenameHeading(axis, preview.Merge),
		Lines:   vocabularyRenamePlanLines(axis, preview, impacts),
	})
}

// handleVocabularyRenameAbandoned releases the census a declined confirmation
// froze. The picker has already stepped back to its name field, so this is a
// release and nothing else.
func (m *AppModel) handleVocabularyRenameAbandoned() {
	m.vocabularyRename = vocabularyRenameSession{}
}

// handleVocabularyRenameConfirmed commits the frozen plan. The preview is
// released before the commit is attempted, so a refusal — stale or otherwise —
// leaves nothing held and sends the reader back to the name step rather than to
// a confirmation whose census is gone.
func (m *AppModel) handleVocabularyRenameConfirmed(msg valuepicker.VocabularyRenameConfirmedMsg) tea.Cmd {
	picker := m.operations.ValuePicker
	axis, ok := vocabularyRenameAxisFor(msg.Provider)
	if picker == nil || !ok || !picker.VocabularyRenameConfirming() {
		return nil
	}
	session := m.vocabularyRename
	m.vocabularyRename = vocabularyRenameSession{}
	if !session.held() {
		picker.RefuseVocabularyRename("That plan is no longer held; press Enter to review it again")
		return nil
	}
	result, err := m.domain.service.ApplyTaskNameRename(session.preview)
	if err != nil {
		picker.RefuseVocabularyRename(vocabularyRenameApplyRefusal(err))
		return nil
	}
	for _, projectID := range result.TouchedProjectIDs {
		m.reloadProjectByID(projectID)
		m.reloadCrossProjectOutlinerProject(projectID)
	}
	m.refreshLoadedTaskWorkspace()
	m.refreshWorkspacePulseAt(time.Now())
	m.refreshVocabularyRenamePicker(picker, axis, session, valuepicker.VocabularyRenameOutcome{
		Heading: vocabularyRenameHeading(axis, session.preview.Merge) + " · done",
		Lines:   vocabularyRenameOutcomeLines(axis, session, result),
	})
	return m.setRecoveryFeedback(
		vocabularyRenameStatus(axis, session.preview, result),
		result.TransactionID,
		false,
	)
}

// vocabularyRenameApplyRefusal names an expired preview as the one thing it is:
// the workspace moved under a plan nobody had accepted yet, nothing was
// written, and the same rename can simply be reviewed again.
func vocabularyRenameApplyRefusal(err error) string {
	if errors.Is(err, storage.ErrStaleRevision) || errors.Is(err, storage.ErrStaleArtifact) {
		return "The workspace changed while this was open; press Enter to review the rename again"
	}
	return operationRefusalReason(err)
}

// operationRefusalReason is what a rejected value is told, on any overlay that
// stays open holding the value it refused. A validation refusal already names
// the field it rejected, and the open step is that field, so only the reason is
// retained: the step says what it is refusing, not which layer refused it.
func operationRefusalReason(err error) string {
	var invalid *app.InvalidInputError
	if errors.As(err, &invalid) {
		return invalid.Reason
	}
	return err.Error()
}

// refreshVocabularyRenamePicker replaces the picker with one built from the
// renamed workspace, carrying the browse state across: the pending multi-select
// with the source row remapped onto the destination, the picker-local search,
// and a cursor on the row the rename produced. Only the search may be given up,
// and only when it would hide that row — a report naming a row nobody can see
// is worse than a query the reader can retype.
func (m *AppModel) refreshVocabularyRenamePicker(
	picker *valuepicker.OperationValuePickerModel,
	axis vocabularyRenameAxis,
	session vocabularyRenameSession,
	outcome valuepicker.VocabularyRenameOutcome,
) {
	rebuilt, err := m.newOperationValuePicker(picker.ExecutionRequest(), picker.Provider())
	if err != nil {
		// The rename committed; only the refreshed list is missing. Retaining the
		// report on the stale picker still tells the truth about what was written.
		logging.Warn("failed to rebuild %s filter picker after a rename: %v", axis.noun, err)
		picker.AdoptVocabularyRenameOutcome(outcome)
		return
	}
	source := listsurface.ListItemID(axis.rowPrefix + session.preview.From)
	destination := listsurface.ListItemID(axis.rowPrefix + session.preview.To)
	surface := rebuilt.Surface()
	if pending := vocabularyRenamePendingSelection(
		picker.Surface(), surface, source, destination,
	); len(pending) > 0 {
		if err := surface.SetSelectedIDs(pending); err != nil {
			logging.Warn("failed to restore pending filter values after a rename: %v", err)
		}
	}
	surface.SetQuery(picker.Surface().Query().Raw)
	if !surface.SetCursorToID(destination) {
		surface.SetQuery("")
		surface.SetCursorToID(destination)
	}
	rebuilt.AdoptVocabularyRenameOutcome(outcome)
	m.operations.ValuePicker = rebuilt
}

// vocabularyRenamePendingSelection maps the choices pending on the replaced
// picker onto the rebuilt one. A merge selects both names, so the remap
// deduplicates, and a row the rename removed is simply dropped rather than
// refused by SetSelectedIDs.
func vocabularyRenamePendingSelection(
	previous, rebuilt *listsurface.ListSurfaceModel,
	source, destination listsurface.ListItemID,
) []listsurface.ListItemID {
	available := make(map[listsurface.ListItemID]bool, len(rebuilt.Items()))
	for _, item := range rebuilt.Items() {
		available[item.ID] = true
	}
	seen := make(map[listsurface.ListItemID]bool, len(available))
	pending := make([]listsurface.ListItemID, 0, len(available))
	for _, id := range previous.SelectedIDs() {
		if id == source {
			id = destination
		}
		if !available[id] || seen[id] {
			continue
		}
		seen[id] = true
		pending = append(pending, id)
	}
	return pending
}

// vocabularyRenameLensImpacts reads the effective saved-lens collection and
// reports the ones whose predicates may select differently afterwards. The
// collection read here is the one the lens picker reads: the portable
// .tt/lenses.json has already replaced the machine-local list at load when it
// was defined, and the machine-local list survives when it was not, so this
// inherits that fallback rather than restating it.
//
// Nothing here rewrites a lens. A saved query is a durable string its author
// wrote, and a rename is not an edit to it; naming the ones that now read
// differently is the whole of what is owed.
func (m *AppModel) vocabularyRenameLensImpacts(field models.QueryField, from, to string) []vocabularyRenameLensImpact {
	machine := m.domain.machine()
	if machine == nil {
		return nil
	}
	var impacts []vocabularyRenameLensImpact
	for _, saved := range machine.NamedLenses {
		clauses := vocabularyRenameAffectedClauses(saved, field, from, to)
		if len(clauses) == 0 {
			continue
		}
		impacts = append(impacts, vocabularyRenameLensImpact{name: saved.Name, clauses: clauses})
	}
	return impacts
}

// vocabularyRenameAffectedClauses names the clauses of one lens that select the
// two names differently. Partitioning is the read path a lens already has, so a
// token this vocabulary cannot parse stays in the remainder and is not guessed
// at. A clause is reported when it would admit exactly one of the two names,
// which is what makes both a full name and a prefix answerable; polarity never
// changes that test, and String already renders the leading `-`.
func vocabularyRenameAffectedClauses(saved config.NamedLens, field models.QueryField, from, to string) []string {
	query, _ := models.PartitionQueryString(saved.Query)
	var affected []string
	seen := make(map[string]bool, len(query.Clauses))
	for _, clause := range query.Clauses {
		if clause.Field != field {
			continue
		}
		if vocabularyRenameClauseSelects(clause, from) == vocabularyRenameClauseSelects(clause, to) {
			continue
		}
		spelled := clause.String()
		if seen[spelled] {
			continue
		}
		seen[spelled] = true
		affected = append(affected, spelled)
	}
	return affected
}

// vocabularyRenameClauseSelects reports whether one clause admits one name,
// ignoring polarity: a negation owns no vocabulary of its own, so it selects
// exactly what its positive twin does and merely subtracts the result.
func vocabularyRenameClauseSelects(clause models.QueryClause, name string) bool {
	switch clause.Operator {
	case models.QueryOperatorIs:
		return clause.Value == name
	case models.QueryOperatorPrefix:
		return strings.HasPrefix(name, clause.Value)
	default:
		// Every other operator answers a question a stored name is not the
		// subject of, so neither spelling is selected and the clause is silent
		// about the rename rather than reported by it.
		return false
	}
}

// vocabularyRenameHeading is the four-way title the undo ledger also writes, so
// the confirmation, the report, and the entry `u` takes back all name the same
// act. A merge is plural there because it names two spellings, and the wording
// is repeated rather than imported: the ledger's is an unexported application
// detail, and a heading that drifted from it would be visible in the very
// status line the commit hands back.
func vocabularyRenameHeading(axis vocabularyRenameAxis, merge bool) string {
	noun := strings.ToUpper(axis.noun[:1]) + axis.noun[1:]
	if merge {
		return "Merge " + noun + "s"
	}
	return "Rename " + noun
}

func vocabularyRenameArrow(axis vocabularyRenameAxis, from, to string) string {
	return axis.sigil + from + " → " + axis.sigil + to
}

func vocabularyRenamePlanLines(
	axis vocabularyRenameAxis,
	preview app.RenameTaskNamePreview,
	impacts []vocabularyRenameLensImpact,
) []string {
	lines := []string{
		vocabularyRenameArrow(axis, preview.From, preview.To),
		fmt.Sprintf(
			"%d %s in %d %s",
			len(preview.Tasks), listsurface.Pluralize(len(preview.Tasks), "task", "tasks"),
			len(preview.ProjectIDs), listsurface.Pluralize(len(preview.ProjectIDs), "project", "projects"),
		),
		// The picker lists the current scope's vocabulary, but the rename is not
		// scoped by it, and a reader who filtered the view has every reason to
		// assume otherwise.
		"Scope: every unarchived project, not the filtered view",
	}
	if preview.Merge {
		lines = append(lines, fmt.Sprintf(
			"%s%s is already in use, so the two become one.", axis.sigil, preview.To,
		))
	}
	return append(lines, vocabularyRenameLensLines(impacts)...)
}

func vocabularyRenameOutcomeLines(
	axis vocabularyRenameAxis,
	session vocabularyRenameSession,
	result app.RenameTaskNameResult,
) []string {
	lines := []string{
		vocabularyRenameArrow(axis, session.preview.From, session.preview.To),
		fmt.Sprintf(
			"%d %s rewritten in %d %s",
			len(result.Tasks), listsurface.Pluralize(len(result.Tasks), "task", "tasks"),
			len(result.TouchedProjectIDs),
			listsurface.Pluralize(len(result.TouchedProjectIDs), "project", "projects"),
		),
	}
	return append(lines, vocabularyRenameLensLines(session.lenses)...)
}

// vocabularyRenameLensLines is the saved-lens report both steps carry, so the
// confirmation and the outcome name the same lenses in the same words.
func vocabularyRenameLensLines(impacts []vocabularyRenameLensImpact) []string {
	if len(impacts) == 0 {
		return []string{"", "No saved lens names it."}
	}
	lines := []string{"", fmt.Sprintf(
		"%d saved %s may select differently, and %s left unchanged:",
		len(impacts), listsurface.Pluralize(len(impacts), "lens", "lenses"),
		listsurface.Pluralize(len(impacts), "it is", "they are"),
	)}
	for _, impact := range impacts {
		lines = append(lines, "  "+impact.name+" · "+strings.Join(impact.clauses, " "))
	}
	return lines
}

func vocabularyRenameStatus(
	axis vocabularyRenameAxis,
	preview app.RenameTaskNamePreview,
	result app.RenameTaskNameResult,
) string {
	verb := "Renamed"
	if preview.Merge {
		verb = "Merged"
	}
	return fmt.Sprintf(
		"%s %s in %d %s",
		verb,
		vocabularyRenameArrow(axis, preview.From, preview.To),
		len(result.Tasks), listsurface.Pluralize(len(result.Tasks), "task", "tasks"),
	)
}
