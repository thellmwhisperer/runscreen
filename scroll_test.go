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

// withChecks is the unit's pipeline, its Checks step with n checks c00, c01… passed.
func withChecks(n int) Pipeline {
	p := unit
	p.Steps = slices.Clone(unit.Steps)
	checks := &p.Steps[4]
	checks.Steps = nil
	for i := range n {
		checks.Steps = append(checks.Steps, Step{Name: fmt.Sprintf("c%02d", i), State: Passed})
	}
	return p
}

// overflowing is a running agent with 40 checks under Checks and 60 thoughts: more than either box holds.
func overflowing(width, height int) *Model {
	var evs []Event
	for i := range 60 {
		evs = append(evs, thinks(fmt.Sprintf("thought %02d", i)))
	}
	return screen(width, height, withChecks(40), evs...)
}

var proofs, thoughts = regexp.MustCompile(`c\d\d`), regexp.MustCompile(`thought \d\d`)

// shownOf is what of re the screen shows.
func shownOf(m *Model, re *regexp.Regexp) []string { return re.FindAllString(view(m), -1) }

func press(m *Model, keys ...tea.KeyType) {
	for _, k := range keys {
		m.Update(tea.KeyMsg{Type: k})
		view(m)
	}
}

func wheelAt(m *Model, x, y int, b tea.MouseButton) {
	m.Update(tea.MouseMsg{X: x, Y: y, Button: b, Action: tea.MouseActionPress})
}

func assertFitsViewport(t *testing.T, m *Model, width, height int) []string {
	t.Helper()
	s := m.View()
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		t.Fatalf("viewport used %d rows at %dx%d:\n%s", len(lines), width, height, ansi.Strip(s))
	}
	for _, line := range lines {
		if ansi.StringWidth(line) > width {
			t.Fatalf("viewport line exceeds %d columns at %dx%d: %q", width, width, height, ansi.Strip(line))
		}
	}
	return strings.Split(ansi.Strip(s), "\n")
}

func topAgentLine(m *Model) string { return strings.Split(view(m), "\n")[1] }
func topPipelineLine(m *Model) string {
	line := []rune(strings.Split(view(m), "\n")[3])
	return string(line[:min(len(line), m.pipelineBounds.width)])
}

func TestBothBoxesScrollIndependentlyAndFollowOnlyAtTheBottom(t *testing.T) {
	m := overflowing(120, 24)
	pipeline, agent := shownOf(m, proofs), shownOf(m, thoughts)
	if len(pipeline) == 0 || len(pipeline) == 40 || agent[len(agent)-1] != "thought 59" || len(agent) == 60 {
		t.Fatalf("want both boxes overflowing at their latest:\n%s", view(m))
	}
	if !strings.Contains(m.View(), paint(cyan, "╭─ ")+bold(cyan, "Agent*")) {
		t.Fatalf("want the Agent box focused, its border lit:\n%q", m.View())
	}
	// The Agent box, focused: reading older lines, new ones do not move it, nor the pipeline box.
	press(m, tea.KeyUp, tea.KeyUp, tea.KeyPgUp)
	held := shownOf(m, thoughts)
	if slices.Contains(held, "thought 59") || !slices.Equal(shownOf(m, proofs), pipeline) {
		t.Fatalf("up and page up scroll only the Agent box:\n%s", view(m))
	}
	feed(m, thinks("thought 60"))
	if !slices.Equal(shownOf(m, thoughts), held) || !slices.Equal(shownOf(m, proofs), pipeline) {
		t.Fatalf("new output moved what is being read:\n%s", view(m))
	}
	press(m, tea.KeyPgDown, tea.KeyDown, tea.KeyEnd)
	feed(m, thinks("thought 61"))
	if got := shownOf(m, thoughts); got[len(got)-1] != "thought 61" || !slices.Equal(shownOf(m, proofs), pipeline) {
		t.Fatalf("back at the bottom, the Agent box follows, alone:\n%s", view(m))
	}
}

// The pipeline box, focused with tab, the same; the Agent box keeps following.
func TestThePipelineScrollsWhenFocusedAndFollowsAtTheBottom(t *testing.T) {
	m := overflowing(120, 24)
	press(m, tea.KeyTab)
	if !strings.Contains(m.View(), paint(cyan, "╭─ ")+bold(cyan, "Pipeline*")) {
		t.Fatalf("want the pipeline box focused, its border lit:\n%q", m.View())
	}
	press(m, tea.KeyHome)
	top := shownOf(m, proofs)
	if top[0] != "c00" {
		t.Fatalf("home goes to the pipeline box's first rows:\n%s", view(m))
	}
	feed(m, withChecks(41), thinks("thought 62"))
	if got := shownOf(m, thoughts); !slices.Equal(shownOf(m, proofs), top) || got[len(got)-1] != "thought 62" {
		t.Fatalf("the pipeline box held, the Agent box following:\n%s", view(m))
	}
	for range 60 {
		press(m, tea.KeyDown)
	}
	feed(m, withChecks(42))
	if got := shownOf(m, proofs); got[len(got)-1] != "c41" {
		t.Fatalf("back at the bottom, the pipeline box follows:\n%s", view(m))
	}
}

func TestTheWheelScrollsTheBoxUnderThePointer(t *testing.T) {
	m := overflowing(120, 24)
	pipeline, agent := shownOf(m, proofs), shownOf(m, thoughts)
	wheelAt(m, 5, 10, tea.MouseButtonWheelUp)
	if got := shownOf(m, proofs); slices.Equal(got, pipeline) || !slices.Equal(shownOf(m, thoughts), agent) {
		t.Fatalf("the wheel over the pipeline box scrolls it, though the Agent box has focus:\n%s", view(m))
	}
	wheelAt(m, 80, 10, tea.MouseButtonWheelUp)
	if slices.Equal(shownOf(m, thoughts), agent) {
		t.Fatalf("the wheel over the Agent box scrolls it:\n%s", view(m))
	}
	wheelAt(m, 80, 10, tea.MouseButtonWheelDown)
	if !slices.Equal(shownOf(m, thoughts), agent) {
		t.Fatalf("the wheel back down returns to the latest:\n%s", view(m))
	}
	m = overflowing(80, 40)
	pipeline, agent = shownOf(m, proofs), shownOf(m, thoughts)
	wheelAt(m, 70, 35, tea.MouseButtonWheelUp)
	if !slices.Equal(shownOf(m, proofs), pipeline) || slices.Equal(shownOf(m, thoughts), agent) {
		t.Fatalf("stacked, the wheel low on the screen scrolls the Agent box:\n%s", view(m))
	}
	before := shownOf(m, thoughts)
	for _, point := range [][2]int{{40, 38}, {40, 39}} {
		wheelAt(m, point[0], point[1], tea.MouseButtonWheelUp)
	}
	if !slices.Equal(shownOf(m, thoughts), before) {
		t.Fatalf("the key line does not scroll either box:\n%s", view(m))
	}
}

func TestResizeKeepsBothBoxesInBordersAndTheirReadingPosition(t *testing.T) {
	m := overflowing(120, 24)
	press(m, tea.KeyTab, tea.KeyHome)
	for _, size := range [][2]int{{80, 40}, {60, 30}, {150, 50}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		lines := assertFitsViewport(t, m, size[0], size[1])
		if shownOf(m, proofs)[0] != "c00" || len(shownOf(m, thoughts)) == 0 {
			t.Fatalf("at %v, want both boxes on screen, the pipeline box still at its top:\n%s", size,
				strings.Join(lines, "\n"))
		}
	}
	// So tall that every row fits: the bottom is reached, the pipeline box follows again.
	m.Update(tea.WindowSizeMsg{Width: 150, Height: 120})
	view(m)
	m.Update(tea.WindowSizeMsg{Width: 150, Height: 24})
	feed(m, withChecks(41))
	if got := shownOf(m, proofs); got[len(got)-1] != "c40" {
		t.Fatalf("want the pipeline box following once it showed its bottom:\n%s", view(m))
	}
}

func TestViewportFitsActualHeightWhenBothBoxesOverflow(t *testing.T) {
	for _, size := range [][2]int{{80, 40}, {80, 24}, {80, 16}, {120, 9}} {
		m := overflowing(size[0], size[1])
		m.SetBoxes(Boxes{{Title: "Log", Rows: []Row{{Text: "retained log line"}}}})
		s := strings.Join(assertFitsViewport(t, m, size[0], size[1]), "\n")
		if !strings.Contains(s, "q quit") || !strings.Contains(s, "Pipeline") || !strings.Contains(s, "Agent") {
			t.Fatalf("at %v, both overflowing boxes and the key line must fit:\n%s", size, s)
		}
		if size[1] >= 16 && !strings.Contains(s, "retained log line") {
			t.Fatalf("at %v, the caller's box must fit too:\n%s", size, s)
		}
	}
}

func TestStackedPanelsUseRenderedRowsForBoundsAndScrolling(t *testing.T) {
	for _, height := range []int{15, 24} {
		m := overflowing(80, height)
		m.SetBoxes(Boxes{{Title: "Log", Rows: []Row{{Text: "retained log line"}}}})
		lines := assertFitsViewport(t, m, 80, height)
		agentY := m.historyBounds.y
		logY := agentY + m.historyBounds.height
		if agentY != m.pipelineBounds.height || !strings.Contains(lines[agentY], "Agent*") ||
			!strings.Contains(lines[logY], "Log") {
			t.Fatalf("panel bounds must match their rendered top rows at 80x%d:\n%s", height, strings.Join(lines, "\n"))
		}
		before, pipeline := shownOf(m, thoughts), shownOf(m, proofs)
		for _, y := range []int{agentY, agentY + m.historyBounds.height - 1} {
			wheelAt(m, 5, y, tea.MouseButtonWheelUp)
		}
		if slices.Equal(shownOf(m, thoughts), before) || !slices.Equal(shownOf(m, proofs), pipeline) {
			t.Fatalf("wheel on Agent border rows must scroll Agent at 80x%d:\n%s", height, view(m))
		}
		held := shownOf(m, thoughts)
		for _, y := range []int{logY, height - 1} {
			wheelAt(m, 5, y, tea.MouseButtonWheelUp)
		}
		if !slices.Equal(shownOf(m, thoughts), held) || !slices.Equal(shownOf(m, proofs), pipeline) {
			t.Fatalf("wheel on Log or key line must not scroll Agent at 80x%d:\n%s", height, view(m))
		}
	}
}

func TestTabMovesFocusThroughTheBoxesShown(t *testing.T) {
	m := screen(120, 40, unit)
	press(m, tea.KeyTab)
	if !strings.Contains(m.View(), bold(cyan, "Pipeline*")) {
		t.Fatalf("with the pipeline box alone, tab keeps it focused:\n%s", view(m))
	}
	feed(m, says("Working."), Boxes{{Title: "Gate", Rows: []Row{{Text: "review"}}}})
	for _, want := range []string{"Agent*", "Gate*", "Pipeline*", "Agent*"} {
		if s := m.View(); !strings.Contains(s, bold(cyan, want)) || strings.Count(ansi.Strip(s), "*") != 1 {
			t.Fatalf("want %s focused alone:\n%s", want, view(m))
		}
		press(m, tea.KeyTab)
	}
}

func TestHeldRowsStayAnchoredAcrossConnectorAndWrapReflow(t *testing.T) {
	m := overflowing(120, 24)
	press(m, tea.KeyTab, tea.KeyHome)
	for range 4 {
		press(m, tea.KeyDown)
	}
	before := strings.Split(view(m), "\n")
	if len(before) < 4 || !strings.Contains(before[3], "Checks") {
		t.Fatalf("fixture must hold the Checks row before connectors appear:\n%s", strings.Join(before, "\n"))
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	after := strings.Split(view(m), "\n")
	if len(after) < 4 || !strings.Contains(after[3], "Checks") {
		t.Fatalf("connector insertion moved the selected pipeline row:\n%s", strings.Join(after, "\n"))
	}
	m = screen(120, 24, unit)
	for i := range 5 {
		feed(m, thinks(fmt.Sprintf("preceding message %02d %s", i, strings.Repeat("x", 45))))
	}
	feed(m, thinks("anchor identity marker"))
	for i := range 40 {
		feed(m, thinks(fmt.Sprintf("later message %02d", i)))
	}
	view(m)
	press(m, tea.KeyHome)
	for range 5 {
		press(m, tea.KeyDown)
	}
	if got := view(m); !strings.Contains(got, "anchor identity marker") {
		t.Fatalf("fixture must select the anchored Agent message:\n%s", got)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	if got := view(m); !strings.Contains(got, "anchor identity marker") {
		t.Fatalf("wrapping moved the selected Agent message:\n%s", got)
	}
}

func TestHeldTopTracksSecondToolGroupAfterNewOutput(t *testing.T) {
	m := screen(120, 24, unit, call("t1", "first-tool", "", Passed), thinks("group separator"),
		call("t2", "second-tool", "", Passed))
	for i := range 40 {
		feed(m, thinks(fmt.Sprintf("later %02d", i)))
	}
	view(m)
	press(m, tea.KeyHome, tea.KeyDown, tea.KeyDown)
	if line := topAgentLine(m); !strings.Contains(line, "second-tool") {
		t.Fatalf("the selected top row must be the second tool group: %q", line)
	}
	feed(m, thinks("new output"))
	if line := topAgentLine(m); !strings.Contains(line, "second-tool") || strings.Contains(line, "first-tool") {
		t.Fatalf("new output moved the selected second tool group: %q", line)
	}
}

func TestHeldWrappedPartClampsToItsMessageWhenWidened(t *testing.T) {
	m := screen(120, 24, unit, thinks("WRAP-ANCHOR "+strings.Repeat("word ", 18)))
	for i := range 30 {
		feed(m, thinks(fmt.Sprintf("later %02d", i)))
	}
	view(m)
	pressH(m) // the wrapped thinking this test anchors to shows only once unfolded
	press(m, tea.KeyHome, tea.KeyDown)
	if line := topAgentLine(m); strings.Contains(line, "WRAP-ANCHOR") {
		t.Fatalf("the fixture must select a continuation row: %q", line)
	}
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 24})
	if line := topAgentLine(m); !strings.Contains(line, "WRAP-ANCHOR") {
		t.Fatalf("widening must keep the same message at the top: %q", line)
	}
}

func TestHeldConnectorMapsToItsRowWhenConnectorsDisappear(t *testing.T) {
	m := overflowing(120, 30)
	view(m)
	press(m, tea.KeyTab, tea.KeyHome)
	for range 7 {
		press(m, tea.KeyDown)
	}
	if line := topPipelineLine(m); strings.Trim(line, "│ ") != "" {
		t.Fatalf("the fixture must select the Checks connector: %q", line)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 29})
	if line := topPipelineLine(m); !strings.Contains(line, "Checks") {
		t.Fatalf("when its connector disappears, the Checks row must stay at the top: %q", line)
	}
	feed(m, withChecks(41))
	if line := topPipelineLine(m); !strings.Contains(line, "Checks") {
		t.Fatalf("new rows moved the surviving Checks row: %q", line)
	}
}

func TestAnEndedPipelineDropsItsConnectorsBeforeItsSteps(t *testing.T) {
	p := Pipeline{Title: "ended", State: Passed}
	for i := range 20 {
		p.Steps = append(p.Steps, Step{Name: fmt.Sprintf("s%02d", i), State: Passed})
	}
	s := view(screen(80, 34, p))
	if strings.Count(s, "│ │") != 0 || !strings.Contains(s, "s19") {
		t.Fatalf("an ended pipeline that does not fit drops its connectors:\n%s", s)
	}
	p.Steps[19].State = Running
	if s := view(screen(80, 34, p)); strings.Count(s, "│ │") == 0 {
		t.Fatalf("a running pipeline keeps them:\n%s", s)
	}
}
