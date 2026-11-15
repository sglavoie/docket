package outline

import "strings"

// Sibling order is a string key compared bytewise, so a row can always be
// placed between two neighbours without renumbering them. Keys never end in
// the lowest digit, which is what guarantees room below every key.
const digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

const midDigit = len(digits) / 2

// Between returns a key sorting strictly after a and strictly before b.
// An empty a means "before everything", an empty b "after everything".
// When both are set, a must sort before b.
func Between(a, b string) string {
	if a == "" && b == "" {
		return string(digits[midDigit])
	}
	if b != "" {
		// Carry the shared prefix, reading a as padded with the lowest digit.
		n := 0
		for n < len(b) {
			ca := digits[0]
			if n < len(a) {
				ca = a[n]
			}
			if ca != b[n] {
				break
			}
			n++
		}
		if n > 0 {
			rest := ""
			if n < len(a) {
				rest = a[n:]
			}
			return b[:n] + Between(rest, b[n:])
		}
	}

	da := 0
	if a != "" {
		da = strings.IndexByte(digits, a[0])
	}
	db := len(digits)
	if b != "" {
		db = strings.IndexByte(digits, b[0])
	}

	switch {
	case b == "" && da+1 < len(digits):
		// Appending takes one step, not half the remaining space, so a long
		// run of appends grows the key by a character every ~60 rows.
		return string(digits[da+1])
	case a == "" && db > 1:
		return string(digits[db-1])
	case db-da > 1:
		return string(digits[(da+db)/2])
	case len(b) > 1:
		return b[:1]
	default:
		rest := ""
		if len(a) > 1 {
			rest = a[1:]
		}
		return string(digits[da]) + Between(rest, "")
	}
}
