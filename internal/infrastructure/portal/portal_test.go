package portal

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thisham/kodingin/internal/domain/credit"
)

func TestCredit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "sid=ok" {
			w.WriteHeader(401)
			w.Write([]byte(`{"error":{"message":"portal session required"}}`))
			return
		}
		switch r.URL.Path {
		case "/portal/api/usage":
			if r.URL.Query().Get("days") != "30" {
				t.Errorf("days = %s", r.URL.Query().Get("days"))
			}
			w.Write([]byte(`{"daily":[{"date":"2026-11-18","requests":3,"prompt_tokens":100,"completion_tokens":10,"cost_usd":0.5}],
				"budgets":[{"period":"monthly","limit_usd":10,"spent_usd":2.5,"usd_remaining":7.5,"usd_pct_used":25}],
				"current_period":{"total_requests":42,"cost_usd":2.5}}`))
		case "/portal/api/token":
			w.Write([]byte(`{"exists":true,"plan_name":"Starter","limit_tokens":1000,"used_tokens":250,"expires_at":"2026-12-31T00:00:00Z"}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	for _, tt := range []struct {
		name, cookie string
		wantErr      error
	}{
		{"ok", "sid=ok", nil},
		{"expired", "sid=old", credit.ErrNoSession},
		{"missing", "", credit.ErrNoSession},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rep, err := New(srv.URL+"/v1", tt.cookie).Credit(context.Background(), 30)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if rep.MonthRequests != 42 || len(rep.Budgets) != 1 || rep.Budgets[0].USDRemaining != 7.5 {
				t.Fatalf("report = %+v", rep)
			}
			if rep.Package == nil || rep.Package.Remaining() != 750 {
				t.Fatalf("package = %+v", rep.Package)
			}
		})
	}
}
