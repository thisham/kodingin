package cli

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPinnedPromptHistoryNavigation(t *testing.T) {
	m := newPinned("", false)
	m.promptHistory = []string{"older", "newer"}
	m.historyIndex = len(m.promptHistory)
	m.composer.SetValue("unsent draft")
	m.composer.CursorEnd()

	m.historyKey(-1)
	if got := m.composer.Value(); got != "newer" {
		t.Fatalf("up recalled %q, want newer", got)
	}
	m.historyKey(-1)
	if got := m.composer.Value(); got != "older" {
		t.Fatalf("second up recalled %q, want older", got)
	}
	m.historyKey(1)
	if got := m.composer.Value(); got != "newer" {
		t.Fatalf("down recalled %q, want newer", got)
	}
	m.historyKey(1)
	if got := m.composer.Value(); got != "unsent draft" {
		t.Fatalf("down restored %q, want draft", got)
	}
}

func TestPinnedUpDownPreserveMultilineCursorMovement(t *testing.T) {
	m := newPinned("", false)
	m.Init()
	m.composer.SetValue("first\nsecond")
	m.composer.CursorEnd()
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if got := m.composer.Line(); got != 0 || !strings.Contains(m.composer.Value(), "first\nsecond") {
		t.Fatalf("up failed to move cursor in multiline draft: line=%d value=%q", got, m.composer.Value())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if got := m.composer.Line(); got != 1 {
		t.Fatalf("down moved to line %d, want 1", got)
	}
}
