package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestPinnedTagTabsRenderHashLabels(t *testing.T) {
	project := &models.Project{PinnedTags: []string{"backend", " urgent ", "backend"}}

	got := PinnedTagTabs(project)
	want := []string{"All", "#backend", "#urgent"}

	if !slices.Equal(got, want) {
		t.Fatalf("PinnedTagTabs() = %v, want %v", got, want)
	}
}

func TestGlobalPinnedTagTabsRenderHashLabels(t *testing.T) {
	cfg := &config.Config{GlobalPinnedTags: []string{"cross", "#urgent", "cross"}}

	got := GlobalPinnedTagTabs(cfg)
	want := []string{"All", "#cross", "#urgent"}

	if !slices.Equal(got, want) {
		t.Fatalf("GlobalPinnedTagTabs() = %v, want %v", got, want)
	}
}

func TestGlobalPinnedTagTabsForProjectsHidesStalePins(t *testing.T) {
	cfg := &config.Config{GlobalPinnedTags: []string{"backend", "stale"}}
	projects := []*models.Project{
		{
			Tags: []string{"backend"},
			Tasks: []*models.Task{
				{Tags: []string{"frontend"}},
			},
		},
	}

	got := GlobalPinnedTagTabsForProjects(cfg, projects)
	want := []string{"All", "#backend"}

	if !slices.Equal(got, want) {
		t.Fatalf("GlobalPinnedTagTabsForProjects() = %v, want %v", got, want)
	}
	if !slices.Equal(cfg.GlobalPinnedTagList(), []string{"backend", "stale"}) {
		t.Fatalf("GlobalPinnedTagList() = %v, want stale pin retained", cfg.GlobalPinnedTagList())
	}
}

func TestActivePinnedTagFilterReturnsRawTag(t *testing.T) {
	project := &models.Project{PinnedTags: []string{"backend"}}

	if got := ActivePinnedTagFilter(project, 1); got != "backend" {
		t.Fatalf("ActivePinnedTagFilter() = %q, want backend", got)
	}
}
