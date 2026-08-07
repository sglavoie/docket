package keymap

import (
	"sort"
	"strings"
)

// HelpItem is a key/description pair for help display.
type HelpItem struct {
	Key         string
	Description string
	Category    string
}

// FooterHint is renderer-neutral compact guidance derived from one live
// binding. It intentionally retains the binding's rank and required state so
// callers can apply their own row and width budgets without parsing strings.
type FooterHint struct {
	Key      string
	Label    string
	Rank     int
	Required bool
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

// FooterHints returns ranked guidance for active non-hidden bindings. Help and
// guidance therefore apply the same gate and visibility rules, while
// HandledKeys continues to describe every physical key the view can consume.
func (km *KeyMap) FooterHints() []FooterHint {
	if km == nil {
		return nil
	}
	type candidate struct {
		index int
		hint  FooterHint
	}
	candidates := make([]candidate, 0, len(km.Entries))
	for index, entry := range km.Entries {
		binding := entry.Binding
		if binding == nil || binding.Hidden || binding.FooterRank <= 0 ||
			(binding.Gate != nil && !binding.Gate()) {
			continue
		}
		label := strings.TrimSpace(binding.FooterLabel)
		if label == "" {
			label = strings.TrimSpace(binding.Description)
		}
		key := strings.TrimSpace(binding.FooterKey)
		if key == "" {
			key = strings.TrimSpace(binding.Key)
		}
		candidates = append(candidates, candidate{
			index: index,
			hint: FooterHint{
				Key:      key,
				Label:    label,
				Rank:     binding.FooterRank,
				Required: binding.FooterRequired,
			},
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].hint.Rank != candidates[j].hint.Rank {
			return candidates[i].hint.Rank < candidates[j].hint.Rank
		}
		return candidates[i].index < candidates[j].index
	})
	hints := make([]FooterHint, len(candidates))
	for index, candidate := range candidates {
		hints[index] = candidate.hint
	}
	return hints
}
