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
		selectedOption: optionInstallService,
	}

	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	if m.selectedOption != optionUnlockDB {
		t.Fatalf("expected selectedOption=4, got %d", m.selectedOption)
	}

	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyEnter})
	if m.currentState != stateConfirm {
		t.Fatalf("expected stateConfirm, got %v", m.currentState)
	}
	if m.pendingOption != optionUnlockDB {
		t.Fatalf("expected pendingOption=4, got %d", m.pendingOption)
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
		selectedOption: optionInstallService,
	}

	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyUp})
	if m.selectedOption != optionInstallService {
		t.Fatalf("expected stay at first option, got %d", m.selectedOption)
	}

	m.selectedOption = optionUnlockDB
	_, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyDown})
	if m.selectedOption != optionUnlockDB {
		t.Fatalf("expected stay at last option, got %d", m.selectedOption)
	}
}
