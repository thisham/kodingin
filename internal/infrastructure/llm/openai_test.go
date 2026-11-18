package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

func TestCompleteClosesResponseBodies(t *testing.T) {
	for _, tt := range []struct {
		name         string
		status       int
		body         string
		cancelAt     int
		wantAttempts int
		wantErr      bool
	}{
		{"success", 200, `{"choices":[{"message":{"content":"ok"}}]}`, 0, 1, false},
		{"invalid response", 200, `invalid`, 0, 1, true},
		{"cancel retry wait", 500, `{"error":{"type":"server","message":"retry"}}`, 1, 1, true},
		{"final failed response", 500, `{"error":{"type":"server","message":"retry"}}`, 0, 3, true},
		{"cancel final response", 429, `{"error":{"type":"rate_limit","message":"retry"}}`, 3, 3, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var bodies []*trackedBody
			c := New("http://test.invalid", "", "test")
			c.http = &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
				body := &trackedBody{Reader: strings.NewReader(tt.body)}
				bodies = append(bodies, body)
				if len(bodies) == tt.cancelAt {
					cancel()
				}
				return &http.Response{StatusCode: tt.status, Header: make(http.Header), Body: body, Request: r}, nil
			})}
			msg, _, err := c.Complete(ctx, nil, nil)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v", err)
			}
			if tt.cancelAt > 0 && !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want cancellation", err)
			}
			if !tt.wantErr && msg.Content != "ok" {
				t.Fatalf("message = %+v", msg)
			}
			if len(bodies) != tt.wantAttempts {
				t.Fatalf("attempts = %d", len(bodies))
			}
			for i, b := range bodies {
				if !b.closed {
					t.Errorf("attempt %d response body not closed", i+1)
				}
			}
		})
	}
}
