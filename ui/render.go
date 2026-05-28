package ui

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

var (
	styleCode       = lipgloss.NewStyle().Foreground(lipgloss.Color("#D8DEE9"))
	styleCodeBorder = lipgloss.NewStyle().Foreground(lipgloss.Color("#4C566A"))
)

// RenderMD formats code blocks and list items; prose is word-wrapped to width.
func RenderMD(text string, width int) string {
	if width <= 0 {
		width = 80
	}
	proseW := max(width-2, 20)

	lines := strings.Split(strings.TrimSpace(text), "\n")
	var out strings.Builder
	var codeBuf []string
	codeLang := ""
	inCode := false

	flushCode := func() {
		label := codeLang
		if label == "" {
			label = "code"
		}
		out.WriteString("\n" + styleCodeBorder.Render("  ── "+label) + "\n")
		for _, l := range codeBuf {
			l = strings.ReplaceAll(l, "\t", "    ")
			out.WriteString(styleCode.Render("  "+l) + "\n")
		}
		out.WriteString(styleCodeBorder.Render("  ──") + "\n")
		codeBuf = nil
		codeLang = ""
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if !inCode {
				inCode = true
				codeLang = strings.TrimSpace(strings.TrimPrefix(line, "```"))
			} else {
				inCode = false
				flushCode()
			}
			continue
		}
		if inCode {
			codeBuf = append(codeBuf, line)
			continue
		}
		formatted := formatMDLine(line)
		wrapped := wordwrap.String(formatted, proseW)
		out.WriteString(wrapped + "\n")
	}
	if inCode && len(codeBuf) > 0 {
		flushCode()
	}
	return out.String()
}

func formatMDLine(line string) string {
	i := 0
	for i < len(line) && line[i] == ' ' {
		i++
	}
	rest := line[i:]
	depth := i / 2

	bullet := "•"
	if depth > 0 {
		bullet = "◦"
	}
	prefix := strings.Repeat("  ", depth)

	if len(rest) >= 2 && (rest[0] == '-' || rest[0] == '*' || rest[0] == '+') && rest[1] == ' ' {
		return prefix + bullet + " " + rest[2:]
	}
	j := 0
	for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
		j++
	}
	if j > 0 && j < len(rest)-1 && rest[j] == '.' && rest[j+1] == ' ' {
		return prefix + rest[:j+2] + rest[j+2:]
	}
	return line
}

// PrettyArg reformats raw JSON for display with consistent 2-space indent.
func PrettyArg(rawJSON string) string {
	var v any
	if err := json.Unmarshal([]byte(rawJSON), &v); err != nil {
		return "  " + rawJSON
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "  " + rawJSON
	}
	var sb strings.Builder
	for i, l := range strings.Split(string(b), "\n") {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString("  " + l)
	}
	return sb.String()
}
