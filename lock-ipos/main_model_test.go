package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	tui "github.com/lock-ipos/lock-ipos/internal/tui"
)

func TestMainMenu_SelectAndConfirmOption4(t *testing.T) {
	m := &model{
		currentState:   stateMainMenu,
		styles:         tui.DefaultStyles(),
		selectedOption: optionInstallIPPublic,
	}

	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	if m.selectedOption != optionUnlockDB {
		t.Fatalf("expected selectedOption=5, got %d", m.selectedOption)
	}

	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyEnter})
	if m.currentState != stateConfirm {
		t.Fatalf("expected stateConfirm, got %v", m.currentState)
	}
	if m.pendingOption != optionUnlockDB {
		t.Fatalf("expected pendingOption=5, got %d", m.pendingOption)
	}
}

func TestConfirm_CancelBackToMenu(t *testing.T) {
	m := &model{
		currentState:   stateConfirm,
		styles:         tui.DefaultStyles(),
		selectedOption: optionLockDB,
		pendingOption:  optionLockDB,
	}

	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyEsc})
	if m.currentState != stateMainMenu {
		t.Fatalf("expected stateMainMenu, got %v", m.currentState)
	}
	if m.pendingOption != 0 {
		t.Fatalf("expected pendingOption reset to 0, got %d", m.pendingOption)
	}
}

func TestMainMenu_ArrowNavigationLimits(t *testing.T) {
	m := &model{
		currentState:   stateMainMenu,
		styles:         tui.DefaultStyles(),
		selectedOption: optionInstallIPPublic,
	}

	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyUp})
	if m.selectedOption != optionInstallIPPublic {
		t.Fatalf("expected stay at first option, got %d", m.selectedOption)
	}

	m.selectedOption = optionUnlockDB
	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyDown})
	if m.selectedOption != optionUnlockDB {
		t.Fatalf("expected stay at last option, got %d", m.selectedOption)
	}
}

func TestPathInput_AllowsQWithoutQuitting(t *testing.T) {
	m := &model{
		currentState:   statePathDetect,
		pathManualMode: true,
		pathInput:      tui.NewTextInput("", 50),
	}

	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if m.quitting {
		t.Fatal("expected q to be inserted while editing path, but model is quitting")
	}
	if got := m.pathInput.GetValue(); got != "q" {
		t.Fatalf("expected path input %q, got %q", "q", got)
	}
}

func TestPathInput_EmptyEnterShowsValidationError(t *testing.T) {
	m := &model{
		currentState:   statePathDetect,
		pathManualMode: true,
		pathInput:      tui.NewTextInput("", 50),
	}

	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyEnter})

	if !m.pathError {
		t.Fatal("expected empty path to set validation error")
	}
	if m.pathStatus == "" {
		t.Fatal("expected empty path validation message")
	}
}

func TestMainMenu_QStillQuits(t *testing.T) {
	m := &model{currentState: stateMainMenu}

	_, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if !m.quitting {
		t.Fatal("expected q to quit outside path editing")
	}
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestPathInput_CtrlVPastesClipboardText(t *testing.T) {
	m := &model{
		currentState:   statePathDetect,
		pathManualMode: true,
		pathInput:      tui.NewTextInput("", 80),
		readClipboard: func() (string, error) {
			return `D:\IPOS 5 data\Server System 1.0\pgsql9.5`, nil
		},
	}

	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyCtrlV})

	want := `D:\IPOS 5 data\Server System 1.0\pgsql9.5`
	if got := m.pathInput.GetValue(); got != want {
		t.Fatalf("expected pasted path %q, got %q", want, got)
	}
	if m.pathError {
		t.Fatalf("expected successful paste status, got %q", m.pathStatus)
	}
}
