package models

import "strings"

// SemanticTaskTagRole names one work-item kind. The zero value means the task
// carries no semantic tag, and the declared order is the locked precedence: a
// lower non-zero role wins. Precedence orders the `SEMANTIC` picker section and
// its explainer, and selects the accent for a task's workflow-state icon when
// the task carries several semantic tags. Individual tags retain their own chips.
type SemanticTaskTagRole uint8

const (
	SemanticTaskTagNone SemanticTaskTagRole = iota
	SemanticTaskTagBug
	SemanticTaskTagQuestion
	SemanticTaskTagEpic
	SemanticTaskTagStory
	SemanticTaskTagFeature
	SemanticTaskTagIdea
)

// SemanticTaskTag describes one task tag the Outliner reads as a work-item
// kind. It is presentation metadata only: a semantic tag is an ordinary tag in
// storage, in filters, and in every workflow rule.
type SemanticTaskTag struct {
	// Role is the vocabulary's own rank, and its declared order is the
	// precedence.
	Role SemanticTaskTagRole
	// Name is the canonical lowercase tag spelling.
	Name string
	// RowSummary is the compact description shown beside a focusable picker row.
	RowSummary string
	// Summary is the complete explainer shown in the non-focusable block below
	// the picker.
	Summary string
}

// SemanticTaskTags is the single source for every surface that explains a
// semantic task tag: the tag picker sections and their explainers. Unlike
// BehavioralProjectTags it is an ordered slice rather than a map, because the
// bug > question > epic > story > feature > idea precedence is part of what it
// publishes.
var SemanticTaskTags = []SemanticTaskTag{
	{
		Role:       SemanticTaskTagBug,
		Name:       "bug",
		RowSummary: "A defect to fix. Highest precedence.",
		Summary:    "A defect to fix. Highest precedence: it leads the semantic tags wherever they are listed.",
	},
	{
		Role:       SemanticTaskTagQuestion,
		Name:       "question",
		RowSummary: "An open question to investigate.",
		Summary:    "An open question to investigate before the work can proceed. It outranks every kind but bug.",
	},
	{
		Role:       SemanticTaskTagEpic,
		Name:       "epic",
		RowSummary: "A large body of related work.",
		Summary:    "A large body of work that groups related tasks.",
	},
	{
		Role:       SemanticTaskTagStory,
		Name:       "story",
		RowSummary: "A user-facing slice of work.",
		Summary:    "A user-facing slice of work.",
	},
	{
		Role:       SemanticTaskTagFeature,
		Name:       "feature",
		RowSummary: "New capability to build.",
		Summary:    "New capability to build.",
	},
	{
		Role:       SemanticTaskTagIdea,
		Name:       "idea",
		RowSummary: "An unrefined thought. Lowest precedence.",
		Summary:    "An unrefined thought to develop later. Lowest precedence.",
	},
}

// SemanticTaskTagByName resolves one tag value. Matching is exact and
// case-insensitive with no trimming, which is what keeps "type:bug", "bugfix"
// and " idea " ordinary tags.
func SemanticTaskTagByName(tag string) (SemanticTaskTag, bool) {
	for _, entry := range SemanticTaskTags {
		if strings.EqualFold(tag, entry.Name) {
			return entry, true
		}
	}
	return SemanticTaskTag{}, false
}
