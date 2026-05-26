package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/ruiizy/elmi-harness/internal/perm"
)

type confirmState int

const (
	stateChoosing confirmState = iota
	stateTyping
)

var confirmOptions = []struct {
	label  string
	result perm.Result
}{
	{"Yes, run once", perm.ResultYes},
	{"No, deny", perm.ResultNo},
	{"Always allow this tool", perm.ResultAlways},
	{"Other — tell the model", perm.ResultOther},
}

type confirmModel struct {
	toolName string
	rawInput string
	cursor   int
	state    confirmState
	ti       textinput.Model
	result   perm.Result
	custom   string
}

func newConfirmModel(toolName, rawInput string) confirmModel {
	ti := textinput.New()
	ti.Placeholder = "Type your message to the model..."
	ti.CharLimit = 500
	return confirmModel{toolName: toolName, rawInput: rawInput, ti: ti}
}

func (m confirmModel) Init() tea.Cmd { return nil }

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.state == stateTyping {
		return m.updateTyping(msg)
	}
	return m.updateChoosing(msg)
}

func (m confirmModel) updateChoosing(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(confirmOptions)-1 {
				m.cursor++
			}
		case "enter", " ":
			if confirmOptions[m.cursor].result == perm.ResultOther {
				m.state = stateTyping
				m.ti.Focus()
				return m, textinput.Blink
			}
			m.result = confirmOptions[m.cursor].result
			return m, tea.Quit
		case "ctrl+c":
			m.result = perm.ResultNo
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m confirmModel) updateTyping(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.custom = strings.TrimSpace(m.ti.Value())
			if m.custom == "" {
				m.custom = "user declined without a reason"
			}
			m.result = perm.ResultOther
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.result = perm.ResultNo
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.ti, cmd = m.ti.Update(msg)
	return m, cmd
}

func (m confirmModel) View() string {
	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("Tool permission request") + "\n")
	sb.WriteString(StyleMuted.Render(fmt.Sprintf("  tool: %s", m.toolName)) + "\n")
	sb.WriteString(StyleMuted.Render(fmt.Sprintf("  args: %s", m.rawInput)) + "\n\n")

	if m.state == stateTyping {
		sb.WriteString(StylePrompt.Render("  Tell the model:") + "\n")
		sb.WriteString("  " + m.ti.View() + "\n")
		sb.WriteString(StyleDim.Render("\n  enter send  esc back") + "\n")
		return sb.String()
	}

	for i, opt := range confirmOptions {
		if i == m.cursor {
			sb.WriteString(StyleSel.Render("  > ") + StyleOption.Render(opt.label) + "\n")
		} else {
			sb.WriteString(StyleDim.Render("    "+opt.label) + "\n")
		}
	}
	sb.WriteString(StyleDim.Render("\n  ↑↓ navigate  enter select") + "\n")
	return sb.String()
}

// RunConfirmTUI shows the permission TUI and returns the user's decision.
// Opens /dev/tty directly to avoid racing with the bufio.Scanner on stdin.
func RunConfirmTUI(name, rawInput string) (perm.Result, string) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return perm.ResultNo, ""
	}
	defer tty.Close()

	m, err := tea.NewProgram(
		newConfirmModel(name, rawInput),
		tea.WithInput(tty),
		tea.WithOutput(tty),
	).Run()
	if err != nil {
		return perm.ResultNo, ""
	}
	final, ok := m.(confirmModel)
	if !ok {
		return perm.ResultNo, ""
	}
	return final.result, final.custom
}
