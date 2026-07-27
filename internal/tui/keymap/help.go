package keymap

// HelpItem is a key/description pair for help display.
type HelpItem struct {
	Key         string
	Description string
	Category    string
}

// HelpItems returns all active non-hidden direct bindings.
func (km *KeyMap) HelpItems() []HelpItem {
	var items []HelpItem
	for _, entry := range km.Entries {
		binding := entry.Binding
		if binding != nil && !binding.Hidden && (binding.Gate == nil || binding.Gate()) {
			items = append(items, HelpItem{
				Key: binding.Key, Description: binding.Description, Category: binding.Category,
			})
		}
	}
	return items
}

// HandledKeys returns all direct keys the KeyMap can handle.
func (km *KeyMap) HandledKeys() []string {
	seen := map[string]bool{}
	var keys []string
	for _, entry := range km.Entries {
		if entry.Binding != nil && !seen[entry.Binding.Key] {
			seen[entry.Binding.Key] = true
			keys = append(keys, entry.Binding.Key)
		}
	}
	return keys
}
