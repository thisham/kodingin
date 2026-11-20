package cli

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestPinnedFooter(t *testing.T) {
	for _, size := range []tea.WindowSizeMsg{{Width: 80, Height: 24}, {Width: 20, Height: 4}, {Width: 10, Height: 1}} {
		m := newPinned(strings.Repeat("old output\n", 30), false)
		m.Init()
		m.Update(size)
		m.composer.SetValue(strings.Repeat("界", 30) + "draft")
		m.resizeInput()
		m.Update(transcriptText("new output\n"))

		lines := strings.Split(m.View(), "\n")
		if len(lines) != size.Height || (size.Width >= 80 && !strings.Contains(m.View(), "draft")) {
			t.Fatalf("%+v: unexpected layout: %q", size, lines)
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > size.Width {
				t.Fatalf("%+v: line exceeds width: %q", size, line)
			}
		}
		if size.Height > 6 {
			m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
			offset := m.transcript.YOffset
			m.Update(transcriptText("another line\n"))
			if m.transcript.AtBottom() || m.transcript.YOffset != offset {
				t.Fatal("output moved a transcript that was scrolled back")
			}
		}
	}
}

func TestPinnedBusyComposerHasDedicatedThreeLineAreaAndFooter(t *testing.T) {
	m := newPinned("", false)
	m.ctx = context.Background()
	m.run = func(context.Context, string) bool { return false }
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m.composer.SetValue("draft")
	m.resizeInput()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("turn did not start")
	}
	view := m.View()
	if m.composer.Height() != 3 || !strings.Contains(view, "draft") || !strings.Contains(view, "Esc to cancel") || !strings.Contains(view, "© sovcore") {
		t.Fatalf("unexpected busy composer layout (height %d):\n%s", m.composer.Height(), view)
	}
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		if ansi.StringWidth(line) > 100 {
			t.Fatalf("line %d exceeds width: %q", i, line)
		}
	}
}

func TestPinnedSubmission(t *testing.T) {
	for _, tt := range []struct {
		name    string
		input   string
		want    string
		private string
	}{
		{"single line", "hello", "hello", ""},
		{"multiline", "first\nsecond", "first\nsecond", ""},
		{"inline key", "/api-key-set fake-key", "/api-key-set fake-key", "fake-key"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newPinned("", false)
			m.ctx = context.Background()
			m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m.run = func(_ context.Context, line string) bool {
				if line != tt.want {
					t.Fatalf("submitted %q, want %q", line, tt.want)
				}
				return false
			}
			m.composer.SetValue(tt.input)
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd == nil {
				t.Fatal("submission did not start")
			}
			if !strings.Contains(m.View(), "Working") {
				t.Fatal("busy status is missing")
			}
			m.Update(cmd())
			if tt.private != "" && strings.Contains(m.output.String(), tt.private) {
				t.Fatal("credential was copied into the transcript")
			}
		})
	}
}

func TestPinnedNewlineKeys(t *testing.T) {
	for _, key := range []tea.KeyMsg{{Type: tea.KeyCtrlJ}, {Type: tea.KeyEnter, Alt: true}} {
		m := newPinned("", false)
		m.ctx = context.Background()
		m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		m.run = func(context.Context, string) bool { t.Fatal("newline key submitted"); return false }
		m.Init()
		m.composer.SetValue("first")
		m.Update(key)
		if m.cancelTurn != nil || m.composer.Value() != "first\n" {
			t.Fatalf("%v: composer = %q, want newline inserted", key, m.composer.Value())
		}
	}
}

func TestPinnedPrompt(t *testing.T) {
	for _, tt := range []struct {
		name   string
		hidden bool
		cancel bool
	}{
		{"approval", false, false},
		{"hidden key", true, false},
		{"cancel approval", false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newPinned("", false)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			m.cancelTurn = cancel
			m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			reply := make(chan string, 1)
			m.Update(promptRequest{label: "Prompt: ", hidden: tt.hidden, reply: reply})
			m.input.SetValue("value")
			if view := m.View(); !strings.Contains(view, "Prompt:") || strings.Contains(view, "value") == tt.hidden {
				t.Fatalf("unexpected prompt view: %q", view)
			}
			if tt.cancel {
				m.Update(tea.KeyMsg{Type: tea.KeyEsc})
				m.Update(promptRequest{label: "Late prompt", reply: reply})
				if ctx.Err() == nil || m.request != nil || len(reply) != 0 {
					t.Fatal("cancelled turn still accepts replies")
				}
			} else {
				m.Update(tea.KeyMsg{Type: tea.KeyEnter})
				select {
				case got := <-reply:
					if got != "value" {
						t.Fatalf("reply = %q", got)
					}
				default:
					t.Fatal("missing reply")
				}
			}
			m.Update(turnDone(false))
			if !m.composer.Focused() || m.composer.Value() != "" {
				t.Fatal("composer was not restored")
			}
		})
	}
}

func TestTerminalText(t *testing.T) {
	text := red + "日本語" + reset + "\x1b[2J\x1b]0;title\x07\r\t\n"
	for _, tt := range []struct {
		color bool
		want  string
	}{{true, red + "日本語" + reset + "    \n"}, {false, "日本語    \n"}} {
		if got := terminalText(text, tt.color); got != tt.want {
			t.Errorf("color=%v: terminalText = %q, want %q", tt.color, got, tt.want)
		}
	}
}

func TestPinnedScroll(t *testing.T) {
	newView := func() *pinned {
		m := newPinned("", false)
		m.ctx = context.Background()
		m.Init()
		m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
		for i := range 100 {
			m.appendOutput(fmt.Sprintf("line %d\n", i))
		}
		m.View()
		return m
	}
	wheel := func(b tea.MouseButton) tea.MouseMsg {
		return tea.MouseMsg{Action: tea.MouseActionPress, Button: b}
	}
	for _, tt := range []struct {
		name      string
		draft     string
		keys      []tea.Msg
		atBottom  bool
		offsetMin int // lines scrolled up from bottom, at least
		hint      bool
	}{
		{"follows output", "", nil, true, 0, false},
		{"wheel up", "", []tea.Msg{wheel(tea.MouseButtonWheelUp)}, false, wheelLines, true},
		{"wheel up then down", "", []tea.Msg{wheel(tea.MouseButtonWheelUp), wheel(tea.MouseButtonWheelDown)}, true, 0, false},
		{"shift up", "", []tea.Msg{tea.KeyMsg{Type: tea.KeyShiftUp}}, false, 1, true},
		{"arrow up when idle", "", []tea.Msg{tea.KeyMsg{Type: tea.KeyUp}}, false, 1, true},
		{"arrow up while typing edits instead", "draft", []tea.Msg{tea.KeyMsg{Type: tea.KeyUp}}, true, 0, false},
		{"page up then esc", "", []tea.Msg{tea.KeyMsg{Type: tea.KeyPgUp}, tea.KeyMsg{Type: tea.KeyEsc}}, true, 0, false},
		{"ctrl+home", "", []tea.Msg{tea.KeyMsg{Type: tea.KeyCtrlHome}}, false, 80, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newView()
			m.composer.SetValue(tt.draft)
			bottom := m.transcript.YOffset
			for _, k := range tt.keys {
				m.Update(k)
			}
			view := m.View()
			if m.transcript.AtBottom() != tt.atBottom || bottom-m.transcript.YOffset < tt.offsetMin {
				t.Fatalf("atBottom=%v offset=%d (bottom %d)", m.transcript.AtBottom(), m.transcript.YOffset, bottom)
			}
			if strings.Contains(view, "more lines") != tt.hint {
				t.Fatalf("hint shown = %v:\n%s", !tt.hint, view)
			}
			if m.composer.Value() != tt.draft {
				t.Fatalf("draft changed to %q", m.composer.Value())
			}
		})
	}
	// New output while scrolled up must not yank the view to the bottom.
	m := newView()
	m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	off := m.transcript.YOffset
	m.Update(transcriptText("more output\n"))
	if m.transcript.YOffset != off {
		t.Fatalf("view jumped from %d to %d", off, m.transcript.YOffset)
	}
}
