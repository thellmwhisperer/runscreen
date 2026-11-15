package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/thellmwhisperer/runscreen"
	"github.com/thellmwhisperer/runscreen/markdown"
)

var update = flag.Bool("update", false, "write the golden frames from what the screen draws")

// recordings is each recording and the agent as testdata/recordings/README.md says it ran, what its stream does
// not name.
var recordings = []struct {
	name  string
	agent runscreen.Agent
}{
	{"claude", runscreen.Agent{Effort: "high"}},
	{"codex", runscreen.Agent{Model: "gpt-6.1-sol", Effort: "high"}},
	{"pi", runscreen.Agent{Effort: "high"}},
}

var t0 = time.Date(2026, 11, 15, 4, 15, 0, 0, time.UTC)

// replayed is the screen at width by height once the recording name is replayed whole, a line a second.
func replayed(t *testing.T, name string, agent runscreen.Agent, width, height int) *runscreen.Model {
	t.Helper()
	return replica(t, filepath.Join("..", "..", "testdata", "recordings", name+".jsonl"), agent, width, height)
}

// replica is the screen at width by height once the stream at path is replayed whole, a line a second.
func replica(t *testing.T, path string, agent runscreen.Agent, width, height int) *runscreen.Model {
	t.Helper()
	r, err := load(path, agent, t0, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	m := runscreen.New(width, height)
	m.SetMarkdown(markdown.Inline)
	msgs := r.first()
	for i := range r.lines {
		msgs = append(msgs, r.line(i)...)
	}
	for _, msg := range append(msgs, r.last()...) {
		m.Update(msg)
	}
	return m
}

func TestTheLastFrameOfEachRecordingMatchesItsGolden(t *testing.T) {
	for _, rec := range recordings {
		for _, size := range [][2]int{{80, 24}, {120, 40}, {200, 60}} {
			t.Run(fmt.Sprintf("%s-%dx%d", rec.name, size[0], size[1]), func(t *testing.T) {
				frame := ansi.Strip(replayed(t, rec.name, rec.agent, size[0], size[1]).View())
				path := filepath.Join("..", "..", "testdata", fmt.Sprintf("%s-%dx%d.golden", rec.name, size[0], size[1]))
				if *update {
					if err := os.WriteFile(path, []byte(frame+"\n"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				want, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if frame+"\n" != string(want) {
					t.Fatalf("frame:\n%s\nwant %s:\n%s", frame, path, want)
				}
			})
		}
	}
}

func TestTheReplayCountsItsLinesAndToolCalls(t *testing.T) {
	m := replayed(t, "claude", runscreen.Agent{}, 120, 40)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	frame := ansi.Strip(m.View())
	for _, want := range []string{"10 lines", "3 tools", "10 of 10 lines read", "Agent · claude · claude-sonnet-5-5",
		"✓ ok Bash go vet ./... · (Bash completed with no output)"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("want %q:\n%s", want, frame)
		}
	}
}
