package agent

import (
	"context"
	"log/slog"

	"github.com/nextlevelbuilder/goclaw/internal/eventbus"
	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// runObserve handles the Observe=true path: persist the user message into the
// session, fire session.completed so the consolidation pipeline (episodic →
// semantic → dreaming) picks it up, and exit without calling the LLM or
// dispatching any outbound reply.
//
// This powers per-chat silent overrides (e.g. WhatsApp silent_chats) where
// the agent is present in a group purely to absorb context. Because episodic
// memory is keyed by (agent_id, user_id) — not chat_id — anything captured
// here is recallable later in any chat with the same user.
func (l *Loop) runObserve(ctx context.Context, req RunRequest, emitRun func(AgentEvent)) (*RunResult, error) {
	// Inject tenant scope so session writes land in the right tenant.
	if l.tenantID != [16]byte{} {
		ctx = store.WithTenantID(ctx, l.tenantID)
	}

	// Persist the user message. Skip on empty input (e.g. media-only with no
	// caption) to avoid a useless empty turn — sessions are messy enough.
	if req.Message != "" && l.sessions != nil {
		l.sessions.AddMessage(ctx, req.SessionKey, providers.Message{
			Role:    "user",
			Content: req.Message,
		})
	}

	// Fire session.completed so the episodic worker creates a summary for
	// this turn. MessageCount is best-effort — we don't run the full pipeline
	// so we don't have a precise running tally; the worker reads from the
	// session store anyway.
	if l.domainBus != nil {
		l.domainBus.Publish(eventbus.DomainEvent{
			Type:     eventbus.EventSessionCompleted,
			TenantID: l.tenantID.String(),
			AgentID:  l.agentUUID.String(),
			UserID:   req.UserID,
			SourceID: req.SessionKey,
			Payload: &eventbus.SessionCompletedPayload{
				SessionKey:   req.SessionKey,
				MessageCount: 1,
			},
		})
	}

	slog.Debug("agent loop: observe-only run complete",
		"agent", l.id, "session", req.SessionKey, "user", req.UserID, "chat", req.ChatID)

	// Emit RunCompleted so any UI subscribers see a clean lifecycle.
	emitRun(AgentEvent{
		Type:    protocol.AgentEventRunCompleted,
		AgentID: l.id,
		RunID:   req.RunID,
		Payload: map[string]any{"observe": true},
	})

	return &RunResult{
		RunID:      req.RunID,
		Iterations: 0,
	}, nil
}
