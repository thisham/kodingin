// Package llm adapts the OpenAI-compatible chat/completions API (tokenizer.id/v1) to the domain llm.Client port.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/thisham/kodingin/internal/domain/conversation"
	domainllm "github.com/thisham/kodingin/internal/domain/llm"
	"github.com/thisham/kodingin/internal/domain/tool"
)

type Client struct {
	baseURL, apiKey, model string
	http                   *http.Client
}

func New(baseURL, apiKey, model string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, model: model,
		http: &http.Client{Timeout: 5 * time.Minute}}
}

type wireToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type wireMsg struct {
	Role       string         `json:"role"`
	Content    *string        `json:"content"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type wireTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

type request struct {
	Model    string     `json:"model"`
	Messages []wireMsg  `json:"messages"`
	Tools    []wireTool `json:"tools,omitempty"`
}

type response struct {
	Choices []struct {
		Message wireMsg `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (c *Client) Complete(ctx context.Context, msgs []conversation.Message, tools []tool.Tool) (conversation.Message, domainllm.Usage, error) {
	req := request{Model: c.model}
	for _, m := range msgs {
		w := wireMsg{Role: string(m.Role), ToolCallID: m.ToolCallID}
		content := m.Content
		w.Content = &content
		for _, tc := range m.ToolCalls {
			wt := wireToolCall{ID: tc.ID, Type: "function"}
			wt.Function.Name, wt.Function.Arguments = tc.Name, tc.Arguments
			w.ToolCalls = append(w.ToolCalls, wt)
		}
		req.Messages = append(req.Messages, w)
	}
	for _, t := range tools {
		wt := wireTool{Type: "function"}
		wt.Function.Name, wt.Function.Description, wt.Function.Parameters = t.Name(), t.Description(), t.Schema()
		req.Tools = append(req.Tools, wt)
	}

	body, _ := json.Marshal(req)
	var (
		resp *http.Response
		err  error
	)
	for attempt := 0; attempt < 3; attempt++ {
		hr, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
		hr.Header.Set("Content-Type", "application/json")
		hr.Header.Set("Authorization", "Bearer "+c.apiKey)
		resp, err = c.http.Do(hr)
		if err == nil && resp.StatusCode != 429 && resp.StatusCode < 500 {
			break
		}
		if err == nil && attempt < 2 {
			resp.Body.Close()
		}
		select {
		case <-ctx.Done():
			return conversation.Message{}, domainllm.Usage{}, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 2 * time.Second):
		}
	}
	if err != nil {
		return conversation.Message{}, domainllm.Usage{}, fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var r response
	if err := json.Unmarshal(raw, &r); err != nil {
		return conversation.Message{}, domainllm.Usage{}, fmt.Errorf("llm http %d: %s", resp.StatusCode, truncate(string(raw)))
	}
	if r.Error != nil {
		return conversation.Message{}, domainllm.Usage{}, fmt.Errorf("llm %s: %s", r.Error.Type, r.Error.Message)
	}
	if len(r.Choices) == 0 {
		return conversation.Message{}, domainllm.Usage{}, fmt.Errorf("llm http %d: no choices", resp.StatusCode)
	}
	w := r.Choices[0].Message
	out := conversation.Message{Role: conversation.RoleAssistant}
	if w.Content != nil {
		out.Content = *w.Content
	}
	for _, tc := range w.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, conversation.ToolCall{ID: tc.ID, Name: tc.Function.Name, Arguments: tc.Function.Arguments})
	}
	return out, domainllm.Usage{PromptTokens: r.Usage.PromptTokens, CompletionTokens: r.Usage.CompletionTokens}, nil
}

func truncate(s string) string {
	if len(s) > 500 {
		return s[:500] + "..."
	}
	return s
}

func (c *Client) Model() string     { return c.model }
func (c *Client) SetModel(m string) { c.model = m }

// ListModels calls GET /models.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	hr, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	hr.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(hr)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var r struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("list models http %d: %s", resp.StatusCode, truncate(string(raw)))
	}
	if r.Error != nil {
		return nil, fmt.Errorf("list models: %s", r.Error.Message)
	}
	ids := make([]string, 0, len(r.Data))
	for _, d := range r.Data {
		ids = append(ids, d.ID)
	}
	return ids, nil
}

func (c *Client) SetAPIKey(k string) { c.apiKey = k }
