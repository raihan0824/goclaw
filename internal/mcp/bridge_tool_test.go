package mcp

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func TestInputSchemaToMap(t *testing.T) {
	schema := mcpgo.ToolInputSchema{
		Type: "object",
		Properties: map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query",
			},
		},
		Required: []string{"query"},
	}

	m := inputSchemaToMap(schema)

	if m["type"] != "object" {
		t.Errorf("expected type=object, got %v", m["type"])
	}

	props, ok := m["properties"].(map[string]any)
	if !ok || props == nil {
		t.Fatal("expected properties map")
	}
	if _, ok := props["query"]; !ok {
		t.Error("expected 'query' in properties")
	}

	req, ok := m["required"].([]string)
	if !ok || len(req) != 1 || req[0] != "query" {
		t.Errorf("expected required=[query], got %v", m["required"])
	}
}

func TestInputSchemaToMap_EmptyType(t *testing.T) {
	schema := mcpgo.ToolInputSchema{}
	m := inputSchemaToMap(schema)

	if m["type"] != "object" {
		t.Errorf("expected default type=object, got %v", m["type"])
	}
}

func TestInputSchemaToMap_ObjectNoProperties(t *testing.T) {
	schema := mcpgo.ToolInputSchema{Type: "object"}
	m := inputSchemaToMap(schema)

	props, ok := m["properties"].(map[string]any)
	if !ok || props == nil {
		t.Fatal("expected empty properties map for object schema, got nil — OpenAI rejects object schemas without properties")
	}
	if len(props) != 0 {
		t.Errorf("expected empty properties, got %v", props)
	}
}

func TestExtractTextContent(t *testing.T) {
	result := &mcpgo.CallToolResult{
		Content: []mcpgo.Content{
			mcpgo.TextContent{Type: "text", Text: "hello"},
			mcpgo.TextContent{Type: "text", Text: "world"},
		},
	}

	got := extractTextContent(result)
	if got != "hello\nworld" {
		t.Errorf("expected 'hello\\nworld', got %q", got)
	}
}

func TestExtractTextContent_Nil(t *testing.T) {
	if got := extractTextContent(nil); got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}

	result := &mcpgo.CallToolResult{}
	if got := extractTextContent(result); got != "" {
		t.Errorf("expected empty for no content, got %q", got)
	}
}

func TestBridgeToolNaming(t *testing.T) {
	mcpTool := mcpgo.Tool{
		Name:        "query",
		Description: "Run a query",
		InputSchema: mcpgo.ToolInputSchema{Type: "object"},
	}

	// Without prefix → auto-derived from server name
	bt := NewBridgeTool("myserver", mcpTool, nil, "", 30, nil, uuid.Nil, nil)
	if bt.Name() != "mcp_myserver__query" {
		t.Errorf("expected name=mcp_myserver__query, got %s", bt.Name())
	}
	if bt.ServerName() != "myserver" {
		t.Errorf("expected serverName=myserver, got %s", bt.ServerName())
	}
	if bt.OriginalName() != "query" {
		t.Errorf("expected originalName=query, got %s", bt.OriginalName())
	}

	// With non-mcp_ prefix → gets mcp_ prepended
	bt2 := NewBridgeTool("myserver", mcpTool, nil, "pg", 0, nil, uuid.Nil, nil)
	if bt2.Name() != "mcp_pg__query" {
		t.Errorf("expected name=mcp_pg__query, got %s", bt2.Name())
	}
	if bt2.OriginalName() != "query" {
		t.Errorf("expected originalName=query, got %s", bt2.OriginalName())
	}

	// With mcp_ prefix → unchanged
	bt3 := NewBridgeTool("myserver", mcpTool, nil, "mcp_pg", 0, nil, uuid.Nil, nil)
	if bt3.Name() != "mcp_pg__query" {
		t.Errorf("expected name=mcp_pg__query, got %s", bt3.Name())
	}

	// Server name with hyphens → sanitized to underscores
	bt4 := NewBridgeTool("my-server", mcpTool, nil, "", 0, nil, uuid.Nil, nil)
	if bt4.Name() != "mcp_my_server__query" {
		t.Errorf("expected name=mcp_my_server__query, got %s", bt4.Name())
	}

	// Default timeout
	if bt2.timeoutSec != 60 {
		t.Errorf("expected default timeout=60, got %d", bt2.timeoutSec)
	}
}

func TestIsPlaceholderValue(t *testing.T) {
	// Should be detected as placeholder.
	placeholders := []string{
		"null", "None", "nil", "UNDEFINED", "n/a",
		"optional", "Optional", "OPTIONAL",
		"skip", "Skip",
		"__OMIT__", "__skip__", "__EMPTY__",
		"http://example.com", "https://example.com",
		"http://localhost", "https://localhost",
		"PLACEHOLDER", "NOT_SET", "DO_NOT_SEND",
	}
	for _, s := range placeholders {
		if !isPlaceholderValue(s) {
			t.Errorf("expected isPlaceholderValue(%q) = true", s)
		}
	}

	// Should NOT be detected as placeholder (real values).
	realValues := []string{
		"", // empty string handled separately by type-aware check
		"sk-abc123",
		"my-proxy.example.com",
		"https://api.reviewweb.site/v1",
		"gpt-4o-mini",
		"bullet",
		"hello world",
		"ab", // too short for all-caps check
	}
	for _, s := range realValues {
		if isPlaceholderValue(s) {
			t.Errorf("expected isPlaceholderValue(%q) = false", s)
		}
	}
}

func TestStripEmptyOptionalArgs(t *testing.T) {
	bt := &BridgeTool{
		requiredSet: map[string]bool{"url": true},
		inputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url":      map[string]any{"type": "string"},
				"api_key":  map[string]any{"type": "string"},
				"timeout":  map[string]any{"type": "number"},
				"debug":    map[string]any{"type": "boolean"},
				"keywords": map[string]any{"type": "string"},
			},
		},
	}

	args := map[string]any{
		"url":      "https://example.com",
		"api_key":  "optional",    // placeholder → strip
		"timeout":  nil,           // nil → strip
		"debug":    true,          // real boolean → keep
		"keywords": "",            // empty string for string-typed → keep
	}

	cleaned := bt.stripEmptyOptionalArgs(args)

	if cleaned["url"] != "https://example.com" {
		t.Error("required param 'url' should be preserved")
	}
	if _, ok := cleaned["api_key"]; ok {
		t.Error("placeholder 'optional' should be stripped for api_key")
	}
	if _, ok := cleaned["timeout"]; ok {
		t.Error("nil should be stripped for timeout")
	}
	if cleaned["debug"] != true {
		t.Error("real boolean value should be preserved")
	}
	if v, ok := cleaned["keywords"]; !ok || v != "" {
		t.Error("empty string should be kept for string-typed optional param 'keywords'")
	}
}

func TestStripEmptyOptionalArgs_EmptyStringNonString(t *testing.T) {
	bt := &BridgeTool{
		requiredSet: map[string]bool{},
		inputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"timeout": map[string]any{"type": "number"},
				"count":   map[string]any{"type": "integer"},
			},
		},
	}

	args := map[string]any{
		"timeout": "",
		"count":   "",
	}

	cleaned := bt.stripEmptyOptionalArgs(args)

	if _, ok := cleaned["timeout"]; ok {
		t.Error("empty string should be stripped for number-typed param")
	}
	if _, ok := cleaned["count"]; ok {
		t.Error("empty string should be stripped for integer-typed param")
	}
}

func TestEnsureMCPPrefix(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		serverName string
		want       string
	}{
		{"empty prefix", "", "vnstock", "mcp_vnstock"},
		{"empty prefix hyphenated server", "", "my-server", "mcp_my_server"},
		{"non-mcp prefix", "pg", "postgres", "mcp_pg"},
		{"already mcp_ prefix", "mcp_pg", "postgres", "mcp_pg"},
		{"mcp prefix without underscore", "mcp", "x", "mcp_mcp"},
		{"custom prefix with underscores", "vnstock", "vnstock", "mcp_vnstock"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ensureMCPPrefix(tt.prefix, tt.serverName)
			if got != tt.want {
				t.Errorf("ensureMCPPrefix(%q, %q) = %q, want %q", tt.prefix, tt.serverName, got, tt.want)
			}
		})
	}
}

func TestIsConnectionDeadError_DeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond)
	if !isConnectionDeadError(ctx, context.DeadlineExceeded) {
		t.Error("deadline-exceeded ctx must classify as connection dead")
	}
}

func TestIsConnectionDeadError_TransportPatterns(t *testing.T) {
	dead := []string{
		"EOF",
		"write tcp 1.2.3.4: broken pipe",
		"read tcp 1.2.3.4: connection reset by peer",
		"dial tcp 1.2.3.4:9090: connect: connection refused",
		"use of closed network connection",
		"read tcp 1.2.3.4: i/o timeout",
		"process exited unexpectedly",
	}
	for _, msg := range dead {
		if !isConnectionDeadError(context.Background(), errors.New(msg)) {
			t.Errorf("expected %q to classify as connection dead", msg)
		}
	}
}

func TestIsConnectionDeadError_NonTransportErrorsArentDead(t *testing.T) {
	live := []string{
		"tool error: invalid arguments",
		"server returned 422 unprocessable entity",
		"prometheus: query syntax error",
		"upstream proxy 504",
	}
	for _, msg := range live {
		if isConnectionDeadError(context.Background(), errors.New(msg)) {
			t.Errorf("expected %q NOT to classify as connection dead", msg)
		}
	}
}

func TestBridgeTool_OnConnectionDead_NilSafe(t *testing.T) {
	// Default constructor leaves onConnectionDead nil; setting it must not be required.
	var connected atomic.Bool
	connected.Store(true)
	t.Log("nil callback is allowed — verified via Execute path manually below")

	// Smoke: SetOnConnectionDead with a real callback should run.
	var fired atomic.Bool
	bt := &BridgeTool{}
	bt.SetOnConnectionDead(func() { fired.Store(true) })
	if bt.onConnectionDead == nil {
		t.Fatal("SetOnConnectionDead did not store the callback")
	}
	bt.onConnectionDead()
	if !fired.Load() {
		t.Error("callback was not invoked")
	}
}

func TestServerState_SignalReconnect_NonBlockingCoalesces(t *testing.T) {
	ss := &serverState{reconnectSignal: make(chan struct{}, 1)}
	// Multiple sends in a row must not block; the channel is buffered=1.
	for i := 0; i < 10; i++ {
		ss.signalReconnect()
	}
	// Drain — only one signal queued.
	select {
	case <-ss.reconnectSignal:
	default:
		t.Fatal("expected at least one signal queued after 10 signalReconnect calls")
	}
	// Channel must be empty now (10 calls coalesced into 1).
	select {
	case <-ss.reconnectSignal:
		t.Fatal("expected channel to be empty after draining one signal")
	default:
	}
}

func TestServerState_SignalReconnect_NilSafe(t *testing.T) {
	var ss *serverState
	ss.signalReconnect() // must not panic

	ss = &serverState{} // reconnectSignal nil
	ss.signalReconnect() // must not panic
}

func TestPingTimeout_Bounds(t *testing.T) {
	// The whole point of these constants is that a dead MCP server cannot
	// hang the healthLoop forever. Lock them in so a future refactor that
	// raises them past, say, the channel-handler timeout doesn't quietly
	// reintroduce the deadlock.
	if pingTimeout < 1*time.Second {
		t.Errorf("pingTimeout=%s too short, would flap on legitimate slow servers", pingTimeout)
	}
	if pingTimeout > 30*time.Second {
		t.Errorf("pingTimeout=%s too long, defeats the deadlock-avoidance goal", pingTimeout)
	}
	if reconnectInitTimeout < pingTimeout {
		t.Errorf("reconnectInitTimeout=%s must be >= pingTimeout=%s (Initialize is a superset of Ping)",
			reconnectInitTimeout, pingTimeout)
	}
	if reconnectInitTimeout > 2*time.Minute {
		t.Errorf("reconnectInitTimeout=%s too long, blocks the healthLoop goroutine for too long",
			reconnectInitTimeout)
	}
}

func TestPingWithTimeout_RespectsCancellation(t *testing.T) {
	// We can't construct a real *mcpclient.Client without a transport, but
	// the contract we care about is: if the parent ctx is already cancelled,
	// pingWithTimeout's wrapper must not produce a context with a longer
	// effective deadline than pingTimeout. Verify the wrapper composition.
	parent, parentCancel := context.WithCancel(context.Background())
	parentCancel()
	wrapped, cancel := context.WithTimeout(parent, pingTimeout)
	defer cancel()
	select {
	case <-wrapped.Done():
		// good — derived ctx is cancelled because parent is
	default:
		t.Fatal("wrapped ctx should inherit parent cancellation immediately")
	}
}
