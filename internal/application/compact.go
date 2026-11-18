package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/thisham/kodingin/internal/domain/conversation"
)

const (
	compactToolCap  = 2000   // chars kept per tool result in the transcript
	compactTotalCap = 400000 // chars of transcript sent; oldest part dropped first
	compactMinTurns = 2
)

var (
	ErrNothingToCompact = errors.New("conversation is too short to compact")
	// ErrNoGain means the summary was not smaller than the history, so the history was kept.
	ErrNoGain = errors.New("summary would not be smaller than the conversation; history kept")
)

// Messages returns the current history (without the system prompt).
func (a *Agent) Messages() []conversation.Message { return a.conv.Messages()[1:] }

// Restore loads a saved history into this session.
func (a *Agent) Restore(msgs []conversation.Message) { a.conv.Restore(msgs) }

// Compact asks the model for a summary and replaces the history with it.
// It returns the approximate context size (chars/4) before and after.
// The transcript is sent as plain text without tools, which every provider accepts.
func (a *Agent) Compact(ctx context.Context, focus string, obs Observer) (before, after int, err error) {
	msgs := a.Messages()
	users := 0
	for _, m := range msgs {
		if m.Role == conversation.RoleUser {
			users++
		}
	}
	if users < compactMinTurns && len(msgs) < 4 {
		return 0, 0, ErrNothingToCompact
	}
	before = approxTokens(a.conv.Messages())

	instr := `Summarise this coding session so another engineer can continue it without the transcript.
Include: the user's goals and constraints, decisions made, files created or changed (with paths and what changed),
commands run and their outcome, current state (what works, what fails), and the exact next steps.
Keep code identifiers, paths and error messages verbatim. Be dense; use short bullet points under headings.`
	if f := strings.TrimSpace(focus); f != "" {
		instr += "\nPay special attention to: " + f
	}
	req := []conversation.Message{
		{Role: conversation.RoleSystem, Content: "You compress coding-agent conversations into accurate handover notes."},
		{Role: conversation.RoleUser, Content: instr + "\n\n<transcript>\n" + transcript(msgs) + "\n</transcript>"},
	}
	out, usage, err := a.llm.Complete(ctx, req, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("compact: %w", err)
	}
	obs.OnUsage(usage)
	summary := strings.TrimSpace(out.Content)
	if summary == "" {
		return 0, 0, errors.New("compact: model returned an empty summary; history kept")
	}
	trial := conversation.New(a.conv.Messages()[0].Content)
	trial.Compact(summary)
	if after = approxTokens(trial.Messages()); after >= before {
		return before, after, ErrNoGain
	}
	a.conv.Compact(summary)
	return before, after, nil
}

func transcript(msgs []conversation.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		switch m.Role {
		case conversation.RoleUser:
			fmt.Fprintf(&b, "\n## USER\n%s\n", m.Content)
		case conversation.RoleAssistant:
			if m.Content != "" {
				fmt.Fprintf(&b, "\n## ASSISTANT\n%s\n", m.Content)
			}
			for _, tc := range m.ToolCalls {
				fmt.Fprintf(&b, "\n## TOOL CALL %s %s\n", tc.Name, clip(tc.Arguments, compactToolCap))
			}
		case conversation.RoleTool:
			fmt.Fprintf(&b, "## TOOL RESULT\n%s\n", clip(m.Content, compactToolCap))
		}
	}
	s := b.String()
	if len(s) > compactTotalCap {
		s = "[…earlier part omitted…]\n" + s[len(s)-compactTotalCap:]
	}
	return strings.ToValidUTF8(s, "")
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf("…[%d more chars]", len(s)-n)
}

func approxTokens(msgs []conversation.Message) int {
	n := 0
	for _, m := range msgs {
		n += len(m.Content)
		for _, tc := range m.ToolCalls {
			n += len(tc.Name) + len(tc.Arguments)
		}
	}
	return n / 4
}
