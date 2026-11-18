package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/thisham/kodingin/internal/application"
	"github.com/thisham/kodingin/internal/domain/conversation"
	"github.com/thisham/kodingin/internal/domain/llm"
	"github.com/thisham/kodingin/internal/domain/session"
	"github.com/thisham/kodingin/internal/domain/tool"
	"github.com/thisham/kodingin/internal/infrastructure/sessionstore"
)

type replyLLM struct{ reply string }

func (f replyLLM) Complete(context.Context, []conversation.Message, []tool.Tool) (conversation.Message, llm.Usage, error) {
	return conversation.Message{Content: f.reply}, llm.Usage{PromptTokens: 1}, nil
}

func newTestREPL(t *testing.T, repo session.Repository, reply string) (*REPL, *bytes.Buffer) {
	var out bytes.Buffer
	a := application.NewAgent(replyLLM{reply}, tool.NewRegistry(), conversation.New("sys"), 3)
	r := &REPL{agent: a, out: &out, workdir: "/w", model: "m"}
	r.SetSessionStore(repo)
	return r, &out
}

func TestResumeFlow(t *testing.T) {
	ctx := context.Background()
	repo := sessionstore.New(t.TempDir())

	// Session 1: two turns, saved automatically.
	r1, _ := newTestREPL(t, repo, "answer one")
	r1.handleLine(ctx, "first task\nwith detail")
	r1.handleLine(ctx, "second step")
	first := r1.sessID
	if first == "" {
		t.Fatal("session not created")
	}
	// /clear starts a new session; the old one remains.
	r1.handleLine(ctx, "/clear")
	r1.handleLine(ctx, "other work")
	if r1.sessID == first {
		t.Fatal("/clear did not start a new session")
	}

	// A fresh process lists both and resumes the first by number.
	r2, out := newTestREPL(t, repo, "x")
	r2.handleLine(ctx, "/resume")
	got := out.String()
	if !strings.Contains(got, "first task with detail") || !strings.Contains(got, "other work") || !strings.Contains(got, "2 turns") {
		t.Fatalf("list:\n%s", got)
	}
	out.Reset()
	r2.handleLine(ctx, "/resume 2")
	if r2.sessID != first || len(r2.agent.Messages()) != 4 || !strings.Contains(out.String(), "resumed") || !strings.Contains(out.String(), "answer one") {
		t.Fatalf("resume: id=%s msgs=%d\n%s", r2.sessID, len(r2.agent.Messages()), out.String())
	}
	// Continuing appends to the same session file.
	r2.handleLine(ctx, "third step")
	s, _ := repo.Load(ctx, first)
	if s.Turns() != 3 {
		t.Fatalf("turns = %d", s.Turns())
	}

	for _, bad := range []string{"/resume 99", "/resume nope"} {
		out.Reset()
		r2.handleLine(ctx, bad)
		if !strings.Contains(out.String(), "no session") {
			t.Fatalf("%s:\n%s", bad, out.String())
		}
	}
}

func TestContinueFlagAndCompact(t *testing.T) {
	ctx := context.Background()
	repo := sessionstore.New(t.TempDir())
	r1, _ := newTestREPL(t, repo, "ok")
	r1.handleLine(ctx, "build it "+strings.Repeat("with many details ", 50))
	r1.handleLine(ctx, "test it")

	r2, out := newTestREPL(t, repo, "- summary")
	r2.ResumeLatest(ctx)
	if !strings.Contains(r2.startNotice, "resumed") || out.Len() != 0 {
		t.Fatalf("notice=%q out=%q", r2.startNotice, out.String())
	}
	r2.handleLine(ctx, "/compact the tests")
	msgs := r2.agent.Messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0].Content, "- summary") || !strings.Contains(out.String(), "compacted") {
		t.Fatalf("msgs=%+v\n%s", msgs, out.String())
	}
	s, _ := repo.Load(ctx, r2.sessID)
	if len(s.Messages) != 1 || !strings.HasPrefix(s.Title, "build it with many details") {
		t.Fatalf("saved after compact: %+v", s)
	}

	r3, out3 := newTestREPL(t, repo, "x")
	r3.handleLine(ctx, "/compact")
	if !strings.Contains(out3.String(), "too short") {
		t.Fatalf("%s", out3.String())
	}
}
