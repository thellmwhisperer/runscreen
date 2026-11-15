package runscreen

import (
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// Show draws m on the terminal of in and out until q or Ctrl+C, then leaves its last frame there. feed, when not
// nil, runs beside it and sends m what to draw: a Pipeline, an Event, an Agent or Boxes.
func Show(m *Model, in io.Reader, out io.Writer, feed func(send func(tea.Msg))) error {
	p := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if feed != nil {
		go feed(p.Send)
	}
	// A terminal can give a grapheme more columns than the renderer does. Do not let
	// its automatic wrap move later rows and leave an earlier key line on screen.
	_, _ = fmt.Fprint(out, ansi.ResetAutoWrapMode)
	_, err := p.Run()
	_, _ = fmt.Fprint(out, ansi.SetAutoWrapMode)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, m.View()) // the alternate screen is gone: the last frame stays
	return err
}
