// Command demo replays a recorded agent stream on the run screen: go run ./cmd/demo testdata/recordings/claude.jsonl.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/thellmwhisperer/runscreen"
	"github.com/thellmwhisperer/runscreen/markdown"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "demo:", err)
		os.Exit(2)
	}
}

// run replays the recording args name on the terminal of in and out until q.
func run(args []string, in io.Reader, out io.Writer) error {
	fs := flag.NewFlagSet("demo", flag.ContinueOnError)
	var agent runscreen.Agent
	fs.StringVar(&agent.Harness, "harness", "", "the CLI that wrote the recording: claude, codex or pi; "+
		"by default, the start of its file name")
	fs.StringVar(&agent.Model, "model", "", "the agent's model; by default, the one its stream names")
	fs.StringVar(&agent.Effort, "effort", "", "the agent's effort")
	delay := fs.Duration("delay", 300*time.Millisecond, "the time between two lines of the recording")
	plain := fs.Bool("plain", false, "show what the agent says as written, without rendering its Markdown")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: demo [flags] <recording>")
	}
	r, err := load(fs.Arg(0), agent, time.Now(), *delay)
	if err != nil {
		return err
	}
	m := runscreen.New(80, 24)
	if !*plain {
		m.SetMarkdown(markdown.Inline)
	}
	return runscreen.Show(m, in, out, r.play)
}
