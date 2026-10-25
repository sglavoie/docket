package config

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/sglavoie/docket/internal/models"
)

// Schema 16 and earlier persisted a lens as two filter carriers beside a
// free-text query: one struct per facet family, each field a token whose meaning
// only the TUI knew. Schema 17 replaced all of it with one canonical query
// string, so the carriers live here, in migration-only code, together with the
// one translation from them into that string.
//
// Nothing outside this file and NamedLens.UnmarshalJSON may read these types. A
// runtime that learned them again would be a second answer to "what does this
// lens select", which is exactly what the collapse removed.

// v16LensTaskFilters is the retired task-facet carrier. StatesSet and
// CompletionAgeSet were needed because each distinguished an absent predicate
// from an active one that happened to admit everything; the canonical query says
// that with the presence or absence of a clause instead.
type v16LensTaskFilters struct {
	States           []string `json:"states,omitempty"`
	StatesSet        bool     `json:"states_set,omitempty"`
	Priority         int      `json:"priority,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	Contexts         []string `json:"contexts,omitempty"`
	NoContext        bool     `json:"no_context,omitempty"`
	DueDate          string   `json:"due_date,omitempty"`
	CompletionAge    string   `json:"completion_age,omitempty"`
	CompletionAgeSet bool     `json:"completion_age_set,omitempty"`
	Hierarchy        string   `json:"hierarchy,omitempty"`
	Stale            bool     `json:"stale,omitempty"`
	Deferred         bool     `json:"deferred,omitempty"`
	Pinned           bool     `json:"pinned,omitempty"`
	Review           bool     `json:"review,omitempty"`
}

// v16LensProjectFilters is the retired population-facet carrier.
type v16LensProjectFilters struct {
	Tags         []string `json:"tags,omitempty"`
	Untagged     bool     `json:"untagged,omitempty"`
	ParkedMode   string   `json:"parked,omitempty"`
	ArchivedMode string   `json:"archived,omitempty"`
}

// Retired token vocabularies. Each spelled one carrier field and nothing else,
// so each retires with the carrier that held it.
const (
	v16LensPopulationExclude = "exclude"
	v16LensPopulationInclude = "include"
	v16LensPopulationOnly    = "only"

	v16LensDueAll     = "all"
	v16LensDueOverdue = "overdue"
	v16LensDueToday   = "today"
	v16LensDueWeek    = "week"
	v16LensDueNone    = "none"

	v16LensHierarchyAll    = "all"
	v16LensHierarchyParent = "parent"
	v16LensHierarchyChild  = "child"
	v16LensHierarchyRoot   = "root"
	v16LensHierarchyLeaf   = "leaf"

	v16LensCompletionAgeAll    = "all"
	v16LensCompletionAge7Days  = "7d"
	v16LensCompletionAge30Days = "30d"
	v16LensCompletionAge90Days = "90d"

	// v16LensSortStatus is the retired sort field. Sorting by lifecycle state was
	// dropped from the Outliner, so a persisted lens naming it restores the
	// default order rather than refusing to load.
	v16LensSortStatus = "status"
)

// v16LensReviewTag is the tag name review was spelled with before schema 15
// lifted it onto a stored flag. A lens narrowing by it meant "up for review",
// and that is what it must keep meaning after the tag itself is gone from every
// task document.
const v16LensReviewTag = "review"

// v16NamedLensCarriers is the pair of retired carriers as one persisted unit.
type v16NamedLensCarriers struct {
	tasks    v16LensTaskFilters
	projects v16LensProjectFilters
}

// canonicalQuery renders the carriers and the legacy free-text query as one
// canonical lens query string.
//
// Every carrier value becomes the clause that says the same thing, and the
// legacy query follows as the remainder the user-facing grammar still owns. Two
// translations are not mechanical and are the reason this migration exists at
// all: a task tag named review becomes the stored review predicate, and so does
// a #review term inside the saved query, because schema 15 removed that tag from
// every task as it set the flag.
func (c v16NamedLensCarriers) canonicalQuery(legacyQuery string) (string, error) {
	clauses, err := c.taskClauses()
	if err != nil {
		return "", err
	}
	population, err := c.projectClauses()
	if err != nil {
		return "", err
	}
	facets := models.Query{Clauses: append(clauses, population...)}
	return models.JoinQueryString(facets.Canonical(), rewriteV16ReviewTagQuery(legacyQuery)), nil
}

func (c v16NamedLensCarriers) taskClauses() ([]models.QueryClause, error) {
	tasks := c.tasks
	clauses := make([]models.QueryClause, 0, 12)
	if tasks.Priority != models.PriorityUnset {
		// The carrier meant "this priority or more urgent", which is the at-least
		// operator and never the exact one.
		clause, err := v16LensClause(models.QueryFieldPriority, models.QueryOperatorAtLeast, strconv.Itoa(tasks.Priority))
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, clause)
	}
	// An explicitly empty state set has no clause: the vocabulary has no
	// match-nothing term, and inventing one would narrow a lens that narrowed
	// nothing.
	for _, state := range tasks.States {
		clause, err := v16LensClause(models.QueryFieldState, models.QueryOperatorIs, state)
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, clause)
	}
	if tasks.CompletionAgeSet {
		clause, err := v16LensCompletionAgeClause(tasks.CompletionAge)
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, clause)
	}
	review := tasks.Review
	for _, tag := range slices.Sorted(slices.Values(tasks.Tags)) {
		if strings.EqualFold(tag, v16LensReviewTag) {
			review = true
			continue
		}
		clause, err := v16LensClause(models.QueryFieldTag, models.QueryOperatorIs, tag)
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, clause)
	}
	for _, name := range slices.Sorted(slices.Values(tasks.Contexts)) {
		clause, err := v16LensClause(models.QueryFieldContext, models.QueryOperatorIs, name)
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, clause)
	}
	if tasks.NoContext {
		clause, err := v16LensClause(models.QueryFieldContext, models.QueryOperatorNone, "")
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, clause)
	}
	due, err := v16LensDueClauses(tasks.DueDate)
	if err != nil {
		return nil, err
	}
	clauses = append(clauses, due...)
	hierarchy, err := v16LensHierarchyClauses(tasks.Hierarchy)
	if err != nil {
		return nil, err
	}
	clauses = append(clauses, hierarchy...)
	flags := []struct {
		set   bool
		field models.QueryField
	}{
		{tasks.Stale, models.QueryFieldStale},
		{tasks.Deferred, models.QueryFieldDeferred},
		{tasks.Pinned, models.QueryFieldPinned},
		{review, models.QueryFieldReview},
	}
	for _, flag := range flags {
		if !flag.set {
			continue
		}
		clause, err := v16LensClause(flag.field, models.QueryOperatorPresent, "")
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, clause)
	}
	return clauses, nil
}

func (c v16NamedLensCarriers) projectClauses() ([]models.QueryClause, error) {
	projects := c.projects
	clauses := make([]models.QueryClause, 0, 4)
	for _, tag := range slices.Sorted(slices.Values(projects.Tags)) {
		clause, err := v16LensClause(models.QueryFieldProjectTag, models.QueryOperatorIs, tag)
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, clause)
	}
	if projects.Untagged {
		clause, err := v16LensClause(models.QueryFieldProjectTag, models.QueryOperatorNone, "")
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, clause)
	}
	for _, axis := range []struct {
		field models.QueryField
		mode  string
	}{
		{models.QueryFieldParked, projects.ParkedMode},
		{models.QueryFieldArchived, projects.ArchivedMode},
	} {
		population, err := v16LensPopulationClauses(axis.field, axis.mode)
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, population...)
	}
	return clauses, nil
}

// v16LensCompletionAgeClause spells the cleanup facet. The retired "all"
// threshold was an active predicate admitting every completion timestamp, which
// is the bare present operator rather than an age.
func v16LensCompletionAgeClause(age string) (models.QueryClause, error) {
	switch age {
	case v16LensCompletionAge7Days, v16LensCompletionAge30Days, v16LensCompletionAge90Days:
		return v16LensClause(models.QueryFieldCompleted, models.QueryOperatorAtLeast, age)
	case lensTokenDefaultSentinel, v16LensCompletionAgeAll:
		return v16LensClause(models.QueryFieldCompleted, models.QueryOperatorPresent, "")
	default:
		return models.QueryClause{}, v16LensUnknownValue("completion age", age)
	}
}

func v16LensDueClauses(bucket string) ([]models.QueryClause, error) {
	switch bucket {
	case v16LensDueOverdue, v16LensDueToday, v16LensDueWeek:
		clause, err := v16LensClause(models.QueryFieldDue, models.QueryOperatorIs, bucket)
		return []models.QueryClause{clause}, err
	case v16LensDueNone:
		clause, err := v16LensClause(models.QueryFieldDue, models.QueryOperatorNone, "")
		return []models.QueryClause{clause}, err
	case lensTokenDefaultSentinel, v16LensDueAll:
		return nil, nil
	default:
		return nil, v16LensUnknownValue("due bucket", bucket)
	}
}

// v16LensHierarchyClauses spells one hierarchy position. Root was the negation of
// child even in the carrier, which is why the canonical vocabulary carries no
// root field of its own.
func v16LensHierarchyClauses(position string) ([]models.QueryClause, error) {
	switch position {
	case v16LensHierarchyParent:
		clause, err := v16LensClause(models.QueryFieldParent, models.QueryOperatorPresent, "")
		return []models.QueryClause{clause}, err
	case v16LensHierarchyChild:
		clause, err := v16LensClause(models.QueryFieldChild, models.QueryOperatorPresent, "")
		return []models.QueryClause{clause}, err
	case v16LensHierarchyLeaf:
		clause, err := v16LensClause(models.QueryFieldLeaf, models.QueryOperatorPresent, "")
		return []models.QueryClause{clause}, err
	case v16LensHierarchyRoot:
		clause, err := models.NewQueryClause(
			models.QueryFieldChild, models.QueryPolarityNegative, models.QueryOperatorPresent, "")
		if err != nil {
			return nil, fmt.Errorf("translate retired lens hierarchy %q: %w", position, err)
		}
		return []models.QueryClause{clause}, nil
	case lensTokenDefaultSentinel, v16LensHierarchyAll:
		return nil, nil
	default:
		return nil, v16LensUnknownValue("hierarchy", position)
	}
}

// v16LensPopulationClauses spells one population axis. Exclude was the base
// workspace the loader already owns, so it translates to no clause at all.
func v16LensPopulationClauses(field models.QueryField, mode string) ([]models.QueryClause, error) {
	switch mode {
	case v16LensPopulationInclude:
		clause, err := v16LensClause(field, models.QueryOperatorIs, models.QueryPopulationInclude)
		return []models.QueryClause{clause}, err
	case v16LensPopulationOnly:
		clause, err := v16LensClause(field, models.QueryOperatorIs, models.QueryPopulationOnly)
		return []models.QueryClause{clause}, err
	case lensTokenDefaultSentinel, v16LensPopulationExclude:
		return nil, nil
	default:
		return nil, v16LensUnknownValue(string(field)+" population", mode)
	}
}

// v16LensUnknownValue refuses a carrier value this build cannot translate. Only
// the empty string and the retired axis default mean "no clause"; every other
// unrecognized spelling is a refusal, because dropping it would widen the lens to
// select rows it never selected.
func v16LensUnknownValue(axis, value string) error {
	return fmt.Errorf("retired lens %s %q is not a known value", axis, value)
}

// v16LensClause mints one positive clause, naming the retired carrier in the
// failure. A value the canonical vocabulary refuses is a migration refusal and
// never a silently dropped facet: dropping it would widen the lens to select
// rows it never selected.
func v16LensClause(field models.QueryField, operator models.QueryOperator, value string) (models.QueryClause, error) {
	clause, err := models.NewQueryClause(field, models.QueryPolarityPositive, operator, value)
	if err != nil {
		return models.QueryClause{}, fmt.Errorf("translate retired lens filter to canonical query: %w", err)
	}
	return clause, nil
}

// rewriteV16ReviewTagQuery lifts a saved #review search term onto the stored
// review predicate. It is a token substitution rather than a parse: this layer
// persists the query string and does not own the user-facing grammar, and the
// review tag is the one term whose meaning schema 15 moved out from under a
// saved query. Every other token is passed through untouched, including a tag
// that merely starts with the word.
func rewriteV16ReviewTagQuery(query string) string {
	fields := strings.Fields(query)
	for index, token := range fields {
		switch {
		case strings.EqualFold(token, "#"+v16LensReviewTag):
			fields[index] = "=review"
		case strings.EqualFold(token, "-#"+v16LensReviewTag):
			fields[index] = "-=review"
		}
	}
	return strings.Join(fields, " ")
}

// v16LensSortField translates the retired sort token. Only one field retired, so
// only one spelling is rewritten and every other token is carried through for
// LensSort.validate to accept or refuse.
func v16LensSortField(field string) string {
	if field == v16LensSortStatus {
		return LensSortDefault
	}
	return field
}
