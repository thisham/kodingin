// Package session is a saved conversation that can be resumed later.
package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/thisham/kodingin/internal/domain/conversation"
)

var ErrNotFound = errors.New("session not found")

type Session struct {
	ID       string                 `json:"id"`
	Workdir  string                 `json:"workdir"`
	Model    string                 `json:"model"`
	Title    string                 `json:"title"`
	Created  time.Time              `json:"created"`
	Updated  time.Time              `json:"updated"`
	Messages []conversation.Message `json:"messages"`
}

// NewID is sortable by creation time and unique enough for one user.
func NewID(now time.Time) string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return now.UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b)
}

// TitleFrom builds a one-line title from the first real user prompt.
func TitleFrom(msgs []conversation.Message) string {
	for _, m := range msgs {
		if m.Role != conversation.RoleUser || strings.HasPrefix(m.Content, conversation.CompactPrefix) {
			continue
		}
		t := strings.Join(strings.Fields(m.Content), " ")
		if r := []rune(t); len(r) > 72 {
			t = string(r[:72]) + "…"
		}
		return t
	}
	return CompactedTitle
}

// CompactedTitle is used when no original prompt is left to name the session.
const CompactedTitle = "(compacted session)"

// Turns counts user prompts.
func (s Session) Turns() int {
	n := 0
	for _, m := range s.Messages {
		if m.Role == conversation.RoleUser {
			n++
		}
	}
	return n
}

type Repository interface {
	Save(ctx context.Context, s Session) error
	Load(ctx context.Context, id string) (Session, error)
	// List returns sessions newest first, Messages left empty except for turn counting.
	List(ctx context.Context, limit int) ([]Session, error)
}
