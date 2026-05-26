package ui

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

const banner = `
███████╗██╗     ███╗   ███╗██╗ ██████╗ ███╗   ██╗ █████╗
██╔════╝██║     ████╗ ████║██║██╔═══██╗████╗  ██║██╔══██╗
█████╗  ██║     ██╔████╔██║██║██║   ██║██╔██╗ ██║███████║
██╔══╝  ██║     ██║╚██╔╝██║██║██║   ██║██║╚██╗██║██╔══██║
███████╗███████╗██║ ╚═╝ ██║██║╚██████╔╝██║ ╚████║██║  ██║
╚══════╝╚══════╝╚═╝     ╚═╝╚═╝ ╚═════╝ ╚═╝  ╚═══╝╚═╝  ╚═╝`

// TermWidth returns the terminal width, or 0 on error/non-TTY (pipe, redirect).
func TermWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 0
	}
	return w
}

// PrintBanner prints the wordmark. Falls back to a one-liner when the
// terminal is too narrow or stdout is not a TTY.
func PrintBanner() {
	if TermWidth() >= 62 {
		fmt.Printf("\033[1;36m%s\033[0m\n", banner)
		fmt.Print("\033[90m  build your own coding agent\033[0m\n\n")
	} else {
		fmt.Println("  elmiona  ·  build your own coding agent")
		fmt.Println()
	}
}
