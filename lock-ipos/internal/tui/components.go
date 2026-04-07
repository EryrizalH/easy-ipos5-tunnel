package tui

import (
	"strings"

	"github.com/charmbracelet/bubbletea"
)

// Text input component for manual path entry
type TextInput struct {
	placeholder string
	value       string
	focused     bool
	cursor      int
	width       int
}

// NewTextInput creates a new text input component
func NewTextInput(placeholder string, width int) *TextInput {
	return &TextInput{
		placeholder: placeholder,
		focused:     true,
		width:       width,
	}
}

// SetValue sets the input value
func (t *TextInput) SetValue(value string) {
	t.value = value
	t.cursor = len(value)
}

// GetValue returns the current input value
func (t *TextInput) GetValue() string {
	return t.value
}

// Focus sets focus on the input
func (t *TextInput) Focus() {
	t.focused = true
}

// Blur removes focus from the input
func (t *TextInput) Blur() {
	t.focused = false
}

// Update handles key presses for the text input
func (t *TextInput) Update(msg tea.Msg) (*TextInput, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyBackspace:
			if len(t.value) > 0 && t.cursor > 0 {
				t.value = t.value[:t.cursor-1] + t.value[t.cursor:]
				t.cursor--
			}
		case tea.KeyLeft:
			if t.cursor > 0 {
				t.cursor--
			}
		case tea.KeyRight:
			if t.cursor < len(t.value) {
				t.cursor++
			}
		case tea.KeyHome:
			t.cursor = 0
		case tea.KeyEnd:
			t.cursor = len(t.value)
		case tea.KeyDelete:
			if t.cursor < len(t.value) {
				t.value = t.value[:t.cursor] + t.value[t.cursor+1:]
			}
		default:
			// Handle character input - any non-special key
			if len(msg.Runes) > 0 {
				char := msg.Runes[0]
				// Filter for printable ASCII characters
				if char >= 32 && char <= 126 {
					t.value = t.value[:t.cursor] + string(char) + t.value[t.cursor:]
					t.cursor++
				}
			}
		}
	}
	return t, nil
}

// View renders the text input
func (t *TextInput) View(styles *Styles) string {
	var displayValue string
	if t.value == "" && !t.focused {
		displayValue = t.placeholder
	} else {
		displayValue = t.value
	}

	// Truncate if too long
	if t.width > 0 && len(displayValue) > t.width {
		displayValue = displayValue[:t.width]
	}

	// Add padding
	if len(displayValue) < t.width {
		displayValue += strings.Repeat(" ", t.width-len(displayValue))
	}

	if t.focused {
		return styles.InputActive.Render(displayValue)
	}
	return styles.InputBox.Render(displayValue)
}

// StatusBar component for displaying status messages
type StatusBar struct {
	message string
	err     bool
}

// NewStatusBar creates a new status bar
func NewStatusBar() *StatusBar {
	return &StatusBar{}
}

// SetMessage sets the status message
func (s *StatusBar) SetMessage(message string, isError bool) {
	s.message = message
	s.err = isError
}

// View renders the status bar
func (s *StatusBar) View(styles *Styles) string {
	if s.message == "" {
		return ""
	}

	if s.err {
		return "\n" + styles.ErrorText.Render("✗ "+s.message)
	}
	return "\n" + styles.SuccessText.Render("✓ "+s.message)
}

// Button component
type Button struct {
	label    string
	shortcut string
	active   bool
}

// NewButton creates a new button
func NewButton(label, shortcut string) *Button {
	return &Button{
		label:    label,
		shortcut: shortcut,
	}
}

// SetActive sets the button active state
func (b *Button) SetActive(active bool) {
	b.active = active
}

// IsActive returns whether the button is active
func (b *Button) IsActive() bool {
	return b.active
}

// GetLabel returns the button label
func (b *Button) GetLabel() string {
	return b.label
}

// GetShortcut returns the button shortcut
func (b *Button) GetShortcut() string {
	return b.shortcut
}

// View renders the button
func (b *Button) View(styles *Styles) string {
	label := b.shortcut + " - " + b.label
	if b.active {
		return styles.ButtonActive.Render(label)
	}
	return styles.Button.Render(label)
}
