package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/thisham/kodingin/internal/domain/credit"
	"github.com/thisham/kodingin/internal/domain/llm"
	"github.com/thisham/kodingin/internal/domain/usage"
)

const portalCookieKey = "KODINGIN_PORTAL_COOKIE"

// CreditSource reads the portal report and lets the session swap its cookie.
type CreditSource interface {
	credit.Reader
	SetCookie(cookie string)
	HasCookie() bool
}

// SetUsageStore injects persistence for /usage history. historyPath is shown to the user.
func (r *REPL) SetUsageStore(repo usage.Repository, historyPath string) {
	r.usageRepo, r.usagePath = repo, historyPath
}

// SetCreditSource injects the portal client used by /usage-credit.
func (r *REPL) SetCreditSource(c CreditSource) { r.credit = c }

func (r *REPL) recordUsage(u llm.Usage) {
	rec := usage.Record{Time: time.Now(), Model: u.Model, Prompt: u.PromptTokens, Cached: u.CachedTokens, Completion: u.CompletionTokens}
	if rec.Model == "" {
		rec.Model = r.model
	}
	r.sessionUsage = append(r.sessionUsage, rec)
	if r.usageRepo == nil {
		return
	}
	if err := r.usageRepo.Append(context.Background(), rec); err != nil && !r.usageWarned {
		r.usageWarned = true
		fmt.Fprintf(r.out, "%snote: usage history not saved: %v%s\n", yellow, err, reset)
	}
}

// usageCmd prints this session per model, then saved history for today, 7 and 30 days.
func (r *REPL) usageCmd(ctx context.Context) {
	s := usage.Summarize(r.sessionUsage)
	fmt.Fprintf(r.out, "\n%sThis session%s\n", bold, reset)
	if s.Total.Calls == 0 {
		fmt.Fprintf(r.out, "  %sno model calls yet%s\n", dim, reset)
	} else {
		rows := [][]string{{"MODEL", "CALLS", "INPUT", "CACHED", "OUTPUT"}}
		for _, m := range s.ByModel {
			rows = append(rows, []string{m.Model, fmt.Sprint(m.Calls), compact(m.Prompt), compact(m.Cached), compact(m.Completion)})
		}
		if len(s.ByModel) > 1 {
			rows = append(rows, []string{"total", fmt.Sprint(s.Total.Calls), compact(s.Total.Prompt), compact(s.Total.Cached), compact(s.Total.Completion)})
		}
		r.table(rows, false)
	}

	fmt.Fprintf(r.out, "\n%sHistory%s %s%s%s\n", bold, reset, dim, r.usagePath, reset)
	if r.usageRepo == nil {
		fmt.Fprintf(r.out, "  %snot available%s\n\n", dim, reset)
		return
	}
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	recs, err := r.usageRepo.Since(ctx, midnight.AddDate(0, 0, -29))
	if err != nil {
		fmt.Fprintf(r.out, "  %s✗ %v%s\n\n", red, err, reset)
		return
	}
	if len(recs) == 0 {
		fmt.Fprintf(r.out, "  %sno saved history yet%s\n\n", dim, reset)
		return
	}
	rows := [][]string{{"PERIOD", "CALLS", "INPUT", "CACHED", "OUTPUT", "TOP MODEL"}}
	for _, p := range []struct {
		label string
		from  time.Time
	}{{"today", midnight}, {"7 days", midnight.AddDate(0, 0, -6)}, {"30 days", midnight.AddDate(0, 0, -29)}} {
		ps := usage.Summarize(usage.Since(recs, p.from))
		top := "-"
		if len(ps.ByModel) > 0 {
			top = ps.ByModel[0].Model
		}
		rows = append(rows, []string{p.label, fmt.Sprint(ps.Total.Calls), compact(ps.Total.Prompt), compact(ps.Total.Cached), compact(ps.Total.Completion), top})
	}
	r.table(rows, true)
	fmt.Fprintf(r.out, "  %sCounted by kodingin on this machine. Use /usage-credit for the account balance.%s\n\n", dim, reset)
}

// usageCreditCmd: "/usage-credit" shows portal credit, "login" saves a portal cookie, "logout" clears it.
func (r *REPL) usageCreditCmd(ctx context.Context, arg string) {
	if r.credit == nil {
		fmt.Fprintf(r.out, "%scredit lookup is not available%s\n", red, reset)
		return
	}
	// pending is a newly pasted cookie; it is saved only after the portal accepts it.
	var pending string
	switch {
	case arg == "logout":
		r.credit.SetCookie("")
		if r.saveSettings != nil {
			if _, err := r.saveSettings(map[string]string{portalCookieKey: ""}); err != nil {
				fmt.Fprintf(r.out, "%s✗ %v%s\n", red, err, reset)
				return
			}
		}
		fmt.Fprintf(r.out, "%s✓ portal session removed%s\n", green, reset)
		return
	case arg == "login":
		pending = r.portalLogin()
		if pending == "" {
			return
		}
	case arg != "":
		fmt.Fprintf(r.out, "%susage: /usage-credit [login|logout]. Paste the cookie at the hidden prompt, not on the command line.%s\n", yellow, reset)
		return
	case !r.credit.HasCookie():
		fmt.Fprintf(r.out, "  tokenizer.id shows credit only to a logged-in portal session, not to the API key.\n")
		pending = r.portalLogin()
		if pending == "" {
			return
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	rep, err := r.credit.Credit(ctx, 30)
	if errors.Is(err, credit.ErrNoSession) {
		if pending != "" {
			r.credit.SetCookie("")
			fmt.Fprintf(r.out, "  %s✗ the portal rejected that cookie, nothing saved. Copy it again from a logged-in tab.%s\n", red, reset)
			return
		}
		fmt.Fprintf(r.out, "  %s✗ portal session missing or expired. Run /usage-credit login with a fresh cookie.%s\n", red, reset)
		return
	}
	if err != nil {
		fmt.Fprintf(r.out, "  %s✗ %v%s\n", red, err, reset)
		return
	}
	if pending != "" {
		r.savePortalCookie(pending)
	}
	r.printCredit(rep)
}

// portalLogin asks for a cookie and applies it to this session only. It returns "" when nothing usable was pasted.
func (r *REPL) portalLogin() string {
	fmt.Fprintf(r.out, "  %sIn a browser logged in to tokenizer.id: DevTools → Network → open any /portal/api request → Request Headers → copy the Cookie value.%s\n", dim, reset)
	v, err := r.askInput("  paste portal Cookie (hidden): ", true)
	if err != nil {
		return ""
	}
	cookie := strings.TrimSpace(v)
	if strings.HasPrefix(strings.ToLower(cookie), "cookie:") {
		cookie = strings.TrimSpace(cookie[len("cookie:"):])
	}
	if cookie == "" || !strings.Contains(cookie, "=") || strings.ContainsAny(cookie, "\r\n") {
		fmt.Fprintf(r.out, "  %s✗ that does not look like a Cookie header (expected name=value), nothing saved%s\n", red, reset)
		return ""
	}
	r.credit.SetCookie(cookie)
	return cookie
}

func (r *REPL) savePortalCookie(cookie string) {
	if r.saveSettings == nil {
		return
	}
	path, err := r.saveSettings(map[string]string{portalCookieKey: cookie})
	if err != nil {
		fmt.Fprintf(r.out, "  %s✗ save failed (used for this session only): %v%s\n", red, err, reset)
		return
	}
	fmt.Fprintf(r.out, "  %s✓ portal session saved to %s (mode 600)%s\n", green, path, reset)
}

func (r *REPL) printCredit(rep credit.Report) {
	fmt.Fprintf(r.out, "\n%sAccount credit%s %s(tokenizer.id portal)%s\n", bold, reset, dim, reset)
	fmt.Fprintf(r.out, "  This month   %s · %d requests\n", usd(rep.MonthCostUSD), rep.MonthRequests)

	var req, in, out int64
	var cost float64
	for _, d := range rep.Daily {
		req, in, out, cost = req+d.Requests, in+d.PromptTokens, out+d.CompletionTokens, cost+d.CostUSD
	}
	fmt.Fprintf(r.out, "  Last %d days %s · %d requests · %s in / %s out\n", rep.Days, usd(cost), req, compact(int(in)), compact(int(out)))

	fmt.Fprintf(r.out, "\n%sLimits%s\n", bold, reset)
	if len(rep.Budgets) == 0 {
		fmt.Fprintf(r.out, "  %sUnrestricted. No budget limits on this key.%s\n", green, reset)
	}
	for _, b := range rep.Budgets {
		name := "All-time"
		if b.Period != "total" && b.Period != "" {
			name = strings.ToUpper(b.Period[:1]) + b.Period[1:]
		}
		flag := ""
		if b.Alert {
			flag = " " + red + "EXCEEDED" + reset
		}
		fmt.Fprintf(r.out, "  %s limit%s\n", name, flag)
		if b.LimitTokens > 0 {
			fmt.Fprintf(r.out, "    tokens  %s %s / %s · %s left\n", bar(b.TokensPctUsed, b.Alert), compact(int(b.TokensUsed)), compact(int(b.LimitTokens)), compact(int(b.TokensRemaining)))
		}
		if b.LimitUSD > 0 {
			fmt.Fprintf(r.out, "    spend   %s %s / %s · %s left\n", bar(b.USDPctUsed, b.Alert), usd(b.SpentUSD), usd(b.LimitUSD), usd(b.USDRemaining))
		}
	}

	if p := rep.Package; p != nil && p.Exists {
		pct := 0.0
		if p.LimitTokens > 0 {
			pct = float64(p.UsedTokens) / float64(p.LimitTokens) * 100
		}
		fmt.Fprintf(r.out, "\n%sToken package%s %s\n", bold, reset, p.PlanName)
		fmt.Fprintf(r.out, "    tokens  %s %s / %s · %s left\n", bar(pct, false), compact(int(p.UsedTokens)), compact(int(p.LimitTokens)), compact(int(p.Remaining())))
		if !p.ExpiresAt.IsZero() {
			fmt.Fprintf(r.out, "    expires %s\n", p.ExpiresAt.Local().Format("2006-01-02 15:04"))
		}
	}
	fmt.Fprintln(r.out)
}

// bar renders a 20-cell meter coloured by how much is used.
func bar(pct float64, alert bool) string {
	pct = min(max(pct, 0), 100)
	n := int(pct/5 + 0.5)
	col := green
	switch {
	case alert || pct >= 90:
		col = red
	case pct >= 70:
		col = yellow
	}
	return fmt.Sprintf("%s%s%s%s %3.0f%%", col, strings.Repeat("█", n), dim+strings.Repeat("░", 20-n), reset, pct)
}

func compact(n int) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1e9)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 10_000:
		return fmt.Sprintf("%.1fK", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

func usd(v float64) string { return fmt.Sprintf("$%.2f", v) }

// table prints rows with the first column left-aligned and numbers right-aligned.
// When lastLeft is set the final column is text and left-aligned too. The first row is a dim header.
func (r *REPL) table(rows [][]string, lastLeft bool) {
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, c := range row {
			widths[i] = max(widths[i], len([]rune(c)))
		}
	}
	for n, row := range rows {
		var b strings.Builder
		b.WriteString("  ")
		for i, c := range row {
			pad := strings.Repeat(" ", widths[i]-len([]rune(c)))
			if i == 0 || (lastLeft && i == len(row)-1) {
				b.WriteString(c + pad)
			} else {
				b.WriteString(pad + c)
			}
			if i < len(row)-1 {
				b.WriteString("  ")
			}
		}
		line := strings.TrimRight(b.String(), " ")
		if n == 0 {
			line = dim + line + reset
		}
		fmt.Fprintln(r.out, line)
	}
}
