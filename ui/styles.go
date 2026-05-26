package ui

import "github.com/charmbracelet/lipgloss"

var (
	StyleTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	StyleMuted   = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
	StyleSel     = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	StyleOption  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	StyleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	StyleSpinner = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	StyleLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	StylePrompt  = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
)
