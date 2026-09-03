package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/theme"
)

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
	return status.StatusMsg(fmt.Sprintf("No %s rows", strings.ToUpper(label)))
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
	if model.membershipCurrent == nil {
		return nil
	}
	for id := range model.membershipCurrent {
		if isTaskContextRowID(id) {
			model.membershipCurrent[id] = operationValueNone
		}
	}
	model.membershipCurrent[item.ID] = operationValueAll
	return model.syncValueMembershipRows()
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
		if model.membershipCurrent != nil {
			model.membershipInitial[id] = operationValueNone
			model.membershipCurrent[id] = operationValueNone
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
	style := theme.TagChipStyle(tag)
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
	if model.membershipInitial != nil {
		model.membershipInitial[id] = operationValueNone
		model.membershipCurrent[id] = operationValueAll
	}
	return model.selectTagForAll(id)
}

func (model *OperationValuePickerModel) selectTagForAll(id ListItemID) error {
	if model.membershipCurrent != nil {
		model.membershipCurrent[id] = operationValueAll
	}
	if !model.surface.IsSelected(id) {
		selected := model.surface.SelectedIDs()
		selected = append(selected, id)
		if err := model.surface.SetSelectedIDs(selected); err != nil {
			return err
		}
	}
	if model.membershipCurrent != nil {
		if err := model.syncValueMembershipRows(); err != nil {
			return err
		}
	}
	if !model.surface.SetCursorToID(id) {
		return fmt.Errorf("%w: %s", ErrListItemNotFound, id)
	}
	return nil
}

func (model *OperationValuePickerModel) toggleCurrentMultiValue() error {
	// Membership presence, not provider identity, decides the toggle: every
	// tri-state editor moves a row to All or None on Space, and a picker with no
	// membership is an ordinary checked set.
	if len(model.membershipCurrent) == 0 {
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
	state := model.membershipCurrent[item.ID]
	if state == operationValueAll {
		if _, err := model.surface.ToggleCurrentSelection(); err != nil {
			return err
		}
		model.membershipCurrent[item.ID] = operationValueNone
	} else {
		if !model.surface.IsSelected(item.ID) {
			if _, err := model.surface.ToggleCurrentSelection(); err != nil {
				return err
			}
		}
		model.membershipCurrent[item.ID] = operationValueAll
	}
	return model.syncValueMembershipRows()
}

func (model *OperationValuePickerModel) configureValueMembership(
	membership map[ListItemID]operationValueMembership,
) error {
	if model == nil || model.surface == nil {
		return nil
	}
	model.membershipInitial = make(map[ListItemID]operationValueMembership, len(membership))
	model.membershipCurrent = make(map[ListItemID]operationValueMembership, len(membership))
	for id, state := range membership {
		model.membershipInitial[id] = state
		model.membershipCurrent[id] = state
	}
	// The checked set is built in row order rather than map order, so the
	// preselection a picker opens with is the order the list already reads in.
	items := model.surface.Items()
	selected := make([]ListItemID, 0, len(membership))
	for _, item := range items {
		if membership[item.ID] == operationValueAll {
			selected = append(selected, item.ID)
		}
	}
	if err := model.surface.SetSelectedIDs(selected); err != nil {
		return err
	}
	return model.syncValueMembershipRows()
}

func (model *OperationValuePickerModel) syncValueMembershipRows() error {
	items := model.surface.Items()
	for index := range items {
		items[index].Indeterminate = model.membershipCurrent[items[index].ID] == operationValueSome
	}
	return model.surface.SetItems(items)
}

func (model *OperationValuePickerModel) selectedValue() (any, error) {
	if model.provider == ValueProviderTags && len(model.membershipCurrent) > 0 {
		value := OperationTagsValue{}
		for _, item := range model.surface.Items() {
			if _, isContext := taskContextRowName(item); isContext {
				continue
			}
			initial := model.membershipInitial[item.ID]
			current := model.membershipCurrent[item.ID]
			switch {
			case current == operationValueAll && initial != operationValueAll:
				tag, ok := item.Data.(string)
				if !ok {
					return nil, fmt.Errorf("selected value %s has invalid payload", item.ID)
				}
				value.Add = append(value.Add, tag)
			case current == operationValueNone && initial != operationValueNone:
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
	// The reference picker emits what changed rather than what is checked. It is
	// the one multi-select whose population it does not own: an accepted
	// cross-project load can republish a stored set while the overlay is up, and
	// only a difference can leave that republished change standing — which is the
	// same reason a selection's untouched row must stay untouched.
	if model.provider == ValueProviderTaskReference && len(model.membershipCurrent) > 0 {
		return taskReferenceSelection(model.membershipInitial, model.membershipCurrent), nil
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
