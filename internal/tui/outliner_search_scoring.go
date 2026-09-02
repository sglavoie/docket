package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// searchProjectionMatchesWithin applies an optional authoritative candidate
// boundary. A present empty entry means that project contributes no matches;
// a missing entry retains the scope-wide behavior used outside Focus.
func searchProjectionMatchesWithin(
	projects []*models.Project,
	query string,
	candidates map[string][]*models.Task,
	matcher fuzzy.Matcher,
	predicates *searchPredicates,
) []outlinerSearchMatch {
	if predicates.invalid() {
		return nil
	}
	if projectRef, shortID, exact := parseProjectShortIDQuery(query); exact {
		return exactProjectShortIDMatchesWithin(projects, projectRef, shortID, candidates, predicates)
	}

	matches := make([]outlinerSearchMatch, 0)
	sourceIndex := 0
	for _, project := range projects {
		if project == nil || !predicates.admitsProject(project) {
			continue
		}
		tasks := project.Tasks
		if bounded, ok := candidates[project.ID]; ok {
			tasks = bounded
		}
		for _, task := range tasks {
			if task == nil || !predicates.admitsPopulation(task) {
				sourceIndex++
				continue
			}
			if !predicates.admits(task, project) {
				sourceIndex++
				continue
			}
			fieldRank, score, source, ok := scoreSearchProjectionTask(task, project, query, matcher)
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
	matcher fuzzy.Matcher,
) (int, int, outlinerMatchSource, bool) {
	if query == "" {
		return searchFieldRankTitle, 1, matchSourceNone, true
	}

	checklistText := searchChecklistText(task)
	commentText := searchCommentText(task)
	tagText := strings.Join(task.Tags, " ")
	attachmentText := searchAttachmentText(task)
	projectText := searchProjectText(task, project)

	if score := matcher.ScoreTerms(task.Title, query); score > 0 {
		return searchFieldRankTitle, score, matchSourceTitle, true
	}
	if score := matcher.ScoreTerms(task.Description, query); score > 0 {
		return searchFieldRankDescription, score, matchSourceDescription, true
	}
	if score := matcher.ScoreTerms(checklistText, query); score > 0 {
		return searchFieldRankChecklist, score, matchSourceChecklist, true
	}
	if score := matcher.ScoreTerms(commentText, query); score > 0 {
		return searchFieldRankComment, score, matchSourceComment, true
	}
	// Tags and context share a rank, so the better score wins and an exact tie
	// falls to the declared field order: tags precede context.
	tagScore := matcher.ScoreTerms(tagText, query)
	contextScore := matcher.ScoreTerms(task.Context, query)
	if tagScore > 0 || contextScore > 0 {
		if tagScore >= contextScore {
			return searchFieldRankTagContext, tagScore, matchSourceTag, true
		}
		return searchFieldRankTagContext, contextScore, matchSourceContext, true
	}
	if score := matcher.ScoreTerms(attachmentText, query); score > 0 {
		return searchFieldRankAttachment, score, matchSourceAttachment, true
	}
	if score := matcher.ScoreTerms(projectText, query); score > 0 {
		return searchFieldRankProject, score, matchSourceProject, true
	}

	details := strings.Join([]string{
		task.Title,
		task.Description,
		checklistText,
		commentText,
		tagText,
		task.Context,
		attachmentText,
		projectText,
	}, " ")
	if score := matcher.ScoreTerms(details, query); score > 0 {
		return searchFieldRankDetails, score, matchSourceDetails, true
	}
	return searchFieldRankTitle, 0, matchSourceNone, false
}

// searchChecklistText is the item text alone: completion state is presentation,
// never a term a text query can match or miss on.
func searchChecklistText(task *models.Task) string {
	if len(task.Checklist) == 0 {
		return ""
	}
	parts := make([]string, 0, len(task.Checklist))
	for _, item := range task.Checklist {
		parts = append(parts, item.Text)
	}
	return strings.Join(parts, " ")
}

func searchCommentText(task *models.Task) string {
	if len(task.Comments) == 0 {
		return ""
	}
	parts := make([]string, 0, len(task.Comments))
	for _, comment := range task.Comments {
		if comment != nil {
			parts = append(parts, comment.Text)
		}
	}
	return strings.Join(parts, " ")
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
	predicates *searchPredicates,
) []outlinerSearchMatch {
	for _, project := range projects {
		if project == nil || !projectSearchRefMatches(project, projectRef) {
			continue
		}
		if !predicates.admitsProject(project) {
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
			if !predicates.admitsPopulation(task) {
				continue
			}
			if predicates.admits(task, project) {
				// The qualified reference lives in the project tier, so an
				// exact hit is attributed there while keeping the exact
				// path's top rank.
				return []outlinerSearchMatch{{
					task: task, project: project, fieldRank: searchFieldRankTitle,
					fuzzyScore: 1, source: matchSourceProject, sourceIndex: sourceIndex,
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
