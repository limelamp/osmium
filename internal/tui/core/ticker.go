package core

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// TickMsg is one beat of a Ticker. It names the ticker it came from and the
// generation of the loop that scheduled it, which is what lets a component tell
// its own current beat from an old one.
type TickMsg struct {
	Source     string
	Generation int
}

// Ticker is a repeating timer owned by a component.
//
// A tea.Tick is delivered to whatever page is on screen when it fires, so a
// timer belonging to a page the user has left is simply lost - and arming a new
// one on every visit leaves the old loops running as well. A Ticker fixes both:
// it refuses to arm twice, and Restart abandons the generation that was lost
// rather than adding to it.
type Ticker struct {
	source   string
	interval time.Duration

	generation int
	armed      bool
}

// NewTicker returns a ticker that beats every interval. The source only has to
// be unique among the tickers a single component owns.
func NewTicker(source string, interval time.Duration) Ticker {
	return Ticker{source: source, interval: interval}
}

// Arm schedules the next beat, unless one is already scheduled.
func (t Ticker) Arm() (Ticker, tea.Cmd) {
	if t.armed {
		return t, nil
	}

	t.armed = true
	generation := t.generation

	return t, tea.Tick(t.interval, func(time.Time) tea.Msg {
		return TickMsg{Source: t.source, Generation: generation}
	})
}

// Restart abandons whatever was scheduled and arms a fresh loop. It is what a
// component calls when its page becomes active again, since any beat scheduled
// before that was delivered somewhere else.
func (t Ticker) Restart() (Ticker, tea.Cmd) {
	t.generation++
	t.armed = false
	return t.Arm()
}

// Stop abandons the current loop without starting another.
func (t Ticker) Stop() Ticker {
	t.generation++
	t.armed = false
	return t
}

// Accepts reports whether a message is this ticker's current beat, and marks
// the ticker as free to be armed again. A beat from an abandoned generation is
// dropped: something newer is already running.
func (t Ticker) Accepts(msg tea.Msg) (Ticker, bool) {
	tick, ok := msg.(TickMsg)
	if !ok || tick.Source != t.source || tick.Generation != t.generation {
		return t, false
	}

	t.armed = false
	return t, true
}
