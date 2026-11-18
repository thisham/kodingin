package sessionstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thisham/kodingin/internal/domain/conversation"
	"github.com/thisham/kodingin/internal/domain/session"
)

func TestDir(t *testing.T) {
	ctx := context.Background()
	d := New(filepath.Join(t.TempDir(), "sessions"))
	if l, err := d.List(ctx, 10); err != nil || len(l) != 0 {
		t.Fatalf("empty: %v %v", l, err)
	}
	now := time.Now()
	msgs := []conversation.Message{{Role: conversation.RoleUser, Content: "hi"},
		{Role: conversation.RoleAssistant, ToolCalls: []conversation.ToolCall{{ID: "1", Name: "read_file", Arguments: `{"path":"a"}`}}}}
	for i, id := range []string{"a-1", "b-2", "c-3"} {
		s := session.Session{ID: id, Updated: now.Add(time.Duration(i) * time.Minute), Messages: msgs}
		if err := d.Save(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.Save(ctx, session.Session{ID: "empty"}); err != nil {
		t.Fatal(err)
	}
	l, err := d.List(ctx, 2)
	if err != nil || len(l) != 2 || l[0].ID != "c-3" || l[1].ID != "b-2" {
		t.Fatalf("list = %+v %v", l, err)
	}
	s, err := d.Load(ctx, "a-1")
	if err != nil || s.Messages[1].ToolCalls[0].Arguments != `{"path":"a"}` {
		t.Fatalf("load = %+v %v", s, err)
	}
	for _, bad := range []string{"missing", "../etc/passwd", ""} {
		if _, err := d.Load(ctx, bad); !errors.Is(err, session.ErrNotFound) {
			t.Fatalf("%q: err = %v", bad, err)
		}
	}
	st, _ := os.Stat(filepath.Join(d.Path, "a-1.json"))
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", st.Mode().Perm())
	}
}
