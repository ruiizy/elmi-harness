package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ruiizy/elmi-harness/ui"
)

type pickerItem struct {
	provider string
	name     string
	header   bool
	label    string
}

type pickerReadyMsg struct {
	items   []pickerItem
	current string
}

func fetchPickerItems(ollamaBase, current string) tea.Cmd {
	return func() tea.Msg {
		var items []pickerItem
		items = append(items, pickerItem{header: true, label: "Anthropic"})
		for _, m := range ui.AnthropicModels {
			items = append(items, pickerItem{provider: "anthropic", name: m})
		}
		if names := ui.FetchOllamaModels(ollamaBase); len(names) > 0 {
			items = append(items, pickerItem{header: true, label: "Ollama (local)"})
			for _, n := range names {
				items = append(items, pickerItem{provider: "ollama", name: n})
			}
		}
		return pickerReadyMsg{items: items, current: current}
	}
}

func pickerNext(items []pickerItem, from int) int {
	for i := from + 1; i < len(items); i++ {
		if !items[i].header {
			return i
		}
	}
	return from
}

func pickerPrev(items []pickerItem, from int) int {
	for i := from - 1; i >= 0; i-- {
		if !items[i].header {
			return i
		}
	}
	return from
}

func (m sessModel) pickerAreaH() int {
	return max(min(len(m.pickerItems)+3, 14), 6)
}

func (m sessModel) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.pickerItems) == 0 {
		return m, nil
	}
	switch msg.Type {
	case tea.KeyUp:
		m.pickerCursor = pickerPrev(m.pickerItems, m.pickerCursor)
	case tea.KeyDown:
		m.pickerCursor = pickerNext(m.pickerItems, m.pickerCursor)
	case tea.KeyEnter:
		it := m.pickerItems[m.pickerCursor]
		if !it.header {
			choice := ui.ModelChoice{Provider: it.provider, Model: it.name}
			m.llm = newProvider(choice, m.ollamaBase)
			m.messages = nil
			m.buf.Reset()
			m.buf.WriteString(styleDim.Render("switched to "+it.name) + "\n\n")
		}
		m.state = sessIdle
		m.pickerItems = nil
		m.vp.Height = max(m.h-taHeight-sepHeight, 1)
		m.vp.SetContent(m.vpContent())
		m.vp.GotoBottom()
	case tea.KeyEsc:
		m.state = sessIdle
		m.pickerItems = nil
		m.vp.Height = max(m.h-taHeight-sepHeight, 1)
		m.vp.SetContent(m.vpContent())
		m.vp.GotoBottom()
	}
	return m, nil
}

func (m sessModel) renderPicker() string {
	if len(m.pickerItems) == 0 {
		return styleDim.Render("  loading models...")
	}
	var sb strings.Builder
	for i, it := range m.pickerItems {
		if it.header {
			sb.WriteString(styleDim.Render("  "+it.label) + "\n")
			continue
		}
		suffix := ""
		if it.name == m.llm.Model() {
			suffix = styleDim.Render(" (current)")
		}
		if i == m.pickerCursor {
			sb.WriteString(styleUser.Render("  > "+it.name) + suffix + "\n")
		} else {
			sb.WriteString("    " + it.name + suffix + "\n")
		}
	}
	sb.WriteString("\n" + styleDim.Render("  ↑↓ navigate · Enter select · Esc cancel"))
	return sb.String()
}
