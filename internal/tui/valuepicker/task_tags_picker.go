package valuepicker

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

const (
	TaskTagSemanticCategory listsurface.ListCategoryID = "task-tag:semantic"
	TaskContextCategory     listsurface.ListCategoryID = "task-tag:context"

	// taskContextRowPrefix keeps the context radio rows in an ID namespace the
	// "tag:" rows they share the CONTEXTS section with can never enter.
	taskContextRowPrefix  = "context:"
	TaskContextClearRowID = listsurface.ListItemID(taskContextRowPrefix + "0")

	semanticTaskTagExplainerLabel     = "semantic"
	semanticTaskTagExplainerSeparator = " › "
	SemanticTaskTagExplainerSummary   = "— the highest one a task carries colors its title."

	taskTagJumpHintLabel     = "jump"
	taskTagJumpHintSeparator = " · "
)

// taskTagJumpSections is the one table the jump keys, the hint line, and the
// refusal status all read, so a section can never answer a key it is not named
// by. The order is the order the picker renders the sections in.
var taskTagJumpSections = []struct {
	Key      string
	Label    string
	Category listsurface.ListCategoryID
}{
	{Key: "s", Label: "semantic", Category: TaskTagSemanticCategory},
	{Key: "c", Label: "contexts", Category: TaskContextCategory},
	{Key: "t", Label: "tags", Category: TagDescriptiveCategory},
}

// taskTagJumpCategory resolves one section jump key.
func taskTagJumpCategory(key string) (listsurface.ListCategoryID, string, bool) {
	for _, section := range taskTagJumpSections {
		if section.Key == key {
			return section.Category, section.Label, true
		}
	}
	return "", "", false
}

// semanticTaskTagProvider reports whether a picker draws the task tag
// vocabulary. The project namespace has its own two tiers and reserves parked,
// so a semantic name means nothing there.
func semanticTaskTagProvider(provider operation.ValueProviderID) bool {
	return provider == operation.ValueProviderTags || provider == operation.ValueProviderFilterTags
}

// TaskTagListSource pins the semantic vocabulary above the tags in use.
// seedUnused offers every semantic tag even when nothing carries it, which the
// editor wants for discovery and the filter must not do, because a filter row
// nothing matches can only empty the projection.
func TaskTagListSource(tags []query.TagInfo, seedUnused bool) listsurface.ListSource {
	// The picker offers the semantic vocabulary and the tags in use, and nothing
	// else. Review used to lead it as a promoted workflow tag; it is a stored
	// boolean now, reached by =review and f=, so offering it here would mint an
	// ordinary tag that looks like the lane and drives nothing.
	items := make([]listsurface.ListItem, 0, len(tags)+len(models.SemanticTaskTags))
	promoted := make(map[string]bool, len(tags))
	for _, entry := range models.SemanticTaskTags {
		canonicalInUse := false
		for _, tag := range tags {
			if !strings.EqualFold(tag.Name, entry.Name) {
				continue
			}
			// Task tags keep their stored spelling, so a task tagged Bug must
			// round-trip as Bug when the row is toggled off.
			promoted[tag.Name] = true
			canonicalInUse = canonicalInUse || tag.Name == entry.Name
			items = append(items, semanticTaskTagItem(tag.Name, entry, tag.TaskCount))
		}
		if seedUnused && !canonicalInUse {
			items = append(items, semanticTaskTagItem(entry.Name, entry, 0))
		}
	}

	descriptive := make([]query.TagInfo, 0, len(tags))
	for _, tag := range tags {
		if !promoted[tag.Name] {
			descriptive = append(descriptive, tag)
		}
	}
	sort.Slice(descriptive, func(i, j int) bool {
		return descriptive[i].Name < descriptive[j].Name
	})
	for _, tag := range descriptive {
		// One facet is one row: a value carried on both axes matches either
		// field, so it belongs under CONTEXTS with the merged count rather than
		// appearing twice under a shared ID.
		category := TagDescriptiveCategory
		aliases := []string{tag.Name, "#" + tag.Name}
		fields := []listsurface.ListPreviewField{{Label: "Tasks", Value: strconv.Itoa(tag.TaskCount)}}
		if tag.ContextCount > 0 {
			category = TaskContextCategory
			aliases = append(aliases, "##"+tag.Name)
			fields = append(fields, listsurface.ListPreviewField{Label: "Context", Value: "##" + tag.Name})
		}
		// A descriptive tag changes no other color, so its own chip is its whole
		// identity: the row wears exactly what the task row and Peek will render.
		// The full stored spelling hashes here as it does there, so an 8-column
		// row chip and this row are one color.
		style := theme.TagChipStyle(tag.Name)
		items = append(items, listsurface.ListItem{
			ID:            listsurface.ListItemID("tag:" + tag.Name),
			CategoryID:    category,
			PrimaryText:   tag.Name,
			SecondaryText: fmt.Sprintf("%d tasks", tag.TaskCount),
			Aliases:       aliases,
			PrimaryStyle:  &style,
			Preview: listsurface.ListPreview{
				Title:  tag.Name,
				Fields: fields,
			},
			Data: tag.Name,
		})
	}

	return listsurface.ListSource{
		Items: items,
		Categories: []listsurface.ListCategory{
			{ID: TaskTagSemanticCategory, Label: "Semantic"},
			{ID: TaskContextCategory, Label: "Contexts"},
			{ID: TagDescriptiveCategory, Label: "Tags"},
		},
	}
}

// taskContextRow is the payload one selectable context row carries. The
// CONTEXTS section also holds tag rows recategorized by context usage, so a
// shared string payload could not tell the two populations apart.
type taskContextRow struct{ Name string }

// TaskContextListItems builds the radio rows: one clear row plus one row per
// context in use. The clear row borrows the permanently reserved "0" context
// name for its ID, so a real context can never mint a colliding row.
func TaskContextListItems(names []string) []listsurface.ListItem {
	items := make([]listsurface.ListItem, 0, len(names)+1)
	items = append(items, listsurface.ListItem{
		ID:            TaskContextClearRowID,
		CategoryID:    TaskContextCategory,
		PrimaryText:   "No context",
		SecondaryText: "clears the task context",
		Aliases:       []string{"no context", "clear context", "##0"},
		Preview: listsurface.ListPreview{Title: "No context", Fields: []listsurface.ListPreviewField{
			{Label: "Sigil", Value: "##0"},
		}},
		Data: taskContextRow{},
	})
	for _, name := range names {
		items = append(items, taskContextListItem(name))
	}
	return items
}

func taskContextListItem(name string) listsurface.ListItem {
	return listsurface.ListItem{
		ID:            TaskContextRowID(name),
		CategoryID:    TaskContextCategory,
		PrimaryText:   "##" + name,
		SecondaryText: "task context",
		Aliases:       []string{name, "##" + name},
		Preview: listsurface.ListPreview{Title: "##" + name, Fields: []listsurface.ListPreviewField{
			{Label: "Context", Value: name},
		}},
		Data: taskContextRow{Name: name},
	}
}

func TaskContextRowID(name string) listsurface.ListItemID {
	if name == "" {
		return TaskContextClearRowID
	}
	return listsurface.ListItemID(taskContextRowPrefix + name)
}

// taskContextRowName reports the context a row sets, and whether the row is a
// context row at all. The clear row answers true with an empty name.
func taskContextRowName(item listsurface.ListItem) (string, bool) {
	row, ok := item.Data.(taskContextRow)
	return row.Name, ok
}

func isTaskContextRowID(id listsurface.ListItemID) bool {
	return strings.HasPrefix(string(id), taskContextRowPrefix)
}

// taskTagPickerLess orders the rows a minted tag or context is inserted into. A
// global alphabetical sort would reorder the semantic section into bug, epic,
// feature, idea, story and contradict the precedence line under it, and would
// bury the context radio rows among the tag rows they share a section with.
func taskTagPickerLess(items []listsurface.ListItem) func(i, j int) bool {
	return func(i, j int) bool {
		left, right := items[i], items[j]
		leftSemantic := left.CategoryID == TaskTagSemanticCategory
		rightSemantic := right.CategoryID == TaskTagSemanticCategory
		leftName, leftContext := taskContextRowName(left)
		rightName, rightContext := taskContextRowName(right)
		switch {
		case leftSemantic != rightSemantic:
			return leftSemantic
		case leftSemantic:
			return semanticTaskTagPrecedence(left.PrimaryText) < semanticTaskTagPrecedence(right.PrimaryText)
		case leftContext != rightContext:
			return leftContext
		case leftContext:
			return leftName < rightName
		default:
			return left.PrimaryText < right.PrimaryText
		}
	}
}

// semanticTaskTagItem trades the usage count for the meaning, because a tag the
// picker offers before anything carries it has no count worth showing and the
// meaning is the reason it is offered at all.
func semanticTaskTagItem(name string, entry models.SemanticTaskTag, taskCount int) listsurface.ListItem {
	fields := []listsurface.ListPreviewField{{Label: "Meaning", Value: entry.Summary}}
	if taskCount > 0 {
		fields = append(fields, listsurface.ListPreviewField{Label: "Tasks", Value: strconv.Itoa(taskCount)})
	}
	style := theme.SemanticTaskTitleStyle(entry.Role, 0, false, false)
	return listsurface.ListItem{
		ID:            listsurface.ListItemID("tag:" + name),
		CategoryID:    TaskTagSemanticCategory,
		PrimaryText:   name,
		SecondaryText: entry.RowSummary,
		Aliases:       []string{name, "#" + name},
		PrimaryStyle:  &style,
		Preview:       listsurface.ListPreview{Title: name, Fields: fields},
		Data:          name,
	}
}

// SemanticTaskTagExplainer states the precedence the section order applies
// silently, which is the one thing the SEMANTIC rows above it cannot show. Five
// restated summaries would swamp the overlay, so it is one line. Its names and
// those rows resolve one SemanticTaskTitleStyle lookup, so the chain here and
// the section above it can never disagree and a future registry entry cannot
// render uncolored. The complete block wraps inline when needed.
func SemanticTaskTagExplainer(provider operation.ValueProviderID, width int) string {
	if !semanticTaskTagProvider(provider) {
		return ""
	}
	names := make([]string, 0, len(models.SemanticTaskTags))
	for _, entry := range models.SemanticTaskTags {
		names = append(names, theme.SemanticTaskTitleStyle(entry.Role, 0, false, false).Render(entry.Name))
	}
	head := theme.MutedStyle.Render(semanticTaskTagExplainerLabel) + "  " +
		strings.Join(names, theme.MutedStyle.Render(semanticTaskTagExplainerSeparator))
	return strings.Join(textfit.WrapStyled(
		head+" "+theme.MutedStyle.Render(SemanticTaskTagExplainerSummary),
		width,
	), "\n")
}

// TaskTagCategoryJumpHint names the section jump keys inside the overlay, since
// the footer's browse profile has one row and four more hints would evict Enter
// and Esc. It lists only the sections the source populates, so a key it names
// always lands somewhere.
func TaskTagCategoryJumpHint(provider operation.ValueProviderID, items []listsurface.ListItem, width int) string {
	if !semanticTaskTagProvider(provider) {
		return ""
	}
	populated := make(map[listsurface.ListCategoryID]bool, len(items))
	for _, item := range items {
		populated[item.CategoryID] = true
	}
	hints := make([]string, 0, len(taskTagJumpSections))
	for _, section := range taskTagJumpSections {
		if populated[section.Category] {
			// The key wears the footer's key color, so one hint reads the same
			// whether the reader met it in the footer or in the overlay.
			hints = append(hints, theme.HelpKeyStyle.Render(section.Key)+theme.MutedStyle.Render(" "+section.Label))
		}
	}
	if len(hints) == 0 {
		return ""
	}
	return strings.Join(textfit.WrapStyled(
		theme.MutedStyle.Render(taskTagJumpHintLabel+"  ")+
			strings.Join(hints, theme.MutedStyle.Render(taskTagJumpHintSeparator)),
		width,
	), "\n")
}

// semanticTaskTagPrecedence ranks one semantic row by the registry order the
// explainer states. A name the registry does not know sorts last instead of
// reshuffling the section it was put in.
func semanticTaskTagPrecedence(name string) models.SemanticTaskTagRole {
	if entry, ok := models.SemanticTaskTagByName(name); ok {
		return entry.Role
	}
	return models.SemanticTaskTagRole(len(models.SemanticTaskTags) + 1)
}
