//go:build linux || darwin

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/thellmwhisperer/runscreen"
)

func TestMain(m *testing.M) {
	code := m.Run()
	if bin, err := build(); err == nil {
		_ = os.RemoveAll(filepath.Dir(bin))
	}
	os.Exit(code)
}

// at is where m, drawn, shows text: its column and its row.
func at(t *testing.T, m *runscreen.Model, text string) (int, int) {
	t.Helper()
	lines := strings.Split(ansi.Strip(m.View()), "\n")
	y := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, text) })
	if y < 0 {
		t.Fatalf("no row with %q:\n%s", text, strings.Join(lines, "\n"))
	}
	return ansi.StringWidth(lines[y][:strings.Index(lines[y], text)]), y
}

// mouse is button pressed at x, y, as the terminal writes it.
func mouse(button, x, y int) string { return fmt.Sprintf("\x1b[<%d;%d;%dM", button, x+1, y+1) }

const (
	left                   = 0
	up, down, pgUp, pgDown = "\x1b[A", "\x1b[B", "\x1b[5~", "\x1b[6~"
	home, end, tab         = "\x1b[H", "\x1b[F", "\t"
)

func TestTheDemoReplaysEachRecordingOnATerminal(t *testing.T) {
	for _, rec := range []struct{ name, title, lines string }{
		{"claude", "Agent · claude · claude-sonnet-5-5 · high", "10 of 10 lines read"},
		{"codex", "Agent · codex · gpt-6.1-sol · high", "9 of 9 lines read"},
		{"pi", "Agent · pi · openai-codex/gpt-6.1-sol · high", "106 of 106 lines read"},
	} {
		t.Run(rec.name, func(t *testing.T) {
			args := []string{"-delay", "1ms", "-effort", "high", "../../testdata/recordings/" + rec.name + ".jsonl"}
			if rec.name == "codex" {
				args = append([]string{"-model", "gpt-6.1-sol"}, args...)
			}
			d := start(t, 200, 60, args...)
			d.until(0, rec.lines, "✓ Show the last frame")
			last := d.quit()
			for _, want := range []string{rec.title, rec.lines, "▸ · ", "✎ ", "✓ passed", "q quit  t show tool use"} {
				if !strings.Contains(last, want) {
					t.Fatalf("want %q in the last frame:\n%s", want, last)
				}
			}
		})
	}
}

func runes(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func TestTheDemoFoldsWhatAClickOrAKeyFolds(t *testing.T) {
	d := start(t, 120, 40, "-delay", "1ms", "-effort", "high", "../../testdata/recordings/claude.jsonl")
	d.m = replayed(t, "claude", runscreen.Agent{Effort: "high"}, 120, 40)
	d.until(0, "10 of 10 lines read", "✓ Show the last frame")
	d.click("▸ · 3 tools: Read ×2, Bash", "▾ · 3 tools", "✓ ok Read /work/runscreen/doc.go", "✓ ok Bash go vet ./...")
	d.click("▾ · 3 tools: Read ×2, Bash", "▸ · 3 tools: Read ×2, Bash")
	d.act("t", runes('t'), "t hide tool use", "✓ ok Read /work/runscreen/.golangci.yml")
	d.act("t", runes('t'), "t show tool use", "▸ · 3 tools: Read ×2, Bash")
	d.act("h", runes('h'), "h hide thinking")
	d.act("h", runes('h'), "h show thinking")
	d.click("▸ Line types (4)", "▾ Line types (4)", "system ×1", "assistant ×5", "user ×3", "result ×1")
	d.act(mouse(left, 2, 1), tea.MouseMsg{X: 2, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress},
		"▾ claude.jsonl: a recorded", "claude run, replayed line")
	d.act("i", runes('i'), "▸ claude.jsonl: a recorded")
	d.act(tab, tea.KeyMsg{Type: tea.KeyTab}, "Recording*")
	d.act(tab, tea.KeyMsg{Type: tea.KeyTab}, "Pipeline*")
	d.act(tab, tea.KeyMsg{Type: tea.KeyTab}, "Agent · claude · claude-sonnet-5-5 · high*")
	if last := d.quit(); !strings.Contains(last, "q quit") || !strings.Contains(last, "▾ Line types (4)") {
		t.Fatalf("the last frame:\n%s", last)
	}
}

func TestTheDemoScrollsAndFollowsTheTerminalsSize(t *testing.T) {
	d := start(t, 120, 40, "-delay", "1ms", "../../testdata/recordings/claude.jsonl")
	d.m = replayed(t, "claude", runscreen.Agent{}, 80, 24)
	d.until(0, "10 of 10 lines read", "✓ Show the last frame")
	resize(t, d.tty, 80, 24)
	d.until(d.out.len(), "│ claude.jsonl: a recorded claude run, replayed line by line")
	d.key(home, tea.KeyMsg{Type: tea.KeyHome})
	if top := d.agentTop(); top != "▸ · 3 tools: Read ×2, Bash" {
		t.Fatalf("home goes to the Agent box's first row, not %q", top)
	}
	d.key(pgDown, tea.KeyMsg{Type: tea.KeyPgDown})
	d.key(up, tea.KeyMsg{Type: tea.KeyUp})
	d.key(pgUp, tea.KeyMsg{Type: tea.KeyPgUp})
	d.key(down, tea.KeyMsg{Type: tea.KeyDown})
	d.key(end, tea.KeyMsg{Type: tea.KeyEnd})
	_, y := at(t, d.m, "╭─ Agent")
	d.wheel(tea.MouseButtonWheelUp, 40, y+2, 10)
	d.wheel(tea.MouseButtonWheelDown, 40, y+2, 10)
	if last := d.quit(); !strings.Contains(last, "ranked it last") || strings.Contains(last, "╮  ╭") {
		t.Fatalf("the last frame, stacked at 80 columns, at its latest:\n%s", last)
	}
}

func TestTheDemoFoldsThinkingAndShowsAFailedCall(t *testing.T) {
	// No recording carries thinking: this stream, in claude's shape, does.
	stream := strings.Join([]string{
		`{"type":"system","subtype":"init","model":"claude-test"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking",` +
			`"thinking":"Plan the **fix**.\nThen test it."}]}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Bash",` +
			`"input":{"command":"go test ./..."}}]}}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1",` +
			`"is_error":true,"content":"FAIL TestX\nmore"}]}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Done."}]}}`,
	}, "\n")
	path := filepath.Join(t.TempDir(), "claude-thinking.jsonl")
	if err := os.WriteFile(path, []byte(stream), 0o600); err != nil {
		t.Fatal(err)
	}
	d := start(t, 120, 40, "-delay", "1ms", path)
	d.m = replica(t, path, runscreen.Agent{}, 120, 40)
	d.until(0, "Agent · claude · claude-test", "▸ ∴ Plan the fix.", "✗ failed Bash go test ./... · FAIL TestX", "✎ Done.")
	d.act("h", runes('h'), "▾ ∴ Plan the fix.", "Then test it.", "h hide thinking")
	d.act("h", runes('h'), "▸ ∴ Plan the fix.")
	d.click("▸ ∴ Plan the fix.", "▾ ∴ Plan the fix.", "Then test it.")
	d.click("▾ ∴ Plan the fix.", "▸ ∴ Plan the fix.")
	if last := d.quit(); !strings.Contains(last, "✗ Bash go test ./...") || strings.Contains(last, "more") {
		t.Fatalf("the failed call's sub-step, its first line only:\n%s", last)
	}
}

func TestTheDemoSaysWhatItCannotReplay(t *testing.T) {
	notes := filepath.Join(t.TempDir(), "notes.jsonl")
	if err := os.WriteFile(notes, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		args []string
		want string
	}{
		{nil, "demo: usage: demo [flags] <recording>"},
		{[]string{"-nope", "x"}, "flag provided but not defined: -nope"},
		{[]string{filepath.Join(t.TempDir(), "missing.jsonl")}, "no such file or directory"},
		{[]string{notes}, `no reader for "notes": name claude, codex or pi with -harness`},
	} {
		out, err := command(t, c.args...).CombinedOutput()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 || !strings.Contains(string(out), c.want) {
			t.Fatalf("demo %q: %v, want exit 2 and %q:\n%s", c.args, err, c.want, out)
		}
	}
}
