// Package credit describes the account balance and limits reported by the provider portal.
package credit

import (
	"context"
	"errors"
	"time"
)

// ErrNoSession means the portal rejected or lacks a login session.
var ErrNoSession = errors.New("portal session missing or expired")

// Budget is one configured limit on the API key (per period or all-time).
type Budget struct {
	Period          string  `json:"period"`
	LimitTokens     int64   `json:"limit_tokens"`
	TokensUsed      int64   `json:"tokens_used"`
	TokensRemaining int64   `json:"tokens_remaining"`
	TokensPctUsed   float64 `json:"tokens_pct_used"`
	LimitUSD        float64 `json:"limit_usd"`
	SpentUSD        float64 `json:"spent_usd"`
	USDRemaining    float64 `json:"usd_remaining"`
	USDPctUsed      float64 `json:"usd_pct_used"`
	Alert           bool    `json:"alert"`
}

// Day is one day of portal usage.
type Day struct {
	Date             string  `json:"date"`
	Requests         int64   `json:"requests"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	CostUSD          float64 `json:"cost_usd"`
}

// Package is a prepaid token package, when the account has one.
type Package struct {
	Exists      bool      `json:"exists"`
	PlanName    string    `json:"plan_name"`
	LimitTokens int64     `json:"limit_tokens"`
	UsedTokens  int64     `json:"used_tokens"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func (p Package) Remaining() int64 { return max(0, p.LimitTokens-p.UsedTokens) }

// Report is what /usage-credit shows.
type Report struct {
	Days          int
	Daily         []Day
	Budgets       []Budget
	MonthRequests int64
	MonthCostUSD  float64
	Package       *Package // nil when unavailable
}

// Reader fetches a Report for the last n days.
type Reader interface {
	Credit(ctx context.Context, days int) (Report, error)
}
