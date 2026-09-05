package valuepicker

import (
	"strconv"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/util"
)

// datePreset is one row of a date picker's vocabulary. The list the picker
// renders and the resolver that decides which row is highlighted both read
// this table, so the two cannot describe different presets.
type datePreset struct {
	id, label string
	clear     bool
	value     func() time.Time
}

// OperationDatePresets returns the preset rows for one date picker. A due date
// is a calendar day, so every one of its rows lands on local midnight; a
// deferral is an instant, so it also offers hour offsets and a morning row.
func OperationDatePresets(deferred bool, now time.Time) []datePreset {
	dayOffset := func(n int) func() time.Time {
		return func() time.Time { return util.StartOfDay(now).AddDate(0, 0, n) }
	}
	hourOffset := func(n int) func() time.Time {
		return func() time.Time { return now.Add(time.Duration(n) * time.Hour) }
	}
	// The morning row is a wall clock, not an elapsed duration: nine hours after
	// midnight reads 08:00 or 10:00 on the two days a year the offset shifts.
	morning := func(n int) func() time.Time {
		return func() time.Time {
			year, month, day := now.Date()
			return time.Date(year, month, day+n, 9, 0, 0, 0, now.Location())
		}
	}
	if deferred {
		return []datePreset{
			{id: "clear", label: "Clear defer", clear: true},
			{id: "1h", label: "In 1 hour", value: hourOffset(1)},
			{id: "4h", label: "In 4 hours", value: hourOffset(4)},
			{id: "8h", label: "In 8 hours", value: hourOffset(8)},
			{id: "morning", label: "Tomorrow morning", value: morning(1)},
			{id: "tomorrow", label: "Until tomorrow", value: dayOffset(1)},
			{id: "week", label: "For one week", value: dayOffset(7)},
			{id: "month", label: "For one month", value: dayOffset(30)},
		}
	}
	return []datePreset{
		{id: "clear", label: "Clear", clear: true},
		{id: "today", label: "Today", value: dayOffset(0)},
		{id: "tomorrow", label: "Tomorrow", value: dayOffset(1)},
		{id: "week", label: "In one week", value: dayOffset(7)},
		{id: "month", label: "In one month", value: dayOffset(30)},
	}
}

// OperationDateSecondaryText names an instant the way a picker row and a search
// alias both need it. A bare clock is never enough: "+8h" at 20:00 lands
// tomorrow, and a lone "04:00" would silently claim today.
func OperationDateSecondaryText(value time.Time) string {
	if value.Equal(util.StartOfDay(value)) {
		return value.Format("Mon Jan 2")
	}
	return value.Format("Mon Jan 2 15:04")
}

// The two date sections share preset names, so a row's identity carries its
// field. Both sections are always built from the same namespace, including in
// the due-only projection, so one resolver answers either list.
const (
	dateListCategoryDue   listsurface.ListCategoryID = "date-due"
	dateListCategoryDefer listsurface.ListCategoryID = "date-defer"
)

func DateListItemID(deferred bool, presetID string) listsurface.ListItemID {
	if deferred {
		return listsurface.ListItemID("date:defer:" + presetID)
	}
	return listsurface.ListItemID("date:due:" + presetID)
}

func operationDateItems(deferred bool, categoryID listsurface.ListCategoryID, disabledReason string) []listsurface.ListItem {
	presets := OperationDatePresets(deferred, time.Now())
	items := make([]listsurface.ListItem, 0, len(presets))
	for _, preset := range presets {
		var date *time.Time
		secondary := "remove date"
		if !preset.clear {
			value := preset.value()
			date = &value
			secondary = OperationDateSecondaryText(value)
		}
		items = append(items, listsurface.ListItem{
			ID:             DateListItemID(deferred, preset.id),
			CategoryID:     categoryID,
			PrimaryText:    preset.label,
			SecondaryText:  secondary,
			Aliases:        []string{preset.id, secondary},
			DisabledReason: disabledReason,
			Data:           operation.OperationDateValue{Date: date, Deferred: deferred},
		})
	}
	return items
}

// OperationDateListSource is the one picker `@` opens. Due rows come first
// because a deadline is the commoner answer, and deferReason disables the whole
// defer section when a resolved task cannot be deferred, which leaves due
// reachable on the same terminal task.
func OperationDateListSource(deferReason string) listsurface.ListSource {
	items := operationDateItems(false, dateListCategoryDue, "")
	items = append(items, operationDateItems(true, dateListCategoryDefer, deferReason)...)
	return listsurface.ListSource{
		Items: items,
		Categories: []listsurface.ListCategory{
			{ID: dateListCategoryDue, Label: "Due"},
			{ID: dateListCategoryDefer, Label: "Defer"},
		},
	}
}

// OperationDueDateListSource is the due-only projection. The waiting check-back
// chain is the one caller: that step asks for a deadline specifically, so a
// defer section there would offer an answer to a question nobody asked.
func OperationDueDateListSource() listsurface.ListSource {
	return listsurface.ListSource{Items: operationDateItems(false, "", "")}
}

// parentListNoneID is the row expressing the `>0` grammar: no parent, task at
// the project root.
const parentListNoneID listsurface.ListItemID = "parent:none"

// OperationParentListSource offers every structural candidate in the task's own
// project, carrying the refusal reason on the row rather than behind Enter, so
// a cycle, self-reference, or depth-six overflow reads as a disabled row.
// Children resolve only in the effective project, exactly as `>parent` does.
func OperationParentListSource(task *models.Task, project *models.Project) listsurface.ListSource {
	candidates := grammar.StructuralCandidates(project)
	items := make([]listsurface.ListItem, 0, len(candidates)+1)
	items = append(items, listsurface.ListItem{
		ID:            parentListNoneID,
		PrimaryText:   "No parent (project root)",
		SecondaryText: ">0",
		Aliases:       []string{"none", "root", "0", ">0", "detach"},
		Data:          operation.OperationParentValue{},
	})
	for _, candidate := range candidates {
		id := candidate.ID
		shortID := "#" + strconv.Itoa(candidate.ShortID)
		items = append(items, listsurface.ListItem{
			ID:             listsurface.ListItemID("parent:" + id),
			PrimaryText:    candidate.Title,
			SecondaryText:  shortID,
			Aliases:        []string{shortID, strconv.Itoa(candidate.ShortID), candidate.Title},
			DisabledReason: grammar.InvalidQuickEditParent(task, project, project, candidate),
			Data:           operation.OperationParentValue{ParentID: &id},
		})
	}
	return listsurface.ListSource{Items: items}
}

// OperationChildListSource offers every structural candidate in the task's own
// project that the task does not already parent, exactly the population
// `<child` resolves against. Omitting the existing children is what keeps the
// picker additive: with no row for a child the task already has, no selection
// state can ask for a detach, which `<0` is not.
func OperationChildListSource(task *models.Task, project *models.Project) listsurface.ListSource {
	candidates := grammar.StructuralCandidates(project)
	items := make([]listsurface.ListItem, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ID == task.ID {
			continue
		}
		if candidate.ParentID != nil && *candidate.ParentID == task.ID {
			continue
		}
		id := candidate.ID
		shortID := "#" + strconv.Itoa(candidate.ShortID)
		items = append(items, listsurface.ListItem{
			ID:             listsurface.ListItemID("child:" + id),
			PrimaryText:    candidate.Title,
			SecondaryText:  shortID,
			Aliases:        []string{shortID, strconv.Itoa(candidate.ShortID), candidate.Title},
			DisabledReason: grammar.InvalidQuickEditParent(candidate, project, project, task),
			Data:           id,
		})
	}
	return listsurface.ListSource{Items: items}
}

// MoveProjectPickerItem adapts one destination project for the Move to Project
// list.
type MoveProjectPickerItem struct {
	Project *models.Project
}

func (item MoveProjectPickerItem) FilterText() string {
	return item.Project.Name + " " + item.Project.Slug
}

func (item MoveProjectPickerItem) DisplayText() string {
	return item.Project.Name
}

func (item MoveProjectPickerItem) CountValue() int {
	return item.Project.CountTasks()
}

func (item MoveProjectPickerItem) ItemID() string {
	return "project:" + item.Project.ID
}

func (item MoveProjectPickerItem) IsPinned() bool {
	return item.Project.Pinned
}

// OperationProjectListSource offers every available project except the task's
// own, since a move to the current owner is not a move. The population is
// AvailableProjects rather than the Outliner's TaskWorkspaceProjects, so a
// parked project stays a destination exactly as it does for `%project`.
func OperationProjectListSource(projects []*models.Project, currentProjectID string) listsurface.ListSource {
	items := make([]MoveProjectPickerItem, 0, len(projects))
	for _, project := range projects {
		if project == nil || project.ID == currentProjectID {
			continue
		}
		items = append(items, MoveProjectPickerItem{Project: project})
	}
	source := listsurface.ListSourceFromPickerItems(items, false)
	for i := range source.Items {
		project := items[i].Project
		source.Items[i].SecondaryText = moveProjectRowSecondary(project)
		// The executor's request names a project by ID, so the adapted payload is
		// narrowed from the picker item to that identifier.
		source.Items[i].Data = project.ID
	}
	return source
}

// moveProjectRowSecondary names the destination by slug rather than by task
// count. These are catalog records whose task trees are not loaded here, so a
// count would read as "empty"; the slug is what the Outliner's own project
// headers show, and the tag chips are what make a parked destination legible.
func moveProjectRowSecondary(project *models.Project) string {
	secondary := "[" + project.Slug + "]"
	if chips := projecthub.ProjectTagChips(project.Tags); chips != "" {
		secondary += " · " + chips
	}
	return secondary
}

func ParentSelectedID(task *models.Task) []listsurface.ListItemID {
	if task == nil || task.ParentID == nil {
		return []listsurface.ListItemID{parentListNoneID}
	}
	return []listsurface.ListItemID{listsurface.ListItemID("parent:" + *task.ParentID)}
}

func DueDateSelectedID(date *time.Time) []listsurface.ListItemID {
	if date == nil {
		return []listsurface.ListItemID{DateListItemID(false, "clear")}
	}
	return RelativeDateSelectedID(date, false)
}

// OperationDateSelectedID highlights the row the task's own values name. Due
// wins when both fields are set, because the cursor can only sit in one section
// and the due section is the one the list opens on.
func OperationDateSelectedID(due, deferred *time.Time) []listsurface.ListItemID {
	if due != nil {
		return RelativeDateSelectedID(due, false)
	}
	if deferred != nil {
		return RelativeDateSelectedID(deferred, true)
	}
	return []listsurface.ListItemID{DateListItemID(false, "clear")}
}

// RelativeDateSelectedID highlights a preset only when the stored value is
// reproducible from now alone, so the hour rows never match: four hours from
// when some earlier picker opened is a different fact than four hours from now,
// and a tolerance window would conflate the two.
func RelativeDateSelectedID(date *time.Time, deferred bool) []listsurface.ListItemID {
	for _, preset := range OperationDatePresets(deferred, time.Now()) {
		if preset.clear {
			continue
		}
		if preset.value().Equal(*date) {
			return []listsurface.ListItemID{DateListItemID(deferred, preset.id)}
		}
	}
	return nil
}
