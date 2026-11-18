// Package llm is the port the domain uses to talk to a model. Adapters live in infrastructure.
package llm

import (
	"context"

	"github.com/thisham/kodingin/internal/domain/conversation"
	"github.com/thisham/kodingin/internal/domain/tool"
)

type Usage struct{ PromptTokens, CompletionTokens int }

type Client interface {
	Complete(ctx context.Context, msgs []conversation.Message, tools []tool.Tool) (conversation.Message, Usage, error)
}

// ModelSelector is implemented by clients that can switch models at runtime.
type ModelSelector interface {
	Model() string
	SetModel(model string)
	ListModels(ctx context.Context) ([]string, error)
}

// KeySetter is implemented by clients whose credentials can change at runtime.
type KeySetter interface{ SetAPIKey(key string) }
