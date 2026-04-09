package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Styles holds all the lipgloss styles for the TUI
type Styles struct {
	// Base styles
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	NormalText  lipgloss.Style
	MutedText   lipgloss.Style
	ErrorText   lipgloss.Style
	SuccessText lipgloss.Style
	WarningText lipgloss.Style

	// Box styles
	Box      lipgloss.Style
	BoxTitle lipgloss.Style
	BoxInner lipgloss.Style

	// Status styles
	StatusAllow lipgloss.Style
	StatusLock  lipgloss.Style
	StatusLabel lipgloss.Style

	// Menu styles
	MenuItem       lipgloss.Style
	MenuItemActive lipgloss.Style
	MenuItemNumber lipgloss.Style

	// Button styles
	Button       lipgloss.Style
	ButtonActive lipgloss.Style

	// Input styles
	InputLabel  lipgloss.Style
	InputBox    lipgloss.Style
	InputActive lipgloss.Style

	// Code/SQL styles
	CodeBox  lipgloss.Style
	CodeText lipgloss.Style

	// Misc
	Separator       lipgloss.Style
	HelpText        lipgloss.Style
	ProgressBar     lipgloss.Style
	ProgressPending lipgloss.Style
	ProgressRunning lipgloss.Style
	ProgressSuccess lipgloss.Style
	ProgressFailed  lipgloss.Style
	LogBox          lipgloss.Style
	LogText         lipgloss.Style
}

// DefaultStyles returns the default styles for the application
func DefaultStyles() *Styles {
	s := &Styles{}

	// Color definitions
	colorTitle := lipgloss.Color("#8695F7")   // Light purple
	colorAllow := lipgloss.Color("#04DB5F")   // Green
	colorLock := lipgloss.Color("#FF5C5C")    // Red
	colorWarning := lipgloss.Color("#FFB86C") // Orange
	colorMuted := lipgloss.Color("#6272A4")   // Grayish blue
	colorBorder := lipgloss.Color("#44475A")  // Dark gray

	// Base styles
	s.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorTitle).
		MarginBottom(1)

	s.Subtitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F8F8F2")).
		MarginBottom(1)

	s.NormalText = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F8F8F2"))

	s.MutedText = lipgloss.NewStyle().
		Foreground(colorMuted)

	s.ErrorText = lipgloss.NewStyle().
		Foreground(colorLock).
		Bold(true)

	s.SuccessText = lipgloss.NewStyle().
		Foreground(colorAllow).
		Bold(true)

	s.WarningText = lipgloss.NewStyle().
		Foreground(colorWarning).
		Bold(true)

	// Box styles
	s.Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(1, 2)

	s.BoxTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorTitle).
		MarginBottom(1)

	s.BoxInner = lipgloss.NewStyle().
		MarginTop(1).
		MarginBottom(1)

	// Status styles
	s.StatusAllow = lipgloss.NewStyle().
		Foreground(colorAllow).
		Bold(true)

	s.StatusLock = lipgloss.NewStyle().
		Foreground(colorLock).
		Bold(true)

	s.StatusLabel = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F8F8F2"))

	// Menu styles
	s.MenuItem = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F8F8F2")).
		MarginBottom(1)

	s.MenuItemActive = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F8F8F2")).
		Background(colorTitle).
		Bold(true).
		MarginBottom(1).
		Padding(0, 1)

	s.MenuItemNumber = lipgloss.NewStyle().
		Foreground(colorTitle).
		Bold(true).
		Width(3)

	// Button styles
	s.Button = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F8F8F2")).
		Background(colorMuted).
		Padding(0, 2).
		MarginRight(1)

	s.ButtonActive = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#282A36")).
		Background(colorTitle).
		Bold(true).
		Padding(0, 2).
		MarginRight(1)

	// Input styles
	s.InputLabel = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F8F8F2")).
		Bold(true)

	s.InputBox = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F8F8F2")).
		Background(lipgloss.Color("#44475A")).
		Padding(0, 1)

	s.InputActive = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F8F8F2")).
		Background(colorTitle).
		Padding(0, 1)

	// Code/SQL styles
	s.CodeBox = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#50FA7B")).
		Background(lipgloss.Color("#282A36")).
		Padding(0, 1).
		MarginTop(1).
		MarginBottom(1)

	s.CodeText = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#50FA7B"))

	// Misc
	s.Separator = lipgloss.NewStyle().
		Foreground(colorBorder)

	s.HelpText = lipgloss.NewStyle().
		Foreground(colorMuted).
		MarginTop(1)

	s.ProgressBar = lipgloss.NewStyle().
		Foreground(colorAllow).
		Background(colorMuted).
		Width(30)

	s.ProgressPending = lipgloss.NewStyle().
		Foreground(colorMuted)

	s.ProgressRunning = lipgloss.NewStyle().
		Foreground(colorTitle).
		Bold(true)

	s.ProgressSuccess = lipgloss.NewStyle().
		Foreground(colorAllow).
		Bold(true)

	s.ProgressFailed = lipgloss.NewStyle().
		Foreground(colorLock).
		Bold(true)

	s.LogBox = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1)

	s.LogText = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F8F8F2"))

	return s
}
