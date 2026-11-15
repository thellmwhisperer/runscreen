package runscreen

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func summed(e Event, summary string) Event {
	e.Summary = summary
	return e
}

func TestScreenShowsWhatTheAgentSaysBetweenItsDimmerToolCalls(t *testing.T) {
	m := agentScreen(120, says("I'll run the tests.\nThen fix."), call("t1", "Bash", "go test ./...", Running),
		Event{Kind: ToolCall, ID: "t1", Result: Passed}, says("Green; done."))
	if s := view(m); !order(s, "╭─ Agent", "✎ I'll run the tests.", "▸ · 1 tool: Bash", "✎ Green; done.") {
		t.Fatalf("agent box:\n%s", s)
	}
	if s := m.View(); !strings.Contains(s, paint(dim, "· 1 tool: Bash")) ||
		strings.Contains(s, paint(dim, "✎ I'll run the tests.")) {
		t.Fatalf("want the tool calls dim and the prose not:\n%q", s)
	}
}

func TestAgentBoxFoldsToolCallsBetweenWhatTheAgentSaysIntoOneDimLine(t *testing.T) {
	m := agentScreen(120, thinks("Plan the fix."), says("Writing the red tests first."),
		call("t1", "Edit", "model_test.go", Passed), call("t2", "Bash", "go test ./...", Passed),
		call("t3", "Bash", "go vet ./...", Passed), says("Red on behaviour. Now the fix."),
		call("t4", "Bash", "go build ./...", Failed), call("t5", "Read", "view.go\nmodel.go", Running))
	s := view(m)
	if !order(s, "∴ Plan the fix.", "✎ Writing the red tests first.", "▸ · 3 tools: Edit, Bash ×2",
		"✎ Red on behaviour. Now the fix.", "✗ failed Bash go build ./...", "⠋ 1 tool: Read · view.go") ||
		strings.Contains(s, "✓ passed Bash") || strings.Contains(s, "model.go") {
		t.Fatalf("agent box:\n%s", s)
	}
	if s := m.View(); !strings.Contains(s, paint(dim, "· 3 tools: Edit, Bash ×2")) {
		t.Fatalf("want the tool batch dim:\n%q", s)
	}
	s = view(agentScreen(120, says("Working."), call("t1", "skill", "", Running)))
	if !strings.Contains(s, "⠋ 1 tool: skill") || strings.Contains(s, "⠋ 1 tool: skill ·") {
		t.Fatalf("want an empty-detail running tool to keep its spinner without a detail suffix:\n%s", s)
	}
}

func TestAgentBoxFillsTheColumnAndWrapsWhatTheAgentSaysToItsLatestLine(t *testing.T) {
	lines := strings.Split(view(agentScreen(120, says("Working."))), "\n")
	if len(lines) < 4 || len(lines) > 40 || !strings.Contains(lines[len(lines)-3], "╰") ||
		!strings.Contains(lines[len(lines)-1], "q quit") {
		t.Fatalf("want the Agent box down to the key line:\n%s", strings.Join(lines, "\n"))
	}
	words := strings.Repeat("lorem ipsum ", 16) + "omega" // three rows of the 74 columns inside the box
	s := view(agentScreen(120, says(words)))
	rows := regexp.MustCompile(`(?m)│ ✎ lorem.*│\n.*│   lorem.*│\n.*│   .*omega +│$`)
	if strings.Contains(s, "…") || !rows.MatchString(s) {
		t.Fatalf("want the long line whole over three rows:\n%s", s)
	}
	var evs []Event
	for i := range 60 {
		evs = append(evs, thinks(fmt.Sprintf("thought %d", i)))
	}
	lines = strings.Split(view(agentScreen(120, evs...)), "\n")
	if !strings.Contains(lines[35], "∴ thought 59 ") || !strings.Contains(lines[36], "╰") {
		t.Fatalf("want the last line on the box's last row:\n%s", strings.Join(lines, "\n"))
	}
}

func TestStackedAgentBoxFitsAboveTheFooter(t *testing.T) {
	lines := strings.Split(view(screen(80, 40, unit, says("Working."))), "\n")
	if len(lines) > 40 || !strings.Contains(lines[len(lines)-3], "╰") || !strings.Contains(lines[len(lines)-1], "q quit") {
		t.Fatalf("want the stacked Agent box above the footer:\n%s", strings.Join(lines, "\n"))
	}
}

func TestAgentBoxShowsEveryLogicalLine(t *testing.T) {
	if s := view(agentScreen(120, says("first line\nsecond line"))); !order(s, "✎ first line", "  second line") {
		t.Fatalf("want every line the agent says:\n%s", s)
	}
}

func TestSayingNothingAddsNothing(t *testing.T) {
	m := agentScreen(120, says(""), thinks(""))
	if len(m.agent) != 0 {
		t.Fatalf("empty events made rows: %d", len(m.agent))
	}
}

func TestTUnfoldsAndFoldsEveryToolCall(t *testing.T) {
	m := agentScreen(240, says("Reading first."), call("a", "read", "AGENTS.md", Passed), thinks("Exploring."),
		call("b", "bash", "ls packages", Running))
	today := agentLines(m, 240)
	if !slices.Contains(today, "▸ · 1 tool: read") {
		t.Fatalf("folded lines changed: %q", today)
	}
	pressT(m)
	got := agentLines(m, 240)
	if !slices.ContainsFunc(got, func(l string) bool { return strings.Contains(l, "ok read AGENTS.md") }) ||
		!slices.ContainsFunc(got, func(l string) bool { return strings.Contains(l, "running bash ls packages") }) {
		t.Fatalf("t did not unfold every call:\n%q", got)
	}
	pressT(m)
	if got := agentLines(m, 240); !reflect.DeepEqual(got, today) {
		t.Fatalf("second t did not fold back:\n%q\nwant:\n%q", got, today)
	}
}

func TestUnfoldedCallShowsItsSummaryOrNone(t *testing.T) {
	m := agentScreen(240, summed(call("c1", "bash", "ls", Passed), "a.go"), call("c2", "read", "b.go", Passed))
	pressT(m)
	want := []string{"▾ · 2 tools: bash, read", "✓ ok bash ls · a.go", "✓ ok read b.go"}
	if got := agentLines(m, 240); !reflect.DeepEqual(got, want) {
		t.Fatalf("unfolded lines:\n%q\nwant:\n%q", got, want)
	}
}

func TestUnfoldedCallIsCutToThePanel(t *testing.T) {
	m := agentScreen(40, summed(call("c1", "bash", "ls", Passed), strings.Repeat("x", 100)))
	pressT(m)
	for _, l := range strings.Split(m.agentBox(40, 10), "\n") {
		if w := ansi.StringWidth(l); w != 40 {
			t.Fatalf("row %q is %d wide, want 40", ansi.Strip(l), w)
		}
	}
}

func TestClickUnfoldsOneGroupAndTFoldsOrUnfoldsAll(t *testing.T) {
	m := agentScreen(160, summed(call("c1", "bash", "echo a\necho b\necho c", Passed), "a\nb"), says("next"),
		summed(call("c2", "read", "x.go", Passed), "package x"))
	view(m)
	click(t, m, "1 tool: bash")
	s := view(m)
	if !strings.Contains(s, "ok bash echo a · a") || strings.Contains(s, "echo b") || strings.Contains(s, "x.go") {
		t.Fatalf("a click unfolds its group only, each call on one row:\n%s", s)
	}
	lines := strings.Split(s, "\n")
	at := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "ok bash echo a") })
	if !strings.Contains(lines[at+1], "✎ next") {
		t.Fatalf("the call takes more than one row:\n%s", s)
	}
	clickRow(m, at+2) // the next group's line
	if s := view(m); !strings.Contains(s, "ok read x.go · package x") {
		t.Fatalf("a click on the next group's row did not unfold it:\n%s", s)
	}
	clickRow(m, at) // a call: nothing
	if s := view(m); !strings.Contains(s, "ok bash echo a") || !strings.Contains(s, "ok read x.go") {
		t.Fatalf("a click on a call changed the box:\n%s", s)
	}
	click(t, m, "1 tool: read")
	if s := view(m); strings.Contains(s, "ok read x.go") || !strings.Contains(s, "ok bash echo a") {
		t.Fatalf("a click on an unfolded group's line folds only it:\n%s", s)
	}
	pressT(m)
	if s := view(m); !strings.Contains(s, "ok bash echo a") || !strings.Contains(s, "ok read x.go") {
		t.Fatalf("t did not unfold every call:\n%s", s)
	}
	pressT(m)
	if s := view(m); strings.Contains(s, "ok bash") || strings.Contains(s, "ok read") {
		t.Fatalf("a second t did not fold every call:\n%s", s)
	}
}

func TestCallClickedOpenWhileRunningShowsItsErrorOnceItFails(t *testing.T) {
	m := agentScreen(160, call("c1", "bash", "go test", Running))
	view(m)
	click(t, m, "1 tool: bash")
	if s := view(m); !strings.Contains(s, "running bash go test") {
		t.Fatalf("running call not unfolded:\n%s", s)
	}
	feed(m, summed(Event{Kind: ToolCall, ID: "c1", Result: Failed}, "FAIL TestX\nmore"))
	if s := view(m); !strings.Contains(s, "✗ failed bash go test · FAIL TestX") || strings.Contains(s, "more") {
		t.Fatalf("failed call without its error:\n%s", s)
	}
}

func TestFooterAnnouncesTWhileTheAgentBoxShows(t *testing.T) {
	m := screen(160, 40, unit)
	if s := view(m); !strings.Contains(s, "q quit") || strings.Contains(s, "tool use") {
		t.Fatalf("footer announces t without the Agent box:\n%s", s)
	}
	feed(m, Agent{Harness: "claude"})
	if s := view(m); !strings.Contains(s, "t show tool use") {
		t.Fatalf("footer:\n%s", s)
	}
}

func TestUnfoldingKeepsTheReadingPosition(t *testing.T) {
	var evs []Event
	for i := range 40 {
		evs = append(evs, thinks(fmt.Sprintf("thought %02d", i)), call(fmt.Sprintf("c%d", i), "bash", "ls", Passed))
	}
	m := agentScreen(160, evs...)
	agentLines(m, 160)
	for range 20 {
		m.Update(tea.KeyMsg{Type: tea.KeyUp})
	}
	top := agentLines(m, 160)[0]
	pressT(m)
	if got := agentLines(m, 160)[0]; got != top {
		t.Fatalf("unfolding moved the view from %q to %q", top, got)
	}
	pressT(m)
	if got := agentLines(m, 160)[0]; got != top {
		t.Fatalf("folding moved the view from %q to %q", top, got)
	}
}

func TestToolSummaryDoesNotPassTerminalControlsThrough(t *testing.T) {
	m := agentScreen(60, summed(call("c1", "bash", "ls\x1b[2J", Passed), "safe\x1b]0;title\a \x1b[2Jtext\r\b"))
	pressT(m)
	raw := m.agentBox(60, 10)
	if got := agentLines(m, 60); !slices.Contains(got, "✓ ok bash ls · safe text") {
		t.Fatalf("Agent box lines: %q", got)
	}
	for _, c := range []string{"\x1b]", "\x1b[2J", "\a", "\r", "\b"} {
		if strings.Contains(raw, c) {
			t.Fatalf("Agent box passes %q through:\n%q", c, raw)
		}
	}
}

func TestFoldedFailureShowsErrorOnOneRow(t *testing.T) {
	m := agentScreen(240, summed(call("f", "shell", "cat missing\necho extra", Failed), "missing\nmore"),
		call("b", "read", "b.go", Passed))
	s := view(m)
	rows := strings.Split(s, "\n")
	at := slices.IndexFunc(rows, func(row string) bool {
		return strings.Contains(row, "failed shell cat missing · missing")
	})
	if at < 0 || at+1 >= len(rows) || !strings.Contains(rows[at+1], "1 tool: read") ||
		strings.Contains(s, "echo extra") || strings.Contains(s, "more") {
		t.Fatalf("want one row per call:\n%s", s)
	}
	clickRow(m, at+1)
	if s := view(m); !strings.Contains(s, "ok read b.go") {
		t.Fatalf("row after the failure must still be clickable:\n%s", s)
	}
}

func TestFailureSeparatesIndependentToolGroups(t *testing.T) {
	evs := []Event{call("a", "read", "a.go", Passed), summed(call("f", "shell", "false", Failed), "failed command"),
		call("b", "bash", "echo b", Passed)}
	m := agentScreen(240, evs...)
	for _, group := range []struct{ header, shown, hidden string }{
		{"1 tool: bash", "ok bash echo b", "ok read a.go"},
		{"1 tool: read", "ok read a.go", "ok bash echo b"},
	} {
		click(t, m, group.header)
		if s := view(m); !strings.Contains(s, group.shown) || strings.Contains(s, group.hidden) {
			t.Fatalf("want independent groups:\n%s", s)
		}
		click(t, m, group.header)
		if s := view(m); strings.Contains(s, group.shown) || strings.Contains(s, group.hidden) {
			t.Fatalf("group did not fold independently:\n%s", s)
		}
	}
	for range 2 {
		before := view(m)
		click(t, m, "failed shell false")
		if after := view(m); after != before {
			t.Fatalf("a click on a failed call alone changes nothing:\nbefore:\n%s\nafter:\n%s", before, after)
		}
		pressT(m)
	}
}
