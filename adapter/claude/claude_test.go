package claude

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/thellmwhisperer/runscreen"
)

// recorded is what Read makes of each line of the recording: an event a line, as line has it.
func recorded(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("../../testdata/recordings/claude.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range bytes.Split(b, []byte("\n")) {
		for _, e := range Read(l) {
			out = append(out, line(e))
		}
	}
	return out
}

// line is e on one line: what it says or thinks, its first 40 bytes, or its tool call.
func line(e runscreen.Event) string {
	if e.Kind != runscreen.ToolCall {
		text := strings.ReplaceAll(e.Text, "\n", `\n`)
		return fmt.Sprintf("%d %s", e.Kind, text[:min(40, len(text))])
	}
	return fmt.Sprintf("tool %s %q %q %s %q", e.ID[:min(12, len(e.ID))], e.Tool, e.Detail, e.Result, e.Summary)
}

func TestReadTurnsTheRecordedStreamIntoEvents(t *testing.T) {
	want := []string{
		`tool toolu_018sJT "Read" "/work/runscreen/doc.go" running ""`,
		`tool toolu_018Dmw "Read" "/work/runscreen/.golangci.yml" running ""`,
		`tool toolu_018sJT "" "" passed "1\t// Package runscreen draws the run screen of an agent pipeline: a pipeline` +
			` box of steps on the left and the"`,
		`tool toolu_018Dmw "" "" passed "1\tversion: \"2\""`,
		`tool toolu_019jnA "Bash" "go vet ./..." running ""`,
		`tool toolu_019jnA "" "" passed "(Bash completed with no output)"`,
		"0 - **Context:** `go vet ./...` came back ",
	}
	if got := recorded(t); !reflect.DeepEqual(got, want) {
		t.Fatalf("events:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestModelIsTheInitLines(t *testing.T) {
	b, err := os.ReadFile("../../testdata/recordings/claude.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(b, []byte("\n"))
	if got := Model(lines[0]); got != "claude-sonnet-5-5" {
		t.Fatalf("model %q", got)
	}
	if got := Model(lines[1]); got != "" {
		t.Fatalf("an assistant line names no model: %q", got)
	}
	if got := Model([]byte("not json")); got != "" {
		t.Fatalf("a line that is not JSON names no model: %q", got)
	}
}

// No recording carries thinking text or a failed call: these lines do.
func TestReadThinkingAndAFailedCall(t *testing.T) {
	thinking := `{"type":"assistant","message":{"role":"assistant","content":[` +
		`{"type":"thinking","thinking":"Plan the fix.\nThen test.","signature":"x"},{"type":"thinking","thinking":""}]}}`
	failed := `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1",` +
		`"is_error":true,"content":[{"type":"text","text":"exit status 1\nmore"}]}]}}`
	prompt := `{"type":"user","message":{"role":"user","content":[{"type":"text","text":"a prompt"}]}}`
	got := append(Read([]byte(thinking)), append(Read([]byte(failed)), Read([]byte(prompt))...)...)
	want := []runscreen.Event{{Kind: runscreen.Thinks, Text: "Plan the fix.\nThen test."},
		{Kind: runscreen.ToolCall, ID: "t1", Result: runscreen.Failed, Summary: "exit status 1"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events:\n%#v\nwant:\n%#v", got, want)
	}
}
