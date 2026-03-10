package tui

import "testing"

func TestCategoryWeight(t *testing.T) {
	tests := []struct {
		category ItemCategory
		want     int
	}{
		{ItemCategoryCommands, weightCommands},
		{ItemCategoryProjects, weightProjects},
		{ItemCategorySavedSearch, weightCommands}, // Saved searches rank like commands
		{ItemCategoryTasks, weightDefault},
		{ItemCategoryTags, weightDefault},
		{ItemCategorySections, weightDefault},
		{ItemCategory("unknown"), weightDefault},
	}

	for _, tc := range tests {
		t.Run(string(tc.category), func(t *testing.T) {
			got := categoryWeight(tc.category)
			if got != tc.want {
				t.Errorf("categoryWeight(%q) = %d, want %d", tc.category, got, tc.want)
			}
		})
	}
}

func TestGetCategoryDisplayName(t *testing.T) {
	tests := []struct {
		category ItemCategory
		want     string
	}{
		{ItemCategoryCommands, "Commands"},
		{ItemCategoryTasks, "Tasks"},
		{ItemCategoryProjects, "Projects"},
		{ItemCategoryTags, "Tags"},
		{ItemCategorySections, "Sections"},
		{ItemCategorySavedSearch, "Saved Searches"},
		{ItemCategory("unknown"), "Other"},
	}

	for _, tc := range tests {
		t.Run(string(tc.category), func(t *testing.T) {
			got := getCategoryDisplayName(tc.category)
			if got != tc.want {
				t.Errorf("getCategoryDisplayName(%q) = %q, want %q", tc.category, got, tc.want)
			}
		})
	}
}

func TestCategoryPrefixes(t *testing.T) {
	// Verify all expected prefixes exist
	expectedPrefixes := map[string]ItemCategory{
		"cmd":    ItemCategoryCommands,
		"task":   ItemCategoryTasks,
		"prj":    ItemCategoryProjects,
		"tag":    ItemCategoryTags,
		"sec":    ItemCategorySections,
		"saved":  ItemCategorySavedSearch,
		"recent": ItemCategoryCommands,
		"note":   ItemCategoryNotes,
		"sv":     ItemCategorySavedFilter,
		"view":   ItemCategoryViews,
	}

	for prefix, expectedCat := range expectedPrefixes {
		t.Run(prefix, func(t *testing.T) {
			got, ok := categoryPrefixes[prefix]
			if !ok {
				t.Errorf("prefix %q not found in categoryPrefixes", prefix)
				return
			}
			if got != expectedCat {
				t.Errorf("categoryPrefixes[%q] = %q, want %q", prefix, got, expectedCat)
			}
		})
	}

	// Verify no unexpected prefixes
	if len(categoryPrefixes) != len(expectedPrefixes) {
		t.Errorf("categoryPrefixes has %d entries, want %d", len(categoryPrefixes), len(expectedPrefixes))
	}
}

func TestCategoryOrder(t *testing.T) {
	// Verify all categories are in the order slice
	expectedCategories := []ItemCategory{
		ItemCategorySavedFilter,
		ItemCategorySavedSearch,
		ItemCategoryCommands,
		ItemCategoryProjects,
		ItemCategoryTasks,
		ItemCategoryTags,
		ItemCategorySections,
		ItemCategoryNotes,
		ItemCategoryViews,
	}

	if len(categoryOrder) != len(expectedCategories) {
		t.Errorf("categoryOrder has %d entries, want %d", len(categoryOrder), len(expectedCategories))
	}

	for i, cat := range expectedCategories {
		if categoryOrder[i] != cat {
			t.Errorf("categoryOrder[%d] = %q, want %q", i, categoryOrder[i], cat)
		}
	}
}

func TestMatchFieldConstants(t *testing.T) {
	// Verify MatchField constants are distinct
	fields := []MatchField{
		MatchFieldNone,
		MatchFieldName,
		MatchFieldDescription,
		MatchFieldAlias,
		MatchFieldTag,
		MatchFieldSection,
	}

	seen := make(map[MatchField]bool)
	for _, f := range fields {
		if seen[f] {
			t.Errorf("duplicate MatchField value: %d", f)
		}
		seen[f] = true
	}

	// Verify MatchFieldNone is 0
	if MatchFieldNone != 0 {
		t.Errorf("MatchFieldNone = %d, want 0", MatchFieldNone)
	}
}

func TestItemCategoryConstants(t *testing.T) {
	// Verify category constants have expected string values
	tests := []struct {
		cat  ItemCategory
		want string
	}{
		{ItemCategoryCommands, "Commands"},
		{ItemCategoryTasks, "Tasks"},
		{ItemCategoryProjects, "Projects"},
		{ItemCategoryTags, "Tags"},
		{ItemCategorySections, "Sections"},
		{ItemCategorySavedSearch, "Saved"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if string(tc.cat) != tc.want {
				t.Errorf("ItemCategory constant = %q, want %q", tc.cat, tc.want)
			}
		})
	}
}

func TestWeightConstants(t *testing.T) {
	// Commands should rank highest
	if weightCommands <= weightProjects {
		t.Error("weightCommands should be greater than weightProjects")
	}
	if weightProjects <= weightDefault {
		t.Error("weightProjects should be greater than weightDefault")
	}
	if weightDefault < 1 {
		t.Error("weightDefault should be at least 1")
	}
}

func TestPaletteItemFields(t *testing.T) {
	// Test that PaletteItem can hold different types
	cmdItem := PaletteItem{
		Name:        "Test Command",
		Description: "A test command",
		Shortcut:    "ctrl+t",
		Category:    ItemCategoryCommands,
		Command:     &Command{Name: "test"},
	}

	_ = cmdItem.Name
	_ = cmdItem.Description
	_ = cmdItem.Shortcut
	_ = cmdItem.Category

	if cmdItem.Command == nil {
		t.Error("Command should not be nil")
	}
	if cmdItem.Task != nil {
		t.Error("Task should be nil for command item")
	}

	tagItem := PaletteItem{
		Name:     "bug",
		Category: ItemCategoryTags,
		Tag:      "bug",
	}

	_ = tagItem.Name
	_ = tagItem.Category

	if tagItem.Tag != "bug" {
		t.Errorf("Tag = %q, want 'bug'", tagItem.Tag)
	}
}

func TestFilteredPaletteItem(t *testing.T) {
	item := filteredPaletteItem{
		Item: PaletteItem{
			Name:     "Test",
			Category: ItemCategoryTasks,
		},
		NamePositions: []int{0, 2},
		MatchedField:  MatchFieldName,
		MatchedTerm:   "ts",
	}

	if len(item.NamePositions) != 2 {
		t.Errorf("NamePositions length = %d, want 2", len(item.NamePositions))
	}
	if item.MatchedField != MatchFieldName {
		t.Errorf("MatchedField = %d, want %d", item.MatchedField, MatchFieldName)
	}
	if item.MatchedTerm != "ts" {
		t.Errorf("MatchedTerm = %q, want 'ts'", item.MatchedTerm)
	}
}

func TestSearchTermGroup(t *testing.T) {
	// Test positive group with OR alternatives
	posGroup := searchTermGroup{
		alternatives: []string{"bug", "fix", "issue"},
		isNegative:   false,
		isExact:      false,
	}

	_ = posGroup.isExact

	if posGroup.isNegative {
		t.Error("positive group should not be negative")
	}
	if len(posGroup.alternatives) != 3 {
		t.Errorf("alternatives length = %d, want 3", len(posGroup.alternatives))
	}

	// Test negative exact group
	negGroup := searchTermGroup{
		alternatives: []string{"completed"},
		isNegative:   true,
		isExact:      true,
	}

	_ = negGroup.alternatives

	if !negGroup.isNegative {
		t.Error("negative group should be negative")
	}
	if !negGroup.isExact {
		t.Error("exact group should be exact")
	}
}

func TestFieldFilter(t *testing.T) {
	// Test single value filter
	singleFilter := fieldFilter{
		field: "priority",
		op:    "eq",
		value: "1",
	}

	_ = singleFilter.value

	if singleFilter.field != "priority" {
		t.Errorf("field = %q, want 'priority'", singleFilter.field)
	}
	if singleFilter.op != "eq" {
		t.Errorf("op = %q, want 'eq'", singleFilter.op)
	}

	// Test multi-value filter
	multiFilter := fieldFilter{
		field: "status",
		op:    "in",
		alt:   []string{"todo", "progress"},
	}

	_ = multiFilter.field
	_ = multiFilter.op

	if len(multiFilter.alt) != 2 {
		t.Errorf("alt length = %d, want 2", len(multiFilter.alt))
	}
}

func TestMaxRecentItemsConstant(t *testing.T) {
	if maxRecentItems < 1 {
		t.Error("maxRecentItems should be at least 1")
	}
	if maxRecentItems > 100 {
		t.Error("maxRecentItems seems too high")
	}
}

func TestRecentBoostMultiplierConstant(t *testing.T) {
	if recentBoostMultiplier < 1 {
		t.Error("recentBoostMultiplier should be at least 1")
	}
}
