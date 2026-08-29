package core

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// FieldResult is what a key did to a field.
type FieldResult int

const (
	// FieldIgnored means the key was not the field's to handle.
	FieldIgnored FieldResult = iota
	// FieldTyping means the key changed what is being typed.
	FieldTyping
	// FieldCommitted means enter was pressed: the value is to be saved.
	FieldCommitted
	// FieldCancelled means esc was pressed: the edit was abandoned.
	FieldCancelled
)

// Field is a text input that opens, commits and cancels the same way wherever
// it appears: enter opens it on the value already there with the caret at the
// end, enter saves, esc restores what it held before.
//
// Every field in the app went through the same half-dozen lines to do that, and
// they did not all agree — one forgot the caret, another cleared instead of
// restoring. Here it is written once.
type Field struct {
	input textinput.Model

	open  bool
	saved string
}

// NewField returns a closed field with the given placeholder.
func NewField(placeholder string) Field {
	input := textinput.New()
	input.Placeholder = placeholder
	input.Prompt = "› "
	input.CharLimit = 500

	return Field{input: input}
}

// WithPrompt changes the marker drawn before the text.
func (f Field) WithPrompt(prompt string) Field {
	f.input.Prompt = prompt
	return f
}

// WithLimit caps how much can be typed.
func (f Field) WithLimit(limit int) Field {
	f.input.CharLimit = limit
	return f
}

// Open puts the keyboard in the field, on the end of the value it is editing.
func (f Field) Open(value string) (Field, tea.Cmd) {
	f.open = true
	f.saved = value
	f.input.SetValue(value)
	f.input.CursorEnd()

	return f, f.input.Focus()
}

// Close leaves the field as it is and takes the keyboard back.
func (f Field) Close() Field {
	f.open = false
	f.input.Blur()
	return f
}

// Cancel abandons the edit, restoring the value the field was opened on.
func (f Field) Cancel() Field {
	f.input.SetValue(f.saved)
	return f.Close()
}

// HandleKey applies the field's own keys and reports what happened, so the
// caller only has to say what committing means for it.
func (f Field) HandleKey(msg tea.KeyPressMsg) (Field, FieldResult, tea.Cmd) {
	if !f.open {
		return f, FieldIgnored, nil
	}

	switch msg.String() {
	case "enter":
		return f.Close(), FieldCommitted, nil

	case "esc":
		return f.Cancel(), FieldCancelled, nil

	// A field opens on the value it is editing with the caret after it, which
	// is right for a correction and wrong for a replacement. ctrl+u empties it
	// in one keystroke, as it does in a shell.
	case "ctrl+u":
		f.input.SetValue("")
		return f, FieldTyping, nil
	}

	input, cmd := f.input.Update(msg)
	f.input = input
	return f, FieldTyping, cmd
}

// IsOpen reports whether the field currently has the keyboard, which is what
// tells a page to stop treating esc and backspace as navigation.
func (f Field) IsOpen() bool { return f.open }

// Value is what has been typed.
func (f Field) Value() string { return f.input.Value() }

// SetValue replaces the contents without opening the field.
func (f Field) SetValue(value string) Field {
	f.input.SetValue(value)
	return f
}

// SetWidth sizes the field to the space it is drawn in.
func (f Field) SetWidth(width int) Field {
	f.input.SetWidth(max(width, 8))
	return f
}

// View renders the field.
func (f Field) View() string { return f.input.View() }
