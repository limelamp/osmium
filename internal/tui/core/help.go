package core

import "github.com/limelamp/osmium/internal/tui/styles"

// HelpSection is one titled block of key bindings in the help modal.
type HelpSection struct {
	Title string
	Hints []styles.Hint
}

// Helper is a page that can describe the keys that work right now. The help
// modal asks the active page, which asks whatever inside it holds focus, so the
// modal always lists the keys of the thing in front of the user rather than a
// fixed list that goes stale.
type Helper interface {
	Help() []HelpSection
}

// GlobalHints are the keys that work anywhere in the app.
var GlobalHints = []styles.Hint{
	{Key: "tab", Desc: "switch pane"},
	{Key: "esc", Desc: "back / home"},
	{Key: "mouse", Desc: "click, wheel"},
	{Key: "?", Desc: "toggle help"},
	{Key: "ctrl+c", Desc: "quit"},
}
