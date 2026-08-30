package core

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// Filter narrows a list to what is being looked for.
//
// It is one thing rather than five because every list in the app filters the
// same way: "/" opens it, typing narrows as you go, esc clears it and enter
// leaves it in place. A list that filtered differently from the one beside it
// would be a list nobody trusts.
type Filter struct {
	field Field
}

// NewFilter returns a closed filter.
func NewFilter() Filter {
	return Filter{field: NewField("filter…").WithPrompt("/ ").WithLimit(40).SetWidth(28)}
}

// Open puts the keyboard in the filter, keeping whatever it already narrows to.
func (f Filter) Open() (Filter, tea.Cmd) {
	field, cmd := f.field.Open(f.field.Value())
	f.field = field

	return f, cmd
}

// IsOpen reports whether the filter has the keyboard.
func (f Filter) IsOpen() bool { return f.field.IsOpen() }

// Active reports whether the filter is narrowing anything.
func (f Filter) Active() bool { return strings.TrimSpace(f.field.Value()) != "" }

// Value is what is being looked for.
func (f Filter) Value() string { return strings.TrimSpace(f.field.Value()) }

// HandleKey applies a keypress while the filter is open.
//
// Esc clears it and closes it, which is the one thing a filter must always do:
// a list narrowed to nothing with no obvious way back is how people lose their
// data in an interface.
func (f Filter) HandleKey(msg tea.KeyPressMsg) (Filter, tea.Cmd) {
	field, result, cmd := f.field.HandleKey(msg)
	f.field = field

	switch result {
	case FieldCancelled:
		return f.Clear(), nil
	case FieldCommitted:
		f.field = f.field.Close()
		return f, nil
	}

	return f, cmd
}

// Clear empties the filter and closes it.
func (f Filter) Clear() Filter {
	f.field = f.field.Reset()
	return f
}

// Matches reports whether any of the given texts contain what is being looked
// for. An empty filter matches everything.
func (f Filter) Matches(texts ...string) bool {
	needle := strings.ToLower(f.Value())
	if needle == "" {
		return true
	}

	for _, text := range texts {
		if strings.Contains(strings.ToLower(text), needle) {
			return true
		}
	}
	return false
}

// View is the line to draw for the filter, or an empty string when there is
// nothing to say.
func (f Filter) View() string {
	switch {
	case f.IsOpen():
		return f.field.View()
	case f.Active():
		return lipgloss.NewStyle().Foreground(theme.Accent).Render("/" + f.Value())
	default:
		return ""
	}
}

// Count says how much of a list is showing, for a title or a heading.
func (f Filter) Count(shown, total int) string {
	if !f.Active() && !f.IsOpen() {
		return ""
	}
	if shown == total {
		return ""
	}

	return lipgloss.NewStyle().Foreground(theme.Inactive).
		Render(strings.TrimSpace(itoa(shown) + " of " + itoa(total)))
}

// itoa is strconv.Itoa without the import, for the two numbers this needs.
func itoa(value int) string {
	if value == 0 {
		return "0"
	}

	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
