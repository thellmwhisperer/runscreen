package runscreen

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// gate is a box of a gate's steps: review unfolds to its findings, test is folded.
var gate = Boxes{{Title: "Gate", Rows: []Row{
	{Text: "✓ intent"},
	{Text: "⏸ review: **2** findings", Open: true, Rows: []Row{{Text: "W `view.go:12` cut too early"},
		{Text: "E `model.go:3` lost a fold", Rows: []Row{{Text: "nested detail"}}}}},
	{Text: "✗ test", Rows: []Row{{Text: "FAIL TestX"}}},
}}}

// clickText is a left click on the row of the right column that shows text.
func clickText(t *testing.T, m *Model, text string) {
	t.Helper()
	lines := strings.Split(view(m), "\n")
	y := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, text) })
	if y < 0 {
		t.Fatalf("no row with %q:\n%s", text, view(m))
	}
	m.Update(tea.MouseMsg{X: m.historyBounds.x + 3, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
}

func TestBoxesShowUnderTheAgentBox(t *testing.T) {
	m := agentScreen(120, says("Working."))
	feed(m, gate)
	s := view(m)
	if !order(s, "╭─ Agent*", "✎ Working.", "╰", "╭─ Gate ", "✓ intent", "▾ ⏸ review: 2 findings",
		"  W view.go:12 cut too early", "▸ E model.go:3 lost a fold", "▸ ✗ test", "╰", "q quit") ||
		strings.Contains(s, "nested detail") || strings.Contains(s, "FAIL TestX") {
		t.Fatalf("want the Gate box under the Agent box, its open row unfolded:\n%s", s)
	}
	if raw := m.View(); !strings.Contains(raw, "\x1b[1m2") || !strings.Contains(raw, "\x1b[4m▾") {
		t.Fatalf("a box's rows are Markdown, their arrows underlined:\n%q", raw)
	}
	lines := strings.Split(s, "\n")
	if !strings.Contains(lines[len(lines)-3], "╰") || len(lines) > 40 {
		t.Fatalf("want the column down to the key line:\n%s", s)
	}
}

func TestABoxShowsBesideThePipelineWithoutAnAgent(t *testing.T) {
	m := screen(120, 40, unit)
	m.SetBoxes(gate)
	s := view(m)
	first, _, _ := strings.Cut(s, "\n")
	if !strings.Contains(first, "╮  ╭─ Gate ") || strings.Contains(s, "Agent ·") || strings.Contains(s, "tool use") {
		t.Fatalf("want the Gate box beside the pipeline box, no Agent box:\n%s", s)
	}
	if s := view(screen(80, 40, unit)); strings.Contains(s, "Gate") {
		t.Fatalf("no box until the caller adds one:\n%s", s)
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	if s := view(m); !order(s, "╭─ Pipeline", "○ Delivery", "╰", "╭─ Gate", "✗ test") {
		t.Fatalf("under 100 columns, want the Gate box below the pipeline box:\n%s", s)
	}
}

func TestAClickFoldsOrUnfoldsABoxRow(t *testing.T) {
	m := agentScreen(120, says("Working."))
	m.SetBoxes(gate)
	clickText(t, m, "✗ test")
	if s := view(m); !order(s, "▾ ✗ test", "  FAIL TestX") {
		t.Fatalf("a click unfolds a folded row:\n%s", s)
	}
	clickText(t, m, "E model.go:3")
	if s := view(m); !order(s, "▾ E model.go:3 lost a fold", "    nested detail") {
		t.Fatalf("a click unfolds a nested row, its rows further in:\n%s", s)
	}
	clickText(t, m, "review: 2 findings")
	if s := view(m); strings.Contains(s, "cut too early") || strings.Contains(s, "nested detail") ||
		!strings.Contains(s, "▸ ⏸ review") {
		t.Fatalf("a click folds an open row with every row under it:\n%s", s)
	}
	before := view(m)
	clickText(t, m, "✓ intent")
	if after := view(m); after != before {
		t.Fatalf("a click on a row without rows changes nothing:\n%s", after)
	}
	feed(m, gate) // the caller sends the box again: what was clicked stays
	pressT(m)
	pressH(m)
	if s := view(m); !strings.Contains(s, "▸ ⏸ review") || !strings.Contains(s, "FAIL TestX") {
		t.Fatalf("sending the box again, t and h keep its folds:\n%s", s)
	}
}

func TestABoxWrapsItsRowsUnderTheirText(t *testing.T) {
	long := strings.Repeat("word ", 20) + "END"
	m := screen(80, 40, unit)
	m.SetBoxes(Boxes{{Title: "Questions", Rows: []Row{{Text: long, Open: true, Rows: []Row{{Text: long}}}}}})
	s := view(m)
	if !regexp.MustCompile(`│ ▾ word.*│\n│   word.*END +│\n│     word.*│\n│     word.*END +│`).MatchString(s) {
		t.Fatalf("want each row whole, its lines under its text:\n%s", s)
	}
	for _, l := range strings.Split(m.View(), "\n") {
		if ansi.StringWidth(l) > 80 {
			t.Fatalf("a row past the box: %q", ansi.Strip(l))
		}
	}
}

// findings is a box of n findings, f00, f01…
func findings(n int) Boxes {
	b := Box{Title: "Findings"}
	for i := range n {
		b.Rows = append(b.Rows, Row{Text: fmt.Sprintf("f%02d", i)})
	}
	return Boxes{b}
}

var findingRows = regexp.MustCompile(`f\d\d`)

func TestABoxTakesAThirdOfTheColumnAndScrolls(t *testing.T) {
	m := agentScreen(120, says("Working."))
	feed(m, findings(40))
	shown := shownOf(m, findingRows)
	if available := 40 - 3; len(shown) != available/3-boxBorderRows || shown[len(shown)-1] != "f39" {
		t.Fatalf("want a third of the column, its last rows:\n%s", view(m))
	}
	wheelAt(m, 80, m.boxBounds[0].y+2, tea.MouseButtonWheelUp)
	held := shownOf(m, findingRows)
	if slices.Contains(held, "f39") || !strings.Contains(view(m), "✎ Working.") {
		t.Fatalf("the wheel over the box scrolls it alone:\n%s", view(m))
	}
	feed(m, findings(41))
	if !slices.Equal(shownOf(m, findingRows), held) {
		t.Fatalf("new rows moved what is being read:\n%s", view(m))
	}
	press(m, tea.KeyTab, tea.KeyHome)
	if got := shownOf(m, findingRows); got[0] != "f00" || !strings.Contains(m.View(), bold(cyan, "Findings*")) {
		t.Fatalf("tab focuses the box and home goes to its first row:\n%s", view(m))
	}
	press(m, tea.KeyEnd)
	feed(m, findings(42))
	if got := shownOf(m, findingRows); got[len(got)-1] != "f41" {
		t.Fatalf("back at the bottom, the box follows:\n%s", view(m))
	}
	m = screen(120, 40, unit)
	feed(m, findings(60))
	if got := shownOf(m, findingRows); len(got) != 40-3-boxBorderRows {
		t.Fatalf("without the Agent box, a box takes the column:\n%s", view(m))
	}
}

func TestBoxesShareTheColumnAndThoseWithoutRoomWait(t *testing.T) {
	m := agentScreen(120, says("Working."))
	feed(m, Boxes{findings(40)[0], {Title: "Questions", Rows: []Row{{Text: "Which port?"}}}})
	if s := view(m); !order(s, "╭─ Agent", "╭─ Findings", "f39", "╭─ Questions", "Which port?", "q quit") {
		t.Fatalf("want both boxes under the Agent box:\n%s", s)
	}
	assertFitsViewport(t, m, 120, 40)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 9})
	lines := assertFitsViewport(t, m, 120, 9)
	if s := strings.Join(lines, "\n"); strings.Contains(s, "Questions") || !strings.Contains(s, "Findings") ||
		!strings.Contains(s, "Agent") {
		t.Fatalf("a box without room stays off the screen, the Agent box on:\n%s", s)
	}
	for _, want := range []string{"Findings*", "Pipeline*", "Agent*"} {
		press(m, tea.KeyTab)
		if !strings.Contains(m.View(), bold(cyan, want)) {
			t.Fatalf("want %s: tab skips the box off the screen:\n%s", want, view(m))
		}
	}
}
