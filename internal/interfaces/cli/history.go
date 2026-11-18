package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/thisham/kodingin/internal/domain/history"
)

const defaultHistory = 20

// SetHistoryStore injects persistence for /history. path is shown to the user.
func (r *REPL) SetHistoryStore(repo history.Repository, path string) {
	r.historyRepo, r.historyPath = repo, path
}

func (r *REPL) recordPrompt(prompt string) {
	if r.historyRepo == nil {
		return
	}
	e := history.Entry{Time: time.Now(), Workdir: r.workdir, Prompt: prompt}
	if err := r.historyRepo.Append(context.Background(), e); err != nil && !r.historyWarned {
		r.historyWarned = true
		fmt.Fprintf(r.out, "%snote: prompt history not saved: %v%s\n", yellow, err, reset)
	}
}

// historyCmd lists the newest prompts, oldest first, numbered so the latest is #1.
func (r *REPL) historyCmd(ctx context.Context, arg string) {
	if r.historyRepo == nil {
		fmt.Fprintf(r.out, "%sprompt history is not available%s\n", red, reset)
		return
	}
	n := defaultHistory
	if arg != "" {
		v, err := strconv.Atoi(arg)
		if err != nil || v < 1 {
			fmt.Fprintf(r.out, "%susage: /history [count]%s\n", yellow, reset)
			return
		}
		n = min(v, 1000)
	}
	entries, err := r.historyRepo.Recent(ctx, n)
	if err != nil {
		fmt.Fprintf(r.out, "%s✗ %v%s\n", red, err, reset)
		return
	}
	fmt.Fprintf(r.out, "\n%sHistory%s %s%s%s\n", bold, reset, dim, r.historyPath, reset)
	if len(entries) == 0 {
		fmt.Fprintf(r.out, "  %sno prompts yet%s\n\n", dim, reset)
		return
	}
	now := time.Now()
	for i, e := range entries {
		num := len(entries) - i
		where := ""
		if e.Workdir != "" && e.Workdir != r.workdir {
			where = " " + shortPath(e.Workdir)
		}
		fmt.Fprintf(r.out, "  %s%3d%s  %s%s%s%s\n", cyan, num, reset, dim, when(e.Time, now), where, reset)
		lines := strings.Split(strings.TrimSpace(e.Prompt), "\n")
		for j, l := range lines {
			if j == 3 {
				fmt.Fprintf(r.out, "       %s… %d more lines%s\n", dim, len(lines)-3, reset)
				break
			}
			if rs := []rune(l); len(rs) > 160 {
				l = string(rs[:160]) + "…"
			}
			fmt.Fprintf(r.out, "       %s\n", l)
		}
	}
	fmt.Fprintln(r.out)
}

func when(t, now time.Time) string {
	if t.IsZero() {
		return "unknown time"
	}
	t = t.Local()
	y1, m1, d1 := t.Date()
	y2, m2, d2 := now.Date()
	if y1 == y2 && m1 == m2 && d1 == d2 {
		return "today " + t.Format("15:04")
	}
	return t.Format("2006-01-02 15:04")
}

func shortPath(p string) string {
	if rs := []rune(p); len(rs) > 50 {
		return "…" + string(rs[len(rs)-49:])
	}
	return p
}
