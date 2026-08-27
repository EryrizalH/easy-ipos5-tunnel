package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTextInputInsertsAllPastedRunes(t *testing.T) {
	input := NewTextInput("", 50)
	path := `C:\Program Files (x86)\Inspirasibiz\Server System 1.0\pgsql9.5\bin`

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(path)})

	if got := input.GetValue(); got != path {
		t.Fatalf("expected complete pasted path %q, got %q", path, got)
	}
}

func TestTextInputEditsUnicodeByRune(t *testing.T) {
	input := NewTextInput("", 20)
	input.SetValue("a\u00e9c")

	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyLeft})
	input, _ = input.Update(tea.KeyMsg{Type: tea.KeyBackspace})

	if got := input.GetValue(); got != "ac" {
		t.Fatalf("expected rune-safe edit result %q, got %q", "ac", got)
	}
}

func TestTextInputVisibleValueFollowsCursor(t *testing.T) {
	input := NewTextInput("", 5)
	input.SetValue("1234567890")

	if got := input.visibleValue(); got != "67890" {
		t.Fatalf("expected tail viewport %q, got %q", "67890", got)
	}

	for i := 0; i < 6; i++ {
		input, _ = input.Update(tea.KeyMsg{Type: tea.KeyLeft})
	}
	if got := input.visibleValue(); got != "12345" {
		t.Fatalf("expected cursor-following viewport %q, got %q", "12345", got)
	}
}

func TestTextInputInsertFiltersControlCharacters(t *testing.T) {
	input := NewTextInput("", 80)
	input.Insert("  D:\\IPOS 5 data\\pgsql9.5\r\n")

	if got := input.GetValue(); got != `  D:\IPOS 5 data\pgsql9.5` {
		t.Fatalf("expected printable clipboard text, got %q", got)
	}
}
