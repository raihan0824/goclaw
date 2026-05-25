package agent

// Tests for the Observe=true short-circuit in Loop.Run.
// Covers: user message persisted, session.completed emitted, no LLM call
// (the observe path simply doesn't reach the provider, so we assert by
// confirming the loop returns an empty result and the stubs recorded the
// expected side effects).

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/eventbus"
	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// stubObserveSessionStore records AddMessage calls. Embeds the full
// SessionStore so any other method called would panic — runObserve must
// only touch AddMessage.
type stubObserveSessionStore struct {
	store.SessionStore
	mu       sync.Mutex
	appended []providers.Message
}

func (s *stubObserveSessionStore) AddMessage(_ context.Context, _ string, msg providers.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appended = append(s.appended, msg)
}

// stubObserveDomainBus records published events. Other methods are unused by
// runObserve so we satisfy the interface with no-ops.
type stubObserveDomainBus struct {
	mu        sync.Mutex
	published []eventbus.DomainEvent
}

func (b *stubObserveDomainBus) Publish(event eventbus.DomainEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.published = append(b.published, event)
}

func (b *stubObserveDomainBus) Subscribe(_ eventbus.EventType, _ eventbus.DomainEventHandler) func() {
	return func() {}
}

func (b *stubObserveDomainBus) Start(_ context.Context) {}

func (b *stubObserveDomainBus) Drain(_ time.Duration) error { return nil }

// emitNoop discards AgentEvents; tests don't care about the event stream.
func emitNoop(AgentEvent) {}

func TestRunObserve_PersistsUserMessage(t *testing.T) {
	ss := &stubObserveSessionStore{}
	db := &stubObserveDomainBus{}
	l := &Loop{
		id:        "test-agent",
		agentUUID: uuid.New(),
		sessions:  ss,
		domainBus: db,
	}

	req := RunRequest{
		SessionKey: "agent:test:whatsapp:group:120363@g.us",
		Message:    "members chatting in silent room",
		UserID:     "u-1",
		ChatID:     "120363@g.us",
		RunID:      "run-1",
		Observe:    true,
	}

	res, err := l.runObserve(context.Background(), req, emitNoop)
	if err != nil {
		t.Fatalf("runObserve: %v", err)
	}
	if res == nil {
		t.Fatal("runObserve must return non-nil RunResult so callers see a clean lifecycle")
	}
	if res.Content != "" {
		t.Errorf("observe result must have empty Content (no LLM reply), got %q", res.Content)
	}

	ss.mu.Lock()
	defer ss.mu.Unlock()
	if len(ss.appended) != 1 {
		t.Fatalf("expected 1 message appended, got %d", len(ss.appended))
	}
	got := ss.appended[0]
	if got.Role != "user" || got.Content != req.Message {
		t.Errorf("appended message = %+v, want role=user content=%q", got, req.Message)
	}
}

func TestRunObserve_EmitsSessionCompleted(t *testing.T) {
	ss := &stubObserveSessionStore{}
	db := &stubObserveDomainBus{}
	l := &Loop{
		id:        "test-agent",
		agentUUID: uuid.New(),
		sessions:  ss,
		domainBus: db,
	}

	req := RunRequest{
		SessionKey: "agent:test:whatsapp:group:G@g.us",
		Message:    "absorbed message",
		UserID:     "u-1",
		Observe:    true,
	}

	if _, err := l.runObserve(context.Background(), req, emitNoop); err != nil {
		t.Fatalf("runObserve: %v", err)
	}

	db.mu.Lock()
	defer db.mu.Unlock()
	if len(db.published) != 1 {
		t.Fatalf("expected 1 domain event published, got %d", len(db.published))
	}
	ev := db.published[0]
	if ev.Type != eventbus.EventSessionCompleted {
		t.Errorf("event type = %q, want %q", ev.Type, eventbus.EventSessionCompleted)
	}
	if ev.SourceID != req.SessionKey {
		t.Errorf("event SourceID = %q, want %q", ev.SourceID, req.SessionKey)
	}
	if ev.UserID != req.UserID {
		t.Errorf("event UserID = %q, want %q", ev.UserID, req.UserID)
	}
	if _, ok := ev.Payload.(*eventbus.SessionCompletedPayload); !ok {
		t.Errorf("event payload type = %T, want *SessionCompletedPayload", ev.Payload)
	}
}

// TestRunObserve_EmptyMessageSkipsAppend covers media-only / empty-text
// inbound — we don't pollute the session with an empty user turn.
func TestRunObserve_EmptyMessageSkipsAppend(t *testing.T) {
	ss := &stubObserveSessionStore{}
	db := &stubObserveDomainBus{}
	l := &Loop{
		id:        "test-agent",
		agentUUID: uuid.New(),
		sessions:  ss,
		domainBus: db,
	}

	req := RunRequest{
		SessionKey: "agent:test:whatsapp:group:G@g.us",
		Message:    "",
		UserID:     "u-1",
		Observe:    true,
	}

	if _, err := l.runObserve(context.Background(), req, emitNoop); err != nil {
		t.Fatalf("runObserve: %v", err)
	}

	ss.mu.Lock()
	defer ss.mu.Unlock()
	if len(ss.appended) != 0 {
		t.Errorf("empty Message must not append; got %d entries", len(ss.appended))
	}
}
