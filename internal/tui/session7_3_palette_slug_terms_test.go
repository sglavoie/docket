package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestPaletteProjectSlugExactTermInclusion pins that a quoted exact positive
// term matches on a project's slug alone (name and description do not contain
// the term), and that the slug-only match keeps MatchFieldProjectSlug
// provenance so the preview can attribute the result.
func TestPaletteProjectSlugExactTermInclusion(t *testing.T) {
	items := []PaletteItem{
		{Name: "Toggle Tree", Category: ItemCategoryCommands, Command: &Command{Name: "Toggle Tree"}},
		{Name: "Backend Service", Category: ItemCategoryProjects, Project: &models.Project{Name: "Backend Service", Slug: "api"}},
	}
	m := newScoringPalette(items)
	// Quoted phrase => exact positive group; the term only appears in the slug.
	m.searchInput.SetValue(`"api"`)
	m.filterItems()

	if len(m.filteredItems) != 1 {
		t.Fatalf("exact quoted slug term = %d results, want 1 (%v)", len(m.filteredItems), slugOrder(m.filteredItems))
	}
	top := m.filteredItems[0]
	if top.Item.Category != ItemCategoryProjects || top.Item.Project == nil || top.Item.Project.Slug != "api" {
		t.Fatalf("top result = %q (%s), want project slug 'api'", top.Item.Name, top.Item.Category)
	}
	if top.MatchedField != MatchFieldProjectSlug {
		t.Fatalf("slug-only exact match field = %v, want MatchFieldProjectSlug", top.MatchedField)
	}
}

// TestPaletteProjectSlugNegativeExactExclusion pins that a negated quoted exact
// term excludes a project matched only on its slug, while a non-matching
// project survives.
func TestPaletteProjectSlugNegativeExactExclusion(t *testing.T) {
	items := []PaletteItem{
		{Name: "Backend Service", Category: ItemCategoryProjects, Project: &models.Project{Name: "Backend Service", Slug: "api"}},
		{Name: "Frontend", Category: ItemCategoryProjects, Project: &models.Project{Name: "Frontend", Slug: "web"}},
	}
	m := newScoringPalette(items)
	// Negated quoted phrase => exact negative group on the slug.
	m.searchInput.SetValue(`-"api"`)
	m.filterItems()

	for _, fi := range m.filteredItems {
		if fi.Item.Project != nil && fi.Item.Project.Slug == "api" {
			t.Fatalf("negated exact slug term should exclude slug 'api', order=%v", slugOrder(m.filteredItems))
		}
	}
	if !containsSlug(m.filteredItems, "web") {
		t.Fatalf("non-matching project slug 'web' should survive negation, order=%v", slugOrder(m.filteredItems))
	}
}

// TestPaletteProjectSlugNegativeFuzzyExclusion pins that an unquoted negative
// term excludes a project via a fuzzy slug match, while a non-matching project
// survives.
func TestPaletteProjectSlugNegativeFuzzyExclusion(t *testing.T) {
	items := []PaletteItem{
		// Name deliberately lacks an "s-e-r" subsequence so the exclusion can
		// only come from the slug's fuzzy match.
		{Name: "Backend Box", Category: ItemCategoryProjects, Project: &models.Project{Name: "Backend Box", Slug: "webserver"}},
		{Name: "Gamma", Category: ItemCategoryProjects, Project: &models.Project{Name: "Gamma", Slug: "gamma"}},
	}
	m := newScoringPalette(items)
	// Unquoted negative term => fuzzy negative group; "ser" fuzzy-matches
	// "webserver" but not "gamma".
	m.searchInput.SetValue("-ser")
	m.filterItems()

	for _, fi := range m.filteredItems {
		if fi.Item.Project != nil && fi.Item.Project.Slug == "webserver" {
			t.Fatalf("negated fuzzy slug term should exclude slug 'webserver', order=%v", slugOrder(m.filteredItems))
		}
	}
	if !containsSlug(m.filteredItems, "gamma") {
		t.Fatalf("non-matching project slug 'gamma' should survive negation, order=%v", slugOrder(m.filteredItems))
	}
}

// TestPaletteProjectSlugQuotedExactRanksAheadOfCommands pins FR-007: a quoted
// exact term matching a project's name or slug receives the identity-quality
// ordering bonus even on the exact-match path, so the project ranks ahead of
// higher-weighted command matches in a command-heavy result set. Without the
// bonus the exact branch scored projects with categoryWeight alone, letting a
// quoted exact command outrank an exactly named or slugged project.
func TestPaletteProjectSlugQuotedExactRanksAheadOfCommands(t *testing.T) {
	// Exact project name match wins over an exact command name match.
	nameItems := []PaletteItem{
		{Name: "tt runner", Category: ItemCategoryCommands, Command: &Command{Name: "tt runner"}},
		{Name: "tt export", Category: ItemCategoryCommands, Command: &Command{Name: "tt export"}},
		{Name: "tt import", Category: ItemCategoryCommands, Command: &Command{Name: "tt import"}},
		{Name: "tt", Category: ItemCategoryProjects, Project: &models.Project{Name: "tt", Slug: "tt"}},
	}
	m := newScoringPalette(nameItems)
	m.searchInput.SetValue(`"tt"`)
	m.filterItems()

	if len(m.filteredItems) == 0 {
		t.Fatal(`quoted exact "tt" produced no results`)
	}
	top := m.filteredItems[0].Item
	if top.Category != ItemCategoryProjects || top.Project == nil || top.Project.Slug != "tt" {
		t.Fatalf("top result = %q (%s), want project 'tt' (order %v)", top.Name, top.Category, slugOrder(m.filteredItems))
	}

	// Slug-only exact match also outranks the commands even when the project
	// name does not contain the term.
	slugItems := []PaletteItem{
		{Name: "api runner", Category: ItemCategoryCommands, Command: &Command{Name: "api runner"}},
		{Name: "api export", Category: ItemCategoryCommands, Command: &Command{Name: "api export"}},
		{Name: "Backend Service", Category: ItemCategoryProjects, Project: &models.Project{Name: "Backend Service", Slug: "api"}},
	}
	m2 := newScoringPalette(slugItems)
	m2.searchInput.SetValue(`"api"`)
	m2.filterItems()

	if len(m2.filteredItems) == 0 {
		t.Fatal(`quoted exact "api" produced no results`)
	}
	top2 := m2.filteredItems[0].Item
	if top2.Category != ItemCategoryProjects || top2.Project == nil || top2.Project.Slug != "api" {
		t.Fatalf("top result = %q (%s), want project slug 'api' (order %v)", top2.Name, top2.Category, slugOrder(m2.filteredItems))
	}
	if m2.filteredItems[0].MatchedField != MatchFieldProjectSlug {
		t.Fatalf("slug-only exact match field = %v, want MatchFieldProjectSlug", m2.filteredItems[0].MatchedField)
	}
}

func containsSlug(items []filteredPaletteItem, slug string) bool {
	for _, fi := range items {
		if fi.Item.Project != nil && fi.Item.Project.Slug == slug {
			return true
		}
	}
	return false
}
