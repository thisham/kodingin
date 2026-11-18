// Package usage records token consumption per model call and summarises it.
package usage

import (
	"context"
	"sort"
	"time"
)

// Record is one model call. Cached is the part of Prompt served from the provider cache.
type Record struct {
	Time       time.Time `json:"time"`
	Model      string    `json:"model"`
	Prompt     int       `json:"prompt_tokens"`
	Cached     int       `json:"cached_tokens,omitempty"`
	Completion int       `json:"completion_tokens"`
}

// Totals is an additive tally.
type Totals struct {
	Calls, Prompt, Cached, Completion int
}

func (t *Totals) Add(r Record) {
	t.Calls++
	t.Prompt += r.Prompt
	t.Cached += r.Cached
	t.Completion += r.Completion
}

func (t Totals) Tokens() int { return t.Prompt + t.Completion }

// ModelTotals is Totals for one model.
type ModelTotals struct {
	Model string
	Totals
}

// Summary groups records by model, largest token count first.
type Summary struct {
	Total   Totals
	ByModel []ModelTotals
}

func Summarize(records []Record) Summary {
	by := map[string]*Totals{}
	var s Summary
	for _, r := range records {
		s.Total.Add(r)
		t, ok := by[r.Model]
		if !ok {
			t = &Totals{}
			by[r.Model] = t
		}
		t.Add(r)
	}
	for m, t := range by {
		s.ByModel = append(s.ByModel, ModelTotals{Model: m, Totals: *t})
	}
	sort.Slice(s.ByModel, func(i, j int) bool {
		if s.ByModel[i].Tokens() != s.ByModel[j].Tokens() {
			return s.ByModel[i].Tokens() > s.ByModel[j].Tokens()
		}
		return s.ByModel[i].Model < s.ByModel[j].Model
	})
	return s
}

// Since keeps records at or after t.
func Since(records []Record, t time.Time) []Record {
	var out []Record
	for _, r := range records {
		if !r.Time.Before(t) {
			out = append(out, r)
		}
	}
	return out
}

// Repository persists records across sessions.
type Repository interface {
	Append(ctx context.Context, r Record) error
	Since(ctx context.Context, t time.Time) ([]Record, error)
}
