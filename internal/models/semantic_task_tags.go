package models

import "strings"

// SemanticTaskTagRole names one work-item kind the Outliner colors a task title
// with. The zero value means the task carries no semantic tag, and the declared
// order is the locked precedence: a lower non-zero role wins.
type SemanticTaskTagRole uint8

const (
	SemanticTaskTagNone SemanticTaskTagRole = iota
	SemanticTaskTagBug
	SemanticTaskTagEpic
	SemanticTaskTagStory
	SemanticTaskTagFeature
	SemanticTaskTagIdea
)

// SemanticTaskTag describes one task tag the Outliner reads as a work-item
// kind. It is presentation metadata only: a semantic tag is an ordinary tag in
// storage, in filters, and in every workflow rule.
type SemanticTaskTag struct {
	// Role is the style role the title renders with, and its declared order is
	// the precedence.
	Role SemanticTaskTagRole
	// Name is the canonical lowercase tag spelling.
	Name string
	// Summary is the one-sentence explainer shown wherever the tag is offered.
	Summary string
}

// SemanticTaskTags is the single source for every surface that explains a
// semantic task tag: the tag picker sections and their explainers. Unlike
// BehavioralProjectTags it is an ordered slice rather than a map, because the
// bug > epic > story > feature > idea precedence is part of what it publishes.
var SemanticTaskTags = []SemanticTaskTag{
	{
		Role:    SemanticTaskTagBug,
		Name:    "bug",
		Summary: "A defect to fix. Highest precedence: it colors the title over any other semantic tag.",
	},
	{
		Role:    SemanticTaskTagEpic,
		Name:    "epic",
		Summary: "A large body of work that groups related tasks.",
	},
	{
		Role:    SemanticTaskTagStory,
		Name:    "story",
		Summary: "A user-facing slice of work.",
	},
	{
		Role:    SemanticTaskTagFeature,
		Name:    "feature",
		Summary: "New capability to build.",
	},
	{
		Role:    SemanticTaskTagIdea,
		Name:    "idea",
		Summary: "An unrefined thought to develop later. Lowest precedence.",
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

// SemanticTaskTagForTags returns the highest-precedence semantic tag a task
// carries. Slice order never affects the result.
func SemanticTaskTagForTags(tags []string) (SemanticTaskTag, bool) {
	var best SemanticTaskTag
	for _, tag := range tags {
		entry, ok := SemanticTaskTagByName(tag)
		if !ok {
			continue
		}
		if best.Role == SemanticTaskTagNone || entry.Role < best.Role {
			best = entry
		}
	}
	return best, best.Role != SemanticTaskTagNone
}
