package keymap

// MergeEntries concatenates multiple entry slices into one.
func MergeEntries(groups ...[]Entry) []Entry {
	var result []Entry
	for _, g := range groups {
		result = append(result, g...)
	}
	return result
}

// OverrideEntry replaces the first top-level entry whose key matches key with replacement.
func OverrideEntry(base []Entry, key string, replacement Entry) []Entry {
	result := make([]Entry, len(base))
	copy(result, base)
	for i, e := range result {
		var entryKey string
		if e.Binding != nil {
			entryKey = e.Binding.Key
		} else if e.PrefixNode != nil {
			entryKey = e.PrefixNode.Key
		}
		if entryKey == key {
			result[i] = replacement
			return result
		}
	}
	return result
}
