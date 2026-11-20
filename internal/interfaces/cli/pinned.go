package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
)

func interactiveTerminal() bool {
	return os.Getenv("TERM") != "dumb" && term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd())
}

func (r *REPL) runPinned() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var banner strings.Builder
	session := *r
	session.out = &banner
	session.banner()
	_, noColor := os.LookupEnv("NO_COLOR")
	m := newPinned(banner.String(), !noColor)
	opts := []tea.ProgramOption{tea.WithContext(ctx), tea.WithInput(os.Stdin), tea.WithOutput(os.Stdout), tea.WithAltScreen()}
	// Mouse capture enables wheel scrolling but takes over click-and-drag selection
	// (most terminals still select with Option/Shift held). KODINGIN_NO_MOUSE=1 turns it off.
	if os.Getenv("KODINGIN_NO_MOUSE") == "" {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	p := tea.NewProgram(m, opts...)
	session.out = transcriptWriter{send: p.Send}
	m.run = func(ctx context.Context, line string) bool {
		session.prompt = func(label string, hidden bool) (string, error) {
			reply := make(chan string, 1)
			p.Send(promptRequest{label: label, hidden: hidden, reply: reply})
			select {
			case value := <-reply:
				return value, ctx.Err()
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		return session.handleLine(ctx, line)
	}
	m.ctx = ctx
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run terminal UI: %w", err)
	}
	return nil
}

type transcriptWriter struct{ send func(tea.Msg) }
type transcriptText string

func (w transcriptWriter) Write(p []byte) (int, error) {
	w.send(transcriptText(p))
	return len(p), nil
}

type promptRequest struct {
	label  string
	hidden bool
	reply  chan string
}

type turnDone bool

type pinned struct {
	ctx        context.Context
	run        func(context.Context, string) bool
	cancelTurn context.CancelFunc
	request    *promptRequest
	cancelling bool

	input        textinput.Model
	composer     textarea.Model
	transcript   viewport.Model
	output       strings.Builder
	continuation strings.Builder
	width        int
	height       int
	color        bool
}

func newPinned(banner string, color bool) *pinned {
	composer := textarea.New()
	composer.Prompt = ""
	composer.ShowLineNumbers = false
	composer.SetHeight(1)
	// Enter submits; Alt+Enter or Ctrl-J insert a newline via (*pinned).newline
	// (terminals cannot report Shift+Enter). Unbind the textarea's own newline key.
	composer.KeyMap.InsertNewline.SetEnabled(false)
	m := &pinned{input: textinput.New(), composer: composer, transcript: viewport.New(0, 0), color: color}
	m.appendOutput(banner)
	return m
}

func (m *pinned) Init() tea.Cmd { return m.composer.Focus() }

func (m *pinned) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		follow := m.transcript.AtBottom()
		m.width, m.height = msg.Width, msg.Height
		m.transcript.Width = max(1, m.width-1)
		m.transcript.Height = max(0, m.height-2)
		m.refreshTranscript(follow)
		m.resizeInput()
	case transcriptText:
		m.appendOutput(string(msg))
	case promptRequest:
		if !m.cancelling {
			m.request = &msg
			if msg.hidden {
				m.input.EchoMode = textinput.EchoNone
			}
			m.composer.Blur()
			m.input.Focus()
			m.resizeInput()
			return m, m.input.Focus()
		}
	case turnDone:
		m.cancelTurn()
		m.cancelTurn = nil
		m.cancelling = false
		m.resetInput()
		if msg {
			return m, tea.Quit
		}
		return m, m.composer.Focus()
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.transcript.ScrollUp(wheelLines)
			case tea.MouseButtonWheelDown:
				m.transcript.ScrollDown(wheelLines)
			}
		}
		return m, nil
	case tea.KeyMsg:
		if m.scrollKey(msg) {
			return m, nil
		}
		switch msg.Type {
		case tea.KeyPgUp:
			m.transcript.ScrollUp(max(1, m.transcript.Height))
		case tea.KeyPgDown:
			m.transcript.ScrollDown(max(1, m.transcript.Height))
		case tea.KeyHome:
			if m.request == nil && m.composer.Value() == "" {
				m.transcript.GotoTop()
			} else if m.request != nil {
				return m, m.editInput(msg)
			} else {
				return m, m.editComposer(msg)
			}
		case tea.KeyEnd:
			if m.request == nil && m.composer.Value() == "" {
				m.transcript.GotoBottom()
			} else if m.request != nil {
				return m, m.editInput(msg)
			} else {
				return m, m.editComposer(msg)
			}
		case tea.KeyCtrlC:
			if m.cancelTurn != nil {
				m.cancelling = true
				m.cancelTurn()
				m.resetInput()
				m.input.Blur()
			} else {
				m.resetInput()
				m.continuation.Reset()
				m.resizeInput()
			}
		case tea.KeyCtrlD:
			if m.cancelTurn == nil && m.request == nil && m.composer.Value() == "" && m.input.Value() == "" && m.continuation.Len() == 0 {
				return m, tea.Quit
			}
			if m.request != nil {
				return m, m.editInput(msg)
			}
			return m, m.editComposer(msg)
		case tea.KeyEnter:
			if msg.Alt && m.request == nil {
				return m, m.newline()
			}
			return m, m.submit()
		case tea.KeyCtrlJ:
			if m.request != nil {
				return m, m.submit()
			}
			return m, m.newline()
		default:
			if m.request != nil {
				return m, m.editInput(msg)
			}
			return m, m.editComposer(msg)
		}
	default:
		if m.request != nil {
			return m, m.editInput(msg)
		}
		return m, m.editComposer(msg)
	}
	return m, nil
}

const wheelLines = 3

// scrollKey handles line scrolling and jumps. Plain ↑/↓ scroll only while the input is
// empty, so they still move the cursor inside a multi-line prompt.
func (m *pinned) scrollKey(msg tea.KeyMsg) bool {
	idle := m.request == nil && m.composer.Value() == "" && m.continuation.Len() == 0
	switch msg.String() {
	case "shift+up", "alt+up", "ctrl+up":
		m.transcript.ScrollUp(1)
	case "shift+down", "alt+down", "ctrl+down":
		m.transcript.ScrollDown(1)
	case "up":
		if !idle {
			return false
		}
		m.transcript.ScrollUp(1)
	case "down":
		if !idle {
			return false
		}
		m.transcript.ScrollDown(1)
	case "ctrl+home":
		m.transcript.GotoTop()
	case "ctrl+end":
		m.transcript.GotoBottom()
	case "esc":
		if m.transcript.AtBottom() {
			return false
		}
		m.transcript.GotoBottom()
	default:
		return false
	}
	return true
}

func (m *pinned) editInput(msg tea.Msg) tea.Cmd {
	if !m.input.Focused() {
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

func (m *pinned) editComposer(msg tea.Msg) tea.Cmd {
	if !m.composer.Focused() {
		return nil
	}
	var cmd tea.Cmd
	m.composer, cmd = m.composer.Update(msg)
	m.resizeInput()
	return cmd
}

// newline inserts a line break directly; key-binding dispatch for ctrl+j is unreliable across terminals.
func (m *pinned) newline() tea.Cmd {
	if m.composer.Focused() {
		m.composer.InsertString("\n")
		m.resizeInput()
	}
	return nil
}

func (m *pinned) resizeInput() {
	available := max(1, m.width-2)
	labelWidth := min(ansi.StringWidth(m.label()), max(8, available/3))
	width := max(1, available-labelWidth)
	m.input.Width = width
	m.input.SetCursor(m.input.Position())
	m.composer.SetWidth(width)
	lines := visualInputLines(m.composer.Value(), width)
	m.composer.SetHeight(min(max(1, lines), max(1, m.height-2)))
	m.transcript.Height = max(0, m.height-m.composer.Height()-1)
}

// visualInputLines counts wrapped lines as well as explicit newlines. Without
// this, a long prompt can be clipped by the textarea even though there is
// enough vertical space for it.
func visualInputLines(value string, width int) int {
	if width < 1 {
		return 1
	}
	lines := 0
	for _, line := range strings.Split(value, "\n") {
		lineWidth := ansi.StringWidth(strings.ReplaceAll(line, "\t", "    "))
		lines += max(1, (lineWidth+width-1)/width)
	}
	return lines
}

func (m *pinned) resetInput() {
	m.input.Reset()
	m.composer.Reset()
	m.input.EchoMode = textinput.EchoNormal
	m.request = nil
	m.resizeInput()
}

func (m *pinned) submit() tea.Cmd {
	line := m.composer.Value()
	if m.request != nil {
		line = m.input.Value()
		m.request.reply <- line
		m.resetInput()
		m.input.Blur()
		return nil
	}
	if m.cancelTurn != nil {
		return nil
	}
	if prefix, ok := strings.CutSuffix(line, "\\"); ok && !strings.Contains(line, "\n") {
		m.continuation.WriteString(prefix + "\n")
		m.composer.Reset()
		m.resizeInput()
		return nil
	}
	m.composer.Reset()
	if m.continuation.Len() > 0 {
		line = m.continuation.String() + line
	}
	line = strings.TrimSpace(line)
	m.continuation.Reset()
	m.resizeInput()
	if line == "" {
		return nil
	}
	// Inline credentials are supported by the existing command but must not
	// be copied into the transcript.
	echo := line
	if strings.HasPrefix(line, "/api-key-set ") {
		echo = "/api-key-set [hidden]"
	}
	if rest, ok := strings.CutPrefix(line, "/usage-credit "); ok && rest != "login" && rest != "logout" {
		echo = "/usage-credit [hidden]"
	}
	m.appendOutput(green + "❯ " + reset + echo + "\n")
	m.transcript.GotoBottom()
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancelTurn = cancel
	m.composer.Blur()
	return func() tea.Msg { return turnDone(m.run(ctx, line)) }
}

// Keep recent scrollback bounded, trimming in chunks to avoid copying on every write.
const maxTranscriptBytes = 256 * 1024

func (m *pinned) appendOutput(text string) {
	follow := m.transcript.AtBottom()
	m.output.WriteString(terminalText(text, m.color))
	if m.output.Len() > maxTranscriptBytes {
		output := m.output.String()
		cut := len(output) - maxTranscriptBytes*3/4
		// Prefer a line boundary, but also handle a single oversized line.
		if newline := strings.IndexByte(output[cut:], '\n'); newline >= 0 && newline < 4096 {
			cut += newline + 1
		}
		for cut < len(output) && !utf8.RuneStart(output[cut]) {
			cut++
		}
		// terminalText permits only SGR escapes; never retain a partial one.
		if start := strings.LastIndexByte(output[:cut], '\x1b'); start >= 0 {
			if end := strings.IndexByte(output[start:], 'm'); end >= 0 && start+end >= cut {
				cut = start + end + 1
			}
		}
		if !follow && m.width > 0 {
			dropped := strings.Count(ansi.Hardwrap(output[:cut], m.transcript.Width, true), "\n")
			m.transcript.YOffset = max(0, m.transcript.YOffset-dropped)
		}
		m.output.Reset()
		m.output.WriteString(output[cut:])
	}
	m.refreshTranscript(follow)
}

func (m *pinned) refreshTranscript(follow bool) {
	if m.width == 0 {
		return
	}
	m.transcript.SetContent(ansi.Hardwrap(strings.TrimSuffix(m.output.String(), "\n"), m.transcript.Width, true))
	if follow {
		m.transcript.GotoBottom()
	} else {
		m.transcript.SetYOffset(m.transcript.YOffset)
	}
}

func (m *pinned) label() string {
	label := bold + green + "❯ " + reset
	if m.request != nil {
		label = strings.TrimLeft(m.request.label, " ")
	} else if m.continuation.Len() > 0 {
		label = dim + "… " + reset
	}
	return ansi.Truncate(terminalText(label, m.color), max(0, m.width-8), "…")
}

func (m *pinned) View() string {
	if m.width < 1 || m.height < 1 {
		return ""
	}
	width := max(1, m.width-1)
	var footer string
	switch {
	case m.cancelling:
		footer = "Cancelling…"
	case m.cancelTurn != nil && m.request == nil:
		footer = "Working… · Ctrl-C to cancel"
	default:
		input := m.input
		input.Prompt = ""
		if m.request != nil {
			footer = m.label() + input.View()
		} else {
			composer := m.composer
			composer.Prompt = ""
			footer = m.label() + strings.TrimSuffix(composer.View(), "\n")
		}
	}
	if !m.color {
		footer = ansi.Strip(footer)
	}
	footerLines := strings.Split(footer, "\n")
	for i := range footerLines {
		footerLines[i] = ansi.Truncate(footerLines[i], width, "")
	}
	footer = strings.Join(footerLines, "\n")
	footerHeight := len(footerLines)
	if m.height == 1 {
		return footerLines[len(footerLines)-1]
	}
	divider := strings.Repeat("─", width)
	transcriptHeight := max(0, m.height-footerHeight-1)
	if transcriptHeight == 0 {
		return divider + "\n" + footer
	}
	m.transcript.Height = transcriptHeight
	if below := m.transcript.TotalLineCount() - m.transcript.YOffset - transcriptHeight; below > 0 {
		hint := fmt.Sprintf("─ ↓ %d more lines · Esc to jump to latest ", below)
		if ansi.StringWidth(hint) < width {
			divider = hint + strings.Repeat("─", width-ansi.StringWidth(hint))
			if m.color {
				divider = yellow + divider + reset
			}
		}
	}
	return m.transcript.View() + "\n" + divider + "\n" + footer
}

// terminalText allows the REPL's styles but filters cursor movement and other
// terminal commands from model and tool output before it enters the viewport.
func terminalText(text string, color bool) string {
	text = strings.ToValidUTF8(text, "�")
	var b strings.Builder
	for len(text) > 0 {
		seq, _, n, _ := ansi.DecodeSequence(text, ansi.NormalState, nil)
		if n == 0 {
			break
		}
		text = text[n:]
		switch seq {
		case reset, bold, dim, red, green, yellow, cyan, mag:
			if color {
				b.WriteString(seq)
			}
		case "\t":
			b.WriteString("    ")
		case "\n":
			b.WriteByte('\n')
		default:
			if strings.IndexFunc(seq, unicode.IsControl) == -1 {
				b.WriteString(seq)
			}
		}
	}
	return b.String()
}
