package cli

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPinnedTranscriptBounded(t *testing.T) {
	for _, tt := range []struct {
		name, chunk string
		color       bool
	}{
		{"lines", strings.Repeat("old line\n", 40000), false},
		{"single line", strings.Repeat("x", maxTranscriptBytes*2), false},
		{"unicode", strings.Repeat("界", maxTranscriptBytes), false},
		{"styles", strings.Repeat(red+"x"+reset, maxTranscriptBytes/2), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newPinned(tt.chunk, tt.color)
			for range 3 {
				m.appendOutput(tt.chunk)
				m.appendOutput("latest output\n")
				out := m.output.String()
				if len(out) > maxTranscriptBytes {
					t.Fatalf("retained %d bytes", len(out))
				}
				if !utf8.ValidString(out) {
					t.Fatal("trim split a UTF-8 rune")
				}
				if !strings.HasSuffix(out, "latest output\n") {
					t.Fatal("latest output lost")
				}
				if terminalText(out, tt.color) != out {
					t.Fatal("trim split a terminal escape")
				}
			}
		})
	}
}

func TestPinnedTranscriptEvictionScroll(t *testing.T) {
	m := newPinned("", false)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	line := strings.Repeat("x", 63) + "\n"
	m.appendOutput(strings.Repeat(line, maxTranscriptBytes/len(line)))
	m.transcript.SetYOffset(2000)
	m.appendOutput("new\n")
	if m.transcript.YOffset != 975 {
		t.Fatalf("offset = %d, want 975", m.transcript.YOffset)
	}
	m.transcript.GotoBottom()
	m.appendOutput(strings.Repeat(line, maxTranscriptBytes/len(line)))
	if !m.transcript.AtBottom() {
		t.Fatal("eviction stopped following output")
	}
}
