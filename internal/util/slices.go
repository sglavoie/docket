package util

import "slices"

// SliceAdd appends item to slice if not already present.
// Returns the (possibly modified) slice and whether the item was added.
func SliceAdd[T comparable](s []T, item T) ([]T, bool) {
	if slices.Contains(s, item) {
		return s, false
	}
	return append(s, item), true
}

// SliceRemove removes the first occurrence of item from slice.
// Returns the (possibly modified) slice and whether the item was removed.
func SliceRemove[T comparable](s []T, item T) ([]T, bool) {
	if i := slices.Index(s, item); i >= 0 {
		return slices.Delete(s, i, i+1), true
	}
	return s, false
}
