package textfit

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// bareURLPattern finds the web addresses prose carries: an http or https
// scheme and everything up to the next space. Trailing sentence punctuation is
// peeled off afterwards by trimURLTail rather than excluded here, because a
// period inside a URL is ordinary and only the last one ends a sentence.
var bareURLPattern = regexp.MustCompile(`https?://[^\s<>"'\x1b]+`)

// osc8Pattern reads one OSC 8 opener or closer: its URI is empty for a closer.
// Both terminators are accepted, since a sequence may come from anywhere.
var osc8Pattern = regexp.MustCompile("\x1b\\]8;[^;\x07\x1b]*;([^\x07\x1b]*)(?:\x07|\x1b\\\\)")

// trimURLTail drops the punctuation a sentence puts after an address: a
// closing period, comma, colon, semicolon, or mark, and a closing parenthesis
// or bracket the address never opened.
func trimURLTail(candidate string) string {
	for candidate != "" {
		last := candidate[len(candidate)-1]
		switch {
		case strings.IndexByte(".,;:!?", last) >= 0:
		case last == ')' && strings.Count(candidate, "(") < strings.Count(candidate, ")"):
		case last == ']' && strings.Count(candidate, "[") < strings.Count(candidate, "]"):
		default:
			return candidate
		}
		candidate = candidate[:len(candidate)-1]
	}
	return candidate
}

// LinkifyURLs links every bare web address in plain prose to itself, so a
// terminal that follows OSC 8 links opens it from the text the reader sees.
// The visible text is unchanged, and a string with no address is returned as
// it was.
func LinkifyURLs(text string) string {
	if !strings.Contains(text, "://") {
		return text
	}
	return bareURLPattern.ReplaceAllStringFunc(text, func(match string) string {
		address := trimURLTail(match)
		if !strings.Contains(address, "://") || strings.HasSuffix(address, "://") {
			return match
		}
		return Hyperlink(address, address) + match[len(address):]
	})
}

// BalanceHyperlinks closes every OSC 8 link still open at the end of a row and
// reopens it at the start of the next, so each row of a wrapped text is a whole
// link on its own: padding, clipping, or a pane border drawn after a row never
// inherits a link, and a terminal opens the whole address from any row of it.
func BalanceHyperlinks(lines []string) []string {
	open := ""
	for i, line := range lines {
		state := open
		for _, match := range osc8Pattern.FindAllStringSubmatch(line, -1) {
			state = match[1]
		}
		if open != "" {
			line = ansi.SetHyperlink(open) + line
		}
		if state != "" {
			line += ansi.ResetHyperlink()
		}
		lines[i] = line
		open = state
	}
	return lines
}

// WrapLinkedText wraps plain prose like WrapText after linking its bare web
// addresses, then balances the links so none crosses a row boundary.
func WrapLinkedText(text string, width int) []string {
	return BalanceHyperlinks(WrapText(LinkifyURLs(text), width))
}
