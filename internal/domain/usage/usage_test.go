package usage

import (
	"testing"
	"time"
)

func TestSummarize(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	recs := []Record{
		{Time: now.Add(-48 * time.Hour), Model: "a", Prompt: 100, Cached: 40, Completion: 10},
		{Time: now, Model: "b", Prompt: 500, Completion: 50},
		{Time: now, Model: "a", Prompt: 10, Completion: 1},
	}
	for _, tt := range []struct {
		name      string
		since     time.Time
		calls     int
		tokens    int
		firstName string
	}{
		{"all", time.Time{}, 3, 671, "b"},
		{"last day", now.Add(-24 * time.Hour), 2, 561, "b"},
		{"none", now.Add(time.Hour), 0, 0, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := Summarize(Since(recs, tt.since))
			if s.Total.Calls != tt.calls || s.Total.Tokens() != tt.tokens {
				t.Fatalf("got calls=%d tokens=%d", s.Total.Calls, s.Total.Tokens())
			}
			if tt.firstName != "" && s.ByModel[0].Model != tt.firstName {
				t.Fatalf("first model = %s", s.ByModel[0].Model)
			}
		})
	}
	if s := Summarize(recs); s.Total.Cached != 40 {
		t.Fatalf("cached = %d", s.Total.Cached)
	}
}
