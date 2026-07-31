package format

import "strings"

func unescapeAttachmentField(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '\\', ';', ',':
				b.WriteByte(s[i+1])
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func splitAttachmentEntries(s string) []string {
	var entries []string
	var current strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			current.WriteByte(s[i])
			i++
			current.WriteByte(s[i])
			continue
		}
		if s[i] == ';' {
			entries = append(entries, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(s[i])
	}
	return append(entries, current.String())
}

func isDecimalDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func looksLikeCodeRefPath(prefix string) bool {
	return prefix == "" || strings.ContainsAny(prefix, "/.")
}

func unescapeChecklistText(s string) string {
	s = strings.ReplaceAll(s, `\;`, ";")
	s = strings.ReplaceAll(s, `\,`, ",")
	s = strings.ReplaceAll(s, `\[x\]`, "[x]")
	s = strings.ReplaceAll(s, `\[ \]`, "[ ]")
	return strings.ReplaceAll(s, `\\`, `\`)
}

func splitChecklistItems(s string) []string {
	var items []string
	var current strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			current.WriteByte(s[i])
			i++
			current.WriteByte(s[i])
		} else if s[i] == ';' {
			items = append(items, current.String())
			current.Reset()
		} else {
			current.WriteByte(s[i])
		}
	}
	return append(items, current.String())
}
