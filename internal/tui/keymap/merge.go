package keymap

// MergeEntries concatenates multiple entry slices into one.
func MergeEntries(groups ...[]Entry) []Entry {
	var result []Entry
	for _, g := range groups {
		result = append(result, g...)
	}
	return result
}
