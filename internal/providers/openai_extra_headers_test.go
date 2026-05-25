package providers

// Coverage for OpenAIProvider.WithExtraHeaders — the mechanism Kimi Coding
// uses to send a fixed User-Agent on every request.

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestOpenAIProvider_ExtraHeaders_AppliedOnHTTPRequest verifies that headers
// set via WithExtraHeaders reach the actual outgoing request — not just the
// adapter's header map.
func TestOpenAIProvider_ExtraHeaders_AppliedOnHTTPRequest(t *testing.T) {
	var gotUserAgent, gotXTrace, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		gotXTrace = r.Header.Get("X-Trace-Id")
		gotAuth = r.Header.Get("Authorization")
		// Minimal non-stream response so doRequest returns cleanly.
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","choices":[{"index":0,"message":{"role":"assistant","content":""},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	p := NewOpenAIProvider("kimi-coding-test", "sk-fake", srv.URL, "kimi-k2-turbo-preview").
		WithExtraHeaders(map[string]string{
			"User-Agent": "claude-code/0.1.0",
			"X-Trace-Id": "abc",
		})

	body, err := p.doRequest(context.Background(), map[string]any{
		"model":    "kimi-k2-turbo-preview",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	if err != nil {
		t.Fatalf("doRequest: %v", err)
	}
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()

	if gotUserAgent != "claude-code/0.1.0" {
		t.Errorf("User-Agent = %q, want %q", gotUserAgent, "claude-code/0.1.0")
	}
	if gotXTrace != "abc" {
		t.Errorf("X-Trace-Id = %q, want %q", gotXTrace, "abc")
	}
	// Standard Bearer auth must still apply alongside extra headers.
	if gotAuth != "Bearer sk-fake" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer sk-fake")
	}
}

// TestOpenAIAdapter_ExtraHeaders_MirroredInToRequest verifies the adapter path
// emits the same extra headers as the direct doRequest path — important
// because some call sites use adapter.ToRequest to produce headers separately.
func TestOpenAIAdapter_ExtraHeaders_MirroredInToRequest(t *testing.T) {
	p := NewOpenAIProvider("kimi-coding-test", "sk-fake", "https://api.kimi.com/coding/v1", "kimi-k2-turbo-preview").
		WithExtraHeaders(map[string]string{
			"User-Agent": "claude-code/0.1.0",
		})
	a := &OpenAIAdapter{provider: p}

	_, headers, err := a.ToRequest(ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("ToRequest: %v", err)
	}
	if got := headers.Get("User-Agent"); got != "claude-code/0.1.0" {
		t.Errorf("adapter User-Agent = %q, want claude-code/0.1.0", got)
	}
}

// TestOpenAIProvider_ExtraHeaders_NoOpWhenEmpty makes sure the
// WithExtraHeaders(nil) / WithExtraHeaders({}) calls leave the provider's
// state alone — protects against accidental nil-map allocations in callers
// that pass through optional config.
func TestOpenAIProvider_ExtraHeaders_NoOpWhenEmpty(t *testing.T) {
	p := NewOpenAIProvider("x", "k", "https://example.com", "m").
		WithExtraHeaders(nil).
		WithExtraHeaders(map[string]string{})

	if got := p.ExtraHeaders(); got != nil {
		t.Errorf("ExtraHeaders after empty calls = %v, want nil", got)
	}
}
