package outliner

import (
	"reflect"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/fuzzy"
	"github.com/sglavoie/docket/internal/models"
)

func TestSearchCacheReevaluatesLiveContentAndCandidateScope(t *testing.T) {
	task := tierTask()
	task.Description = strings.Repeat("ordinary prose ", 40) + "release"
	project := &models.Project{ID: "p", Slug: "p", Name: "Project", Tasks: []*models.Task{task}}
	projects := []*models.Project{project}
	cache := fuzzy.NewScoreCache(1 << 20)
	check := func(text string, candidates map[string][]*models.Task) []outlinerSearchMatch {
		t.Helper()
		matcher := fuzzy.SmartCase(text)
		want := searchProjectionMatchesWithin(projects, text, candidates, matcher, nil)
		got := searchProjectionMatchesWithin(projects, text, candidates, matcher, nil, cache)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("cached %q differs from current uncached projection", text)
		}
		return got
	}
	for _, text := range []string{"r", "re", "rel", "release", "rel", "r", "", "RELEASE", "zzzz"} {
		check(text, nil)
	}
	task.Description = strings.Repeat("ordinary prose ", 40) + "handoff"
	if len(check("release", nil)) != 0 || len(check("handoff", nil)) != 1 {
		t.Fatal("edited content not reflected")
	}
	if len(check("handoff", map[string][]*models.Task{"p": nil})) != 0 {
		t.Fatal("cached score escaped scope")
	}
	task.Comments = []*models.Comment{{Text: strings.Repeat("comment ", 25) + "release"}}
	if got := check("release", nil); len(got) != 1 || got[0].source != MatchSourceComment {
		t.Fatal("new comment did not enter current tier")
	}
	project.Tasks = nil
	if len(check("handoff", nil)) != 0 {
		t.Fatal("deleted task survived via cache")
	}
}
