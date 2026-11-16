// Command pantalla shows a recorded run's screen the way atm attach <run> does on a terminal.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"

	"github.com/thellmwhisperer/agentic-team-member/internal/run"
	"github.com/thellmwhisperer/agentic-team-member/internal/tui"
)

// pantalla <root>/.atm/runs/<run>
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: pantalla <root>/.atm/runs/<run>")
		os.Exit(2)
	}
	dir, err := filepath.Abs(os.Args[1])
	if err == nil {
		err = attach(filepath.Dir(filepath.Dir(filepath.Dir(dir))), filepath.Base(dir))
	}
	// As internal/cli's Execute ends.
	if err != nil {
		fmt.Fprintln(os.Stderr, "atm:", err)
	}
	os.Exit(run.ExitCode(err))
}

// attach is internal/cli's attach with w = os.Stdout.
func attach(root, label string) error {
	f := os.Stdout
	if !term.IsTerminal(int(f.Fd())) || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		o, err := tui.Attach(root, label, f)
		if err != nil {
			return err
		}
		return o.Err()
	}
	o, detached, err := tui.Show(root, label, os.Stdin, f)
	switch {
	case err != nil:
		return err
	case detached:
		_, err = fmt.Fprintln(f, "the run goes on:", strings.TrimSpace("atm attach "+label))
		return err
	}
	return o.Err()
}
