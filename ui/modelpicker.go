package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ModelChoice is the result of RunModelPicker.
type ModelChoice struct {
	Provider string // "anthropic" | "ollama"
	Model    string
}

type modelItem struct {
	provider string
	name     string
	header   bool // true → section header, not selectable
	label    string
}

var anthropicModels = []string{
	"claude-opus-4-7-20250514",
	"claude-sonnet-4-6",
	"claude-haiku-4-5-20251001",
}

// fetchOllamaModels hits the Ollama tags API. Returns nil on any error.
func fetchOllamaModels(baseURL string) []string {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(baseURL + "/api/tags")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var out struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil
	}
	names := make([]string, len(out.Models))
	for i, m := range out.Models {
		names[i] = m.Name
	}
	return names
}

func buildItems(ollamaBase, currentModel string) []modelItem {
	var items []modelItem

	items = append(items, modelItem{header: true, label: "Anthropic"})
	for _, m := range anthropicModels {
		items = append(items, modelItem{provider: "anthropic", name: m})
	}

	ollamaModels := fetchOllamaModels(ollamaBase)
	if len(ollamaModels) > 0 {
		items = append(items, modelItem{header: true, label: "Ollama (local)"})
		for _, m := range ollamaModels {
			items = append(items, modelItem{provider: "ollama", name: m})
		}
	}
	return items
}

// — bubbletea model -----------------------------------------------------------

type pickerModel struct {
	items        []modelItem
	cursor       int // points to a selectable item
	currentModel string
	chosen       *ModelChoice
}

func newPickerModel(ollamaBase, currentModel string) pickerModel {
	items := buildItems(ollamaBase, currentModel)
	// start cursor on first selectable item
	cursor := 0
	for i, it := range items {
		if !it.header {
			cursor = i
			break
		}
	}
	return pickerModel{items: items, cursor: cursor, currentModel: currentModel}
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.cursor = m.prevSelectable(m.cursor)
		case "down", "j":
			m.cursor = m.nextSelectable(m.cursor)
		case "enter", " ":
			it := m.items[m.cursor]
			m.chosen = &ModelChoice{Provider: it.provider, Model: it.name}
			return m, tea.Quit
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m pickerModel) nextSelectable(from int) int {
	for i := from + 1; i < len(m.items); i++ {
		if !m.items[i].header {
			return i
		}
	}
	return from
}

func (m pickerModel) prevSelectable(from int) int {
	for i := from - 1; i >= 0; i-- {
		if !m.items[i].header {
			return i
		}
	}
	return from
}

func (m pickerModel) View() string {
	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("Select model") + "\n\n")
	for i, it := range m.items {
		if it.header {
			sb.WriteString(StylePrompt.Render("  "+it.label) + "\n")
			continue
		}
		active := m.currentModel == it.name
		suffix := ""
		if active {
			suffix = StyleDim.Render(" (current)")
		}
		if i == m.cursor {
			sb.WriteString(StyleSel.Render("  > ") + StyleOption.Render(it.name) + suffix + "\n")
		} else {
			sb.WriteString(StyleDim.Render("    "+it.name) + suffix + "\n")
		}
	}
	sb.WriteString(StyleDim.Render("\n  ↑↓ navigate  enter select  q cancel") + "\n")
	return sb.String()
}

// RunModelPicker shows the model picker TUI and returns the chosen model.
// Returns nil if the user cancelled. ollamaBase should be "http://localhost:11434".
func RunModelPicker(ollamaBase, currentModel string) *ModelChoice {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot open tty: %v\n", err)
		return nil
	}
	defer tty.Close()

	m, err := tea.NewProgram(
		newPickerModel(ollamaBase, currentModel),
		tea.WithInput(tty),
		tea.WithOutput(tty),
	).Run()
	if err != nil {
		return nil
	}
	pm, ok := m.(pickerModel)
	if !ok {
		return nil
	}
	return pm.chosen
}
