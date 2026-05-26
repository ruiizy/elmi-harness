package ui

import (
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

var spinnerFrames = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// Spinner animates a braille loading indicator on stdout.
type Spinner struct {
	stop chan struct{}
	done chan struct{}
	once sync.Once
}

// StartSpinner starts the spinner. Returns a no-op Spinner when stdout is
// not a TTY so piped output stays clean.
func StartSpinner(label string) *Spinner {
	s := &Spinner{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		close(s.done)
		return s
	}
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-s.stop:
				fmt.Print("\r\033[K")
				return
			case <-ticker.C:
				fmt.Printf("\r%s %s", StyleSpinner.Render(string(spinnerFrames[i])), StyleLabel.Render(label))
				i = (i + 1) % len(spinnerFrames)
			}
		}
	}()
	return s
}

// Stop clears the spinner line and blocks until the goroutine has exited.
// Safe to call multiple times.
func (s *Spinner) Stop() {
	s.once.Do(func() { close(s.stop) })
	<-s.done
}
