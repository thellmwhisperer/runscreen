package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thellmwhisperer/runscreen"
	"github.com/thellmwhisperer/runscreen/adapter/claude"
	"github.com/thellmwhisperer/runscreen/adapter/codex"
	"github.com/thellmwhisperer/runscreen/adapter/pi"
)

// readers is each CLI's reader, by its name.
var readers = map[string]func([]byte) []runscreen.Event{"claude": claude.Read, "codex": codex.Read, "pi": pi.Read}

// models is how each CLI's stream names its model, for those whose stream does.
var models = map[string]func([]byte) string{"claude": claude.Model, "pi": pi.Model}

// replay is a recording replayed line by line: the screen's pipeline, its Agent box and its Recording box.
type replay struct {
	name   string
	agent  runscreen.Agent
	lines  [][]byte
	start  time.Time
	delay  time.Duration
	read   func([]byte) []runscreen.Event
	model  func([]byte) string
	calls  []runscreen.Step
	ids    map[string]int // each tool call's sub-step, by its id
	types  []string       // each line's type, in order of first sight
	counts map[string]int // how many lines of each type
}

// load is the replay of the recording at path, written by agent's harness, or, when unnamed, the CLI its file
// name starts with; its line i read at start + i × delay.
func load(path string, agent runscreen.Agent, start time.Time, delay time.Duration) (*replay, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(path)
	if agent.Harness == "" {
		agent.Harness, _, _ = strings.Cut(strings.TrimSuffix(name, filepath.Ext(name)), "-")
	}
	read, ok := readers[agent.Harness]
	if !ok {
		return nil, fmt.Errorf("%s: no reader for %q: name claude, codex or pi with -harness", path, agent.Harness)
	}
	r := &replay{name: name, agent: agent, start: start, delay: delay, read: read, model: models[agent.Harness],
		ids: map[string]int{}, counts: map[string]int{}}
	for _, line := range bytes.Split(b, []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			r.lines = append(r.lines, line)
		}
	}
	return r, nil
}

// at is when line i is read.
func (r *replay) at(i int) time.Time { return r.start.Add(time.Duration(i) * r.delay) }

// first is what the screen is sent before the first line: the agent and the pipeline about to replay.
func (r *replay) first() []tea.Msg { return []tea.Msg{r.agent, r.pipeline(-1), r.boxes(-1)} }

// line is what the screen is sent once line i is read: its events, the model it names, the pipeline and the
// Recording box.
func (r *replay) line(i int) []tea.Msg {
	var msgs []tea.Msg
	for _, e := range r.read(r.lines[i]) {
		r.call(e, i)
		msgs = append(msgs, e)
	}
	if r.model != nil && r.agent.Model == "" {
		if model := r.model(r.lines[i]); model != "" {
			r.agent.Model = model
			msgs = append(msgs, r.agent)
		}
	}
	var typed struct{ Type string }
	_ = json.Unmarshal(r.lines[i], &typed)
	if r.counts[typed.Type]++; r.counts[typed.Type] == 1 {
		r.types = append(r.types, typed.Type)
	}
	return append(msgs, r.pipeline(i), r.boxes(i))
}

// last is what the screen is sent once every line is read: the pipeline passed.
func (r *replay) last() []tea.Msg { return []tea.Msg{r.pipeline(len(r.lines))} }

// call notes tool call e, read on line i, as a sub-step of the replay.
func (r *replay) call(e runscreen.Event, i int) {
	if e.Kind != runscreen.ToolCall {
		return
	}
	n, ok := r.ids[e.ID]
	if !ok {
		n, r.ids[e.ID] = len(r.calls), len(r.calls)
		r.calls = append(r.calls, runscreen.Step{Name: e.Tool, State: runscreen.Running, Start: r.at(i)})
	}
	c := &r.calls[n]
	if e.Detail != "" && !strings.Contains(c.Name, " ") {
		c.Name += " " + e.Detail
	}
	if e.Result == runscreen.Passed || e.Result == runscreen.Failed {
		c.State, c.End, c.Note = e.Result, r.at(i), e.Summary
	}
}

// pipeline is the pipeline once line i is read: -1 before the first, len(r.lines) once all are.
func (r *replay) pipeline(i int) runscreen.Pipeline {
	done := i >= len(r.lines)
	state := map[bool]runscreen.State{true: runscreen.Passed, false: runscreen.Running}[done]
	replay := runscreen.Step{Name: "Replay " + r.agent.Harness + "'s stream", State: state, Start: r.start,
		Note: fmt.Sprintf("%d tools", len(r.calls)), Steps: slices.Clone(r.calls)}
	end := runscreen.Step{Name: "Show the last frame", State: runscreen.Pending}
	if done {
		replay.End = r.at(len(r.lines) - 1)
		end.State, end.Start, end.End = runscreen.Passed, replay.End, replay.End
	}
	return runscreen.Pipeline{
		Title: r.name + ": a recorded " + r.agent.Harness + " run, replayed line by line",
		State: state,
		Steps: []runscreen.Step{
			{Name: "Read " + r.name, State: runscreen.Passed, Start: r.start, End: r.start,
				Note: fmt.Sprintf("%d lines", len(r.lines))},
			replay, end,
		},
	}
}

// boxes is the Recording box once line i is read: how many lines are read, and each type of line they had.
func (r *replay) boxes(i int) runscreen.Boxes {
	types := runscreen.Row{Text: fmt.Sprintf("Line types (%d)", len(r.types))}
	for _, t := range r.types {
		types.Rows = append(types.Rows, runscreen.Row{Text: fmt.Sprintf("`%s` ×%d", t, r.counts[t])})
	}
	read := runscreen.Row{Text: fmt.Sprintf("**%d** of %d lines read", i+1, len(r.lines))}
	return runscreen.Boxes{{Title: "Recording", Rows: []runscreen.Row{read, types}}}
}

// play sends the screen the whole replay, a line every delay.
func (r *replay) play(send func(tea.Msg)) {
	sendAll := func(msgs []tea.Msg) {
		for _, msg := range msgs {
			send(msg)
		}
	}
	sendAll(r.first())
	for i := range r.lines {
		time.Sleep(time.Until(r.at(i)))
		sendAll(r.line(i))
	}
	sendAll(r.last())
}
