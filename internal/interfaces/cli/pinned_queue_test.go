package cli

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPinnedQueuesWhileRunningAndKeepsComposerAvailable(t *testing.T) {
	m := newPinned("", false)
	m.ctx = context.Background()
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	var got []string
	m.run = func(_ context.Context, line string) bool { got = append(got, line); return false }
	m.composer.SetValue("first")
	_, first := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if first == nil || !strings.Contains(m.View(), "Working") || !m.composer.Focused() {
		t.Fatal("first run did not start with an available composer")
	}
	m.composer.SetValue("second")
	_, queued := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if queued != nil || len(m.queued) != 1 || m.queued[0] != "second" {
		t.Fatalf("queued = %v, cmd = %v", m.queued, queued)
	}
	// Complete the active turn. The next queued turn starts without blocking the UI.
	_, queuedCmd := m.Update(turnDone(false))
	if queuedCmd == nil || m.cancelTurn == nil || len(m.queued) != 0 {
		t.Fatal("queued turn did not start")
	}
	// Finish the queued run, then check that the UI is ready for another prompt.
	m.Update(turnDone(false))
	if m.cancelTurn != nil || !m.composer.Focused() {
		t.Fatal("composer not restored after queued work")
	}
}

func TestPinnedUsageUpdate(t *testing.T) {
	m := newPinned("", false)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m.ctx = context.Background()
	m.run = func(context.Context, string) bool { return false }
	m.composer.SetValue("prompt")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(usageUpdate{input: 1234, output: 56})
	if !strings.Contains(m.View(), "1234 in") || !strings.Contains(m.View(), "56 out") {
		t.Fatalf("usage missing from view: %s", m.View())
	}
	m.Update(cmd())
}
