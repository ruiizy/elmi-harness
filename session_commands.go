package main

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/reflow/wrap"
	"github.com/muesli/reflow/wordwrap"

	"github.com/ruiizy/elmi-harness/internal/agent"
	"github.com/ruiizy/elmi-harness/internal/api"
	"github.com/ruiizy/elmi-harness/internal/perm"
	"github.com/ruiizy/elmi-harness/internal/tools"
	"github.com/ruiizy/elmi-harness/ui"
)

func (m sessModel) handleInput(input string) (tea.Model, tea.Cmd) {
	if strings.HasPrefix(input, "/") {
		return m.handleCommand(input)
	}
	return m.submitMessage(input)
}

func (m sessModel) handleCommand(input string) (tea.Model, tea.Cmd) {
	parts := strings.SplitN(strings.TrimPrefix(input, "/"), " ", 2)
	name := parts[0]
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	switch name {
	case "help":
		m.buf.WriteString("commands:\n  /clear  /exit  /help\n  /mode always-ask|always-allow|ask-once|allow-list <tools...>\n  /model [name]  /tools\n\n")

	case "clear":
		m.messages = nil
		m.buf.Reset()
		m.buf.WriteString(styleDim.Render("conversation cleared") + "\n\n")

	case "tools":
		for _, t := range tools.Definitions() {
			m.buf.WriteString(fmt.Sprintf("  %-12s %s\n", t.Name, t.Description))
		}
		m.buf.WriteString("\n")

	case "mode":
		if args == "" {
			m.buf.WriteString(fmt.Sprintf("mode: %s\n\n", m.permCfg.Mode))
		} else {
			p := strings.Fields(args)
			switch p[0] {
			case "always-ask":
				m.permCfg.SetMode(perm.ModeAlwaysAsk)
			case "always-allow":
				m.permCfg.SetMode(perm.ModeAlwaysAllow)
			case "ask-once":
				m.permCfg.SetMode(perm.ModeAskOnce)
				m.permCfg.ResetAskOnce()
			case "allow-list":
				if len(p) >= 2 {
					m.permCfg.SetMode(perm.ModeAllowList)
					m.permCfg.SetAllowList(p[1:])
				}
				m.buf.WriteString(fmt.Sprintf("mode set: %s\n\n", m.permCfg.Mode))
				m.vp.SetContent(m.vpContent())
				m.vp.GotoBottom()
				return m, nil
			default:
				m.buf.WriteString(fmt.Sprintf("unknown mode: %s\n\n", p[0]))
				m.vp.SetContent(m.vpContent())
				m.vp.GotoBottom()
				return m, nil
			}
			m.buf.WriteString(fmt.Sprintf("mode set: %s\n\n", m.permCfg.Mode))
		}

	case "model":
		if args == "" {
			m.state = sessModelPicker
			m.pickerItems = nil
			m.pickerCursor = 0
			m.vp.Height = max(m.h-10-sepHeight, 3)
			m.vp.SetContent(m.vpContent())
			m.vp.GotoBottom()
			return m, fetchPickerItems(m.ollamaBase, m.llm.Model())
		}
		var choice ui.ModelChoice
		if strings.HasPrefix(args, "claude-") {
			choice = ui.ModelChoice{Provider: "anthropic", Model: args}
		} else {
			choice = ui.ModelChoice{Provider: "ollama", Model: args}
		}
		m.llm = newProvider(choice, m.ollamaBase)
		m.messages = nil
		m.buf.Reset()
		m.buf.WriteString(styleDim.Render("switched to "+args) + "\n\n")

	case "exit":
		return m, tea.Quit

	default:
		m.buf.WriteString(fmt.Sprintf("unknown command: /%s (try /help)\n\n", name))
	}

	m.vp.SetContent(m.vpContent())
	m.vp.GotoBottom()
	return m, nil
}

func (m sessModel) submitMessage(input string) (tea.Model, tea.Cmd) {
	// Save snapshot for Esc-interrupt rollback before touching buf.
	m.interruptedInput = input
	m.interruptedBufLen = m.buf.Len()

	w := max(m.w-4, 20)
	// Word-wrap first (respects boundaries), then hard-wrap as fallback so
	// long tokens without spaces never overflow the viewport width.
	displayInput := wrap.String(wordwrap.String(input, w), w)
	m.buf.WriteString(styleUser.Render("▶ "+displayInput) + "\n" + styleDim.Render("  ─────") + "\n\n")
	m.messages = append(m.messages, api.Message{
		Role:    api.RoleUser,
		Content: []api.Block{{Type: api.BlockText, Text: input}},
	})
	m.vp.SetContent(m.vpContent())
	m.vp.GotoBottom()

	ch := make(chan any, 64)
	m.agentCh = ch
	m.state = sessBusy

	ctx, cancel := context.WithCancel(m.ctx)
	m.cancelFn = cancel

	llm := m.llm
	messages := m.messages

	go func() {
		defer cancel()
		defer close(ch)
		updated, err := agent.Run(ctx, llm, messages, tools.Definitions(), agent.Hooks{
			OnText: func(chunk string) {
				ch <- chunkMsg(chunk)
			},
			OnToolUse: func(name, rawIn string) agent.ToolResult {
				// Buffered(1): prevents leak if context cancelled before UI sends.
				replyCh := make(chan agent.ToolResult, 1)
				ch <- toolReqMsg{name: name, rawIn: rawIn, replyCh: replyCh}
				select {
				case result := <-replyCh:
					return result
				case <-ctx.Done():
					return agent.ToolResult{Content: "context cancelled", IsError: true}
				}
			},
		})
		ch <- agentDoneMsg{updated: updated, err: err}
	}()

	return m, tea.Batch(listenAgent(ch), m.sp.Tick)
}
