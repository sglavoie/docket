package keymap

// HelpItem is a key/description pair for help display.
type HelpItem struct {
	Key         string
	Description string
	Category    string
}

// PrefixHint is a key/description pair shown in the which-key popup.
type PrefixHint struct {
	Key         string
	Description string
}

// HelpItems returns all non-hidden bindings with compound keys for nested entries.
// Leaf bindings inside prefix nodes use compound keys (e.g. ",s", "Ft").
func (km *KeyMap) HelpItems() []HelpItem {
	var items []HelpItem
	collectHelp("", km.Entries, &items)
	return items
}

func collectHelp(prefix string, entries []Entry, out *[]HelpItem) {
	for _, e := range entries {
		switch {
		case e.Binding != nil:
			if !e.Binding.Hidden {
				*out = append(*out, HelpItem{
					Key:         prefix + e.Binding.Key,
					Description: e.Binding.Description,
					Category:    e.Binding.Category,
				})
			}
		case e.PrefixNode != nil:
			collectHelp(prefix+e.PrefixNode.Key, e.PrefixNode.Children, out)
		}
	}
}

// HandledKeys returns all key sequences the KeyMap can handle: every prefix
// activator key (at all levels, as compound sequences) and every leaf binding key
// (as its full compound path).
func (km *KeyMap) HandledKeys() []string {
	seen := map[string]bool{}
	var keys []string
	collectHandledKeys("", km.Entries, seen, &keys)
	return keys
}

func collectHandledKeys(prefix string, entries []Entry, seen map[string]bool, out *[]string) {
	for _, e := range entries {
		switch {
		case e.Binding != nil:
			k := prefix + e.Binding.Key
			if !seen[k] {
				seen[k] = true
				*out = append(*out, k)
			}
		case e.PrefixNode != nil:
			activator := prefix + e.PrefixNode.Key
			if !seen[activator] {
				seen[activator] = true
				*out = append(*out, activator)
			}
			collectHandledKeys(activator, e.PrefixNode.Children, seen, out)
		}
	}
}
