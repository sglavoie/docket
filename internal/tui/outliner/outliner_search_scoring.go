package outliner

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sglavoie/docket/internal/fuzzy"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/query"
)

// searchAutocompleteCandidateTasks returns exactly the tasks search itself can
// inspect: the loaded scope, narrowed to the same candidate boundary the
// projection is built from — the Focus branch and the active facets. Completion
// reads the same authority, so it can never offer a value no admissible result
// carries.

// searchProjectionMatchesWithin applies an optional authoritative candidate
// boundary. A present empty entry means that project contributes no matches;
// a missing entry retains the scope-wide behavior used outside Focus.
func searchProjectionMatchesWithin(
	projects []*models.Project,
	query string,
	candidates map[string][]*models.Task,
	matcher fuzzy.Matcher,
	predicates *query.SearchPredicates,
	caches ...*fuzzy.ScoreCache,
) []outlinerSearchMatch {
	return searchProjectionMatchesWithinContext(context.Background(), projects, query, candidates, matcher, predicates, caches...)
}

func searchProjectionMatchesWithinContext(
	ctx context.Context,
	projects []*models.Project,
	query string,
	candidates map[string][]*models.Task,
	matcher fuzzy.Matcher,
	predicates *query.SearchPredicates,
	caches ...*fuzzy.ScoreCache,
) []outlinerSearchMatch {
	if ctx.Err() != nil || predicates.Invalid() {
		return nil
	}
	if projectRef, shortID, exact := parseProjectShortIDQuery(query); exact {
		return exactProjectShortIDMatchesWithin(projects, projectRef, shortID, candidates, predicates)
	}

	matches := make([]outlinerSearchMatch, 0)
	terms := matcher.CompileTerms(query)
	if len(caches) > 0 {
		terms = terms.WithCache(caches[0])
	}
	sourceIndex := 0
	for _, project := range projects {
		if project == nil || !predicates.AdmitsProject(project) {
			continue
		}
		tasks := project.Tasks
		if bounded, ok := candidates[project.ID]; ok {
			tasks = bounded
		}
		for _, task := range tasks {
			if ctx.Err() != nil {
				return nil
			}
			if task == nil || !predicates.AdmitsPopulation(task, project) {
				sourceIndex++
				continue
			}
			if !predicates.Admits(task, project) {
				sourceIndex++
				continue
			}
			fieldRank, score, source, ok := scoreSearchProjectionTask(task, project, query, terms)
			if ok {
				matches = append(matches, outlinerSearchMatch{
					task: task, project: project, fieldRank: fieldRank,
					fuzzyScore: score, source: source, sourceIndex: sourceIndex,
				})
			}
			sourceIndex++
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].fieldRank != matches[j].fieldRank {
			return matches[i].fieldRank < matches[j].fieldRank
		}
		if matches[i].fuzzyScore != matches[j].fuzzyScore {
			return matches[i].fuzzyScore > matches[j].fuzzyScore
		}
		return matches[i].sourceIndex < matches[j].sourceIndex
	})
	return matches
}

// scoreSearchProjectionTask ranks one task against the free-text remainder in
// the declared field tiers. Each tier is scored on its own text, so a hit is
// attributable to exactly one source; only the last tier, details, scores the
// concatenation, and it exists solely for multi-term queries whose terms are
// satisfied across fields. Because rank sorts before score, a details hit can
// never outrank a single-field one.
func scoreSearchProjectionTask(
	task *models.Task,
	project *models.Project,
	query string,
	matcher fuzzy.Terms,
) (int, int, MatchSource, bool) {
	if query == "" {
		return searchFieldRankTitle, 1, MatchSourceNone, true
	}

	if score := matcher.Score(task.Title); score > 0 {
		return searchFieldRankTitle, score, MatchSourceTitle, true
	}
	if score := matcher.Score(task.Description); score > 0 {
		return searchFieldRankDescription, score, MatchSourceDescription, true
	}
	if score := matcher.Score(task.ResumeNote); score > 0 {
		return searchFieldRankResume, score, MatchSourceResume, true
	}
	if score := matcher.ScoreParts(searchChecklistParts(task)); score > 0 {
		return searchFieldRankChecklist, score, MatchSourceChecklist, true
	}
	if score := matcher.ScoreParts(searchCommentParts(task)); score > 0 {
		return searchFieldRankComment, score, MatchSourceComment, true
	}
	// Tags and context share a rank, so the better score wins and an exact tie
	// falls to the declared field order: tags precede context.
	tagScore := matcher.Score(strings.Join(task.Tags, " "))
	contextScore := matcher.Score(task.Context)
	if tagScore > 0 || contextScore > 0 {
		if tagScore >= contextScore {
			return searchFieldRankTagContext, tagScore, MatchSourceTag, true
		}
		return searchFieldRankTagContext, contextScore, MatchSourceContext, true
	}
	if score := matcher.Score(searchAttachmentText(task)); score > 0 {
		return searchFieldRankAttachment, score, MatchSourceAttachment, true
	}
	if score := matcher.Score(searchProjectText(task, project)); score > 0 {
		return searchFieldRankProject, score, MatchSourceProject, true
	}

	details := searchMatchText(task, project, MatchSourceDetails)
	if score := matcher.Score(details); score > 0 {
		return searchFieldRankDetails, score, MatchSourceDetails, true
	}
	return searchFieldRankTitle, 0, MatchSourceNone, false
}

// searchChecklistText is the item text alone: completion state is presentation,
// never a term a text query can match or miss on.
func searchChecklistText(task *models.Task) string {
	return strings.Join(searchChecklistParts(task), " ")
}

func searchChecklistParts(task *models.Task) []string {
	if len(task.Checklist) == 0 {
		return nil
	}
	parts := make([]string, 0, len(task.Checklist))
	for _, item := range task.Checklist {
		parts = append(parts, item.Text)
	}
	return parts
}

func searchCommentText(task *models.Task) string {
	return strings.Join(searchCommentParts(task), " ")
}

func searchCommentParts(task *models.Task) []string {
	if len(task.Comments) == 0 {
		return nil
	}
	parts := make([]string, 0, len(task.Comments))
	for _, comment := range task.Comments {
		if comment != nil {
			parts = append(parts, comment.Text)
		}
	}
	return parts
}

// searchAttachmentText covers both the displayed name (filename for files, the
// full URL for links) and the original path a code reference or local file was
// captured from.
func searchAttachmentText(task *models.Task) string {
	if len(task.Attachments) == 0 {
		return ""
	}
	parts := make([]string, 0, 2*len(task.Attachments))
	for _, attachment := range task.Attachments {
		parts = append(parts, attachment.Filename, attachment.OriginalPath)
	}
	return strings.Join(parts, " ")
}

// searchProjectText carries the owning project's metadata plus the qualified
// task reference, which is the fuzzy counterpart of the exact project#ID path.
func searchProjectText(task *models.Task, project *models.Project) string {
	return strings.Join([]string{
		project.Name,
		project.Slug,
		project.Description,
		fmt.Sprintf("%s#%d", project.Slug, task.ShortID),
	}, " ")
}

func parseProjectShortIDQuery(query string) (string, int, bool) {
	query = strings.TrimSpace(query)
	separator := strings.LastIndex(query, "#")
	if separator <= 0 || separator == len(query)-1 {
		return "", 0, false
	}
	shortID, err := strconv.Atoi(query[separator+1:])
	if err != nil || shortID <= 0 {
		return "", 0, false
	}
	projectRef := strings.TrimSpace(query[:separator])
	return projectRef, shortID, projectRef != ""
}

func exactProjectShortIDMatchesWithin(
	projects []*models.Project,
	projectRef string,
	shortID int,
	candidates map[string][]*models.Task,
	predicates *query.SearchPredicates,
) []outlinerSearchMatch {
	for _, project := range projects {
		if project == nil || !projectSearchRefMatches(project, projectRef) {
			continue
		}
		if !predicates.AdmitsProject(project) {
			return nil
		}
		tasks := project.Tasks
		if bounded, ok := candidates[project.ID]; ok {
			tasks = bounded
		}
		for sourceIndex, task := range tasks {
			if task == nil || task.ShortID != shortID {
				continue
			}
			if !predicates.AdmitsPopulation(task, project) {
				continue
			}
			if predicates.Admits(task, project) {
				// The qualified reference lives in the project tier, so an
				// exact hit is attributed there while keeping the exact
				// path's top rank.
				return []outlinerSearchMatch{{
					task: task, project: project, fieldRank: searchFieldRankTitle,
					fuzzyScore: 1, source: MatchSourceProject, sourceIndex: sourceIndex,
				}}
			}
		}
		return nil
	}
	return nil
}

func projectSearchRefMatches(project *models.Project, ref string) bool {
	return strings.EqualFold(ref, project.Slug) ||
		strings.EqualFold(ref, project.Name) ||
		strings.EqualFold(ref, project.ID)
}

// searchMatchText uses the same field text as ranking, including its combined
// details tier when separate query terms occur in different fields.
func searchMatchText(task *models.Task, project *models.Project, source MatchSource) string {
	switch source {
	case MatchSourceTitle:
		return task.Title
	case MatchSourceDescription:
		return task.Description
	case MatchSourceResume:
		return task.ResumeNote
	case MatchSourceChecklist:
		return searchChecklistText(task)
	case MatchSourceComment:
		return searchCommentText(task)
	case MatchSourceTag:
		return strings.Join(task.Tags, " ")
	case MatchSourceContext:
		return task.Context
	case MatchSourceAttachment:
		return searchAttachmentText(task)
	case MatchSourceProject:
		if project != nil {
			return searchProjectText(task, project)
		}
		return ""
	case MatchSourceDetails:
		var fields []string
		for field := MatchSourceTitle; field < MatchSourceDetails; field++ {
			fields = append(fields, searchMatchText(task, project, field))
		}
		return strings.Join(fields, " ")
	default:
		return ""
	}
}
