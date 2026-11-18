package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/thisham/kodingin/internal/domain/conversation"
	"github.com/thisham/kodingin/internal/domain/llm"
	"github.com/thisham/kodingin/internal/domain/tool"
)

type fakeLLM struct {
	reply string
	err   error
	got   []conversation.Message
	tools []tool.Tool
}

func (f *fakeLLM) Complete(_ context.Context, m []conversation.Message, t []tool.Tool) (conversation.Message, llm.Usage, error) {
	f.got, f.tools = m, t
	return conversation.Message{Content: f.reply}, llm.Usage{PromptTokens: 7}, f.err
}

type nopObs struct{ usage int }

func (nopObs) OnAssistantText(string)                            {}
func (nopObs) OnToolCall(conversation.ToolCall)                  {}
func (nopObs) OnToolResult(conversation.ToolCall, string, error) {}
func (nopObs) Approve(conversation.ToolCall) bool                { return true }
func (o *nopObs) OnUsage(u llm.Usage)                            { o.usage += u.PromptTokens }

func TestCompact(t *testing.T) {
	long := strings.Repeat("x", 5000)
	history := []conversation.Message{
		{Role: conversation.RoleUser, Content: "build a thing"},
		{Role: conversation.RoleAssistant, ToolCalls: []conversation.ToolCall{{ID: "1", Name: "read_file", Arguments: `{"path":"main.go"}`}}},
		{Role: conversation.RoleTool, ToolCallID: "1", Content: long},
		{Role: conversation.RoleAssistant, Content: "done"},
		{Role: conversation.RoleUser, Content: "now test it"},
	}
	for _, tt := range []struct {
		name    string
		history []conversation.Message
		reply   string
		llmErr  error
		wantErr bool
		wantLen int
	}{
		{"summarises", history, "- built thing", nil, false, 1},
		{"too short", history[:1], "x", nil, true, 1},
		{"llm error keeps history", history, "", errors.New("boom"), true, 5},
		{"empty summary keeps history", history, "  ", nil, true, 5},
		{"bigger summary keeps history", history, strings.Repeat("y", 20000), nil, true, 5},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeLLM{reply: tt.reply, err: tt.llmErr}
			a := NewAgent(f, tool.NewRegistry(), conversation.New("sys"), 5)
			a.Restore(tt.history)
			obs := &nopObs{}
			before, after, err := a.Compact(context.Background(), "tests", obs)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
			if len(a.Messages()) != tt.wantLen {
				t.Fatalf("history len = %d", len(a.Messages()))
			}
			if err == nil {
				if after >= before || f.tools != nil || obs.usage != 7 {
					t.Fatalf("before=%d after=%d tools=%v usage=%d", before, after, f.tools, obs.usage)
				}
				sent := f.got[1].Content
				if !strings.Contains(sent, "Pay special attention to: tests") || strings.Contains(sent, long) || !strings.Contains(sent, "main.go") {
					t.Fatal("transcript not built as expected")
				}
			}
		})
	}
}
