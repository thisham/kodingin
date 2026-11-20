package application

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thisham/kodingin/internal/domain/conversation"
	"github.com/thisham/kodingin/internal/domain/llm"
	"github.com/thisham/kodingin/internal/domain/tool"
)

type harnessClient struct{}

func (harnessClient) Complete(_ context.Context, msgs []conversation.Message, _ []tool.Tool) (conversation.Message, llm.Usage, error) {
	if msgs[len(msgs)-1].Role == conversation.RoleUser {
		return conversation.Message{ToolCalls: []conversation.ToolCall{{ID: "1", Name: "test", Arguments: "{}"}}}, llm.Usage{Model: "test"}, nil
	}
	return conversation.Message{Content: "done"}, llm.Usage{Model: "test"}, nil
}

type harnessTool struct{}

func (harnessTool) Name() string                                         { return "test" }
func (harnessTool) Description() string                                  { return "test" }
func (harnessTool) Schema() json.RawMessage                              { return json.RawMessage(`{"type":"object"}`) }
func (harnessTool) Dangerous() bool                                      { return false }
func (harnessTool) Run(context.Context, json.RawMessage) (string, error) { return "ok", nil }

func TestRunHarness(t *testing.T) {
	a := NewAgent(harnessClient{}, tool.NewRegistry(harnessTool{}), conversation.New("system"), 3)
	report := RunHarness(context.Background(), a, "hello")
	if !report.Success || report.Error != "" {
		t.Fatalf("report = %+v", report)
	}
	if len(report.ToolCalls) != 1 || report.ToolCalls[0].Output != "ok" {
		t.Fatalf("tool trace = %+v", report.ToolCalls)
	}
	if len(report.Assistant) != 1 || report.Assistant[0] != "done" {
		t.Fatalf("assistant trace = %+v", report.Assistant)
	}
}
