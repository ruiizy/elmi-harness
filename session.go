package main

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ruiizy/elmi-harness/internal/agent"
	"github.com/ruiizy/elmi-harness/internal/api"
	"github.com/ruiizy/elmi-harness/internal/perm"
	"github.com/ruiizy/elmi-harness/provider"
	"github.com/ruiizy/elmi-harness/ui"
)

// ── layout ────────────────────────────────────────────────────────────────────

const (
	taHeight     = 3
	sepHeight    = 1
	confirmAreaH = 6 // 3 options + nav hint + 2 padding lines
)

// ── session state ─────────────────────────────────────────────────────────────

type sessState int

const (
	sessIdle        sessState = iota
	sessBusy                  // agent goroutine running
	sessConfirm               // waiting for user to approve a tool
	sessModelPicker           // inline model picker open
)

// ── model ─────────────────────────────────────────────────────────────────────

type sessModel struct {
	vp    viewport.Model
	ta    textarea.Model
	sp    spinner.Model
	w, h  int
	ready bool

	// buf holds finalized conversation content (lipgloss-styled user/tool msgs +
	// glamour-rendered past AI responses). Pointer: strings.Builder must not be
	// copied by value (bubbletea copies the model on every Update).
	buf       *strings.Builder
	assistBuf []byte // current streaming AI response — raw markdown, copyable slice

	llm        provider.Provider
	permCfg    *perm.Config
	messages   []api.Message
	inputHist  []string
	histIdx    int
	histDraft  string
	ollamaBase string

	// ctx is the session-level context. Stored in struct because bubbletea
	// Update receives no extra params; cancelFn lets us abort the agent.
	ctx      context.Context
	cancelFn context.CancelFunc

	notice  string // transient status-bar notification
	state   sessState
	agentCh <-chan any
	pending *toolReqMsg

	pickerItems  []pickerItem
	pickerCursor int

	confirmCursor int

	// interrupt state: set on Esc during sessBusy; cleared by agentDoneMsg.
	interrupted       bool
	interruptedInput  string // input to restore to textarea on interrupt
	interruptedBufLen int    // buf.Len() before user message was written
}

const welcomeBanner = `
  ███████╗██╗     ███╗   ███╗██╗ ██████╗ ███╗   ██╗ █████╗
  ██╔════╝██║     ████╗ ████║██║██╔═══██╗████╗  ██║██╔══██╗
  █████╗  ██║     ██╔████╔██║██║██║   ██║██╔██╗ ██║███████║
  ██╔══╝  ██║     ██║╚██╔╝██║██║██║   ██║██║╚██╗██║██╔══██║
  ███████╗███████╗██║ ╚═╝ ██║██║╚██████╔╝██║ ╚████║██║  ██║
  ╚══════╝╚══════╝╚═╝     ╚═╝╚═╝ ╚═════╝ ╚═╝  ╚═══╝╚═╝  ╚═╝
`

// bannerMaxWidth returns the display-column width of the widest banner line.
func bannerMaxWidth() int {
	w := 0
	for _, line := range strings.Split(welcomeBanner, "\n") {
		if lw := lipgloss.Width(line); lw > w {
			w = lw
		}
	}
	return w
}

func newSessModel(ctx context.Context, llm provider.Provider, permCfg *perm.Config, ollamaBase string) sessModel {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.KeyMap.InsertNewline.SetEnabled(false)
	plain := lipgloss.NewStyle()
	ta.FocusedStyle.Base = plain
	ta.FocusedStyle.CursorLine = plain
	ta.FocusedStyle.Text = plain
	ta.BlurredStyle.Base = plain
	ta.BlurredStyle.CursorLine = plain
	ta.BlurredStyle.Text = plain
	ta.SetWidth(80)
	ta.SetHeight(taHeight)
	ta.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#88C0D0"))

	return sessModel{
		vp:         viewport.New(80, 20),
		ta:         ta,
		sp:         sp,
		llm:        llm,
		permCfg:    permCfg,
		ollamaBase: ollamaBase,
		ctx:        ctx,
		histIdx:    -1,
		buf:        new(strings.Builder),
	}
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m sessModel) Init() tea.Cmd { return textarea.Blink }

// vpContent returns the viewport string: finalized history + streaming text.
// Leading whitespace is stripped from the streaming portion so LLM preamble
// gaps (common after tool use) never show in the UI.
func (m sessModel) vpContent() string {
	if len(m.assistBuf) == 0 {
		return m.buf.String()
	}
	return m.buf.String() + strings.TrimLeftFunc(string(m.assistBuf), unicode.IsSpace)
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m sessModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Global Ctrl+C: abort agent, unblock any pending tool, quit cleanly.
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.Type == tea.KeyCtrlC || key.Type == tea.KeyCtrlD {
			if m.cancelFn != nil {
				m.cancelFn()
			}
			if m.pending != nil {
				// Non-blocking: buffered channel, agent goroutine may already be gone.
				select {
				case m.pending.replyCh <- agent.ToolResult{Content: "user aborted", IsError: true}:
				default:
				}
			}
			return m, tea.Quit
		}
	}

	// Global keys: scroll and copy (all states).
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.Type {
		case tea.KeyPgUp:
			m.vp.HalfPageUp()
			return m, nil
		case tea.KeyPgDown:
			m.vp.HalfPageDown()
			return m, nil
		case tea.KeyCtrlY:
			if text := lastAssistantText(m.messages); text != "" {
				m.notice = "copied!"
				return m, tea.Batch(copyCmd(text), noticeClearAfter(2*time.Second))
			}
			return m, nil
		}
	}

	switch msg := msg.(type) {

	case noticeClearMsg:
		m.notice = ""
		return m, nil

	case tea.WindowSizeMsg:
		prevReady := m.ready
		m.w, m.h = msg.Width, msg.Height
		m.ready = true
		if !prevReady && m.w > bannerMaxWidth() {
			m.buf.WriteString(styleWelcome.Render(welcomeBanner) + "\n\n")
		}
		m.vp.Width = m.w
		m.ta.SetWidth(m.w)
		switch m.state {
		case sessModelPicker:
			m.vp.Height = max(m.h-m.pickerAreaH()-sepHeight, 3)
		case sessConfirm:
			m.vp.Height = max(m.h-confirmAreaH-sepHeight, 3)
		default:
			m.vp.Height = max(m.h-taHeight-sepHeight, 1)
		}
		m.vp.SetContent(m.vpContent())
		m.vp.GotoBottom()
		return m, nil

	case chunkMsg:
		m.assistBuf = append(m.assistBuf, string(msg)...)
		m.vp.SetContent(m.vpContent())
		m.vp.GotoBottom()
		return m, listenAgent(m.agentCh)

	case toolReqMsg:
		// Finalize partial AI response (fast — pure string processing).
		if len(m.assistBuf) > 0 {
			m.buf.WriteString(ui.RenderMD(string(m.assistBuf), m.vp.Width))
			m.assistBuf = nil
		}
		if !m.permCfg.NeedsConfirm(msg.name) {
			m.buf.WriteString(styleDim.Render(fmt.Sprintf("[auto] %s", msg.name)) + "\n")
			m.vp.SetContent(m.vpContent())
			m.vp.GotoBottom()
			return m, execToolCmd(msg.name, msg.rawIn, msg.replyCh)
		}
		m.state = sessConfirm
		m.pending = &msg
		m.confirmCursor = 0
		m.buf.WriteString(styleTool.Render("  ⚙ "+msg.name) + "\n" +
			styleDim.Render(ui.PrettyArg(msg.rawIn)) + "\n")
		m.vp.Height = max(m.h-confirmAreaH-sepHeight, 3)
		m.vp.SetContent(m.vpContent())
		m.vp.GotoBottom()
		return m, nil

	case toolResultMsg:
		// Tool finished async — unblock agent goroutine and resume listening.
		msg.replyCh <- msg.result
		m.pending = nil
		return m, listenAgent(m.agentCh)

	case agentDoneMsg:
		if m.cancelFn != nil {
			m.cancelFn()
			m.cancelFn = nil
		}
		m.state = sessIdle
		m.agentCh = nil
		m.pending = nil
		m.vp.Height = max(m.h-taHeight-sepHeight, 1)

		if m.interrupted {
			// Esc was pressed: roll back buf and messages to pre-submit state,
			// restore the input text so the user can edit and resend.
			prev := m.buf.String()[:m.interruptedBufLen]
			m.buf.Reset()
			m.buf.WriteString(prev)
			m.assistBuf = nil
			if len(m.messages) > 0 {
				m.messages = m.messages[:len(m.messages)-1]
			}
			m.ta.SetValue(m.interruptedInput)
			m.ta.CursorEnd()
			m.interrupted = false
		} else {
			if len(m.assistBuf) > 0 {
				m.buf.WriteString(ui.RenderMD(string(m.assistBuf), m.vp.Width))
				m.buf.WriteString("\n")
				m.assistBuf = nil
			}
			if msg.err != nil {
				m.buf.WriteString(styleErr.Render(fmt.Sprintf("error: %v", msg.err)) + "\n\n")
			}
			m.messages = msg.updated
			if len(m.messages) > maxMessages {
				m.messages = m.messages[len(m.messages)-maxMessages:]
			}
		}
		m.vp.SetContent(m.vpContent())
		m.vp.GotoBottom()
		return m, nil

	case pickerReadyMsg:
		m.pickerItems = msg.items
		for i, it := range m.pickerItems {
			if !it.header {
				m.pickerCursor = i
				break
			}
		}
		m.vp.Height = max(m.h-m.pickerAreaH()-sepHeight, 3)
		m.vp.SetContent(m.vpContent())
		m.vp.GotoBottom()
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case sessConfirm:
			return m.updateConfirm(msg)
		case sessModelPicker:
			return m.updatePicker(msg)
		case sessBusy:
			if msg.Type == tea.KeyEsc && !m.interrupted {
				return m.interruptAgent()
			}
			return m, nil
		default:
			return m.updateIdle(msg)
		}

	case spinner.TickMsg:
		if m.state == sessBusy {
			var cmd tea.Cmd
			m.sp, cmd = m.sp.Update(msg)
			return m, cmd
		}
		return m, nil

	default:
		var vpCmd tea.Cmd
		m.vp, vpCmd = m.vp.Update(msg)
		return m, vpCmd
	}
}

func (m sessModel) updateIdle(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEnter && !msg.Alt:
		input := strings.TrimSpace(m.ta.Value())
		m.ta.SetValue("")
		m.histIdx = -1
		if input == "" {
			return m, nil
		}
		m.inputHist = append(m.inputHist, input)
		return m.handleInput(input)

	case msg.Type == tea.KeyEnter && msg.Alt:
		m.ta.InsertString("\n")
		return m, nil

	case msg.Type == tea.KeyUp && (m.ta.Value() == "" || m.histIdx != -1):
		if len(m.inputHist) == 0 {
			break
		}
		if m.histIdx == -1 {
			m.histDraft = m.ta.Value()
			m.histIdx = len(m.inputHist) - 1
		} else if m.histIdx > 0 {
			m.histIdx--
		}
		m.ta.SetValue(m.inputHist[m.histIdx])
		m.ta.CursorEnd()
		return m, nil

	case msg.Type == tea.KeyDown && m.histIdx != -1:
		m.histIdx++
		if m.histIdx >= len(m.inputHist) {
			m.histIdx = -1
			m.ta.SetValue(m.histDraft)
		} else {
			m.ta.SetValue(m.inputHist[m.histIdx])
		}
		m.ta.CursorEnd()
		return m, nil

	case msg.Type == tea.KeyTab:
		return m, nil
	}

	var taCmd tea.Cmd
	m.ta, taCmd = m.ta.Update(msg)
	return m, taCmd
}

// ── View ──────────────────────────────────────────────────────────────────────

// makeSep builds a full-width separator line. Uses lipgloss.Width() so ANSI
// codes and multi-byte Unicode in left/right don't corrupt the calculation.
func (m sessModel) makeSep(left, right string) string {
	if m.notice != "" {
		right = m.notice
	}
	leftPart := styleSepHi.Render(" " + left + " ")
	rightPart := styleDim.Render(right + " ")
	innerW := max(m.w-lipgloss.Width(leftPart)-lipgloss.Width(rightPart), 0)
	return leftPart + styleSep.Render(strings.Repeat("─", innerW)) + rightPart
}

func (m sessModel) View() string {
	if !m.ready {
		return "loading..."
	}
	switch m.state {
	case sessModelPicker:
		sep := m.makeSep("◎ model picker", "↑↓ · Enter · Esc")
		return m.vp.View() + "\n" + sep + "\n" + m.renderPicker()
	case sessBusy:
		label := m.sp.View() + " elmiona"
		if m.interrupted {
			label = "↩ cancelling…"
		}
		sep := m.makeSep(label, m.llm.Model())
		return m.vp.View() + "\n" + sep + "\n" + m.ta.View()
	case sessConfirm:
		sep := m.makeSep("⚙ "+m.pending.name, "↑↓ · Enter · Esc")
		return m.vp.View() + "\n" + sep + "\n" + m.renderConfirmPicker()
	default:
		sep := m.makeSep("⬡ elmiona", m.llm.Model()+" · "+m.permCfg.Mode.String())
		return m.vp.View() + "\n" + sep + "\n" + m.ta.View()
	}
}

// interruptAgent cancels the running agent and marks the session as interrupted.
// The actual rollback happens in the agentDoneMsg handler once the goroutine exits.
func (m sessModel) interruptAgent() (tea.Model, tea.Cmd) {
	if m.cancelFn != nil {
		m.cancelFn()
	}
	if m.pending != nil {
		select {
		case m.pending.replyCh <- agent.ToolResult{Content: "interrupted", IsError: true}:
		default:
		}
		m.pending = nil
	}
	m.interrupted = true
	return m, nil
}

// ── Entry point ───────────────────────────────────────────────────────────────

func RunSession(ctx context.Context, llm provider.Provider, permCfg *perm.Config, ollamaBase string) error {
	m := newSessModel(ctx, llm, permCfg, ollamaBase)
	_, err := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	).Run()
	return err
}
