package main

import (
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ruiizy/elmi-harness/internal/agent"
	"github.com/ruiizy/elmi-harness/internal/api"
	"github.com/ruiizy/elmi-harness/internal/tools"
)

type (
	chunkMsg   string
	toolReqMsg struct {
		name    string
		rawIn   string
		replyCh chan<- agent.ToolResult // buffered(1): send never blocks
	}
	toolResultMsg struct {
		result  agent.ToolResult
		replyCh chan<- agent.ToolResult
	}
	agentDoneMsg struct {
		updated []api.Message
		err     error
	}
	noticeClearMsg struct{}
)

func noticeClearAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return noticeClearMsg{} })
}

func copyToClipboard(text string) {
	var candidates [][]string
	switch runtime.GOOS {
	case "darwin":
		candidates = [][]string{{"pbcopy"}}
	default:
		candidates = [][]string{
			{"wl-copy"},
			{"xclip", "-selection", "clipboard"},
			{"xsel", "-bi"},
		}
	}
	for _, args := range candidates {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if cmd.Run() == nil {
			return
		}
	}
}

func copyCmd(text string) tea.Cmd {
	return func() tea.Msg {
		copyToClipboard(text)
		return nil
	}
}

func lastAssistantText(messages []api.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == api.RoleAssistant {
			for _, b := range messages[i].Content {
				if b.Type == api.BlockText && b.Text != "" {
					return b.Text
				}
			}
		}
	}
	return ""
}

func listenAgent(ch <-chan any) tea.Cmd {
	return func() tea.Msg {
		v, ok := <-ch
		if !ok {
			return agentDoneMsg{}
		}
		return v
	}
}

func execToolCmd(name, rawIn string, replyCh chan<- agent.ToolResult) tea.Cmd {
	return func() tea.Msg {
		out, isErr := tools.Execute(name, rawIn)
		return toolResultMsg{
			result:  agent.ToolResult{Content: out, IsError: isErr},
			replyCh: replyCh,
		}
	}
}
