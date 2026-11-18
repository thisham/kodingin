package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/thisham/kodingin/internal/domain/credit"
	"github.com/thisham/kodingin/internal/domain/llm"
)

type fakeCredit struct {
	rep    credit.Report
	err    error
	cookie string
}

func (f *fakeCredit) Credit(context.Context, int) (credit.Report, error) { return f.rep, f.err }
func (f *fakeCredit) SetCookie(c string)                                 { f.cookie = c }
func (f *fakeCredit) HasCookie() bool                                    { return f.cookie != "" }

func TestUsageCreditCommand(t *testing.T) {
	report := credit.Report{
		Days:          30,
		Daily:         []credit.Day{{Date: "2026-11-18", Requests: 3, PromptTokens: 1200, CompletionTokens: 80, CostUSD: 0.25}},
		Budgets:       []credit.Budget{{Period: "monthly", LimitUSD: 10, SpentUSD: 9.5, USDRemaining: 0.5, USDPctUsed: 95, Alert: true}},
		MonthRequests: 42, MonthCostUSD: 9.5,
		Package: &credit.Package{Exists: true, PlanName: "Starter", LimitTokens: 1_000_000, UsedTokens: 250_000, ExpiresAt: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range []struct {
		name, arg, cookie, input string
		err                      error
		want, notWant            []string
		savedCookie              string
	}{
		{name: "report", cookie: "sid=1", want: []string{"$9.50 · 42 requests", "Monthly limit", "EXCEEDED", "$0.50 left", "Starter", "750.0K left"}},
		{name: "no budgets", cookie: "sid=1", want: []string{"Unrestricted"}},
		{name: "expired", cookie: "sid=1", err: credit.ErrNoSession, want: []string{"expired"}, notWant: []string{"Account credit"}},
		{name: "login prompt", input: "Cookie: sid=abc\n", want: []string{"Account credit"}, savedCookie: "sid=abc"},
		{name: "rejected cookie", input: "sid=old\n", err: credit.ErrNoSession, want: []string{"rejected"}, notWant: []string{"Account credit"}},
		{name: "bad cookie", input: "garbage\n", want: []string{"nothing saved"}, notWant: []string{"Account credit"}},
		{name: "inline rejected", cookie: "sid=1", arg: "sid=leak", want: []string{"hidden prompt"}, notWant: []string{"Account credit"}},
		{name: "logout", cookie: "sid=1", arg: "logout", want: []string{"removed"}, savedCookie: "-"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			fc := &fakeCredit{rep: report, err: tt.err, cookie: tt.cookie}
			if tt.name == "no budgets" {
				fc.rep.Budgets = nil
			}
			saved := map[string]string{}
			r := &REPL{out: &out, credit: fc,
				prompt: func(string, bool) (string, error) { return tt.input, nil },
				saveSettings: func(m map[string]string) (string, error) {
					for k, v := range m {
						saved[k] = v
					}
					return "/tmp/env", nil
				}}
			r.usageCreditCmd(context.Background(), tt.arg)
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
			switch tt.savedCookie {
			case "":
				if _, ok := saved[portalCookieKey]; ok && tt.name != "login prompt" {
					t.Errorf("cookie saved unexpectedly")
				}
			case "-":
				if v, ok := saved[portalCookieKey]; !ok || v != "" || fc.HasCookie() {
					t.Errorf("logout did not clear cookie")
				}
			default:
				if saved[portalCookieKey] != tt.savedCookie || fc.cookie != tt.savedCookie {
					t.Errorf("saved %q, client %q", saved[portalCookieKey], fc.cookie)
				}
			}
		})
	}
}

func TestUsageCommandSession(t *testing.T) {
	var out bytes.Buffer
	r := &REPL{out: &out, model: "fallback"}
	r.OnUsage(llm.Usage{Model: "m1", PromptTokens: 12000, CachedTokens: 9000, CompletionTokens: 40})
	r.OnUsage(llm.Usage{PromptTokens: 10, CompletionTokens: 1})
	r.usageCmd(context.Background())
	got := out.String()
	for _, w := range []string{"m1", "12.0K", "9000", "fallback", "total", "History"} {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
}
