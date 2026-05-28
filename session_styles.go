package main

import "github.com/charmbracelet/lipgloss"

// Nord-inspired palette — readable on dark terminals.
var (
	styleUser    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A3BE8C"))
	styleTool    = lipgloss.NewStyle().Foreground(lipgloss.Color("#EBCB8B"))
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4C566A"))
	styleSep     = lipgloss.NewStyle().Foreground(lipgloss.Color("#3B4252"))
	styleSepHi   = lipgloss.NewStyle().Foreground(lipgloss.Color("#88C0D0")).Bold(true)
	styleErr     = lipgloss.NewStyle().Foreground(lipgloss.Color("#BF616A"))
	styleWelcome = lipgloss.NewStyle().Foreground(lipgloss.Color("#88C0D0")).Bold(true)
)
