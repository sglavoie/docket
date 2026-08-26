package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// tierTask is a deliberately bland task: its title shares no subsequence with
// any tier probe below, so a hit can only come from the field the case sets.
func tierTask() *models.Task {
	task := primaryRowsTask("tiered", models.StateBacklog)
	task.Title = "Untitled row"
	task.ShortID = 4
	return task
}

func singleTierMatch(t *testing.T, task *models.Task, project *models.Project, query string) outlinerSearchMatch {
	t.Helper()
	matches := searchProjectionMatchesWithin(
		[]*models.Project{project}, query, nil, fuzzy.Matcher{}, nil)
	if len(matches) != 1 || matches[0].task != task {
		t.Fatalf("query %q matched %v, want the single seeded task", query, searchMatchTaskIDs(matches))
	}
	return matches[0]
}

func TestSearchScoresEveryApprovedFieldAtItsDeclaredTier(t *testing.T) {
	tests := []struct {
		name   string
		seed   func(*models.Task, *models.Project)
		query  string
		rank   int
		source outlinerMatchSource
	}{
		{
			name:   "title",
			seed:   func(task *models.Task, _ *models.Project) { task.Title = "Quantum drive" },
			query:  "quantum",
			rank:   searchFieldRankTitle,
			source: matchSourceTitle,
		},
		{
			name:   "description",
			seed:   func(task *models.Task, _ *models.Project) { task.Description = "zeppelin hangar notes" },
			query:  "zeppelin",
			rank:   searchFieldRankDescription,
			source: matchSourceDescription,
		},
		{
			name: "checklist text of an open item",
			seed: func(task *models.Task, _ *models.Project) {
				task.Checklist = []models.ChecklistItem{{Text: "Ship kubernetes config"}}
			},
			query:  "kubernetes",
			rank:   searchFieldRankChecklist,
			source: matchSourceChecklist,
		},
		{
			name: "checklist text of a ticked item",
			seed: func(task *models.Task, _ *models.Project) {
				task.Checklist = []models.ChecklistItem{{Text: "Ship kubernetes config", Done: true}}
			},
			query:  "kubernetes",
			rank:   searchFieldRankChecklist,
			source: matchSourceChecklist,
		},
		{
			name: "comment text",
			seed: func(task *models.Task, _ *models.Project) {
				task.Comments = []*models.Comment{{Text: "Ask about the starboard rig"}}
			},
			query:  "starboard",
			rank:   searchFieldRankComment,
			source: matchSourceComment,
		},
		{
			name:   "tag",
			seed:   func(task *models.Task, _ *models.Project) { task.Tags = []string{"waffles"} },
			query:  "waffles",
			rank:   searchFieldRankTagContext,
			source: matchSourceTag,
		},
		{
			name:   "context",
			seed:   func(task *models.Task, _ *models.Project) { task.Context = "greenhouse" },
			query:  "greenhouse",
			rank:   searchFieldRankTagContext,
			source: matchSourceContext,
		},
		{
			name: "attachment filename",
			seed: func(task *models.Task, _ *models.Project) {
				task.Attachments = []models.Attachment{{Filename: "invoice.pdf", OriginalPath: "/tmp/x.pdf"}}
			},
			query:  "invoice",
			rank:   searchFieldRankAttachment,
			source: matchSourceAttachment,
		},
		{
			name: "attachment URL",
			seed: func(task *models.Task, _ *models.Project) {
				url := "https://example.com/spec"
				task.Attachments = []models.Attachment{{Filename: url, IsURL: true, OriginalPath: url}}
			},
			query:  "spec",
			rank:   searchFieldRankAttachment,
			source: matchSourceAttachment,
		},
		{
			name: "attachment original path",
			seed: func(task *models.Task, _ *models.Project) {
				task.Attachments = []models.Attachment{
					{Filename: "notes.md", OriginalPath: "/home/dana/blueprint/notes.md"},
				}
			},
			query:  "blueprint",
			rank:   searchFieldRankAttachment,
			source: matchSourceAttachment,
		},
		{
			name:   "project name",
			seed:   func(_ *models.Task, project *models.Project) { project.Name = "Quarry Rebuild" },
			query:  "quarry",
			rank:   searchFieldRankProject,
			source: matchSourceProject,
		},
		{
			name:   "project slug",
			seed:   func(_ *models.Task, project *models.Project) { project.Slug = "quarry-rebuild" },
			query:  "quarry",
			rank:   searchFieldRankProject,
			source: matchSourceProject,
		},
		{
			name:   "project description",
			seed:   func(_ *models.Task, project *models.Project) { project.Description = "zeppelin yard works" },
			query:  "zeppelin",
			rank:   searchFieldRankProject,
			source: matchSourceProject,
		},
		{
			// A bare `quarry#4` takes the exact lookup path; adding a term makes
			// it an ordinary query whose reference text lives in the project tier.
			name: "qualified reference text",
			seed: func(_ *models.Task, project *models.Project) {
				project.Slug = "quarry"
				project.Name = "Quarry Rebuild"
			},
			query:  "quarry#4 rebuild",
			rank:   searchFieldRankProject,
			source: matchSourceProject,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := tierTask()
			project := primaryRowsProject("holdings", task)
			tt.seed(task, project)
			match := singleTierMatch(t, task, project, tt.query)
			if match.fieldRank != tt.rank || match.source != tt.source {
				t.Fatalf("query %q ranked %d/source %d, want %d/%d",
					tt.query, match.fieldRank, match.source, tt.rank, tt.source)
			}
		})
	}
}

func TestSearchTagAndContextShareARankButKeepDistinctSources(t *testing.T) {
	tests := []struct {
		name    string
		tags    []string
		context string
		want    outlinerMatchSource
	}{
		{"tag only", []string{"greenhouse"}, "", matchSourceTag},
		{"context only", []string{"waffles"}, "greenhouse", matchSourceContext},
		{"context scores higher", []string{"greenhouse-north-wing"}, "greenhouse", matchSourceContext},
		{"exact tie falls to the declared field order", []string{"greenhouse"}, "greenhouse", matchSourceTag},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := tierTask()
			task.Tags = tt.tags
			task.Context = tt.context
			project := primaryRowsProject("holdings", task)
			match := singleTierMatch(t, task, project, "greenhouse")
			if match.fieldRank != searchFieldRankTagContext {
				t.Fatalf("rank = %d, want the shared tag/context rank %d",
					match.fieldRank, searchFieldRankTagContext)
			}
			if match.source != tt.want {
				t.Fatalf("source = %d, want %d", match.source, tt.want)
			}
		})
	}
}

func TestSearchCrossFieldTermsUseTheDetailsTierAndNeverOutrankOneField(t *testing.T) {
	spanning := primaryRowsTask("spanning", models.StateBacklog)
	spanning.Title = "Quantum drive"
	spanning.Description = "zeppelin hangar"
	singleField := primaryRowsTask("single-field", models.StateBacklog)
	singleField.Title = "Untitled row"
	singleField.Description = "quantum zeppelin refit in one field"
	project := primaryRowsProject("holdings", spanning, singleField)

	matches := searchProjectionMatchesWithin(
		[]*models.Project{project}, "quantum zeppelin", nil, fuzzy.Matcher{}, nil)
	if got := searchMatchTaskIDs(matches); !slices.Equal(got, []string{singleField.ID, spanning.ID}) {
		t.Fatalf("cross-field ranking = %v, want the single-field match first", got)
	}
	if matches[0].fieldRank != searchFieldRankDescription || matches[0].source != matchSourceDescription {
		t.Fatalf("single-field match ranked %d/source %d, want the description tier",
			matches[0].fieldRank, matches[0].source)
	}
	if matches[1].fieldRank != searchFieldRankDetails || matches[1].source != matchSourceDetails {
		t.Fatalf("spanning match ranked %d/source %d, want the combined details tier",
			matches[1].fieldRank, matches[1].source)
	}
}

func TestSearchDetailsTierIsUnreachableWhileAnySingleFieldMatches(t *testing.T) {
	task := tierTask()
	task.Description = "quantum zeppelin together"
	task.Comments = []*models.Comment{{Text: "quantum"}}
	project := primaryRowsProject("holdings", task)
	match := singleTierMatch(t, task, project, "quantum zeppelin")
	if match.fieldRank != searchFieldRankDescription {
		t.Fatalf("rank = %d, want the description tier rather than details", match.fieldRank)
	}
}

func TestSearchAttributionRefreshesWithEveryRebuildAndOrdinaryRowsCarryNone(t *testing.T) {
	task := primaryRowsTask("moving", models.StateBacklog)
	task.Title = "Needle title"
	project := primaryRowsProject("alpha", task)
	outliner := scopeTestOutliner(project)

	outliner.crossForceRebuild = true
	outliner.crossProjectRebuildRows()
	for _, row := range outliner.rows {
		if row.MatchSource != matchSourceNone {
			t.Fatalf("ordinary row %q carried source %d, want none", row.SectionName, row.MatchSource)
		}
	}

	rebuild := func() {
		outliner.crossForceRebuild = true
		outliner.crossProjectRebuildRows()
	}
	searchSource := func() outlinerMatchSource {
		t.Helper()
		if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{task.ID}) {
			t.Fatalf("search rows = %v, want the seeded task", got)
		}
		return outliner.rows[0].MatchSource
	}

	outliner.SetSearchQuery("needle")
	if got := searchSource(); got != matchSourceTitle {
		t.Fatalf("initial source = %d, want title", got)
	}

	task.Title = "Nothing here"
	task.Description = "Needle moved to the body"
	rebuild()
	if got := searchSource(); got != matchSourceDescription {
		t.Fatalf("post-mutation source = %d, want description", got)
	}

	task.Description = ""
	task.Comments = []*models.Comment{{Text: "Needle moved to a comment"}}
	rebuild()
	if got := searchSource(); got != matchSourceComment {
		t.Fatalf("post-mutation source = %d, want comment", got)
	}

	// A sigil-only query leaves no free text to attribute, so the gate admits
	// rows with the same no-cue source an ordinary projection carries.
	outliner.SetSearchQuery("*unpin")
	if got := searchSource(); got != matchSourceNone {
		t.Fatalf("sigil-only source = %d, want none", got)
	}

	outliner.ClearSearch()
	for _, row := range outliner.rows {
		if row.MatchSource != matchSourceNone {
			t.Fatalf("restored row carried source %d, want none", row.MatchSource)
		}
	}
}

func TestSearchExactQualifiedIDKeepsTopRankAndProjectAttribution(t *testing.T) {
	target := primaryRowsTask("target", models.StateBacklog)
	target.Title = "No shared words"
	target.ShortID = 9
	project := primaryRowsProject("alpha", target)

	matches := searchProjectionMatchesWithin([]*models.Project{project}, "alpha#9", nil, fuzzy.Matcher{}, nil)
	if len(matches) != 1 || matches[0].task != target {
		t.Fatalf("exact lookup = %v, want the qualified task", searchMatchTaskIDs(matches))
	}
	if matches[0].fieldRank != searchFieldRankTitle || matches[0].source != matchSourceProject {
		t.Fatalf("exact lookup ranked %d/source %d, want top rank with project attribution",
			matches[0].fieldRank, matches[0].source)
	}
}

func TestSearchDenseFuzzyFallbackStillReachesTheNewTiers(t *testing.T) {
	task := tierTask()
	task.Comments = []*models.Comment{{Text: "Migrate the Postgres replica"}}
	project := primaryRowsProject("holdings", task)

	// Case-sensitive smart case first, then the relaxed retry, must both land on
	// the comment tier rather than degrading to a title-only search.
	match := singleTierMatch(t, task, project, "Postgres")
	if match.source != matchSourceComment {
		t.Fatalf("case-sensitive source = %d, want comment", match.source)
	}
	outliner := scopeTestOutliner(project)
	outliner.SetSearchQuery("POSTGRES")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{task.ID}) {
		t.Fatalf("relaxed retry = %v, want the comment match", got)
	}
	if !outliner.searchCaseRelaxed {
		t.Fatal("the uppercase query did not report a relaxed pass")
	}
	if outliner.rows[0].MatchSource != matchSourceComment {
		t.Fatalf("relaxed source = %d, want comment", outliner.rows[0].MatchSource)
	}
}
