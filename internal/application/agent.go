// Package application holds use cases. The Agent orchestrates domain + ports; it knows nothing about HTTP or terminals.
package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/thisham/kodingin/internal/domain/conversation"
	"github.com/thisham/kodingin/internal/domain/llm"
	"github.com/thisham/kodingin/internal/domain/tool"
)

// Observer lets the presentation layer watch and gate the loop.
type Observer interface {
	OnAssistantText(text string)
	OnToolCall(call conversation.ToolCall)
	OnToolResult(call conversation.ToolCall, out string, err error)
	Approve(call conversation.ToolCall) bool
	OnUsage(u llm.Usage)
}

type Agent struct {
	llm      llm.Client
	tools    *tool.Registry
	conv     *conversation.Conversation
	maxSteps int
}

func NewAgent(c llm.Client, r *tool.Registry, conv *conversation.Conversation, maxSteps int) *Agent {
	return &Agent{llm: c, tools: r, conv: conv, maxSteps: maxSteps}
}

func (a *Agent) Reset() { a.conv.Reset() }

const maxToolOutput = 20000

// Ask runs one user turn: call the model, run tools it asks for, repeat until it answers in text.
func (a *Agent) Ask(ctx context.Context, input string, obs Observer) error {
	a.conv.AddUser(input)
	for step := 0; step < a.maxSteps; step++ {
		msg, usage, err := a.llm.Complete(ctx, a.conv.Messages(), a.tools.All())
		if err != nil {
			return err
		}
		obs.OnUsage(usage)
		a.conv.AddAssistant(msg)
		if msg.Content != "" {
			obs.OnAssistantText(msg.Content)
		}
		if len(msg.ToolCalls) == 0 {
			return nil
		}
		for _, call := range msg.ToolCalls {
			if err := ctx.Err(); err != nil {
				return err
			}
			a.conv.AddToolResult(call.ID, a.runTool(ctx, call, obs))
		}
	}
	return errors.New("stopped: max steps reached")
}

func (a *Agent) runTool(ctx context.Context, call conversation.ToolCall, obs Observer) string {
	obs.OnToolCall(call)
	t, err := a.tools.Get(call.Name)
	if err != nil {
		obs.OnToolResult(call, "", err)
		return "error: " + err.Error()
	}
	if t.Dangerous() && !obs.Approve(call) {
		obs.OnToolResult(call, "", errors.New("denied by user"))
		return "error: the user denied this action. Ask them how to proceed."
	}
	out, err := t.Run(ctx, []byte(call.Arguments))
	obs.OnToolResult(call, out, err)
	if len(out) > maxToolOutput {
		out = out[:maxToolOutput] + fmt.Sprintf("\n...[truncated %d bytes]", len(out)-maxToolOutput)
	}
	if err != nil {
		return fmt.Sprintf("error: %v\n%s", err, out)
	}
	return out
}

// Models exposes model switching when the LLM adapter supports it.
func (a *Agent) Models() (llm.ModelSelector, bool) {
	s, ok := a.llm.(llm.ModelSelector)
	return s, ok
}
