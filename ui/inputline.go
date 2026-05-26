package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

var stdinFallback = bufio.NewReader(os.Stdin)

type inputLineModel struct {
	ti      textinput.Model
	history []string
	histIdx int    // -1 = editing current draft
	draft   string // saved when entering history navigation
	done    bool
	quit    bool
}

func newInputLineModel(prompt string, history []string) inputLineModel {
	ti := textinput.New()
	ti.Prompt = prompt
	ti.PromptStyle = StylePrompt
	ti.CharLimit = 4096
	ti.Focus()
	return inputLineModel{ti: ti, history: history, histIdx: -1}
}

func (m inputLineModel) Init() tea.Cmd { return textinput.Blink }

func (m inputLineModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.done = true
			return m, tea.Quit
		case tea.KeyCtrlC, tea.KeyCtrlD:
			m.quit = true
			return m, tea.Quit
		case tea.KeyUp:
			if len(m.history) == 0 {
				break
			}
			if m.histIdx == -1 {
				m.draft = m.ti.Value()
				m.histIdx = len(m.history) - 1
			} else if m.histIdx > 0 {
				m.histIdx--
			}
			m.ti.SetValue(m.history[m.histIdx])
			m.ti.CursorEnd()
			return m, nil
		case tea.KeyDown:
			if m.histIdx == -1 {
				break
			}
			m.histIdx++
			if m.histIdx >= len(m.history) {
				m.histIdx = -1
				m.ti.SetValue(m.draft)
			} else {
				m.ti.SetValue(m.history[m.histIdx])
			}
			m.ti.CursorEnd()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.ti, cmd = m.ti.Update(msg)
	return m, cmd
}

func (m inputLineModel) View() string {
	if m.done || m.quit {
		return "\r\033[K" // clear the line; caller reprints as permanent text
	}
	return m.ti.View()
}

// ReadLine shows an interactive prompt with ↑↓ history navigation.
// Returns ("", true) on Ctrl+C / Ctrl+D (quit signal).
// Falls back to plain stdin read when /dev/tty is unavailable.
func ReadLine(prompt string, history []string) (string, bool) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		fmt.Print(prompt)
		line, err := stdinFallback.ReadString('\n')
		if err != nil {
			return "", true
		}
		return strings.TrimRight(line, "\r\n"), false
	}
	defer tty.Close()

	m, err := tea.NewProgram(
		newInputLineModel(prompt, history),
		tea.WithInput(tty),
		tea.WithOutput(tty),
	).Run()
	if err != nil {
		return "", true
	}
	final, ok := m.(inputLineModel)
	if !ok || final.quit {
		fmt.Fprintln(os.Stdout)
		return "", true
	}
	value := strings.TrimSpace(final.ti.Value())
	// Reprint as permanent scrollback line after bubbletea clears its render.
	fmt.Fprintf(os.Stdout, "%s%s\n", prompt, value)
	return value, false
}
