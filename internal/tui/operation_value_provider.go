package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
	"github.com/sglavoie/tt/internal/util"
)

const (
	operationTaskSetState    OperationID = "tui.task.set-state"
	operationTaskSetPriority OperationID = "tui.task.set-priority"
	operationTaskSetDueDate  OperationID = "tui.task.set-due-date"
	operationTaskSetTags     OperationID = "tui.task.set-tags"
	operationTaskSetParent   OperationID = "tui.task.set-parent"
	operationTaskIndent      OperationID = "tui.task.indent"
	operationTaskOutdent     OperationID = "tui.task.outdent"

	// operationTaskAdoptChildren is the `<child` sigil's key. It is a separate
	// operation from set-parent because the sigil is repeatable and additive,
	// which set-parent's single-valued replace semantics cannot express.
	operationTaskAdoptChildren OperationID = "tui.task.adopt-children"

	// operationTaskMoveToProject is the `%project` sigil's key. It travels the
	// same cross-project edit path the sigil does rather than adding a second
	// relocation route.
	operationTaskMoveToProject OperationID = "tui.task.move-to-project"
)

// OperationParentValue is the picker-neutral reparent payload. A nil ParentID
// is the project root, which is what the `>0` edit sigil expresses.
type OperationParentValue struct {
	ParentID *string
}

// OperationDateValue is the picker-neutral due/defer payload. Nil clears the
// field. Deferred names which of the two fields the value writes, because one
// picker answers both.
type OperationDateValue struct {
	Date     *time.Time
	Deferred bool
}

// OperationTagsValue is the tri-state bulk tag edit emitted by the value
// picker. Add and Remove are uniform patches; tags in neither set preserve
// their existing per-task membership. ContextSet carries the singleton task
// context in the same payload so both land in one transaction; an empty Context
// clears the field.
type OperationTagsValue struct {
	Add        []string
	Remove     []string
	ContextSet bool
	Context    string
}

type operationTagMembership uint8

const (
	operationTagNone operationTagMembership = iota
	operationTagSome
	operationTagAll
)

// OperationValueSelectedMsg resumes a pending operation with the chosen value.
type OperationValueSelectedMsg struct {
	Request OperationExecutionRequest
	Value   any
}

// OperationValueCanceledMsg closes the value picker and returns to its action
// sheet without executing the operation.
type OperationValueCanceledMsg struct{}

// OperationValuePickerModel is the shared ListSurface value-picker control.
type OperationValuePickerModel struct {
	surface    *ListSurfaceModel
	request    OperationExecutionRequest
	provider   ValueProviderID
	title      string
	subtitle   string
	multi      bool
	searching  bool
	tagInitial map[ListItemID]operationTagMembership
	tagCurrent map[ListItemID]operationTagMembership
	tagAdder   TagAdder
	dateAdder  DateAdder
	// dateDeferred records which section the inline date mode was opened from,
	// so the typed value writes the same field the row under the cursor names.
	dateDeferred bool
	// contextEntry records that the inline add mode was opened from a context
	// row, so the typed name mints a context instead of a tag.
	contextEntry bool
	// contextInitial is the context row every target already agreed on. An empty
	// ID means the targets disagreed, so any chosen row is a change.
	contextInitial ListItemID
}

func newOperationValuePickerModel(
	request OperationExecutionRequest,
	provider ValueProviderID,
	title string,
	subtitle string,
	source ListSource,
	multi bool,
	selected []ListItemID,
) (*OperationValuePickerModel, error) {
	mode := ListSelectionSingle
	if multi {
		mode = ListSelectionMulti
	}
	surface, err := source.NewModel(ListSurfaceConfig{
		SelectionMode: mode,
		EmptyState: ListEmptyState{
			Title: "No values available.",
		},
		NoMatchState: ListEmptyState{
			Title: "No matching values.",
		},
	})
	if err != nil {
		return nil, err
	}
	if len(selected) > 0 {
		if multi {
			if err := surface.SetSelectedIDs(selected); err != nil {
				return nil, err
			}
		} else {
			surface.SetCursorToID(selected[0])
		}
	}
	model := &OperationValuePickerModel{
		surface:  surface,
		request:  request,
		provider: provider,
		title:    title,
		subtitle: subtitle,
		multi:    multi,
	}
	if provider.tagEntry() {
		model.tagAdder = NewTagAdder()
	}
	if provider.dateEntry() {
		model.dateAdder = NewDateAdder()
	}
	return model, nil
}

// dateSectionUnderCursor reports whether the cursor sits on a defer row, and
// refuses when that row is disabled so the inline mode cannot type past a
// refusal the preset rows already carry.
func (model *OperationValuePickerModel) dateSectionUnderCursor() (bool, error) {
	item, ok := model.surface.CurrentItem()
	if !ok {
		return false, ErrListItemNotFound
	}
	if !item.Enabled() {
		return false, fmt.Errorf("%w: %s", ErrListItemDisabled, item.DisabledReason)
	}
	value, ok := item.Data.(OperationDateValue)
	if !ok {
		return false, ErrListItemNotFound
	}
	return value.Deferred, nil
}

//nolint:gocyclo // Centralized key dispatch preserves picker, freeform, and tag-input precedence.
func (model *OperationValuePickerModel) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	if model == nil || model.surface == nil {
		return nil, true, OperationValueCanceledMsg{}
	}
	if model.provider.tagEntry() && model.tagAdder.AddTagMode {
		handled, cmd := model.tagAdder.handleAddTagInputKey(msg, model.addOrSelectTag)
		if handled {
			return cmd, false, nil
		}
	}
	if model.provider.dateEntry() && model.dateAdder.AddDateMode {
		var chosen *time.Time
		cmd := model.dateAdder.handleAddDateInputKey(msg, func(date time.Time) error {
			chosen = &date
			return nil
		})
		// A typed date carries the same payload the preset rows carry, so the
		// executor, the single handler, and the bulk handler stay untouched.
		if chosen != nil {
			return nil, true, OperationValueSelectedMsg{
				Request: model.request,
				Value:   OperationDateValue{Date: chosen, Deferred: model.dateDeferred},
			}
		}
		return cmd, false, nil
	}
	key := msg.String()
	if model.searching {
		switch key {
		case "up":
			model.surface.MoveCursor(-1)
			return nil, false, nil
		case "down":
			model.surface.MoveCursor(1)
			return nil, false, nil
		case "esc":
			if model.surface.Query().Raw != "" {
				model.surface.SetQuery("")
				return nil, false, nil
			}
			model.searching = false
			return nil, false, nil
		case "enter":
			model.searching = false
			return nil, false, nil
		case "backspace":
			query := model.surface.Query().Raw
			if query != "" {
				_, size := utf8.DecodeLastRuneInString(query)
				model.surface.SetQuery(query[:len(query)-size])
			}
			return nil, false, nil
		default:
			if msg.Text != "" {
				model.surface.SetQuery(model.surface.Query().Raw + msg.Text)
			}
			return nil, false, nil
		}
	}

	switch key {
	case "esc":
		return nil, true, OperationValueCanceledMsg{}
	case "/":
		model.searching = true
	case "a":
		if model.provider.tagEntry() {
			model.contextEntry = model.contextRowUnderCursor()
			return model.tagAdder.EnterAddMode(), false, nil
		}
		if model.provider.dateEntry() {
			deferred, err := model.dateSectionUnderCursor()
			if err != nil {
				return nil, false, StatusMsg(err.Error())
			}
			model.dateDeferred = deferred
			if deferred {
				model.dateAdder = NewInstantDateAdder()
			} else {
				model.dateAdder = NewDateAdder()
			}
			return model.dateAdder.EnterAddMode(), false, nil
		}
	case "up", "k", "ctrl+p":
		model.surface.MoveCursor(-1)
	case "down", "j", "ctrl+n":
		model.surface.MoveCursor(1)
	case "home", "g":
		model.surface.MoveCursorToStart()
	case "end", "G":
		model.surface.MoveCursorToEnd()
	case "pgup":
		model.surface.PageCursor(-1)
	case "pgdown":
		model.surface.PageCursor(1)
	case "v":
		// The project tag facet picker hands Space to the vocabulary sheet, so
		// selection moves to the key the project picker already uses for it.
		if model.provider == ValueProviderFilterProjectTags {
			if err := model.toggleCurrentMultiValue(); err != nil {
				return nil, false, StatusMsg(err.Error())
			}
		}
	case "space":
		if model.provider == ValueProviderFilterProjectTags {
			item, ok := model.surface.CurrentItem()
			if !ok {
				return nil, false, nil
			}
			tag, ok := projectTagVocabularySelectableTag(item)
			if !ok {
				return nil, false, StatusMsg("Untagged is a facet, not a tag")
			}
			return nil, true, ProjectTagVocabularyActionsMsg{Tag: tag}
		}
		if model.multi {
			if err := model.toggleCurrentMultiValue(); err != nil {
				return nil, false, StatusMsg(err.Error())
			}
		}
	case "enter":
		value, err := model.selectedValue()
		if err != nil {
			return nil, false, StatusMsg(err.Error())
		}
		return nil, true, OperationValueSelectedMsg{
			Request: model.request,
			Value:   value,
		}
	case "w", "s", "t", "c":
		if semanticTaskTagProvider(model.provider) {
			return nil, false, model.jumpToTagCategory(key)
		}
	case "#":
		// The context is single-valued, so its rows are a radio group rather than
		// the checkboxes the tag rows beside them are.
		if model.provider == ValueProviderTags {
			if err := model.selectContextUnderCursor(); err != nil {
				return nil, false, StatusMsg(err.Error())
			}
		}
	default:
		if model.provider == ValueProviderPriority && len(key) == 1 && key[0] >= '0' && key[0] <= '3' {
			if model.surface.SetCursorToID(ListItemID("priority:" + key)) {
				value, err := model.selectedValue()
				if err == nil {
					return nil, true, OperationValueSelectedMsg{
						Request: model.request, Value: value,
					}
				}
			}
		}
	}
	return nil, false, nil
}

// jumpToTagCategory moves the cursor to the first row of one tag section. It
// reads the matches rather than the items so a jump under an active search
// lands on a row that is actually on screen.
func (model *OperationValuePickerModel) jumpToTagCategory(key string) tea.Msg {
	category, label, ok := taskTagJumpCategory(key)
	if !ok {
		return nil
	}
	for _, match := range model.surface.Matches() {
		if match.Item.CategoryID == category {
			model.surface.SetCursorToID(match.Item.ID)
			return nil
		}
	}
	return StatusMsg(fmt.Sprintf("No %s rows", strings.ToUpper(label)))
}

// tagAdderNoun names what the inline add mode is minting, which the cursor
// decided when the mode was entered.
func (model *OperationValuePickerModel) tagAdderNoun() string {
	if model.contextEntry {
		return "context"
	}
	return "tag"
}

// contextRowUnderCursor reports whether the cursor sits on a context row.
func (model *OperationValuePickerModel) contextRowUnderCursor() bool {
	item, ok := model.surface.CurrentItem()
	if !ok {
		return false
	}
	_, isContext := taskContextRowName(item)
	return isContext
}

// selectContextUnderCursor makes the context row under the cursor the only
// selected context, leaving the tag checkboxes beside it untouched.
func (model *OperationValuePickerModel) selectContextUnderCursor() error {
	item, ok := model.surface.CurrentItem()
	if !ok {
		return ErrListItemNotFound
	}
	if _, isContext := taskContextRowName(item); !isContext {
		return fmt.Errorf("# sets a context: press c to jump to CONTEXTS")
	}
	selected := make([]ListItemID, 0, len(model.surface.SelectedIDs())+1)
	for _, id := range model.surface.SelectedIDs() {
		if !isTaskContextRowID(id) {
			selected = append(selected, id)
		}
	}
	selected = append(selected, item.ID)
	if err := model.surface.SetSelectedIDs(selected); err != nil {
		return err
	}
	if model.tagCurrent == nil {
		return nil
	}
	for id := range model.tagCurrent {
		if isTaskContextRowID(id) {
			model.tagCurrent[id] = operationTagNone
		}
	}
	model.tagCurrent[item.ID] = operationTagAll
	return model.syncTagMembershipRows()
}

// selectedContextRow returns the chosen context row, if any. The clear row
// answers with an empty name, which is what an empty context field means.
func (model *OperationValuePickerModel) selectedContextRow() (ListItemID, string, bool) {
	for _, item := range model.surface.Items() {
		name, isContext := taskContextRowName(item)
		if isContext && model.surface.IsSelected(item.ID) {
			return item.ID, name, true
		}
	}
	return "", "", false
}

// addOrSelectContext mints one context row and makes it the selection. "0" is
// permanently reserved, because `##0` is how the grammar clears the field.
func (model *OperationValuePickerModel) addOrSelectContext(raw string) error {
	name, err := models.NormalizeTaskContext(raw)
	if err != nil {
		return err
	}
	if name == "0" {
		return fmt.Errorf("0 is reserved: the No context row clears the field")
	}
	id := taskContextRowID(name)
	items := model.surface.Items()
	known := false
	for _, item := range items {
		if item.ID == id {
			known = true
			break
		}
	}
	model.surface.SetQuery("")
	if !known {
		items = append(items, taskContextListItem(name))
		sort.SliceStable(items, taskTagPickerLess(items))
		if err := model.surface.SetItems(items); err != nil {
			return err
		}
		if model.tagCurrent != nil {
			model.tagInitial[id] = operationTagNone
			model.tagCurrent[id] = operationTagNone
		}
	}
	if !model.surface.SetCursorToID(id) {
		return fmt.Errorf("%w: %s", ErrListItemNotFound, id)
	}
	return model.selectContextUnderCursor()
}

func (model *OperationValuePickerModel) addOrSelectTag(tag string) error {
	if model.contextEntry {
		return model.addOrSelectContext(tag)
	}
	if model.provider == ValueProviderProjectTags {
		// The project tag namespace is folded at rest, so the row a typed tag
		// creates must already carry the spelling the catalog will store.
		tag = strings.ToLower(tag)
	}
	id := ListItemID("tag:" + tag)
	items := model.surface.Items()
	for _, item := range items {
		if item.ID == id {
			model.surface.SetQuery("")
			return model.selectTagForAll(id)
		}
	}

	source := ListSourceFromTags([]TagInfo{{Name: tag}})
	if len(source.Items) != 1 {
		return fmt.Errorf("building tag row for %q", tag)
	}
	item := source.Items[0]
	item.Data = tag
	// A typed tag lands under TAGS beside rows that already wear their chip, so
	// it takes the same color here rather than being the one plain row in a
	// colored section.
	style := tagChipStyle(tag)
	item.PrimaryStyle = &style
	if model.provider == ValueProviderProjectTags {
		item.SecondaryText = ""
	}
	// A minted tag the registry knows belongs in the semantic section, or it
	// would sit under TAGS while it colors the title.
	if entry, ok := models.SemanticTaskTagByName(tag); ok && semanticTaskTagProvider(model.provider) {
		item = semanticTaskTagItem(tag, entry, 0)
	}
	items = append(items, item)
	sort.SliceStable(items, taskTagPickerLess(items))

	model.surface.SetQuery("")
	if err := model.surface.SetItems(items); err != nil {
		return err
	}
	if model.tagInitial != nil {
		model.tagInitial[id] = operationTagNone
		model.tagCurrent[id] = operationTagAll
	}
	return model.selectTagForAll(id)
}

func (model *OperationValuePickerModel) selectTagForAll(id ListItemID) error {
	if model.tagCurrent != nil {
		model.tagCurrent[id] = operationTagAll
	}
	if !model.surface.IsSelected(id) {
		selected := model.surface.SelectedIDs()
		selected = append(selected, id)
		if err := model.surface.SetSelectedIDs(selected); err != nil {
			return err
		}
	}
	if model.tagCurrent != nil {
		if err := model.syncTagMembershipRows(); err != nil {
			return err
		}
	}
	if !model.surface.SetCursorToID(id) {
		return fmt.Errorf("%w: %s", ErrListItemNotFound, id)
	}
	return nil
}

func (model *OperationValuePickerModel) toggleCurrentMultiValue() error {
	if model.provider != ValueProviderTags || len(model.tagCurrent) == 0 {
		_, err := model.surface.ToggleCurrentSelection()
		return err
	}
	item, ok := model.surface.CurrentItem()
	if !ok {
		return ErrListItemNotFound
	}
	if _, isContext := taskContextRowName(item); isContext {
		return fmt.Errorf("a task holds one context: press # to set it")
	}
	state := model.tagCurrent[item.ID]
	if state == operationTagAll {
		if _, err := model.surface.ToggleCurrentSelection(); err != nil {
			return err
		}
		model.tagCurrent[item.ID] = operationTagNone
	} else {
		if !model.surface.IsSelected(item.ID) {
			if _, err := model.surface.ToggleCurrentSelection(); err != nil {
				return err
			}
		}
		model.tagCurrent[item.ID] = operationTagAll
	}
	return model.syncTagMembershipRows()
}

func (model *OperationValuePickerModel) configureTagMembership(
	membership map[ListItemID]operationTagMembership,
) error {
	if model == nil || model.surface == nil {
		return nil
	}
	model.tagInitial = make(map[ListItemID]operationTagMembership, len(membership))
	model.tagCurrent = make(map[ListItemID]operationTagMembership, len(membership))
	selected := make([]ListItemID, 0, len(membership))
	for id, state := range membership {
		model.tagInitial[id] = state
		model.tagCurrent[id] = state
		if state == operationTagAll {
			selected = append(selected, id)
		}
	}
	if err := model.surface.SetSelectedIDs(selected); err != nil {
		return err
	}
	return model.syncTagMembershipRows()
}

func (model *OperationValuePickerModel) syncTagMembershipRows() error {
	items := model.surface.Items()
	for index := range items {
		items[index].Indeterminate = model.tagCurrent[items[index].ID] == operationTagSome
	}
	return model.surface.SetItems(items)
}

func (model *OperationValuePickerModel) selectedValue() (any, error) {
	if model.provider == ValueProviderTags && len(model.tagCurrent) > 0 {
		value := OperationTagsValue{}
		for _, item := range model.surface.Items() {
			if _, isContext := taskContextRowName(item); isContext {
				continue
			}
			initial := model.tagInitial[item.ID]
			current := model.tagCurrent[item.ID]
			switch {
			case current == operationTagAll && initial != operationTagAll:
				tag, ok := item.Data.(string)
				if !ok {
					return nil, fmt.Errorf("selected value %s has invalid payload", item.ID)
				}
				value.Add = append(value.Add, tag)
			case current == operationTagNone && initial != operationTagNone:
				tag, ok := item.Data.(string)
				if !ok {
					return nil, fmt.Errorf("selected value %s has invalid payload", item.ID)
				}
				value.Remove = append(value.Remove, tag)
			}
		}
		sort.Strings(value.Add)
		sort.Strings(value.Remove)
		if id, name, ok := model.selectedContextRow(); ok && id != model.contextInitial {
			value.ContextSet = true
			value.Context = name
		}
		return value, nil
	}
	if model.multi {
		items := model.surface.SelectedItems()
		values := make([]string, 0, len(items))
		for _, item := range items {
			value, ok := item.Data.(string)
			if !ok {
				return nil, fmt.Errorf("selected value %s has invalid payload", item.ID)
			}
			values = append(values, value)
		}
		sort.Strings(values)
		return values, nil
	}
	item, ok := model.surface.CurrentItem()
	if !ok {
		return nil, ErrListItemNotFound
	}
	if !item.Enabled() {
		return nil, fmt.Errorf("%w: %s", ErrListItemDisabled, item.DisabledReason)
	}
	return item.Data, nil
}

// operationValuePickerInnerWidth is the overlay box width the picker claims out
// of the terminal.
func operationValuePickerInnerWidth(width int) int {
	return min(max(width*9/10, 46), max(width-6, 24))
}

// operationValuePickerExplainerWidth is the width the explainers are rendered
// at: the box content, inset from the border.
func operationValuePickerExplainerWidth(width int) int {
	return operationValuePickerInnerWidth(width) - 4
}

func (model *OperationValuePickerModel) guidanceSpec() (GuidanceSpec, bool) {
	if model == nil || model.surface == nil {
		return GuidanceSpec{}, false
	}
	if model.provider.tagEntry() && model.tagAdder.AddTagMode {
		return GuidanceSpec{
			Hints: requiredGuidanceHints(
				"Enter add "+model.tagAdderNoun(), "Esc cancel",
			),
			Rows: 2,
		}, true
	}
	if model.provider.dateEntry() && model.dateAdder.AddDateMode {
		return GuidanceSpec{
			Hints: requiredGuidanceHints("Enter set date", "Esc cancel"),
			Rows:  2,
		}, true
	}
	count := len(model.surface.Matches())
	searching := model.searching
	enterLabel := "choose"
	if model.multi {
		enterLabel = "apply"
	}
	if searching {
		enterLabel = "apply search"
	}
	escapeLabel := "back"
	if searching {
		escapeLabel = "cancel search"
		if model.surface.Query().Raw != "" {
			escapeLabel = "clear search"
		}
	}
	km := &keymap.KeyMap{Entries: []keymap.Entry{
		keymap.Bind("j", "Move selection", "Navigation",
			keymap.WithGate(func() bool { return count > 0 }),
			keymap.WithFooter(10, "navigate", false), keymap.WithFooterKey("j/k or ↑/↓")),
		keymap.Bind("/", "Search values", "Filters",
			keymap.WithGate(func() bool { return !searching }), keymap.WithFooter(20, "search", false)),
		keymap.Bind("enter", "Apply current state", "Actions",
			keymap.WithGate(func() bool { return searching || count > 0 }),
			keymap.WithFooter(1, enterLabel, true), keymap.WithFooterKey("Enter")),
		keymap.Bind("esc", "Unwind value picker", "Navigation",
			keymap.WithFooter(2, escapeLabel, true), keymap.WithFooterKey("Esc")),
	}}
	add := func(key, description, label string, rank int, gate keymap.Gate) {
		options := []keymap.BindOption{
			keymap.WithGate(gate), keymap.WithFooter(rank, label, false),
		}
		if key == "space" {
			options = append(options, keymap.WithFooterKey("Space"))
		}
		km.Entries = append(km.Entries, keymap.Bind(key, description, "Actions", options...))
	}
	if model.provider == ValueProviderFilterProjectTags {
		add("v", "Toggle current facet", "toggle", 3, func() bool { return !searching && count > 0 })
		add("space", "Open tag actions", "tag actions", 4, func() bool { return !searching && count > 0 })
	} else if model.multi {
		add("space", "Toggle current value", "toggle", 3, func() bool { return !searching && count > 0 })
	}
	if model.provider.tagEntry() {
		noun := "tag"
		if model.contextRowUnderCursor() {
			noun = "context"
		}
		add("a", "Add a "+noun, "add "+noun, 4, func() bool { return !searching })
	}
	if model.provider == ValueProviderTags {
		add("#", "Set the task context", "set context", 5, func() bool {
			return !searching && count > 0
		})
	}
	if model.provider.dateEntry() {
		add("a", "Type a specific date", "set date", 4, func() bool { return !searching })
	}
	return listSurfaceGuidanceSpec(model.surface.GuidanceState(searching), km, nil)
}

func (model *OperationValuePickerModel) render(width int) string {
	return model.renderWithin(width, 0)
}

// renderWithin budgets the focusable list after reserving viewport rows for
// non-focusable explainers and modal chrome. A zero height keeps the natural
// list height for isolated render callers.
func (model *OperationValuePickerModel) renderWithin(width, height int) string {
	if model == nil || model.surface == nil {
		return ""
	}
	innerWidth := operationValuePickerInnerWidth(width)
	if model.provider.tagEntry() && model.tagAdder.AddTagMode {
		noun := model.tagAdderNoun()
		content := MutedStyle.Render(fmt.Sprintf(
			"Enter a %s name (max %d characters)",
			noun,
			model.tagAdder.AddTagInput.CharLimit,
		)) + "\n\n" + model.tagAdder.AddTagInput.View()
		if model.tagAdder.AddTagError != "" {
			content += "\n\n" + BoldStyle.Foreground(ColorError).Render(model.tagAdder.AddTagError)
		}
		title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(
			"Add New " + strings.ToUpper(noun[:1]) + noun[1:],
		)
		guidance, _ := model.guidanceSpec()
		return renderOverlayBox(
			content,
			innerWidth,
			title,
			renderGuidanceText(guidance, max(innerWidth-4, 1)),
			lipgloss.NewStyle().Foreground(ColorPrimary),
		)
	}
	if model.provider.dateEntry() && model.dateAdder.AddDateMode {
		content := model.dateAdder.AddDateInput.View() + "\n\n" +
			MutedStyle.Render(strings.Join(model.dateAdder.dateAdderFormats(), "\n"))
		if model.dateAdder.AddDateError != "" {
			content += "\n\n" + BoldStyle.Foreground(ColorError).Render(model.dateAdder.AddDateError)
		}
		title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Set a Specific Date")
		guidance, _ := model.guidanceSpec()
		return renderOverlayBox(
			content,
			innerWidth,
			title,
			renderGuidanceText(guidance, max(innerWidth-4, 1)),
			lipgloss.NewStyle().Foreground(ColorPrimary),
		)
	}
	explainerWidth := operationValuePickerExplainerWidth(width)
	explainers := make([]string, 0, 3)
	if explainer := behavioralProjectTagExplainer(model.provider, explainerWidth); explainer != "" {
		explainers = append(explainers, explainer)
	}
	if explainer := semanticTaskTagExplainer(model.provider, explainerWidth); explainer != "" {
		explainers = append(explainers, explainer)
	}
	if hint := taskTagCategoryJumpHint(
		model.provider, model.surface.Items(), explainerWidth,
	); hint != "" {
		explainers = append(explainers, hint)
	}
	explainerBlock := strings.Join(explainers, "\n\n")

	listHeight := min(max(len(model.surface.Rows()), 6), 15)
	if height > 0 {
		// Halo, frame, title, search, search/list gap, and footer consume nine
		// rows. A present explainer adds its own rows plus one separating row.
		reservedRows := 9
		if explainerBlock != "" {
			reservedRows += 1 + lipgloss.Height(explainerBlock)
		}
		listHeight = min(listHeight, max(height-reservedRows, 1))
	}
	query := model.surface.Query().Raw
	search := ""
	if model.searching || query != "" {
		cursor := ""
		if model.searching {
			cursor = "▏"
		}
		search = "Search: " + query + cursor
	}
	content := RenderCompactListSurface(model.surface, innerWidth-4, listHeight)
	if search != "" {
		content = MutedStyle.Render(search) + "\n\n" + content
	}
	if explainerBlock != "" {
		content += "\n\n" + explainerBlock
	}
	titleText := model.title
	if model.subtitle != "" {
		titleText += ": " + truncateToVisualWidth(model.subtitle, max(innerWidth-20, 12))
	}
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(titleText)
	guidance, _ := model.guidanceSpec()
	return renderOverlayBox(
		content,
		innerWidth,
		title,
		renderGuidanceText(guidance, max(innerWidth-4, 1)),
		lipgloss.NewStyle().Foreground(ColorPrimary),
	)
}

func (m *AppModel) taskActionOperationDescriptors() []OperationDescriptor {
	manifest := taskOperationDescriptorManifest()
	descriptors := make([]OperationDescriptor, 0, len(manifest))
	for _, entry := range manifest {
		if entry.Core {
			continue
		}
		source, cardinality := OperationTargetCurrentTask, TargetCardinalityOne
		if entry.Bulk {
			source, cardinality = OperationTargetSelectedTasks, TargetCardinalityOneOrMore
		}
		id := entry.ID
		descriptors = append(descriptors, OperationDescriptor{
			ID:                   id,
			Label:                entry.Label,
			Aliases:              entry.Aliases,
			Category:             OperationCategoryTask,
			TargetSource:         source,
			TargetCardinality:    cardinality,
			RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
			Applicability: func(context CommandContext) bool {
				return m.taskActionOperationApplicable(id, context)
			},
			Risk:            entry.Risk,
			ValueProviderID: entry.Provider,
		})
	}
	return descriptors
}

func (m *AppModel) taskActionOperationApplicable(id OperationID, context CommandContext) bool {
	if !coreOutlinerApplicability(context) {
		return false
	}
	entry, _ := taskOperationManifestEntryFor(id)
	// Direct operations describe a stable grammar, not a menu projection. They
	// stay resolvable over a cursor that names nothing so the executor can report
	// the canonical missing-target error, but a cursor that positively names a
	// non-task entity belongs to that entity's grammar and must not bind a task
	// key. A visual selection still suppresses single-target operations instead
	// of silently falling back to the cursor.
	if entry.DirectOnly {
		if context.Selection.Visual {
			return entry.Bulk
		}
		return !commandContextNamesNonTaskEntity(context)
	}
	entities := []EntityContext{context.Entity}
	if context.Selection.Visual {
		selected := normalizedTaskIdentities(context.Selection.Tasks)
		if len(selected) == 0 || len(context.Selection.Entities) != len(selected) {
			return false
		}
		if !entry.Bulk {
			return false
		}
		entities = context.Selection.Entities
	}
	for _, entity := range entities {
		if entity.TaskID == "" || entity.ProjectID == "" {
			return false
		}
		if entity.Archived && entry.Risk != OperationRiskReadOnly {
			return false
		}
		if !m.taskStructureAllowsOperation(id, entity) {
			return false
		}
		if !m.taskAuxiliaryAllowsOperation(id, entity) {
			return false
		}
	}
	return true
}

// deferRefusalReason disables the whole defer section when any resolved task
// cannot be deferred. One reason covers the single terminal task and the bulk
// selection alike, and it leaves the due section reachable on the same rows the
// old defer-specific applicability check used to hide outright. It reads
// app.DeferralRefusesState so the picker refuses in the same words the mutation
// would, rather than enumerating the deferrable states a second time.
func deferRefusalReason(refs []TaskProjectRef) string {
	for _, ref := range refs {
		if ref.Task == nil {
			continue
		}
		if refusal := app.DeferralRefusesState(ref.Task.State()); refusal != "" {
			return refusal
		}
	}
	return ""
}

// taskStructureAllowsOperation hides destination-dependent actions that the
// domain would reject: an indent with no legal parent above or an outdent at root. A target
// that no longer resolves stays applicable so the handler reports the single
// authoritative "no longer available" error instead of the row vanishing.
func (m *AppModel) taskStructureAllowsOperation(id OperationID, entity EntityContext) bool {
	switch id {
	case operationTaskIndent, operationTaskOutdent:
	default:
		return true
	}
	task, project := m.taskAndProjectForIdentity(TaskIdentity{
		ProjectID: entity.ProjectID,
		TaskID:    entity.TaskID,
	})
	if task == nil || project == nil {
		return true
	}
	switch id {
	case operationTaskIndent:
		above := project.GetTaskAbove(task)
		if above == nil {
			return false
		}
		allowed, _ := task.CanIndent(above)
		return allowed
	case operationTaskOutdent:
		allowed, _ := task.CanOutdent()
		return allowed
	default:
		return true
	}
}

// pendingPickedValueProvider registers a picked provider with the executor
// without supplying a value, so the shell constructs the provider's ListSurface
// picker from the frozen request instead. It is the picker counterpart of
// pendingTextValueProvider.
func pendingPickedValueProvider(OperationInvocation) OperationValueProviderResult {
	return OperationValueProviderResult{}
}

func (m *AppModel) registerTaskActionOperations() error {
	if m.operationRegistry == nil || m.operationExecutor == nil {
		return fmt.Errorf("operation registry and executor are required")
	}
	executor := m.operationExecutor
	for _, provider := range []ValueProviderID{
		ValueProviderState,
		ValueProviderPriority,
		ValueProviderDate,
		ValueProviderDueDate,
		ValueProviderTags,
		ValueProviderParent,
		ValueProviderChild,
		ValueProviderProject,
	} {
		if err := executor.RegisterValueProvider(provider, pendingPickedValueProvider); err != nil {
			return err
		}
	}
	for _, descriptor := range m.taskActionOperationDescriptors() {
		if err := m.registerOperation(descriptor, OperationBinding{
			RequiredCapabilities: NewCapabilitySet(CapabilitySimplifiedInput),
			Applicability:        descriptor.Applicability,
			Handler:              m.executeTaskOperation,
			// Every handler below dispatches to an application-layer coordinator
			// or the existing atomic move message, which already owns commit and
			// rollback policy.
			DelegateTransaction: true,
			// The canonical delete request prompts before it mutates, so a generic
			// executor confirmation in front of it would ask the same question twice.
			DelegateConfirmation: descriptor.ID == operationTaskDelete,
		}); err != nil {
			return err
		}
	}
	return nil
}

// acceptsPresetValue reports whether a key binding may deliver value directly
// instead of opening the provider's picker. It mirrors the type assertions the
// task handlers below make, so a mistyped preset is rejected when the input
// strategy is registered rather than when the user presses the chord.
func (id ValueProviderID) acceptsPresetValue(value any) bool {
	switch id {
	case ValueProviderState:
		state, ok := value.(models.WorkflowState)
		return ok && state.IsValid()
	case ValueProviderPriority, ValueProviderFilterPriority:
		_, ok := value.(int)
		return ok
	case ValueProviderFilterDue:
		if _, ok := value.(filterDeferredOnly); ok {
			return true
		}
		_, ok := value.(DueDateFilter)
		return ok
	case ValueProviderFilterCompletionAge:
		_, ok := value.(CompletionAgeFilter)
		return ok
	case ValueProviderFilterStates, ValueProviderFilterTags, ValueProviderFilterProjectTags:
		_, ok := value.([]string)
		return ok
	case ValueProviderProjectTagDestination, ValueProviderProject:
		_, ok := value.(string)
		return ok
	case ValueProviderDate, ValueProviderDueDate:
		_, ok := value.(OperationDateValue)
		return ok
	case ValueProviderParent:
		_, ok := value.(OperationParentValue)
		return ok
	case ValueProviderChild:
		_, ok := value.([]string)
		return ok
	case ValueProviderTags:
		if _, ok := value.([]string); ok {
			return true
		}
		_, ok := value.(OperationTagsValue)
		return ok
	case ValueProviderNoteTitle, ValueProviderTagName:
		_, ok := value.(string)
		return ok
	default:
		return false
	}
}

func (m *AppModel) executeTaskOperation(
	invocation OperationInvocation,
) OperationHandlerResult {
	if len(invocation.Targets.Targets) == 0 {
		return OperationHandlerResult{Err: fmt.Errorf("operation %s requires at least one task", invocation.Descriptor.ID)}
	}
	if invocation.Descriptor.ValueProviderID != "" && !invocation.HasValue {
		return OperationHandlerResult{Err: fmt.Errorf("operation %s requires a value", invocation.Descriptor.ID)}
	}
	// Deletion takes every resolved target at once because one atomic
	// transaction removes single and multi-root selections alike.
	if invocation.Descriptor.ID == operationTaskDelete {
		return m.executeTaskDeleteOperation(invocation)
	}
	if invocation.Descriptor.ID == operationTaskTogglePin {
		return m.executeTaskTogglePinOperation(invocation)
	}
	if request, action, summary, handled, err := applicationTaskOperationRequest(invocation); handled {
		if err != nil {
			return OperationHandlerResult{Err: err}
		}
		capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
		return OperationHandlerResult{Message: OperationCommandResultMsg{
			Cmd: withWaitingCheckBackStep(invocation, withOutlinerRelocationCapture(
				m.secondaryFeatures.TaskItem.ExecuteOperation(request, action, summary),
				capture,
			)),
		}}
	}
	if len(invocation.Targets.Targets) > 1 {
		return m.executeBulkTaskValueOperation(invocation)
	}
	target := invocation.Targets.Targets[0]
	identity := target.Task
	if !identity.Valid() {
		return OperationHandlerResult{Err: fmt.Errorf("operation %s resolved an invalid task", invocation.Descriptor.ID)}
	}
	return m.executeSingleTaskOperation(invocation, identity)
}

// withWaitingCheckBackStep follows a committed move into Waiting with the
// due-only picker rather than the combined one `@` opens, because the step is
// asking for a check-back deadline and a defer section would answer a different
// question. The chain is keyed off the operation's own value rather than runtime
// context, and it is gated on the transition's own outcome: a rejected move
// returns its error untouched, so no picker opens and no due date can be
// committed on a task that never reached Waiting. On the success path the state
// transition has already committed when the picker opens, so Esc there cancels
// the date alone and the Waiting move stands.
func withWaitingCheckBackStep(invocation OperationInvocation, cmd tea.Cmd) tea.Cmd {
	if invocation.Descriptor.ID != operationTaskSetState {
		return cmd
	}
	if state, ok := invocation.Value.(models.WorkflowState); !ok || state != models.StateWaiting {
		return cmd
	}
	checkBack := OperationValueRequestMsg{
		Request: OperationExecutionRequest{
			ID:      operationTaskSetDueDate,
			Context: cloneCommandContext(invocation.Context),
		},
		Provider: ValueProviderDueDate,
	}
	return func() tea.Msg {
		msg := cmd()
		if !transactionCommitted(msg) {
			return msg
		}
		return tea.Sequence(
			func() tea.Msg { return msg },
			func() tea.Msg { return checkBack },
		)()
	}
}

// transactionCommitted reports whether a coordinator command resolved to a
// durable mutation. The two result types mirror withOutlinerRelocationCapture:
// a task operation lands on the batch path even for one target, while the
// single-task commit path still emits TransactionResultMsg.
func transactionCommitted(msg tea.Msg) bool {
	switch msg.(type) {
	case BatchTransactionResultMsg, TransactionResultMsg:
		return true
	default:
		return false
	}
}

// executeTaskTogglePinOperation applies one deterministic toggle to the whole
// frozen target set: if every target is pinned it unpins all of them, otherwise
// it pins all of them. The application operation validates and commits the
// project-qualified set atomically.
func (m *AppModel) executeTaskTogglePinOperation(
	invocation OperationInvocation,
) OperationHandlerResult {
	capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
	locators := make([]app.TaskLocator, 0, len(invocation.Targets.Targets))
	allPinned := true
	for _, target := range invocation.Targets.Targets {
		if !target.Task.Valid() {
			return OperationHandlerResult{Err: fmt.Errorf(
				"operation %s resolved an invalid task",
				invocation.Descriptor.ID,
			)}
		}
		task, _ := m.taskAndProjectForIdentity(target.Task)
		if task == nil {
			return OperationHandlerResult{Err: fmt.Errorf("task is no longer available")}
		}
		allPinned = allPinned && task.Pinned
		locators = append(locators, target.Task.Locator())
	}
	pinned := !allPinned
	action, verb := "Pin Task", "Pinned"
	if !pinned {
		action, verb = "Unpin Task", "Unpinned"
	}
	return OperationHandlerResult{Message: OperationCommandResultMsg{
		Cmd: withOutlinerRelocationCapture(m.secondaryFeatures.TaskItem.ExecuteOperation(
			app.TaskOperationRequest{
				Operation: app.TaskOperationSetPinned,
				Targets:   locators,
				Pinned:    pinned,
			},
			action,
			fmt.Sprintf("%s %d task(s)", verb, len(locators)),
		), capture),
	}}
}

func applicationTaskOperationRequest(
	invocation OperationInvocation,
) (app.TaskOperationRequest, string, string, bool, error) {
	locators := make([]app.TaskLocator, 0, len(invocation.Targets.Targets))
	for _, target := range invocation.Targets.Targets {
		if !target.Task.Valid() {
			return app.TaskOperationRequest{}, "", "", true, fmt.Errorf(
				"operation %s resolved an invalid task",
				invocation.Descriptor.ID,
			)
		}
		locators = append(locators, target.Task.Locator())
	}
	request := app.TaskOperationRequest{Targets: locators}
	count := len(locators)

	switch invocation.Descriptor.ID {
	case operationTaskSetState:
		state, ok := invocation.Value.(models.WorkflowState)
		if !ok || !state.IsValid() {
			return request, "", "", true, fmt.Errorf("invalid state value")
		}
		request.Operation = app.TaskOperationSetState
		request.State = state
		return request, "Change State",
			fmt.Sprintf("Change State · Changed %d task(s) to %s", count, state), true, nil

	case operationTaskSetPriority:
		priority, ok := invocation.Value.(int)
		if !ok || priority < 0 || priority > models.PriorityMax {
			return request, "", "", true, fmt.Errorf("invalid priority value")
		}
		request.Operation = app.TaskOperationSetPriority
		request.Priority = priority
		return request, "Set Priority",
			fmt.Sprintf("Set Priority · Set priority on %d task(s)", count), true, nil

	case operationTaskSetDueDate:
		value, ok := invocation.Value.(OperationDateValue)
		if !ok {
			return request, "", "", true, fmt.Errorf("invalid date value")
		}
		request.Date = cloneOperationDate(value.Date)
		if value.Deferred {
			request.Operation = app.TaskOperationSetDefer
			verb := "Deferred"
			if value.Date == nil {
				verb = "Cleared defer on"
			}
			return request, "Set Defer",
				fmt.Sprintf("Set Defer · %s %d task(s)", verb, count), true, nil
		}
		request.Operation = app.TaskOperationSetDueDate
		request.PlanForToday = dueDateIsToday(value.Date)
		verb := "Set due date on"
		if value.Date == nil {
			verb = "Cleared due date on"
		}
		return request, "Set Due Date",
			fmt.Sprintf("Set Due Date · %s %d task(s)", verb, count), true, nil

	case operationTaskSetTags:
		switch value := invocation.Value.(type) {
		case []string:
			request.Tags = append([]string(nil), value...)
		case OperationTagsValue:
			request.TagPatch = true
			request.TagsToAdd = append([]string(nil), value.Add...)
			request.TagsToRemove = append([]string(nil), value.Remove...)
			request.ContextSet = value.ContextSet
			request.Context = value.Context
		default:
			return request, "", "", true, fmt.Errorf("invalid tag value")
		}
		request.Operation = app.TaskOperationSetTags
		if request.ContextSet {
			verb := "Set context on"
			if request.Context == "" {
				verb = "Cleared context on"
			}
			return request, "Set Tags & Context",
				fmt.Sprintf("Set Tags & Context · %s %d task(s)", verb, count), true, nil
		}
		return request, "Set Tags",
			fmt.Sprintf("Set Tags · Tagged %d task(s)", count), true, nil

	case operationTaskPin, operationTaskUnpin:
		request.Operation = app.TaskOperationSetPinned
		request.Pinned = invocation.Descriptor.ID == operationTaskPin
		action := "Unpin Task"
		verb := "Unpinned"
		if request.Pinned {
			action = "Pin Task"
			verb = "Pinned"
		}
		return request, action, fmt.Sprintf("%s %d task(s)", verb, count), true, nil

	case operationTaskCopyTitle:
		request.Operation = app.TaskOperationCopyTitle
		return request, "Copy Title", "", true, nil

	case operationTaskCopyDetails:
		request.Operation = app.TaskOperationCopyDescription
		return request, "Copy Description", "", true, nil

	case operationTaskCopyTitleDescription:
		request.Operation = app.TaskOperationCopyTitleDescription
		return request, "Copy Title and Description", "", true, nil

	default:
		return app.TaskOperationRequest{}, "", "", false, nil
	}
}

func (m *AppModel) executeSingleTaskOperation(
	invocation OperationInvocation,
	identity TaskIdentity,
) OperationHandlerResult {
	cmdResult := func(cmd tea.Cmd) OperationHandlerResult {
		return OperationHandlerResult{Message: OperationCommandResultMsg{Cmd: cmd}}
	}
	relocationResult := func(cmd tea.Cmd) OperationHandlerResult {
		capture := m.captureOutlinerRelocationForTargets(invocation.Targets)
		return cmdResult(withOutlinerRelocationCapture(cmd, capture))
	}

	switch invocation.Descriptor.ID {
	case operationTaskSetState:
		state, ok := invocation.Value.(models.WorkflowState)
		if !ok || !state.IsValid() {
			return OperationHandlerResult{Err: fmt.Errorf("invalid state value")}
		}
		return cmdResult(m.secondaryFeatures.TaskItem.Transition(identity, state))
	case operationTaskSetPriority:
		priority, ok := invocation.Value.(int)
		if !ok || priority < 0 || priority > models.PriorityMax {
			return OperationHandlerResult{Err: fmt.Errorf("invalid priority value")}
		}
		return cmdResult(m.secondaryFeatures.TaskItem.SetPriority(identity, priority))
	case operationTaskSetDueDate:
		value, ok := invocation.Value.(OperationDateValue)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid date value")}
		}
		if value.Deferred {
			return cmdResult(m.secondaryFeatures.TaskItem.Defer(identity, cloneOperationDate(value.Date)))
		}
		return cmdResult(m.secondaryFeatures.TaskItem.SetDueDate(identity, cloneOperationDate(value.Date)))
	case operationTaskSetTags:
		tags, ok := invocation.Value.([]string)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid tag value")}
		}
		return cmdResult(m.secondaryFeatures.Bulk.SetTagsLocators(
			[]app.TaskLocator{identity.Locator()},
			tags,
		))
	case operationTaskSetParent:
		value, ok := invocation.Value.(OperationParentValue)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid parent value")}
		}
		return relocationResult(m.secondaryFeatures.TaskItem.Reparent(identity, value.ParentID))
	case operationTaskAdoptChildren:
		children, ok := invocation.Value.([]string)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid child value")}
		}
		return cmdResult(m.adoptChildrenCmd(
			identity,
			children,
			m.captureOutlinerRelocationForTargets(invocation.Targets),
		))
	case operationTaskMoveToProject:
		projectID, ok := invocation.Value.(string)
		if !ok || strings.TrimSpace(projectID) == "" {
			return OperationHandlerResult{Err: fmt.Errorf("invalid project value")}
		}
		return cmdResult(m.moveTaskToProjectCmd(
			identity,
			projectID,
			m.captureOutlinerRelocationForTargets(invocation.Targets),
		))
	case operationTaskIndent:
		return relocationResult(m.secondaryFeatures.TaskItem.Indent(identity))
	case operationTaskOutdent:
		return relocationResult(m.secondaryFeatures.TaskItem.Outdent(identity))
	default:
		return m.executeAuxiliaryTaskOperation(invocation, identity)
	}
}

// adoptChildrenCmd routes the picker's checked set through the same task-line
// edit the `<child` sigil uses, so adoption is validated as one hierarchy patch
// and lands in one transaction with one undo entry. The title is carried
// verbatim and never reparsed, and the request names no parent patch, so the
// adopting task keeps its own place in the tree.
func (m *AppModel) adoptChildrenCmd(
	identity TaskIdentity,
	childIDs []string,
	relocation *outlinerRelocationCapture,
) tea.Cmd {
	task, project := m.taskAndProjectForIdentity(identity)
	if task == nil || project == nil {
		return func() tea.Msg {
			return TaskLineEditFailedMsg{Source: identity, Err: fmt.Errorf("selected task is no longer available")}
		}
	}
	req := app.TaskLineEditRequest{
		Source:               identity.Locator(),
		Title:                task.Title,
		DestinationProjectID: identity.ProjectID,
		Parent:               app.TaskLineParentPatch{Mode: app.TaskLineParentPreserve},
		Action:               "Adopt Children",
		PlanningArtifacts:    shellPlanningReconcileArtifactBuilder(m.runtime, m.configManager),
	}
	for _, childID := range childIDs {
		req.Children = append(req.Children, app.TaskLocator{
			ProjectID: identity.ProjectID,
			TaskID:    childID,
		})
	}
	return m.secondaryFeatures.TaskItem.EditTaskLine(req, relocation)
}

// moveTaskToProjectCmd routes the picked project through the same task-line edit
// the `%project` sigil uses, so the subtree moves under one transaction that
// reassigns destination ShortIDs and reconciles planning membership. The title is
// carried verbatim so the edit never reads as a rename.
func (m *AppModel) moveTaskToProjectCmd(
	identity TaskIdentity,
	projectID string,
	relocation *outlinerRelocationCapture,
) tea.Cmd {
	task, project := m.taskAndProjectForIdentity(identity)
	if task == nil || project == nil {
		return func() tea.Msg {
			return TaskLineEditFailedMsg{Source: identity, Err: fmt.Errorf("selected task is no longer available")}
		}
	}
	req := app.TaskLineEditRequest{
		Source:               identity.Locator(),
		Title:                task.Title,
		DestinationProjectID: projectID,
		Action:               "Move to Project",
		PlanningArtifacts:    shellStructuralPlanningArtifactBuilder(m.runtime, m.configManager),
	}
	return m.secondaryFeatures.TaskItem.EditTaskLine(req, relocation)
}

func (m *AppModel) executeBulkTaskValueOperation(
	invocation OperationInvocation,
) OperationHandlerResult {
	locators := make([]app.TaskLocator, 0, len(invocation.Targets.Targets))
	for _, target := range invocation.Targets.Targets {
		if !target.Task.Valid() {
			return OperationHandlerResult{Err: fmt.Errorf("operation %s resolved an invalid task", invocation.Descriptor.ID)}
		}
		locators = append(locators, target.Task.Locator())
	}
	cmdResult := func(cmd tea.Cmd) OperationHandlerResult {
		return OperationHandlerResult{Message: OperationCommandResultMsg{Cmd: cmd}}
	}
	count := len(locators)

	switch invocation.Descriptor.ID {
	case operationTaskSetState:
		state, ok := invocation.Value.(models.WorkflowState)
		if !ok || !state.IsValid() {
			return OperationHandlerResult{Err: fmt.Errorf("invalid state value")}
		}
		return cmdResult(m.secondaryFeatures.Bulk.TransitionLocators(
			locators,
			state,
			func(changed int) string {
				return fmt.Sprintf("Changed %d task(s) to %s", changed, state)
			},
		))
	case operationTaskSetPriority:
		priority, ok := invocation.Value.(int)
		if !ok || priority < 0 || priority > models.PriorityMax {
			return OperationHandlerResult{Err: fmt.Errorf("invalid priority value")}
		}
		return cmdResult(m.secondaryFeatures.Bulk.SetPriorityLocators(locators, priority))
	case operationTaskSetDueDate:
		value, ok := invocation.Value.(OperationDateValue)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid date value")}
		}
		if value.Deferred {
			return cmdResult(m.secondaryFeatures.Bulk.DeferLocators(locators, cloneOperationDate(value.Date)))
		}
		if value.Date == nil {
			return cmdResult(m.secondaryFeatures.Bulk.ClearDueDateLocators(locators))
		}
		return cmdResult(m.secondaryFeatures.Bulk.SetDueDateLocators(
			locators,
			*cloneOperationDate(value.Date),
		))
	case operationTaskSetTags:
		tags, ok := invocation.Value.([]string)
		if !ok {
			return OperationHandlerResult{Err: fmt.Errorf("invalid tag value")}
		}
		return cmdResult(m.secondaryFeatures.Bulk.SetTagsLocators(locators, tags))
	default:
		return OperationHandlerResult{
			Err: fmt.Errorf("operation %s is not available for %d selected tasks", invocation.Descriptor.ID, count),
		}
	}
}

func cloneOperationDate(date *time.Time) *time.Time {
	if date == nil {
		return nil
	}
	cloned := *date
	return &cloned
}

func (m *AppModel) taskAndProjectForIdentity(identity TaskIdentity) (*models.Task, *models.Project) {
	if !identity.Valid() {
		return nil, nil
	}
	project := m.projectByID(identity.ProjectID)
	if project == nil {
		return nil, nil
	}
	return project.GetTaskByID(identity.TaskID), project
}

//nolint:gocyclo // One factory exhaustively maps typed operation values to their picker contracts.
func (m *AppModel) newOperationValuePicker(
	request OperationExecutionRequest,
	provider ValueProviderID,
) (*OperationValuePickerModel, error) {
	// Filter pickers refine the whole view, so they must open without any task
	// under the cursor and never resolve task targets.
	switch provider {
	case ValueProviderFilterStates:
		return m.newFilterStatesPicker(request)
	case ValueProviderFilterTags:
		return m.newFilterTagsPicker(request)
	case ValueProviderFilterDue:
		return m.newFilterDuePicker(request)
	case ValueProviderFilterCompletionAge:
		return m.newFilterCompletionAgePicker(request)
	case ValueProviderFilterHierarchy:
		return m.newFilterHierarchyPicker(request)
	case ValueProviderFilterProjectTags:
		return m.newFilterProjectTagsPicker(request)
	case ValueProviderProjectTagDestination:
		return m.newProjectTagDestinationPicker(request)
	case ValueProviderProjectTags:
		// A project tag edit resolves a project, not a task, so it must open from
		// a project header where no task ref exists.
		return m.newProjectTagsPicker(request)
	case ValueProviderLensTemplate:
		// The built-in view catalog is global, so the gallery opens from the lens
		// picker where there is no task under the cursor at all.
		return m.newLensTemplatePicker(request)
	default:
	}
	refs, err := m.operationTaskRefs(request.Context)
	if err != nil {
		return nil, err
	}
	task := refs[0].Task
	subtitle := task.Title
	if len(refs) > 1 || request.Context.Selection.Visual {
		subtitle = fmt.Sprintf("%d selected task(s)", len(refs))
	}

	switch provider {
	case ValueProviderState:
		states := make([]models.WorkflowState, 0, len(refs))
		for _, ref := range refs {
			states = append(states, ref.Task.State())
		}
		return NewTaskStatePickerForTargets(request, states, subtitle)
	case ValueProviderPriority:
		priorities := []PriorityInfo{
			{Priority: 0, Description: "Unset"},
			{Priority: 1, Description: "High"},
			{Priority: 2, Description: "Medium"},
			{Priority: 3, Description: "Low"},
		}
		selected := []ListItemID(nil)
		if priority, ok := commonTaskPriority(refs); ok {
			selected = []ListItemID{ListItemID("priority:" + strconv.Itoa(priority))}
		}
		return newOperationValuePickerModel(
			request,
			provider,
			"Set Priority",
			subtitle,
			ListSourceFromPriorities(priorities),
			false,
			selected,
		)
	case ValueProviderDate:
		selected := []ListItemID(nil)
		due, dueShared := commonTaskDate(refs, false)
		deferred, deferShared := commonTaskDate(refs, true)
		if dueShared && deferShared {
			selected = operationDateSelectedID(due, deferred)
		}
		return newOperationValuePickerModel(
			request,
			provider,
			"Set Due or Defer",
			subtitle,
			operationDateListSource(deferRefusalReason(refs)),
			false,
			selected,
		)
	case ValueProviderDueDate:
		selected := []ListItemID(nil)
		if date, ok := commonTaskDate(refs, false); ok {
			selected = dueDateSelectedID(date)
		}
		return newOperationValuePickerModel(
			request,
			provider,
			"Set Due Date",
			subtitle,
			operationDueDateListSource(),
			false,
			selected,
		)
	case ValueProviderParent:
		return newOperationValuePickerModel(
			request,
			provider,
			"Set Parent",
			subtitle,
			operationParentListSource(refs[0].Task, refs[0].Project),
			false,
			parentSelectedID(refs[0].Task),
		)
	case ValueProviderChild:
		// Nothing is preselected: the checked set is the adoption, so a preselected
		// row would make unchecking it read as a detach the `<child` sigil cannot
		// express.
		return newOperationValuePickerModel(
			request,
			provider,
			"Adopt Children",
			subtitle+" · adds children, never detaches",
			operationChildListSource(refs[0].Task, refs[0].Project),
			true,
			nil,
		)
	case ValueProviderProject:
		return newOperationValuePickerModel(
			request,
			provider,
			"Move to Project",
			subtitle,
			operationProjectListSource(m.runtime.AvailableProjects(), refs[0].Project.ID),
			false,
			nil,
		)
	case ValueProviderTags:
		projects := make([]*models.Project, 0, len(refs))
		for _, ref := range refs {
			projects = append(projects, ref.Project)
		}
		tagList := BuildProjectsTagList(projects, false)
		source := taskTagListSource(tagList, true)
		// The context rows lead the CONTEXTS section, ahead of the tag rows the
		// source recategorized there by context usage.
		source.Items = append(
			taskContextListItems(collectContextNames(projects)), source.Items...,
		)
		picker, err := newOperationValuePickerModel(
			request, provider, "Set Tags & Context", subtitle, source, true, nil,
		)
		if err != nil {
			return nil, err
		}
		// Membership is configured at every cardinality, so the picker always
		// emits the patch payload the context field rides on. For one task the
		// patch and a replacement describe the same edit.
		byID := make(map[ListItemID]operationTagMembership, len(source.Items))
		for _, item := range source.Items {
			byID[item.ID] = operationTagNone
		}
		for tag, state := range taskTagMembership(refs) {
			id := ListItemID("tag:" + tag)
			if _, ok := byID[id]; ok {
				byID[id] = state
			}
		}
		contextInitial, contextStates := taskContextMembership(refs)
		for id, state := range contextStates {
			if _, ok := byID[id]; ok {
				byID[id] = state
			}
		}
		picker.contextInitial = contextInitial
		if err := picker.configureTagMembership(byID); err != nil {
			return nil, err
		}
		return picker, nil
	default:
		return nil, fmt.Errorf("unknown operation value provider %q", provider)
	}
}

func (m *AppModel) operationTaskRefs(context CommandContext) ([]TaskProjectRef, error) {
	identities := normalizedTaskIdentities(context.Selection.Tasks)
	if len(identities) == 0 {
		if context.Selection.Visual {
			return nil, fmt.Errorf("select at least one task")
		}
		if current := currentTaskIdentity(context); current.Valid() {
			identities = []TaskIdentity{current}
		}
	}
	refs := make([]TaskProjectRef, 0, len(identities))
	for _, identity := range identities {
		task, project := m.taskAndProjectForIdentity(identity)
		if task == nil || project == nil {
			return nil, fmt.Errorf("selected task is no longer available")
		}
		refs = append(refs, TaskProjectRef{Task: task, Project: project})
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("select a task")
	}
	return refs, nil
}

func commonTaskPriority(refs []TaskProjectRef) (int, bool) {
	if len(refs) == 0 || refs[0].Task == nil {
		return 0, false
	}
	priority := refs[0].Task.Priority
	for _, ref := range refs[1:] {
		if ref.Task == nil || ref.Task.Priority != priority {
			return 0, false
		}
	}
	return priority, true
}

func commonTaskDate(refs []TaskProjectRef, deferred bool) (*time.Time, bool) {
	if len(refs) == 0 || refs[0].Task == nil {
		return nil, false
	}
	dateFor := func(task *models.Task) *time.Time {
		if deferred {
			return task.DeferredUntil
		}
		return task.DueDate
	}
	first := dateFor(refs[0].Task)
	for _, ref := range refs[1:] {
		if ref.Task == nil || !sameOperationDate(first, dateFor(ref.Task)) {
			return nil, false
		}
	}
	return cloneOperationDate(first), true
}

func sameOperationDate(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func taskTagMembership(refs []TaskProjectRef) map[string]operationTagMembership {
	counts := make(map[string]int)
	for _, ref := range refs {
		if ref.Task == nil {
			continue
		}
		for _, tag := range models.DedupeTags(ref.Task.Tags) {
			counts[tag]++
		}
	}
	membership := make(map[string]operationTagMembership, len(counts))
	for tag, count := range counts {
		if count == len(refs) {
			membership[tag] = operationTagAll
		} else if count > 0 {
			membership[tag] = operationTagSome
		}
	}
	return membership
}

// collectContextNames unions the context vocabulary of every project a target
// lives in, so a cross-project selection offers every name it could already
// carry.
func collectContextNames(projects []*models.Project) []string {
	seen := make(map[string]bool)
	for _, project := range projects {
		for _, name := range CollectContextNamesForProject(project) {
			seen[name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// taskContextMembership reports the context row every target already agrees on
// and the per-row state the checkboxes render. Disagreement leaves the agreed
// row empty, so any chosen row reads as a change.
func taskContextMembership(refs []TaskProjectRef) (ListItemID, map[ListItemID]operationTagMembership) {
	counts := make(map[string]int)
	for _, ref := range refs {
		if ref.Task == nil {
			continue
		}
		counts[ref.Task.Context]++
	}
	states := make(map[ListItemID]operationTagMembership, len(counts))
	agreed := ListItemID("")
	for name, count := range counts {
		id := taskContextRowID(name)
		if count == len(refs) {
			states[id] = operationTagAll
			agreed = id
			continue
		}
		states[id] = operationTagSome
	}
	return agreed, states
}

// datePreset is one row of a date picker's vocabulary. The list the picker
// renders and the resolver that decides which row is highlighted both read
// this table, so the two cannot describe different presets.
type datePreset struct {
	id, label string
	clear     bool
	value     func() time.Time
}

// operationDatePresets returns the preset rows for one date picker. A due date
// is a calendar day, so every one of its rows lands on local midnight; a
// deferral is an instant, so it also offers hour offsets and a morning row.
func operationDatePresets(deferred bool, now time.Time) []datePreset {
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

// operationDateSecondaryText names an instant the way a picker row and a search
// alias both need it. A bare clock is never enough: "+8h" at 20:00 lands
// tomorrow, and a lone "04:00" would silently claim today.
func operationDateSecondaryText(value time.Time) string {
	if value.Equal(util.StartOfDay(value)) {
		return value.Format("Mon Jan 2")
	}
	return value.Format("Mon Jan 2 15:04")
}

// The two date sections share preset names, so a row's identity carries its
// field. Both sections are always built from the same namespace, including in
// the due-only projection, so one resolver answers either list.
const (
	dateListCategoryDue   ListCategoryID = "date-due"
	dateListCategoryDefer ListCategoryID = "date-defer"
)

func dateListItemID(deferred bool, presetID string) ListItemID {
	if deferred {
		return ListItemID("date:defer:" + presetID)
	}
	return ListItemID("date:due:" + presetID)
}

func operationDateItems(deferred bool, categoryID ListCategoryID, disabledReason string) []ListItem {
	presets := operationDatePresets(deferred, time.Now())
	items := make([]ListItem, 0, len(presets))
	for _, preset := range presets {
		var date *time.Time
		secondary := "remove date"
		if !preset.clear {
			value := preset.value()
			date = &value
			secondary = operationDateSecondaryText(value)
		}
		items = append(items, ListItem{
			ID:             dateListItemID(deferred, preset.id),
			CategoryID:     categoryID,
			PrimaryText:    preset.label,
			SecondaryText:  secondary,
			Aliases:        []string{preset.id, secondary},
			DisabledReason: disabledReason,
			Data:           OperationDateValue{Date: date, Deferred: deferred},
		})
	}
	return items
}

// operationDateListSource is the one picker `@` opens. Due rows come first
// because a deadline is the commoner answer, and deferReason disables the whole
// defer section when a resolved task cannot be deferred, which leaves due
// reachable on the same terminal task.
func operationDateListSource(deferReason string) ListSource {
	items := operationDateItems(false, dateListCategoryDue, "")
	items = append(items, operationDateItems(true, dateListCategoryDefer, deferReason)...)
	return ListSource{
		Items: items,
		Categories: []ListCategory{
			{ID: dateListCategoryDue, Label: "Due"},
			{ID: dateListCategoryDefer, Label: "Defer"},
		},
	}
}

// operationDueDateListSource is the due-only projection. The waiting check-back
// chain is the one caller: that step asks for a deadline specifically, so a
// defer section there would offer an answer to a question nobody asked.
func operationDueDateListSource() ListSource {
	return ListSource{Items: operationDateItems(false, "", "")}
}

// parentListNoneID is the row expressing the `>0` grammar: no parent, task at
// the project root.
const parentListNoneID ListItemID = "parent:none"

// operationParentListSource offers every structural candidate in the task's own
// project, carrying the refusal reason on the row rather than behind Enter, so
// a cycle, self-reference, or depth-six overflow reads as a disabled row.
// Children resolve only in the effective project, exactly as `>parent` does.
func operationParentListSource(task *models.Task, project *models.Project) ListSource {
	candidates := structuralCandidates(project)
	items := make([]ListItem, 0, len(candidates)+1)
	items = append(items, ListItem{
		ID:            parentListNoneID,
		PrimaryText:   "No parent (project root)",
		SecondaryText: ">0",
		Aliases:       []string{"none", "root", "0", ">0", "detach"},
		Data:          OperationParentValue{},
	})
	for _, candidate := range candidates {
		id := candidate.ID
		shortID := "#" + strconv.Itoa(candidate.ShortID)
		items = append(items, ListItem{
			ID:             ListItemID("parent:" + id),
			PrimaryText:    candidate.Title,
			SecondaryText:  shortID,
			Aliases:        []string{shortID, strconv.Itoa(candidate.ShortID), candidate.Title},
			DisabledReason: invalidQuickEditParent(task, project, project, candidate),
			Data:           OperationParentValue{ParentID: &id},
		})
	}
	return ListSource{Items: items}
}

// operationChildListSource offers every structural candidate in the task's own
// project that the task does not already parent, exactly the population
// `<child` resolves against. Omitting the existing children is what keeps the
// picker additive: with no row for a child the task already has, no selection
// state can ask for a detach, which `<0` is not.
func operationChildListSource(task *models.Task, project *models.Project) ListSource {
	candidates := structuralCandidates(project)
	items := make([]ListItem, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ID == task.ID {
			continue
		}
		if candidate.ParentID != nil && *candidate.ParentID == task.ID {
			continue
		}
		id := candidate.ID
		shortID := "#" + strconv.Itoa(candidate.ShortID)
		items = append(items, ListItem{
			ID:             ListItemID("child:" + id),
			PrimaryText:    candidate.Title,
			SecondaryText:  shortID,
			Aliases:        []string{shortID, strconv.Itoa(candidate.ShortID), candidate.Title},
			DisabledReason: invalidQuickEditParent(candidate, project, project, task),
			Data:           id,
		})
	}
	return ListSource{Items: items}
}

// moveProjectPickerItem adapts one destination project for the Move to Project
// list.
type moveProjectPickerItem struct {
	project *models.Project
}

func (item moveProjectPickerItem) FilterText() string {
	return item.project.Name + " " + item.project.Slug
}

func (item moveProjectPickerItem) DisplayText() string {
	return item.project.Name
}

func (item moveProjectPickerItem) CountValue() int {
	return item.project.CountTasks()
}

func (item moveProjectPickerItem) ItemID() string {
	return "project:" + item.project.ID
}

func (item moveProjectPickerItem) IsPinned() bool {
	return item.project.Pinned
}

// operationProjectListSource offers every available project except the task's
// own, since a move to the current owner is not a move. The population is
// AvailableProjects rather than the Outliner's TaskWorkspaceProjects, so a
// parked project stays a destination exactly as it does for `%project`.
func operationProjectListSource(projects []*models.Project, currentProjectID string) ListSource {
	items := make([]moveProjectPickerItem, 0, len(projects))
	for _, project := range projects {
		if project == nil || project.ID == currentProjectID {
			continue
		}
		items = append(items, moveProjectPickerItem{project: project})
	}
	source := ListSourceFromPickerItems(items, false)
	for i := range source.Items {
		project := items[i].project
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
	if chips := projectTagChips(project.Tags); chips != "" {
		secondary += " · " + chips
	}
	return secondary
}

func parentSelectedID(task *models.Task) []ListItemID {
	if task == nil || task.ParentID == nil {
		return []ListItemID{parentListNoneID}
	}
	return []ListItemID{ListItemID("parent:" + *task.ParentID)}
}

func dueDateSelectedID(date *time.Time) []ListItemID {
	if date == nil {
		return []ListItemID{dateListItemID(false, "clear")}
	}
	return relativeDateSelectedID(date, false)
}

// operationDateSelectedID highlights the row the task's own values name. Due
// wins when both fields are set, because the cursor can only sit in one section
// and the due section is the one the list opens on.
func operationDateSelectedID(due, deferred *time.Time) []ListItemID {
	if due != nil {
		return relativeDateSelectedID(due, false)
	}
	if deferred != nil {
		return relativeDateSelectedID(deferred, true)
	}
	return []ListItemID{dateListItemID(false, "clear")}
}

// relativeDateSelectedID highlights a preset only when the stored value is
// reproducible from now alone, so the hour rows never match: four hours from
// when some earlier picker opened is a different fact than four hours from now,
// and a tolerance window would conflate the two.
func relativeDateSelectedID(date *time.Time, deferred bool) []ListItemID {
	for _, preset := range operationDatePresets(deferred, time.Now()) {
		if preset.clear {
			continue
		}
		if preset.value().Equal(*date) {
			return []ListItemID{dateListItemID(deferred, preset.id)}
		}
	}
	return nil
}

func (m *AppModel) executeOperationOverlayRequest(
	request OperationExecutionRequest,
) tea.Cmd {
	if m.operationExecutor == nil {
		m.err = fmt.Errorf("operation registry is unavailable")
		return statusTimeout(5 * time.Second)
	}
	result := m.operationExecutor.Execute(request)
	if result.Err != nil {
		m.err = result.Err
		return statusTimeout(5 * time.Second)
	}
	if result.ValueRequired != nil {
		return m.openOperationValueStep(*result.ValueRequired)
	}
	return operationResultCmd(result)
}

// openOperationValueStep opens the value overlay one provider needs: a typed
// prompt for text values, the ListSurface picker for everything else.
func (m *AppModel) openOperationValueStep(request OperationValueRequestMsg) tea.Cmd {
	if request.Provider.textEntry() {
		prompt, err := m.newOperationTextPrompt(request.Request, request.Provider)
		if err != nil {
			m.err = err
			return statusTimeout(5 * time.Second)
		}
		m.operationTextPrompt = prompt
		return prompt.initCmd()
	}
	picker, err := m.newOperationValuePicker(request.Request, request.Provider)
	if err != nil {
		m.err = err
		return statusTimeout(5 * time.Second)
	}
	m.operationValuePicker = picker
	return nil
}

func (m *AppModel) clearOperationValueStep() {
	m.operationValuePicker = nil
	m.operationTextPrompt = nil
}

// operationResultCmd converts a completed execution outcome into the Bubble Tea
// command seam.
func operationResultCmd(result OperationExecutionResult) tea.Cmd {
	switch typed := result.AsMessage().(type) {
	case nil:
		return nil
	case OperationCommandResultMsg:
		return typed.Cmd
	default:
		return func() tea.Msg { return typed }
	}
}

func (m *AppModel) handleOperationOverlayMessage(msg tea.Msg) (bool, tea.Cmd) {
	switch typed := msg.(type) {
	case ContextActionSheetResultMsg:
		return true, m.applyContextActionSheetResult(typed)
	case ContextActionSheetClosedMsg:
		m.contextActionSheet = nil
		return true, nil
	case NamedLensPickerClosedMsg:
		m.lensPicker = nil
		return true, nil
	case NamedLensPickerResultMsg:
		return true, m.applyNamedLensPickerResult(typed)
	case NamedLensActionsRequestMsg:
		return true, m.openNamedLensActions(typed)
	case OperationConfirmationRequestMsg:
		m.showOperationConfirmation(typed)
		return true, nil
	case OperationValueRequestMsg:
		return true, m.openOperationValueStep(typed)
	case OperationValueSelectedMsg:
		m.clearOperationValueStep()
		request := typed.Request
		request.Value = typed.Value
		request.HasValue = true
		return true, m.resumeContextActionRequest(request)
	case OperationValueCanceledMsg:
		m.clearOperationValueStep()
		return true, nil
	case ProjectTagVocabularyActionsMsg:
		m.clearOperationValueStep()
		return true, m.openProjectTagVocabularyActions(typed.Tag)
	default:
		return false, nil
	}
}

// applyContextActionSheetResult applies the nested step or completion produced
// by one centralized sheet execution. A failure is already retained on the
// sheet, a pending value or confirmation opens beneath it, and only a completed
// action reports the host lifecycle.
func (m *AppModel) applyContextActionSheetResult(msg ContextActionSheetResultMsg) tea.Cmd {
	switch {
	case msg.Result.Err != nil:
		return nil
	case msg.Result.ValueRequired != nil:
		return m.openOperationValueStep(*msg.Result.ValueRequired)
	case msg.Result.ConfirmationRequired != nil:
		m.showOperationConfirmation(*msg.Result.ConfirmationRequired)
		return nil
	}
	return operationResultCmd(msg.Result)
}

// resumeContextActionRequest re-enters the executor after a nested value or
// confirmation step. An open sheet keeps ownership of the outcome so a failure
// stays visible on it and only a completed action dismisses it.
func (m *AppModel) resumeContextActionRequest(request OperationExecutionRequest) tea.Cmd {
	sheet := m.contextActionSheet
	if sheet == nil {
		// The lens picker is the one surface an operation can be raised from
		// without a sheet between, so it owns the resumed outcome the same way:
		// a refusal stays on the list, and only completion tears it down. Both of
		// its value-taking action rows resume here — the name prompt behind Save
		// and the template gallery behind Add built-in view — so a refused add is
		// retained on the picker exactly as a refused save is.
		if picker := m.lensPicker; picker != nil {
			result, dismissed := picker.Execute(request)
			if dismissed {
				m.closeNamedLensOverlays()
			}
			return m.applyNamedLensPickerResult(NamedLensPickerResultMsg{Result: result})
		}
		return m.executeOperationOverlayRequest(request)
	}
	result, dismissed := sheet.Execute(request)
	if dismissed {
		m.contextActionSheet = nil
		// The sheet was opened over the picker, which stays mounted: the row it
		// acted on may have been renamed, rewritten, or removed underneath.
		m.refreshNamedLensPicker()
	}
	return m.applyContextActionSheetResult(ContextActionSheetResultMsg{
		Result: result,
	})
}

const operationConfirmAction = "contextual-operation"

// showOperationConfirmation prompts for a destructive contextual action and
// freezes the executor-supplied request, which already carries Confirmed.
func (m *AppModel) showOperationConfirmation(request OperationConfirmationRequestMsg) {
	m.pendingOperationRequest = request.Request
	prompt := request.Label + "?"
	if count := len(request.Targets.Entities()); count > 1 {
		prompt = fmt.Sprintf("%s · %d targets?", request.Label, count)
	}
	m.operationConfirm.Show(operationConfirmAction, prompt)
}

// handleOperationConfirmKey gives the contextual destructive confirmation modal
// key priority: it consumes every key while active so nothing leaks into the
// sheet beneath, and cancellation leaves that sheet exactly as it was.
func (m *AppModel) handleOperationConfirmKey(key string) (bool, tea.Cmd) {
	if !m.operationConfirm.IsActive() {
		return false, nil
	}
	confirmed, handled := m.operationConfirm.HandleKey(key)
	if !handled {
		return false, nil
	}
	if confirmed {
		request := m.pendingOperationRequest
		m.pendingOperationRequest = OperationExecutionRequest{}
		return true, m.resumeContextActionRequest(request)
	}
	switch key {
	case "n", "N", "esc":
		m.pendingOperationRequest = OperationExecutionRequest{}
	}
	return true, nil
}
