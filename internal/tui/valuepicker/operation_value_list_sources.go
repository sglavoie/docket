package valuepicker

import (
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/theme"
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
			{id: chrome.DatePresetClear, label: "Clear defer", clear: true},
			{id: chrome.DatePresetOneHour, label: "In 1 hour", value: hourOffset(1)},
			{id: chrome.DatePresetFourHours, label: "In 4 hours", value: hourOffset(4)},
			{id: chrome.DatePresetEightHours, label: "In 8 hours", value: hourOffset(8)},
			{id: chrome.DatePresetMorning, label: "Tomorrow morning", value: morning(1)},
			{id: chrome.DatePresetTomorrow, label: "Until tomorrow", value: dayOffset(1)},
			{id: chrome.DatePresetWeek, label: "For one week", value: dayOffset(7)},
			{id: chrome.DatePresetMonth, label: "For 30 days", value: dayOffset(30)},
		}
	}
	return []datePreset{
		{id: chrome.DatePresetClear, label: "Clear", clear: true},
		{id: chrome.DatePresetToday, label: "Today", value: dayOffset(0)},
		{id: chrome.DatePresetTomorrow, label: "Tomorrow", value: dayOffset(1)},
		{id: chrome.DatePresetWeek, label: "In one week", value: dayOffset(7)},
		{id: chrome.DatePresetMonth, label: "In 30 days", value: dayOffset(30)},
	}
}

// datePresetKeyMarkerWidth is the one column the hint claims, plus its separating
// space. Every preset in both fields is keyed, so the blank slot is only ever the
// statement that this table stopped being total — the marker still holds the
// column so the labels beside it stay in line.
const datePresetKeyMarkerWidth = 2

// datePresetKeyMarker draws one row's shortcut key. It rides PrimaryMarker
// rather than the primary text or an alias, which is what keeps the hint out of
// the query: a key tells you how to apply a row, and it is not a word the row
// should answer a search by.
func datePresetKeyMarker(deferred bool, presetID string) string {
	key, ok := chrome.DatePresetKey(deferred, presetID)
	if !ok {
		return strings.Repeat(" ", datePresetKeyMarkerWidth)
	}
	return theme.MutedStyle.Render(key) + " "
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
	dateListCategoryDue     listsurface.ListCategoryID = "date-due"
	dateListCategoryDefer   listsurface.ListCategoryID = "date-defer"
	dateListCategoryCheckIn listsurface.ListCategoryID = "date-check-in"
)

const dateCheckInEditorItemID listsurface.ListItemID = "date:check-in:editor"

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
			PrimaryMarker:  datePresetKeyMarker(deferred, preset.id),
			SecondaryText:  secondary,
			Aliases:        []string{preset.id, secondary},
			DisabledReason: disabledReason,
			Data:           operation.OperationDateValue{Date: date, Deferred: deferred},
		})
	}
	return items
}

// DateSectionLabel names one of the two fields. The section headers, the
// active-field guidance, and the refusal a section switch reports all spell the
// field through this one function, so no surface can invent a second name for
// it.
func DateSectionLabel(deferred bool) string {
	if deferred {
		return "Defer"
	}
	return "Due"
}

// OperationDateListSource is the one picker `@` opens. Due rows come first
// because a deadline is the commoner answer, and deferReason disables the whole
// defer section when a resolved task cannot be deferred, which leaves due
// reachable on the same terminal task.
func OperationDateListSource(deferReason string) listsurface.ListSource {
	items := operationDateItems(false, dateListCategoryDue, "")
	items = append(items, operationDateItems(true, dateListCategoryDefer, deferReason)...)
	items = append(items, listsurface.ListItem{
		ID:            dateCheckInEditorItemID,
		CategoryID:    dateListCategoryCheckIn,
		PrimaryText:   "Set check-in",
		SecondaryText: "next date and repeat cadence",
		Aliases:       []string{"check-in", "review schedule", "repeat review", "next check-in"},
		Data:          operation.OperationCheckInEditorAction{},
	})
	return listsurface.ListSource{
		Items: items,
		Categories: []listsurface.ListCategory{
			{ID: dateListCategoryDue, Label: DateSectionLabel(false)},
			{ID: dateListCategoryDefer, Label: DateSectionLabel(true)},
			{ID: dateListCategoryCheckIn, Label: "Check-in"},
		},
	}
}

// OperationDueDateListSource is the deadline-only projection, with no
// deferral or check-in rows.
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

// OperationDateSelectedID highlights the row the combined picker opens on. The
// due field alone decides it: the cursor can only sit in one section, that
// section is always Due, and a stored deferral is therefore never what draws it
// somewhere else. A due date no preset reproduces names no row, so the cursor
// falls to Due's Clear row, which is where the list opens anyway — the highlight
// is a starting point, not a selection, and opening the picker writes nothing.
func OperationDateSelectedID(due *time.Time) []listsurface.ListItemID {
	if selected := DueDateSelectedID(due); len(selected) > 0 {
		return selected
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
