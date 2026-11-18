package cli

import (
	"context"
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
		if size.Height > 2 {
			m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
			offset := m.transcript.YOffset
			m.Update(transcriptText("another line\n"))
			if m.transcript.AtBottom() || m.transcript.YOffset != offset {
				t.Fatal("output moved a transcript that was scrolled back")
			}
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
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
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
				m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
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
