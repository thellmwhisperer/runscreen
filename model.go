package runscreen

import (
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thellmwhisperer/runscreen/internal/style"
)

// State is a step's state, or a tool call's result: each has its icon and its word.
type State string

// The states a step can be in; a tool call is running, passed or failed.
const (
	Pending State = "pending"
	Running State = "running"
	Waiting State = "waiting"
	Passed  State = "passed"
	Failed  State = "failed"
	Skipped State = "skipped"
)

// Pipeline is what the pipeline box draws: its title, the pipeline's state beside it when set, and its steps.
type Pipeline struct {
	Title string
	State State
	Steps []Step
}

// Step is a row of the pipeline box: its name, its state, when it started and ended, a note right-aligned before
// its state, and its sub-rows. A running step counts its time from Start; an ended one shows End - Start.
type Step struct {
	Name       string
	State      State
	Start, End time.Time
	Note       string
	Steps      []Step
}

// Kind is what an event of the Agent box is.
type Kind int

// The kinds of event: what the agent says, in Markdown, what it thinks, and a tool call.
const (
	Says Kind = iota
	Thinks
	ToolCall
)

// Event is what the agent does: Says or Thinks Text, or a ToolCall. A tool call is known by its ID: a later event
// with the same ID updates it, its empty fields leaving the call's as they were. Its Result is Running, Passed or
// Failed, and its Summary the first line of what it printed.
type Event struct {
	Kind                      Kind
	Text                      string
	ID, Tool, Detail, Summary string
	Result                    State
}

// Agent names the agent in the Agent box's title: Agent · harness · model · effort, each when set.
type Agent struct {
	Harness, Model, Effort string
}

// Box is a box the caller adds on the right, under the Agent box: a title and its rows.
type Box struct {
	Title string
	Rows  []Row
}

// Row is a row of a Box: its text, in Markdown, and its own rows, which a click on it shows or hides; Open is
// whether they show until then.
type Row struct {
	Text string
	Open bool
	Rows []Row
}

// Boxes is every box the caller adds under the Agent box, in order; sent to the screen, it replaces the last.
type Boxes []Box

// Inline renders one line of Markdown for the terminal: its text, styled; code says it holds code, whose spaces
// wrapping keeps; block says it is a code block, kept as written. markdown.Inline is one.
type Inline func(line string) (text string, code, block bool)

// activity is a line of the Agent box: a tool call, by its id, its result and the first line of its output, or
// what the agent says or thinks.
type activity struct {
	id, tool, detail, summary, thinking, says string
	result                                    State
}

// Model is the run screen: the pipeline, the agent's events and the caller's boxes it was given, drawn at the
// terminal's size. It is a bubbletea model: send it a Pipeline, an Event, an Agent or Boxes.
type Model struct {
	pipelineData        Pipeline
	agentInfo           *Agent
	agent               []*activity
	boxes               Boxes
	inline              Inline
	width, height, spin int
	historyHeight       int
	now                 time.Time
	pipeline, history   pane
	boxPanes            []*pane
	focus               *pane // the box tab moved to, while it shows
	pipelineBounds      bounds
	historyBounds       bounds
	boxBounds           []bounds
	boxHeights          []int // each box's rows as last drawn, 0 when it had no room
	titleArrow          bounds
	unfold              bool            // t: every tool call of the Agent box on its own line
	unfoldThink         bool            // h: every thinking message of the Agent box in full
	unfoldTitle         bool            // i: every line of the pipeline's title
	open                map[string]bool // tool calls, thinking or rows, by key, clicked open or closed
	rowOpen             map[string]bool // each box row's Open, by key, as last drawn
}

type bounds struct{ x, y, width, height int }

type paneLine struct{ key, parent, text, toggle string }

type pane struct {
	top, last, h, part int
	anchor, parent     string
	held               bool
	toggles            []string // what each shown row opens or closes on a click
}

func (p *pane) show(lines []paneLine, h int) []string {
	oldAnchor, oldParent, oldPart, oldTop := p.anchor, p.parent, p.part, p.top
	p.h, p.last = h, max(0, len(lines)-h)
	if p.held && oldAnchor != "" {
		p.top = keyedLine(lines, oldAnchor, oldPart)
		if p.top < 0 && oldParent != "" {
			p.top = keyedLine(lines, oldParent, 0)
		}
		if p.top < 0 {
			p.top = min(oldTop, p.last)
		}
	}
	if !p.held || p.top >= p.last {
		p.top, p.held = p.last, false
	}
	if p.top < len(lines) {
		p.anchor, p.parent = lines[p.top].key, lines[p.top].parent
		p.part = countKey(lines, p.top, lines[p.top].key)
	}
	out := make([]string, 0, min(h, len(lines)-p.top))
	p.toggles = p.toggles[:0]
	for _, line := range lines[p.top:min(len(lines), p.top+h)] {
		out = append(out, line.text)
		p.toggles = append(p.toggles, line.toggle)
	}
	return out
}

func keyedLine(lines []paneLine, key string, part int) int {
	first, count := -1, 0
	for i, line := range lines {
		if line.key == key {
			if first < 0 {
				first = i
			}
			count++
		}
	}
	if first < 0 {
		return -1
	}
	return first + min(part, count-1)
}

func countKey(lines []paneLine, at int, key string) int {
	n := 0
	for i := at - 1; i >= 0 && lines[i].key == key; i-- {
		n++
	}
	return n
}

func (p *pane) scroll(n int) {
	p.top = min(max(0, p.top+n), p.last)
	p.held = p.top < p.last
	p.anchor, p.parent, p.part = "", "", 0
}

// New is the screen at width by height before it is given anything: an empty pipeline box, until the terminal
// says its size.
func New(width, height int) *Model {
	return &Model{width: width, height: height, now: time.Now(), inline: plain}
}

// SetMarkdown renders what the agent says and the boxes' rows with inline, as markdown.Inline does; without it,
// they show as plain text.
func (m *Model) SetMarkdown(inline Inline) { m.inline = inline }

// SetPipeline replaces the pipeline box's title and steps.
func (m *Model) SetPipeline(p Pipeline) { m.pipelineData = p }

// SetAgent names the agent in the Agent box's title, and shows the box.
func (m *Model) SetAgent(a Agent) {
	m.appearing()
	m.agentInfo = &a
}

// appearing gives the Agent box the focus when it shows for the first time, as a new agent's box takes it.
func (m *Model) appearing() {
	if !m.agentVisible() {
		m.focus = nil
	}
}

// SetBoxes replaces the boxes under the Agent box.
func (m *Model) SetBoxes(b Boxes) {
	m.boxes = b
	for len(m.boxPanes) < len(b) {
		m.boxPanes = append(m.boxPanes, &pane{})
	}
	m.boxPanes = m.boxPanes[:len(b)]
}

// Add adds e to the Agent box, and shows the box: what the agent says or thinks on its own line, a tool call by
// its id. Saying or thinking nothing adds nothing.
func (m *Model) Add(e Event) {
	if e.Kind == ToolCall || e.Text != "" {
		m.appearing()
	}
	switch {
	case e.Kind == ToolCall:
		m.tool(e)
	case e.Text == "":
	case e.Kind == Says:
		m.agent = append(m.agent, &activity{says: e.Text})
	case e.Kind == Thinks:
		m.agent = append(m.agent, &activity{thinking: e.Text})
	}
}

// plain is a line as it is, without its terminal controls.
func plain(line string) (string, bool, bool) { return style.DisplayText(line), false, false }

// focused is the pane keys scroll: the box tab moved to while it shows, else the Agent box's while it shows, else
// the pipeline box's.
func (m *Model) focused() *pane {
	if slices.Contains(m.panes(), m.focus) {
		return m.focus
	}
	if m.agentVisible() {
		return &m.history
	}
	return &m.pipeline
}

// panes is the panes of the boxes shown, in the order tab moves through them.
func (m *Model) panes() []*pane {
	shown := []*pane{&m.pipeline}
	if m.agentVisible() {
		shown = append(shown, &m.history)
	}
	for i, p := range m.boxPanes {
		if i >= len(m.boxHeights) || m.boxHeights[i] > 0 {
			shown = append(shown, p)
		}
	}
	return shown
}

type tick time.Time

// Init starts the clock that running steps and the spinner follow.
func (m *Model) Init() tea.Cmd { return next() }

func next() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return tick(t) })
}

// Update reads msg: a size, a key, the mouse, the clock, or what the caller sends to draw.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		if slices.Contains([]string{"q", "ctrl+c"}, msg.String()) {
			return m, tea.Quit
		}
		m.key(msg.String())
	case tea.MouseMsg:
		m.wheel(msg)
	case tick:
		m.now, m.spin = time.Time(msg), m.spin+1
		return m, next()
	case Pipeline:
		m.SetPipeline(msg)
	case Event:
		m.Add(msg)
	case Agent:
		m.SetAgent(msg)
	case Boxes:
		m.SetBoxes(msg)
	}
	return m, nil
}

// key scrolls the focused box, or, on tab, focuses the next, or, on t, folds or unfolds every tool call,
// on h, every thinking message and, on i, the pipeline's title.
func (m *Model) key(k string) {
	p := m.focused()
	switch k {
	case "t":
		m.unfold = !m.unfold
		m.forget("tools:")
	case "h":
		m.unfoldThink = !m.unfoldThink
		m.forget("activity:")
	case "i":
		m.unfoldTitle = !m.unfoldTitle
	case "tab":
		shown := m.panes()
		m.focus = shown[(slices.Index(shown, p)+1)%len(shown)]
	default:
		p.key(k)
	}
}

// forget drops the clicks on the rows whose keys start with prefix, which a key just folded or unfolded.
func (m *Model) forget(prefix string) {
	for k := range m.open {
		if strings.HasPrefix(k, prefix) {
			delete(m.open, k)
		}
	}
}

func (p *pane) key(k string) {
	switch k {
	case "up":
		p.scroll(-1)
	case "down":
		p.scroll(1)
	case "pgup":
		p.scroll(-max(1, p.h-1))
	case "pgdown":
		p.scroll(max(1, p.h-1))
	case "home":
		p.scroll(-p.top)
	case "end":
		p.scroll(p.last)
	}
}

// wheel scrolls the box under the pointer. A click on a row that folds unfolds it, and once unfolded folds it
// again; a click on the pipeline's title arrow folds or unfolds the title.
func (m *Model) wheel(msg tea.MouseMsg) {
	p, b := m.under(msg.X, msg.Y)
	if p == nil {
		return
	}
	switch {
	case msg.Button == tea.MouseButtonWheelUp:
		p.scroll(-3)
	case msg.Button == tea.MouseButtonWheelDown:
		p.scroll(3)
	case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
		if p == &m.pipeline && m.titleArrow.contains(msg.X-b.x, msg.Y-b.y) {
			m.unfoldTitle = !m.unfoldTitle
		}
		row := msg.Y - b.y - 1 // under the box's top border
		if row >= 0 && row < len(p.toggles) && p.toggles[row] != "" {
			if m.open == nil {
				m.open = map[string]bool{}
			}
			m.open[p.toggles[row]] = !m.unfolded(p.toggles[row])
		}
	}
}

// under is the pane of the box at x, y and its bounds; nil for none.
func (m *Model) under(x, y int) (*pane, bounds) {
	for i, b := range m.boxBounds {
		if i < len(m.boxPanes) && b.contains(x, y) {
			return m.boxPanes[i], b
		}
	}
	if m.historyBounds.contains(x, y) {
		return &m.history, m.historyBounds
	}
	if m.pipelineBounds.contains(x, y) {
		return &m.pipeline, m.pipelineBounds
	}
	return nil, bounds{}
}

// unfolded says whether the run of tool calls, the thinking message or the box row by key shows in full.
func (m *Model) unfolded(key string) bool {
	if open, ok := m.open[key]; ok {
		return open
	}
	if strings.HasPrefix(key, "tools:") {
		return m.unfold
	}
	if strings.HasPrefix(key, "box:") {
		return m.rowOpen[key]
	}
	return m.unfoldThink
}

func (b bounds) contains(x, y int) bool {
	return x >= b.x && x < b.x+b.width && y >= b.y && y < b.y+b.height
}

// tool notes a tool call, or its result, by its id; its empty fields leave the call's as they were.
func (m *Model) tool(e Event) {
	i := slices.IndexFunc(m.agent, func(a *activity) bool { return e.ID != "" && a.id == e.ID })
	if i < 0 {
		m.agent = append(m.agent, &activity{id: e.ID})
		i = len(m.agent) - 1
	}
	a := m.agent[i]
	for to, v := range map[*string]string{&a.tool: e.Tool, &a.detail: e.Detail, &a.summary: e.Summary} {
		if v != "" {
			*to = v
		}
	}
	if e.Result != "" {
		a.result = e.Result
	}
}
