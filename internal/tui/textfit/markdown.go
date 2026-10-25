package textfit

import (
	"container/list"
	"strconv"
	"strings"
	"sync"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/theme"
)

// RenderMarkdown renders the given text as styled terminal markdown using glamour.
// Falls back to raw text on error. Output is always bounded to width: Glamour word
// wraps and table wraps, and boundToWidth is a final ANSI-aware guard that re-wraps
// any remaining over-wide lines (e.g. wide tables) without stripping styling.
//
// The result is a pure function of the theme, the width, and the text, and
// callers render on every frame and keypress, so it is memoized in a bounded
// LRU rather than rebuilt through glamour each time.
func RenderMarkdown(text string, width int) string {
	key := markdownKeyFor(theme.ActiveThemeName, width, text)
	if rendered, ok := markdownMemo.get(key); ok {
		return rendered
	}
	rendered := renderMarkdown(text, width)
	markdownMemo.put(key, rendered)
	return rendered
}

func renderMarkdown(text string, width int) string {
	style := styles.DarkStyleConfig
	if theme.ActiveThemeName == "light" {
		style = styles.LightStyleConfig
	}
	noMargin := uint(0)
	style.Document.Margin = &noMargin

	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(width),
		glamour.WithTableWrap(true),
	)
	if err != nil {
		return boundToWidth(text, width)
	}

	rendered, err := r.Render(text)
	if err != nil {
		return boundToWidth(text, width)
	}

	return strings.TrimSpace(boundToWidth(rendered, width))
}

// boundToWidth guarantees every line of s fits within width, measuring visual
// (ANSI-aware) width. Lines already within width pass through untouched so normal
// paragraphs are not re-fragmented. Over-wide lines are word-wrapped first, then
// any residual fragments (e.g. long Markdown separator runs) are hard-wrapped.
func boundToWidth(s string, width int) string {
	if width <= 0 || s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		if lipgloss.Width(line) <= width {
			wrapped = append(wrapped, line)
			continue
		}
		for _, wordWrapped := range WrapStyled(line, width) {
			if lipgloss.Width(wordWrapped) <= width {
				wrapped = append(wrapped, wordWrapped)
				continue
			}
			wrapped = append(wrapped, hardWrapStyled(wordWrapped, width)...)
		}
	}
	return strings.Join(wrapped, "\n")
}

// markdownMemoCapacity bounds the memo. A frame renders at most the comments a
// panel shows plus one reader body, so this holds several screens' worth.
const markdownMemoCapacity = 256

// markdownKey spells the theme, the width, and the text as one string. The text
// comes last and neither of the others can hold the separator, so two different
// triples never share a key.
type markdownKey string

func markdownKeyFor(themeName string, width int, text string) markdownKey {
	return markdownKey(themeName + "\x00" + strconv.Itoa(width) + "\x00" + text)
}

type markdownEntry struct {
	key      markdownKey
	rendered string
}

type markdownLRU struct {
	mu      sync.Mutex
	order   *list.List
	entries map[markdownKey]*list.Element
}

var markdownMemo = &markdownLRU{
	order:   list.New(),
	entries: map[markdownKey]*list.Element{},
}

func (c *markdownLRU) get(key markdownKey) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	element, ok := c.entries[key]
	if !ok {
		return "", false
	}
	c.order.MoveToFront(element)
	return element.Value.(*markdownEntry).rendered, true
}

func (c *markdownLRU) put(key markdownKey, rendered string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.entries[key]; ok {
		element.Value.(*markdownEntry).rendered = rendered
		c.order.MoveToFront(element)
		return
	}
	c.entries[key] = c.order.PushFront(&markdownEntry{key: key, rendered: rendered})
	if c.order.Len() > markdownMemoCapacity {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.entries, oldest.Value.(*markdownEntry).key)
	}
}
