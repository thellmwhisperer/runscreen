package runscreen

import (
	"bytes"
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestShowDrawsWhatItIsFedUntilQAndLeavesItsLastFrame(t *testing.T) {
	in, keys := io.Pipe()
	var out bytes.Buffer
	m := New(100, 30)
	fed := make(chan struct{})
	err := Show(m, in, &out, func(send func(tea.Msg)) {
		send(Pipeline{Title: "shown", State: Passed, Steps: []Step{{Name: "Build", State: Passed}}})
		send(Agent{Harness: "pi"})
		send(Event{Kind: Says, Text: "Done."})
		send(Boxes{{Title: "Gate", Rows: []Row{{Text: "ok"}}}})
		close(fed)
		go func() { _, _ = keys.Write([]byte("q")) }()
	})
	<-fed
	if err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "\x1b[?1049h") || !strings.Contains(s, "\x1b[?1049l") || !strings.Contains(s, "\x1b[?1002h") {
		t.Fatalf("want the alternate screen and the mouse:\n%q", s)
	}
	last := ansi.Strip(s[strings.LastIndex(s, "\x1b[?1049l"):])
	for _, want := range []string{"╭─ Pipeline", "│ shown", "✓ Build", "╭─ Agent · pi", "✎ Done.", "╭─ Gate", "│   ok",
		"q quit"} {
		if !strings.Contains(last, want) {
			t.Fatalf("want %q in the last frame left on the terminal:\n%s", want, last)
		}
	}
}

func TestShowReturnsTheTerminalsError(t *testing.T) {
	if err := Show(New(80, 24), failing{}, io.Discard, nil); err == nil {
		t.Fatal("want the input's error")
	}
}

type failing struct{}

func (failing) Read([]byte) (int, error) { return 0, io.ErrClosedPipe }
