package runscreen

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/thellmwhisperer/runscreen/markdown"
)

var t0 = time.Date(2026, 10, 7, 5, 0, 0, 0, time.UTC)

func at(d time.Duration) time.Time { return t0.Add(d) }

// unit is a pipeline up to its checks: red/green passed and the test command running.
var unit = Pipeline{Title: "#7 Retry on timeout", State: Running, Steps: []Step{
	{Name: "Issue", State: Passed, Start: t0, End: at(120 * time.Millisecond), Note: "fix"},
	{Name: "Clone", State: Passed, Start: t0, End: at(900 * time.Millisecond), Note: "01234567"},
	{Name: "Contract", State: Passed, Start: t0, End: at(3 * time.Millisecond)},
	{Name: "Agent", State: Passed, Start: t0, End: at(268 * time.Second), Note: "1 tools"},
	{Name: "Checks", State: Running, Start: t0, Steps: []Step{
		{Name: "red/green", State: Passed, Start: t0, End: at(1500 * time.Millisecond)},
		{Name: "test", State: Running, Start: t0},
	}},
	{Name: "Review", State: Pending},
	{Name: "Delivery", State: Pending},
}}

func says(text string) Event   { return Event{Kind: Says, Text: text} }
func thinks(text string) Event { return Event{Kind: Thinks, Text: text} }
func call(id, tool, detail string, result State) Event {
	return Event{Kind: ToolCall, ID: id, Tool: tool, Detail: detail, Result: result}
}

// screen is the screen at width by height of pipeline p and the agent's evs, its Markdown rendered, its clock at
// the start of the pipeline.
func screen(width, height int, p Pipeline, evs ...Event) *Model {
	m := New(width, height)
	m.SetMarkdown(markdown.Inline)
	m.now = t0
	m.SetPipeline(p)
	for _, e := range evs {
		m.Add(e)
	}
	return m
}

// agentScreen is the screen of the running unit's agent at width by 40, after evs.
func agentScreen(width int, evs ...Event) *Model { return screen(width, 40, unit, evs...) }

// view is m's screen without its colours, its lines with no trailing spaces.
func view(m *Model) string {
	return regexp.MustCompile(`(?m) +$`).ReplaceAllString(ansi.Strip(m.View()), "")
}

// order says whether each of want is in s, one after the other.
func order(s string, want ...string) bool {
	for _, w := range want {
		i := strings.Index(s, w)
		if i < 0 {
			return false
		}
		s = s[i+len(w):]
	}
	return true
}

func feed(m *Model, msgs ...tea.Msg) {
	for _, msg := range msgs {
		m.Update(msg)
	}
}

func TestScreenHasARowForEachStepAndSubStep(t *testing.T) {
	s := view(screen(80, 40, unit))
	if !order(s, "╭─ Pipeline* ", "#7 Retry on timeout", "running", "✓ Issue  0.1 s", "fix · passed",
		"✓ Clone  0.9 s", "01234567 · passed", "✓ Contract", "✓ Agent  4 min 28 s", "1 tools · passed",
		"Checks", "  ✓ red/green  1.5 s", "  ⠋ test", "running", "○ Review", "pending", "○ Delivery", "pending", "╰") {
		t.Fatalf("rows:\n%s", s)
	}
	if strings.Contains(s, "Agent ·") || strings.Contains(s, "q quit  t") {
		t.Fatalf("no Agent box before the agent is named or does anything:\n%s", s)
	}
}

func TestScreenLaysOutInTwoColumnsFrom100(t *testing.T) {
	running := []Event{thinks("Read the test first."), call("t1", "Bash", "go test ./...", Running)}
	for _, c := range []struct{ width, left int }{{100, 38}, {120, 40}, {150, 48}} {
		first, _, _ := strings.Cut(view(agentScreen(c.width, running...)), "\n")
		if i := strings.Index(first, "╭─ Agent"); len([]rune(first[:max(i, 0)])) != c.left+gap {
			t.Fatalf("at %d columns, want the Agent box beside a %d-column pipeline box: %q", c.width, c.left, first)
		}
	}
	s := view(agentScreen(99, running...))
	pipelineTitle := regexp.MustCompile(`(?m)^╭─ Pipeline\*? ─+╮$`)
	agentTitle := regexp.MustCompile(`(?m)^╭─ Agent\*? ─+╮$`)
	if !pipelineTitle.MatchString(s) || !agentTitle.MatchString(s) {
		t.Fatalf("under 100 columns, want the boxes stacked:\n%s", s)
	}
	connector := regexp.MustCompile(`(?m)^│ │ +│$`)
	if s := view(screen(80, 30, unit, running...)); len(connector.FindAllString(s, -1)) != 6 {
		t.Fatalf("at 30 rows, want a connector between each two of 7 steps:\n%s", s)
	}
	if s := view(screen(80, 29, unit, running...)); connector.MatchString(s) {
		t.Fatalf("under 30 rows, want no connector:\n%s", s)
	}
	if s := view(screen(100, 40, unit)); !strings.Contains(s, "4 min 28 s") || !strings.Contains(s, "passed") {
		t.Fatalf("at 100 columns, want the full state and duration:\n%s", s)
	}
}

func TestStatesHaveAnIconAWordAndANSIColours(t *testing.T) {
	p := Pipeline{Title: "states", State: Waiting}
	for _, s := range []State{Pending, Running, Waiting, Passed, Failed, Skipped} {
		p.Steps = append(p.Steps, Step{Name: "step " + string(s), State: s})
	}
	m := screen(80, 40, p)
	s := view(m)
	if !order(s, "⏸ waiting", "○ step pending", "pending", "⠋ step running", "running", "⏸ step waiting",
		"waiting", "✓ step passed", "passed", "✗ step failed", "failed", "– step skipped", "skipped") {
		t.Fatalf("want every state's icon and word:\n%s", s)
	}
	raw := m.View()
	for _, code := range []string{"\x1b[32m✓", "\x1b[34m⠋", "\x1b[90m○", "\x1b[31m✗", "\x1b[33m⏸", "\x1b[90m–",
		"\x1b[1;36mPipeline"} {
		if !strings.Contains(raw, code) {
			t.Fatalf("want %q in the screen:\n%q", code, raw)
		}
	}
	if regexp.MustCompile(`\x1b\[[0-9;]*[34]8;[25];`).MatchString(raw) {
		t.Fatalf("a colour outside ANSI 1 to 8:\n%q", raw)
	}
}

func TestARunningStepCountsItsTimeLive(t *testing.T) {
	p := Pipeline{Steps: []Step{{Name: "Build", State: Running, Start: t0},
		{Name: "Wait", State: Pending, Start: t0}}}
	m := screen(80, 24, p)
	if s := view(m); !strings.Contains(s, "│ ⠋ Build  0.0 s") || !regexp.MustCompile(`○ Wait +pending`).MatchString(s) {
		t.Fatalf("a running step shows its time, a pending one none:\n%s", s)
	}
	if cmd := m.Init(); cmd == nil {
		t.Fatal("the screen starts no clock")
	}
	_, cmd := m.Update(tick(at(75 * time.Second)))
	if s := view(m); !strings.Contains(s, "⠙ Build  1 min 15 s") || cmd == nil {
		t.Fatalf("a tick moves the clock and the spinner, and asks for the next:\n%s", s)
	}
	m.Update(tick(at(2*time.Hour + 5*time.Minute)))
	if s := view(m); !strings.Contains(s, "Build  2 h 05 min") {
		t.Fatalf("hours and minutes past an hour:\n%s", s)
	}
}

func TestScreenCutsAStepsNoteBeforeItsName(t *testing.T) {
	p := Pipeline{Title: "notes", State: Passed, Steps: []Step{
		{Name: "Agent", State: Passed, Start: t0, End: at(676 * time.Second), Note: "30 tools"},
		{Name: "Review", State: Passed, Start: t0, End: at(695 * time.Second), Note: "1 cuts"},
		{Name: "Delivery", State: Passed, Start: t0, End: at(1379 * time.Second),
			Note: "atm/go-port-report-json-keeps-every-units-proofs-and-the-install"},
	}}
	// A 46-column pipeline box beside a box at 138 columns.
	m := screen(138, 40, p)
	m.SetBoxes(Boxes{{Title: "Findings", Rows: []Row{{Text: "W kept · a.go"}}}})
	s := view(m)
	first, _, _ := strings.Cut(s, "\n")
	left, _, beside := strings.Cut(first, "╮  ╭─ Findings")
	if !beside || !strings.HasPrefix(left, "╭─ Pipeline") || ansi.StringWidth(left+"╮") != 46 {
		t.Fatalf("want Findings beside a 46-column pipeline box:\n%s", s)
	}
	for _, want := range []string{`✓ Agent  11 min 16 s .* passed │`, `✓ Review  11 min 35 s .*passed │`,
		`✓ Delivery  22 min 59 s .*passed │`} {
		if !regexp.MustCompile(want).MatchString(s) {
			t.Fatalf("want %q whole on its row:\n%s", want, s)
		}
	}
}

func TestALongTitleFoldsToOneLineUntilIOrAClick(t *testing.T) {
	title := "Retry the upload when the storage gateway times out, " + strings.Repeat("and say why ", 6) + "END"
	m := screen(120, 40, Pipeline{Title: title, State: Running, Steps: unit.Steps})
	m.Add(says("Working."))
	s := view(m)
	if !regexp.MustCompile(`│ ▸ Retry the upload[^│]*…\s+⠋ running │`).MatchString(s) || strings.Contains(s, "END") {
		t.Fatalf("a long title folds to one line with an arrow and …:\n%s", s)
	}
	if !strings.Contains(m.View(), "\x1b[4m▸") {
		t.Fatalf("the title's arrow is underlined:\n%q", m.View())
	}
	feed(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if s := view(m); !order(s, "▾ Retry the upload", "running", "END", "✓ Issue") {
		t.Fatalf("i unfolds the title in whole lines above the steps:\n%s", s)
	}
	feed(m, tea.MouseMsg{X: 2, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if s := view(m); strings.Contains(s, "END") {
		t.Fatalf("a click on the arrow folds the title again:\n%s", s)
	}
	m.Update(tea.MouseMsg{X: 2, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if s := view(m); !strings.Contains(s, "END") {
		t.Fatalf("a click on the arrow unfolds the title:\n%s", s)
	}
	short := view(screen(120, 40, unit))
	if strings.Contains(short, "▸") || !strings.Contains(short, "│ #7 Retry on timeout") {
		t.Fatalf("a title that fits has no arrow:\n%s", short)
	}
}

func TestAnUnfoldedTitleNeverMakesAShortBoxTaller(t *testing.T) {
	title := strings.Repeat("word ", 40) + "END"
	m := screen(80, 9, Pipeline{Title: title, State: Running, Steps: unit.Steps})
	feed(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if lines := strings.Split(view(m), "\n"); len(lines) > 9 || !strings.Contains(lines[1], "▾ word") {
		t.Fatalf("an unfolded title took more than the terminal:\n%s", strings.Join(lines, "\n"))
	}
	feed(m, tea.KeyMsg{Type: tea.KeyHome})
	if s := view(m); len(regexp.MustCompile(`(?m)^│ .*word word`).FindAllString(s, -1)) < 2 {
		t.Fatalf("the title's other lines scroll above the steps:\n%s", s)
	}
	feed(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	titleRows := regexp.MustCompile(`(?m)^│ .*word word`)
	if s := view(m); !strings.Contains(s, "▸ word") || len(titleRows.FindAllString(s, -1)) != 1 {
		t.Fatalf("folded again, the title's rows leave and the box keeps its place:\n%s", s)
	}
}

func TestTheAgentBoxTitleNamesTheAgent(t *testing.T) {
	for _, c := range []struct {
		agent Agent
		want  string
	}{
		{Agent{Harness: "claude", Model: "claude-sonnet-5-5", Effort: "high"}, "Agent · claude · claude-sonnet-5-5 · high*"},
		{Agent{Harness: "codex", Effort: "high"}, "Agent · codex · high*"},
		{Agent{}, "Agent*"},
	} {
		m := screen(120, 40, unit)
		m.Update(c.agent)
		if s := view(m); !strings.Contains(s, "╭─ "+c.want+" ─") || !strings.Contains(s, "starting") {
			t.Fatalf("want the title %q and the box starting:\n%s", c.want, s)
		}
	}
	m := screen(60, 40, unit)
	m.SetAgent(Agent{Harness: "claude", Model: strings.Repeat("m", 80), Effort: "high"})
	if s := view(m); !strings.Contains(s, "╭─ Agent · claude · mmm") || !strings.Contains(s, "…") {
		t.Fatalf("a long title is cut with …:\n%s", s)
	}
}

func TestQAndCtrlCQuit(t *testing.T) {
	for _, k := range []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune{'q'}}, {Type: tea.KeyCtrlC}} {
		_, cmd := screen(80, 24, unit).Update(k)
		if cmd == nil {
			t.Fatalf("%s did not quit", k)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("%s returned a command other than quit", k)
		}
	}
}

func TestTheKeyLineShowsOnce(t *testing.T) {
	m := agentScreen(120, says("Working."))
	m.SetBoxes(Boxes{{Title: "Gate", Rows: []Row{{Text: "review"}}}})
	for _, size := range [][2]int{{120, 40}, {80, 24}, {80, 9}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		s := view(m)
		if strings.Count(s, "q quit") != 1 || !strings.HasSuffix(s, "q quit  t show tool use  h show thinking") {
			t.Fatalf("at %v, want the key line once, at the end:\n%s", size, s)
		}
	}
}

func TestPlainTextWithoutMarkdown(t *testing.T) {
	m := New(120, 40)
	m.SetPipeline(unit)
	m.Add(says("**bold** and `code`\x1b[2J"))
	if s := view(m); !strings.Contains(s, "✎ **bold** and `code`") || strings.Contains(m.View(), "\x1b[2J") {
		t.Fatalf("without SetMarkdown, what the agent says shows as written, without controls:\n%s", s)
	}
}

func TestHumanDurations(t *testing.T) {
	for d, want := range map[time.Duration]string{1500 * time.Millisecond: "1.5 s", 59 * time.Second: "59.0 s",
		61 * time.Second: "1 min 01 s", 3*time.Hour + 2*time.Minute: "3 h 02 min"} {
		if got := Human(d); got != want {
			t.Fatalf("Human(%v) = %q, want %q", d, got, want)
		}
	}
}
