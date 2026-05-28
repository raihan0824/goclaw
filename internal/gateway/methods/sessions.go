package methods

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	httpapi "github.com/nextlevelbuilder/goclaw/internal/http"
	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// SessionsMethods handles sessions.list, sessions.preview, sessions.patch, sessions.delete, sessions.reset.
// SessionCompactor performs an LLM-summarize-then-truncate compaction for a
// single session. Wired from the agent.Router so the sessions.compact RPC
// can do the proper Claude-Code-style compaction instead of a plain truncate.
// Result indicates whether summarization actually happened (false = truncate-
// only fallback path) so the UI can adjust its toast.
type SessionCompactor func(ctx context.Context, sessionKey string) (original, kept int, summarized bool, err error)

type SessionsMethods struct {
	sessions  store.SessionStore
	eventBus  bus.EventPublisher
	cfg       *config.Config
	compactor SessionCompactor // optional — when nil, handleCompact falls back to plain truncate
}

func NewSessionsMethods(sess store.SessionStore, eventBus bus.EventPublisher, cfg *config.Config) *SessionsMethods {
	return &SessionsMethods{sessions: sess, eventBus: eventBus, cfg: cfg}
}

// SetCompactor wires the LLM compactor. Optional; must be called before
// Register if used. nil = keep the legacy truncate-only behavior.
func (m *SessionsMethods) SetCompactor(c SessionCompactor) { m.compactor = c }

func (m *SessionsMethods) Register(router *gateway.MethodRouter) {
	router.Register(protocol.MethodSessionsList, m.handleList)
	router.Register(protocol.MethodSessionsPreview, m.handlePreview)
	router.Register(protocol.MethodSessionsPatch, m.handlePatch)
	router.Register(protocol.MethodSessionsDelete, m.handleDelete)
	router.Register(protocol.MethodSessionsReset, m.handleReset)
	router.Register(protocol.MethodSessionsCompact, m.handleCompact)
}

type sessionsListParams struct {
	AgentID string `json:"agentId"`
	Channel string `json:"channel"` // optional: filter by channel prefix ("ws", "telegram")
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}

func (m *SessionsMethods) handleList(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params sessionsListParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	if params.Limit <= 0 {
		params.Limit = 20
	}

	opts := store.SessionListOpts{
		AgentID:  params.AgentID,
		Channel:  params.Channel,
		Limit:    params.Limit,
		Offset:   params.Offset,
		TenantID: store.TenantIDFromContext(ctx),
	}
	// Role-based filtering: admins/owners see all sessions; regular users see only their own.
	// Tenant scope is always applied above — admin sees all sessions within the tenant.
	if !canSeeAll(client.Role(), m.cfg.Gateway.OwnerIDs, client.UserID()) {
		opts.UserID = client.UserID()
	}

	result := m.sessions.ListPagedRich(ctx, opts)
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"sessions": result.Sessions,
		"total":    result.Total,
		"limit":    params.Limit,
		"offset":   params.Offset,
	}))
}

type sessionKeyParams struct {
	Key string `json:"key"`
}

func (m *SessionsMethods) handlePreview(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params sessionKeyParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgInvalidJSON)))
		return
	}

	if !canSeeAll(client.Role(), m.cfg.Gateway.OwnerIDs, client.UserID()) {
		sess := m.sessions.Get(ctx, params.Key)
		if sess == nil {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, i18n.T(locale, i18n.MsgNotFound, "session", params.Key)))
			return
		}
		if sess.UserID != client.UserID() {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, i18n.T(locale, i18n.MsgPermissionDenied, "session")))
			return
		}
	}

	history := m.sessions.GetHistory(ctx, params.Key)
	summary := m.sessions.GetSummary(ctx, params.Key)

	// Sign file URLs before delivery — sessions store clean paths.
	secret := httpapi.FileSigningKey()
	for i := range history {
		history[i].Content = httpapi.SignFileURLs(history[i].Content, secret)
		for j := range history[i].MediaRefs {
			history[i].MediaRefs[j].Path = httpapi.SignMediaPath(history[i].MediaRefs[j].Path, secret)
		}
	}
	summary = httpapi.SignFileURLs(summary, secret)

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"key":      params.Key,
		"messages": history,
		"summary":  summary,
	}))
}

// handlePatch updates session metadata fields.
// Matching TS sessions.patch (src/gateway/server-methods/sessions.ts:237-287).
func (m *SessionsMethods) handlePatch(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params struct {
		Key      string            `json:"key"`
		Label    *string           `json:"label,omitempty"`
		Model    *string           `json:"model,omitempty"`
		Metadata map[string]string `json:"metadata,omitempty"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgInvalidJSON)))
		return
	}

	if params.Key == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgRequired, "key")))
		return
	}

	if !canSeeAll(client.Role(), m.cfg.Gateway.OwnerIDs, client.UserID()) {
		sess := m.sessions.Get(ctx, params.Key)
		if sess == nil {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, i18n.T(locale, i18n.MsgNotFound, "session", params.Key)))
			return
		}
		if sess.UserID != client.UserID() {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, i18n.T(locale, i18n.MsgPermissionDenied, "session")))
			return
		}
	}

	// Apply label patch
	if params.Label != nil {
		m.sessions.SetLabel(ctx, params.Key, *params.Label)
	}

	// Apply model patch
	if params.Model != nil {
		m.sessions.UpdateMetadata(ctx, params.Key, *params.Model, "", "")
	}

	// Apply metadata patch
	if len(params.Metadata) > 0 {
		m.sessions.SetSessionMetadata(ctx, params.Key, params.Metadata)
	}

	// Save changes to DB
	m.sessions.Save(ctx, params.Key)

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"ok":  true,
		"key": params.Key,
	}))
	emitAudit(m.eventBus, client, "session.patched", "session", params.Key)
}

func (m *SessionsMethods) handleDelete(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params sessionKeyParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgInvalidJSON)))
		return
	}

	if !canSeeAll(client.Role(), m.cfg.Gateway.OwnerIDs, client.UserID()) {
		sess := m.sessions.Get(ctx, params.Key)
		if sess == nil {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, i18n.T(locale, i18n.MsgNotFound, "session", params.Key)))
			return
		}
		if sess.UserID != client.UserID() {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, i18n.T(locale, i18n.MsgPermissionDenied, "session")))
			return
		}
	}

	if err := m.sessions.Delete(ctx, params.Key); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"ok": true,
	}))
	emitAudit(m.eventBus, client, "session.deleted", "session", params.Key)
}

func (m *SessionsMethods) handleReset(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params sessionKeyParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgInvalidJSON)))
		return
	}

	if !canSeeAll(client.Role(), m.cfg.Gateway.OwnerIDs, client.UserID()) {
		sess := m.sessions.Get(ctx, params.Key)
		if sess == nil {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, i18n.T(locale, i18n.MsgNotFound, "session", params.Key)))
			return
		}
		if sess.UserID != client.UserID() {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, i18n.T(locale, i18n.MsgPermissionDenied, "session")))
			return
		}
	}

	m.sessions.Reset(ctx, params.Key)

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"ok": true,
	}))
	emitAudit(m.eventBus, client, "session.reset", "session", params.Key)
}

type sessionCompactParams struct {
	Key          string `json:"key"`
	KeepLast     int    `json:"keepLast,omitempty"`     // default 4 — only used by the truncate-only fallback
	TruncateOnly bool   `json:"truncateOnly,omitempty"` // skip the LLM summary pass even if a compactor is wired
}

// handleCompact compacts a session's history.
//
// Default behaviour (when a compactor is wired): runs the same LLM-
// summarize-then-truncate path the auto-compactor uses (matches Claude
// Code's `/compact`). The conversation summary is persisted on the
// session, MediaRefs from the oldest messages are preserved on the first
// kept message, and the compaction counter is incremented.
//
// Fallback (compactor nil OR TruncateOnly=true OR summarizer fails):
// truncates the history to the last `keepLast` messages without any LLM
// call. The response sets summarized=false so the UI can show "truncated"
// instead of "summarized".
func (m *SessionsMethods) handleCompact(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params sessionCompactParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgInvalidJSON)))
		return
	}

	if params.Key == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "key is required"))
		return
	}

	keepLast := params.KeepLast
	if keepLast <= 0 {
		keepLast = 4 // default: keep last 2 exchanges
	}

	// Auth check
	sess := m.sessions.Get(ctx, params.Key)
	if sess == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, i18n.T(locale, i18n.MsgNotFound, "session", params.Key)))
		return
	}
	if !canSeeAll(client.Role(), m.cfg.Gateway.OwnerIDs, client.UserID()) {
		if sess.UserID != client.UserID() {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, i18n.T(locale, i18n.MsgPermissionDenied, "session")))
			return
		}
	}

	history := m.sessions.GetHistory(ctx, params.Key)
	originalLen := len(history)
	if originalLen < 6 {
		client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
			"ok":      true,
			"message": "session too short to compact",
			"kept":    originalLen,
		}))
		return
	}

	// Preferred path: LLM-summarize-then-truncate via the wired compactor.
	if m.compactor != nil && !params.TruncateOnly {
		original, kept, summarized, err := m.compactor(ctx, params.Key)
		if err == nil {
			client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
				"ok":         true,
				"original":   original,
				"kept":       kept,
				"summarized": summarized,
			}))
			emitAudit(m.eventBus, client, "session.compacted", "session", params.Key)
			return
		}
		// Summarizer failed — fall through to truncate-only so the user still
		// gets a usable session (a 500 here would leave them with full history).
		slog.Warn("sessions.compact summarizer failed, falling back to truncate-only",
			"session", params.Key, "error", err)
	}

	// Fallback: truncate without LLM call.
	m.sessions.TruncateHistory(ctx, params.Key, keepLast)
	m.sessions.IncrementCompaction(ctx, params.Key)
	m.sessions.Save(ctx, params.Key)

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"ok":         true,
		"original":   originalLen,
		"kept":       keepLast,
		"summarized": false,
	}))
	emitAudit(m.eventBus, client, "session.compacted", "session", params.Key)
}
