// Package portal reads account usage and credit from the tokenizer.id web portal.
// The portal only accepts a browser login session, so requests carry the user's Cookie header.
package portal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/thisham/kodingin/internal/domain/credit"
)

type Client struct {
	base   string // e.g. https://tokenizer.id
	cookie string
	http   *http.Client
}

// New derives the portal origin from the API base URL (strips a trailing /v1).
func New(apiBaseURL, cookie string) *Client {
	base := strings.TrimRight(apiBaseURL, "/")
	base = strings.TrimSuffix(base, "/v1")
	return &Client{base: base, cookie: cookie, http: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) SetCookie(cookie string) { c.cookie = cookie }
func (c *Client) HasCookie() bool         { return c.cookie != "" }

type usageResp struct {
	Daily         []credit.Day    `json:"daily"`
	Budgets       []credit.Budget `json:"budgets"`
	CurrentPeriod struct {
		TotalRequests int64   `json:"total_requests"`
		CostUSD       float64 `json:"cost_usd"`
	} `json:"current_period"`
}

func (c *Client) Credit(ctx context.Context, days int) (credit.Report, error) {
	if c.cookie == "" {
		return credit.Report{}, credit.ErrNoSession
	}
	var u usageResp
	if err := c.get(ctx, fmt.Sprintf("/portal/api/usage?days=%d", days), &u); err != nil {
		return credit.Report{}, err
	}
	rep := credit.Report{Days: days, Daily: u.Daily, Budgets: u.Budgets,
		MonthRequests: u.CurrentPeriod.TotalRequests, MonthCostUSD: u.CurrentPeriod.CostUSD}
	var p credit.Package
	if err := c.get(ctx, "/portal/api/token", &p); err == nil {
		rep.Package = &p
	}
	return rep, nil
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Cookie", c.cookie)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("portal: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return credit.ErrNoSession
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error struct{ Message string } `json:"error"`
		}
		if json.Unmarshal(raw, &e) == nil && e.Error.Message != "" {
			return fmt.Errorf("portal http %d: %s", resp.StatusCode, e.Error.Message)
		}
		return fmt.Errorf("portal http %d", resp.StatusCode)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("portal: unexpected response from %s", path)
	}
	return nil
}
