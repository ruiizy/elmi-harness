package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ruiizy/elmi-harness/internal/agent"
	"github.com/ruiizy/elmi-harness/internal/perm"
)

var confirmChoices = []struct{ label, key string }{
	{"Yes, run once", "y"},
	{"Always allow this tool", "a"},
	{"Deny", "n"},
}

func (m sessModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.confirmCursor > 0 {
			m.confirmCursor--
		}
	case tea.KeyDown:
		if m.confirmCursor < len(confirmChoices)-1 {
			m.confirmCursor++
		}
	case tea.KeyEnter:
		return m.resolveConfirm(m.confirmCursor)
	case tea.KeyEsc:
		return m.resolveConfirm(2) // Esc = deny
	}
	return m, nil
}

func (m sessModel) resolveConfirm(choice int) (tea.Model, tea.Cmd) {
	pending := m.pending
	m.pending = nil
	m.state = sessBusy
	m.vp.Height = max(m.h-taHeight-sepHeight, 1)

	switch choice {
	case 1: // Always allow
		m.permCfg.GrantSession(pending.name)
		m.buf.WriteString(styleDim.Render("[approved · always]") + "\n")
		m.vp.SetContent(m.vpContent())
		m.vp.GotoBottom()
		return m, tea.Batch(execToolCmd(pending.name, pending.rawIn, pending.replyCh), m.sp.Tick)
	case 2: // Deny
		pending.replyCh <- agent.ToolResult{Content: "user denied", IsError: true}
		m.buf.WriteString(styleDim.Render("[denied]") + "\n")
		m.vp.SetContent(m.vpContent())
		m.vp.GotoBottom()
		return m, tea.Batch(listenAgent(m.agentCh), m.sp.Tick)
	default: // 0 = Yes, run once
		if m.permCfg.Mode == perm.ModeAskOnce {
			m.permCfg.GrantAskOnce(pending.name)
		}
		m.buf.WriteString(styleDim.Render("[approved]") + "\n")
		m.vp.SetContent(m.vpContent())
		m.vp.GotoBottom()
		return m, tea.Batch(execToolCmd(pending.name, pending.rawIn, pending.replyCh), m.sp.Tick)
	}
}

func (m sessModel) renderConfirmPicker() string {
	var sb strings.Builder
	for i, c := range confirmChoices {
		if i == m.confirmCursor {
			sb.WriteString(styleUser.Render("  > "+c.label) + styleDim.Render("  ["+c.key+"]") + "\n")
		} else {
			sb.WriteString(styleDim.Render("    "+c.label) + "\n")
		}
	}
	sb.WriteString("\n" + styleDim.Render("  ↑↓ navigate · Enter select · Esc deny"))
	return sb.String()
}
