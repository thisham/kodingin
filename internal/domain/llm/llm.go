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
