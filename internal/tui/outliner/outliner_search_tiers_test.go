package outliner

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

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
		source MatchSource
	}{
		{
			name:   "title",
			seed:   func(task *models.Task, _ *models.Project) { task.Title = "Quantum drive" },
			query:  "quantum",
			rank:   searchFieldRankTitle,
			source: MatchSourceTitle,
		},
		{
			name:   "description",
			seed:   func(task *models.Task, _ *models.Project) { task.Description = "zeppelin hangar notes" },
			query:  "zeppelin",
			rank:   searchFieldRankDescription,
			source: MatchSourceDescription,
		},
		{
			name:   "resume note",
			seed:   func(task *models.Task, _ *models.Project) { task.ResumeNote = "paused at the marmalade step" },
			query:  "marmalade",
			rank:   searchFieldRankResume,
			source: MatchSourceResume,
		},
		{
			name: "checklist text of an open item",
			seed: func(task *models.Task, _ *models.Project) {
				task.Checklist = []models.ChecklistItem{{Text: "Ship kubernetes config"}}
			},
			query:  "kubernetes",
			rank:   searchFieldRankChecklist,
			source: MatchSourceChecklist,
		},
		{
			name: "checklist text of a ticked item",
			seed: func(task *models.Task, _ *models.Project) {
				task.Checklist = []models.ChecklistItem{{Text: "Ship kubernetes config", Done: true}}
			},
			query:  "kubernetes",
			rank:   searchFieldRankChecklist,
			source: MatchSourceChecklist,
		},
		{
			name: "comment text",
			seed: func(task *models.Task, _ *models.Project) {
				task.Comments = []*models.Comment{{Text: "Ask about the starboard rig"}}
			},
			query:  "starboard",
			rank:   searchFieldRankComment,
			source: MatchSourceComment,
		},
		{
			name:   "tag",
			seed:   func(task *models.Task, _ *models.Project) { task.Tags = []string{"waffles"} },
			query:  "waffles",
			rank:   searchFieldRankTagContext,
			source: MatchSourceTag,
		},
		{
			name:   "context",
			seed:   func(task *models.Task, _ *models.Project) { task.Context = "greenhouse" },
			query:  "greenhouse",
			rank:   searchFieldRankTagContext,
			source: MatchSourceContext,
		},
		{
			name: "attachment filename",
			seed: func(task *models.Task, _ *models.Project) {
				task.Attachments = []models.Attachment{{Filename: "invoice.pdf", OriginalPath: "/tmp/x.pdf"}}
			},
			query:  "invoice",
			rank:   searchFieldRankAttachment,
			source: MatchSourceAttachment,
		},
		{
			name: "attachment URL",
			seed: func(task *models.Task, _ *models.Project) {
				url := "https://example.com/spec"
				task.Attachments = []models.Attachment{{Filename: url, IsURL: true, OriginalPath: url}}
			},
			query:  "spec",
			rank:   searchFieldRankAttachment,
			source: MatchSourceAttachment,
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
			source: MatchSourceAttachment,
		},
		{
			name:   "project name",
			seed:   func(_ *models.Task, project *models.Project) { project.Name = "Quarry Rebuild" },
			query:  "quarry",
			rank:   searchFieldRankProject,
			source: MatchSourceProject,
		},
		{
			name:   "project slug",
			seed:   func(_ *models.Task, project *models.Project) { project.Slug = "quarry-rebuild" },
			query:  "quarry",
			rank:   searchFieldRankProject,
			source: MatchSourceProject,
		},
		{
			name:   "project description",
			seed:   func(_ *models.Task, project *models.Project) { project.Description = "zeppelin yard works" },
			query:  "zeppelin",
			rank:   searchFieldRankProject,
			source: MatchSourceProject,
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
			source: MatchSourceProject,
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
			outliner := scopeTestOutliner(project)
			searchRankedList(outliner, tt.query)
			excerpt := ansi.Strip(outliner.peekSearchExcerpt(outliner.Rows[0]))
			if tt.source == MatchSourceTitle {
				if excerpt != "" {
					t.Fatalf("title match gained a redundant excerpt: %q", excerpt)
				}
			} else {
				for _, term := range strings.Fields(tt.query) {
					if !strings.Contains(strings.ToLower(excerpt), term) {
						t.Errorf("excerpt %q does not explain query term %q", excerpt, term)
					}
				}
			}
		})
	}
}

// TestSearchResumeNoteRanksBetweenDescriptionAndChecklist pins the tier the
// resume note earned. The four tasks are seeded in reverse of the order they
// must come out in, and the note is the longest of the four texts, so neither
// source order nor fuzzy score can be what places it: only the rank can.
func TestSearchResumeNoteRanksBetweenDescriptionAndChecklist(t *testing.T) {
	titled := primaryRowsTask("titled", models.StateBacklog)
	titled.Title = "Marmalade drive"
	described := primaryRowsTask("described", models.StateBacklog)
	described.Title = "Untitled row"
	described.Description = "marmalade"
	resumed := primaryRowsTask("resumed", models.StateBacklog)
	resumed.Title = "Untitled row"
	resumed.ResumeNote = "paused halfway through the marmalade reduction"
	checklisted := primaryRowsTask("checklisted", models.StateBacklog)
	checklisted.Title = "Untitled row"
	checklisted.Checklist = []models.ChecklistItem{{Text: "marmalade"}}
	project := primaryRowsProject("holdings", checklisted, resumed, described, titled)

	matches := searchProjectionMatchesWithin(
		[]*models.Project{project}, "marmalade", nil, fuzzy.Matcher{}, nil)
	want := []string{titled.ID, described.ID, resumed.ID, checklisted.ID}
	if got := searchMatchTaskIDs(matches); !slices.Equal(got, want) {
		t.Fatalf("ranking = %v, want %v", got, want)
	}
	if matches[2].source != MatchSourceResume {
		t.Fatalf("resume-only match reported source %d, want %d", matches[2].source, MatchSourceResume)
	}
}

// TestSearchResumeNoteYieldsToTitleAndDescriptionOnOneTask holds the precedence
// half of the same rule: the note is probed after both prose fields, so a task
// whose title or description also carries the term is attributed to that field.
func TestSearchResumeNoteYieldsToTitleAndDescriptionOnOneTask(t *testing.T) {
	task := tierTask()
	task.ResumeNote = "marmalade reduction, halfway"

	task.Description = "the marmalade batch"
	project := primaryRowsProject("holdings", task)
	if match := singleTierMatch(t, task, project, "marmalade"); match.source != MatchSourceDescription {
		t.Fatalf("source with a matching description = %d, want description", match.source)
	}

	task.Title = "Marmalade drive"
	project = primaryRowsProject("holdings", task)
	if match := singleTierMatch(t, task, project, "marmalade"); match.source != MatchSourceTitle {
		t.Fatalf("source with a matching title = %d, want title", match.source)
	}
}

// TestSearchResumeNoteJoinsTheDetailsFallback proves the note is in the
// concatenated tier as well as in one of its own, so a multi-term query
// satisfied across the title and the note still finds the task.
func TestSearchResumeNoteJoinsTheDetailsFallback(t *testing.T) {
	spanning := primaryRowsTask("spanning", models.StateBacklog)
	spanning.Title = "Quantum drive"
	spanning.ResumeNote = "waiting on the zeppelin refit"
	project := primaryRowsProject("holdings", spanning)

	match := singleTierMatch(t, spanning, project, "quantum zeppelin")
	if match.fieldRank != searchFieldRankDetails || match.source != MatchSourceDetails {
		t.Fatalf("title-plus-resume query ranked %d/source %d, want the details tier",
			match.fieldRank, match.source)
	}
}

func TestSearchTagAndContextShareARankButKeepDistinctSources(t *testing.T) {
	tests := []struct {
		name    string
		tags    []string
		context string
		want    MatchSource
	}{
		{"tag only", []string{"greenhouse"}, "", MatchSourceTag},
		{"context only", []string{"waffles"}, "greenhouse", MatchSourceContext},
		{"context scores higher", []string{"greenhouse-north-wing"}, "greenhouse", MatchSourceContext},
		{"exact tie falls to the declared field order", []string{"greenhouse"}, "greenhouse", MatchSourceTag},
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
	if matches[0].fieldRank != searchFieldRankDescription || matches[0].source != MatchSourceDescription {
		t.Fatalf("single-field match ranked %d/source %d, want the description tier",
			matches[0].fieldRank, matches[0].source)
	}
	if matches[1].fieldRank != searchFieldRankDetails || matches[1].source != MatchSourceDetails {
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

	outliner.Population.Invalidate()
	outliner.CrossProjectRebuildRows()
	for _, row := range outliner.Rows {
		if row.MatchSource != MatchSourceNone {
			t.Fatalf("ordinary row %q carried source %d, want none", row.SectionName, row.MatchSource)
		}
	}

	rebuild := func() {
		outliner.Population.Invalidate()
		outliner.CrossProjectRebuildRows()
	}
	searchSource := func() MatchSource {
		t.Helper()
		if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{task.ID}) {
			t.Fatalf("search rows = %v, want the seeded task", got)
		}
		return outliner.Rows[0].MatchSource
	}

	searchRankedList(outliner, "needle")
	if got := searchSource(); got != MatchSourceTitle {
		t.Fatalf("initial source = %d, want title", got)
	}

	task.Title = "Nothing here"
	task.Description = "Needle moved to the body"
	rebuild()
	if got := searchSource(); got != MatchSourceDescription {
		t.Fatalf("post-mutation source = %d, want description", got)
	}

	task.Description = ""
	task.Comments = []*models.Comment{{Text: "Needle moved to a comment"}}
	rebuild()
	if got := searchSource(); got != MatchSourceComment {
		t.Fatalf("post-mutation source = %d, want comment", got)
	}

	// A sigil-only query leaves no free text to attribute, so the gate admits
	// rows with the same no-cue source an ordinary projection carries.
	searchRankedList(outliner, "*unpin")
	if got := searchSource(); got != MatchSourceNone {
		t.Fatalf("sigil-only source = %d, want none", got)
	}

	outliner.ClearSearch()
	for _, row := range outliner.Rows {
		if row.MatchSource != MatchSourceNone {
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
	if matches[0].fieldRank != searchFieldRankTitle || matches[0].source != MatchSourceProject {
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
	if match.source != MatchSourceComment {
		t.Fatalf("case-sensitive source = %d, want comment", match.source)
	}
	outliner := scopeTestOutliner(project)
	searchRankedList(outliner, "POSTGRES")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{task.ID}) {
		t.Fatalf("relaxed retry = %v, want the comment match", got)
	}
	if !outliner.Search.caseRelaxed {
		t.Fatal("the uppercase query did not report a relaxed pass")
	}
	if outliner.Rows[0].MatchSource != MatchSourceComment {
		t.Fatalf("relaxed source = %d, want comment", outliner.Rows[0].MatchSource)
	}
}
