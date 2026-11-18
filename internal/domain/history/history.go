// Package history keeps the prompts the user sent to the agent, across sessions.
package history

import (
	"context"
	"time"
)

// Entry is one prompt sent to the model. Slash commands are never recorded.
type Entry struct {
	Time    time.Time `json:"time"`
	Workdir string    `json:"workdir"`
	Prompt  string    `json:"prompt"`
}

// Repository persists entries.
type Repository interface {
	Append(ctx context.Context, e Entry) error
	// Recent returns up to n newest entries, oldest first.
	Recent(ctx context.Context, n int) ([]Entry, error)
}
