package util

// SliceContains returns true if the slice contains the value.
func SliceContains(slice []string, value string) bool {
	for _, s := range slice {
		if s == value {
			return true
		}
	}
	return false
}

// SliceRemove returns a new slice with the first occurrence of value removed.
// Returns the new slice and a boolean indicating if removal occurred.
func SliceRemove(slice []string, value string) ([]string, bool) {
	for i, s := range slice {
		if s == value {
			return append(slice[:i], slice[i+1:]...), true
		}
	}
	return slice, false
}
