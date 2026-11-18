package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/thisham/kodingin/internal/domain/history"
)

type memHistory struct{ entries []history.Entry }

func (m *memHistory) Append(_ context.Context, e history.Entry) error {
	m.entries = append(m.entries, e)
	return nil
}

func (m *memHistory) Recent(_ context.Context, n int) ([]history.Entry, error) {
	if len(m.entries) > n {
		return m.entries[len(m.entries)-n:], nil
	}
	return m.entries, nil
}

func TestHistoryCommand(t *testing.T) {
	for _, tt := range []struct {
		name, arg     string
		prompts       []string
		want, notWant []string
	}{
		{"empty", "", nil, []string{"no prompts yet"}, nil},
		{"lists newest last", "", []string{"first task", "second task"}, []string{"  2", "first task", "  1", "second task"}, nil},
		{"count", "1", []string{"old", "new"}, []string{"new"}, []string{"old"}},
		{"multiline folded", "", []string{"a\nb\nc\nd\ne"}, []string{"a", "c", "2 more lines"}, []string{"\n       d"}},
		{"bad count", "x", nil, []string{"usage: /history"}, []string{"History"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			repo := &memHistory{}
			r := &REPL{out: &out, workdir: "/w"}
			r.SetHistoryStore(repo, "/tmp/history.jsonl")
			for _, p := range tt.prompts {
				r.recordPrompt(p)
			}
			r.historyCmd(context.Background(), tt.arg)
			got := out.String()
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("missing %q in:\n%s", w, got)
				}
			}
			for _, w := range tt.notWant {
				if strings.Contains(got, w) {
					t.Errorf("unexpected %q in:\n%s", w, got)
				}
			}
		})
	}
}

func TestSlashCommandsNotRecorded(t *testing.T) {
	repo := &memHistory{}
	r := &REPL{out: &bytes.Buffer{}}
	r.SetHistoryStore(repo, "")
	r.handleLine(context.Background(), "/api-key-set secret-key")
	r.handleLine(context.Background(), "/history")
	if len(repo.entries) != 0 {
		t.Fatalf("recorded %+v", repo.entries)
	}
}
