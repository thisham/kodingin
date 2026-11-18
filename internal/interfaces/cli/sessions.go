package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/thisham/kodingin/internal/application"
	"github.com/thisham/kodingin/internal/domain/conversation"
	"github.com/thisham/kodingin/internal/domain/session"
)

const resumeListSize = 15

// SetSessionStore injects persistence for /resume.
func (r *REPL) SetSessionStore(repo session.Repository) { r.sessRepo = repo }

// saveSession writes the current conversation after each turn. A new session id is made lazily.
func (r *REPL) saveSession() {
	if r.sessRepo == nil {
		return
	}
	msgs := r.agent.Messages()
	if len(msgs) == 0 {
		return
	}
	now := time.Now()
	if r.sessID == "" {
		r.sessID, r.sessCreated = session.NewID(now), now
	}
	// The title is fixed by the first prompt and survives /compact.
	if r.sessTitle == "" {
		if t := session.TitleFrom(msgs); t != session.CompactedTitle {
			r.sessTitle = t
		}
	}
	title := r.sessTitle
	if title == "" {
		title = session.CompactedTitle
	}
	s := session.Session{ID: r.sessID, Workdir: r.workdir, Model: r.model, Title: title,
		Created: r.sessCreated, Updated: now, Messages: msgs}
	if err := r.sessRepo.Save(context.Background(), s); err != nil && !r.sessWarned {
		r.sessWarned = true
		fmt.Fprintf(r.out, "%snote: session not saved: %v%s\n", yellow, err, reset)
	}
}

// newSession detaches from the saved session so the next turn starts a new file.
func (r *REPL) newSession() { r.sessID, r.sessCreated, r.sessTitle = "", time.Time{}, "" }

// resumeCmd: "/resume" lists recent sessions; "/resume <n|id>" loads one.
func (r *REPL) resumeCmd(ctx context.Context, arg string) {
	if r.sessRepo == nil {
		fmt.Fprintf(r.out, "%ssessions are not available%s\n", red, reset)
		return
	}
	list, err := r.sessRepo.List(ctx, resumeListSize)
	if err != nil {
		fmt.Fprintf(r.out, "%s✗ %v%s\n", red, err, reset)
		return
	}
	if arg == "" {
		r.printSessions(list)
		return
	}
	id := arg
	if n, err := strconv.Atoi(arg); err == nil {
		if n < 1 || n > len(list) {
			fmt.Fprintf(r.out, "%sno session #%d. Use /resume to list them.%s\n", yellow, n, reset)
			return
		}
		id = list[n-1].ID
	}
	r.ResumeID(ctx, id)
}

// ResumeLatest loads the newest session (used by the -continue flag). Its messages are
// shown under the banner, so they survive the switch to the full-screen view.
func (r *REPL) ResumeLatest(ctx context.Context) {
	if r.sessRepo == nil {
		return
	}
	var buf strings.Builder
	out := r.out
	r.out = &buf
	defer func() { r.out, r.startNotice = out, buf.String() }()
	list, err := r.sessRepo.List(ctx, 1)
	if err != nil || len(list) == 0 {
		fmt.Fprintf(r.out, "%sno previous session to continue%s\n", dim, reset)
		return
	}
	r.ResumeID(ctx, list[0].ID)
}

func (r *REPL) ResumeID(ctx context.Context, id string) {
	if id == r.sessID {
		fmt.Fprintf(r.out, "%sthat is the current session%s\n", dim, reset)
		return
	}
	s, err := r.sessRepo.Load(ctx, id)
	if errors.Is(err, session.ErrNotFound) {
		fmt.Fprintf(r.out, "%sno session %q. Use /resume to list them.%s\n", yellow, id, reset)
		return
	}
	if err != nil {
		fmt.Fprintf(r.out, "%s✗ %v%s\n", red, err, reset)
		return
	}
	// The current conversation is already saved after its last turn, so nothing is lost.
	r.agent.Restore(s.Messages)
	r.sessID, r.sessCreated, r.sessTitle = s.ID, s.Created, ""
	if s.Title != session.CompactedTitle {
		r.sessTitle = s.Title
	}
	r.tokIn, r.tokOut, r.sessionUsage = 0, 0, nil
	fmt.Fprintf(r.out, "%s✓ resumed%s %s %s(%d turns, last active %s)%s\n", green, reset, s.Title, dim, s.Turns(), when(s.Updated, time.Now()), reset)
	if s.Workdir != "" && s.Workdir != r.workdir {
		fmt.Fprintf(r.out, "  %snote: it was started in %s; tools still work in %s%s\n", yellow, s.Workdir, r.workdir, reset)
	}
	if s.Model != "" && s.Model != r.model {
		fmt.Fprintf(r.out, "  %snote: it used model %s; now using %s (switch with /model %s)%s\n", yellow, s.Model, r.model, s.Model, reset)
	}
	r.printTail(r.agent.Messages())
}

func (r *REPL) printSessions(list []session.Session) {
	fmt.Fprintf(r.out, "\n%sSessions%s\n", bold, reset)
	if len(list) == 0 {
		fmt.Fprintf(r.out, "  %sno saved sessions yet%s\n\n", dim, reset)
		return
	}
	now := time.Now()
	for i, s := range list {
		cur := "  "
		if s.ID == r.sessID {
			cur = green + "● " + reset
		}
		where := ""
		if s.Workdir != r.workdir {
			where = " · " + shortPath(s.Workdir)
		}
		fmt.Fprintf(r.out, "%s%s%2d%s  %s\n      %s%s · %d turns%s%s\n", cur, cyan, i+1, reset, s.Title, dim, when(s.Updated, now), s.Turns(), where, reset)
	}
	fmt.Fprintf(r.out, "  %sresume with /resume <number>%s\n\n", dim, reset)
}

// printTail shows the last prompt and answer so the user sees where they left off.
func (r *REPL) printTail(msgs []conversation.Message) {
	var lastUser, lastAnswer string
	for i := len(msgs) - 1; i >= 0 && (lastUser == "" || lastAnswer == ""); i-- {
		m := msgs[i]
		if m.Role == conversation.RoleAssistant && m.Content != "" && lastAnswer == "" && lastUser == "" {
			lastAnswer = m.Content
		}
		if m.Role == conversation.RoleUser && lastUser == "" {
			lastUser = m.Content
		}
	}
	if strings.HasPrefix(lastUser, conversation.CompactPrefix) {
		lastUser = "(compacted summary)"
	}
	if lastUser != "" {
		fmt.Fprintf(r.out, "\n%s❯%s %s\n", green, reset, preview(lastUser, 4))
	}
	if lastAnswer != "" {
		fmt.Fprintf(r.out, "\n%s\n", preview(lastAnswer, 12))
	}
	fmt.Fprintln(r.out)
}

func preview(s string, maxLines int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], dim+fmt.Sprintf("… %d more lines", len(lines)-maxLines)+reset)
	}
	return strings.Join(lines, "\n")
}

// compactCmd summarises the conversation to free context.
func (r *REPL) compactCmd(ctx context.Context, focus string) {
	fmt.Fprintf(r.out, "%scompacting…%s\n", dim, reset)
	before, after, err := r.agent.Compact(ctx, focus, r)
	if errors.Is(err, application.ErrNothingToCompact) || errors.Is(err, application.ErrNoGain) {
		fmt.Fprintf(r.out, "%s%v%s\n", dim, err, reset)
		return
	}
	if err != nil {
		fmt.Fprintf(r.out, "%s✗ %v%s\n", red, err, reset)
		return
	}
	r.saveSession()
	fmt.Fprintf(r.out, "%s✓ compacted%s %s~%s → ~%s tokens of context%s\n", green, reset, dim, compact(before), compact(after), reset)
}
