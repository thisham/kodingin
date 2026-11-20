package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/thisham/kodingin/internal/domain/conversation"
	"github.com/thisham/kodingin/internal/domain/llm"
)

// HarnessReport is the machine-readable result of one autonomous agent run.
// It is deliberately independent of the terminal UI so it can be consumed by
// CI jobs and evaluation runners.
type HarnessReport struct {
	Prompt     string        `json:"prompt"`
	Success    bool          `json:"success"`
	DurationMS int64         `json:"duration_ms"`
	Assistant  []string      `json:"assistant"`
	ToolCalls  []HarnessCall `json:"tool_calls"`
	Usage      []llm.Usage   `json:"usage"`
	Error      string        `json:"error,omitempty"`
}

// HarnessCall records a capability invocation and its result. Output is
// bounded because reports commonly get persisted by CI systems.
type HarnessCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
	Approved  bool   `json:"approved"`
}

const maxHarnessOutput = 20000

type harnessObserver struct {
	report *HarnessReport
	calls  map[string]int
}

func (o *harnessObserver) OnAssistantText(text string) {
	o.report.Assistant = append(o.report.Assistant, text)
}
func (o *harnessObserver) OnUsage(u llm.Usage) { o.report.Usage = append(o.report.Usage, u) }
func (o *harnessObserver) OnToolCall(call conversation.ToolCall) {
	o.calls[call.ID] = len(o.report.ToolCalls)
	o.report.ToolCalls = append(o.report.ToolCalls, HarnessCall{
		ID: call.ID, Name: call.Name, Arguments: call.Arguments, Approved: true,
	})
}
func (o *harnessObserver) Approve(conversation.ToolCall) bool { return true }
func (o *harnessObserver) OnToolResult(call conversation.ToolCall, output string, err error) {
	i, ok := o.calls[call.ID]
	if !ok {
		return
	}
	if len(output) > maxHarnessOutput {
		output = output[:maxHarnessOutput]
	}
	o.report.ToolCalls[i].Output = output
	if err != nil {
		o.report.ToolCalls[i].Error = err.Error()
	}
}

// RunHarness executes one prompt without terminal interaction and records the
// complete agent trace. Harness mode intentionally auto-approves dangerous
// tools; callers should run it in an isolated workspace when evaluating code.
func RunHarness(ctx context.Context, agent *Agent, prompt string) HarnessReport {
	started := time.Now()
	report := HarnessReport{Prompt: prompt, Assistant: []string{}, ToolCalls: []HarnessCall{}, Usage: []llm.Usage{}}
	observer := &harnessObserver{report: &report, calls: make(map[string]int)}
	if err := agent.Ask(ctx, prompt, observer); err != nil {
		report.Error = err.Error()
	}
	report.Success = report.Error == ""
	report.DurationMS = time.Since(started).Milliseconds()
	return report
}

// MarshalHarnessReport is kept as a small helper for delivery adapters that
// want stable JSON output without depending on the report's implementation.
func MarshalHarnessReport(report HarnessReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}
