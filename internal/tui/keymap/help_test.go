package keymap

import (
	"reflect"
	"testing"
)

// TestHelpKeyRenamesOnlyTheHelpRow holds the display spelling to the one
// projection it belongs to. A surface reaching an act through a chord and an
// alternate key names the whole path in help, while dispatch, HandledKeys, and
// the compact footer keep reading the physical key the binding declares.
func TestHelpKeyRenamesOnlyTheHelpRow(t *testing.T) {
	km := &KeyMap{Entries: []Entry{
		Bind("home", "Jump to first row", "Navigation",
			WithHelpKey("gg/Home"), WithFooter(1, "top", false), WithFooterKey("gg")),
		Bind("G", "Jump to last row", "Navigation", WithHelpKey("G/End")),
		Bind("end", "Jump to last row", "Navigation", AsHidden()),
		Bind("j", "Move cursor down", "Navigation"),
	}}

	wantHelp := []HelpItem{
		{Key: "gg/Home", Description: "Jump to first row", Category: "Navigation"},
		{Key: "G/End", Description: "Jump to last row", Category: "Navigation"},
		{Key: "j", Description: "Move cursor down", Category: "Navigation"},
	}
	if got := km.HelpItems(); !reflect.DeepEqual(got, wantHelp) {
		t.Errorf("HelpItems() = %#v, want %#v", got, wantHelp)
	}

	wantKeys := []string{"home", "G", "end", "j"}
	if got := km.HandledKeys(); !reflect.DeepEqual(got, wantKeys) {
		t.Errorf("HandledKeys() = %v, want %v", got, wantKeys)
	}

	wantHints := []FooterHint{{Key: "gg", Label: "top", Rank: 1}}
	if got := km.FooterHints(); !reflect.DeepEqual(got, wantHints) {
		t.Errorf("FooterHints() = %#v, want %#v", got, wantHints)
	}
}
