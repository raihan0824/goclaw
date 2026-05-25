package store

import (
	"context"
	"log/slog"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/cache"
)

const contactSeenTTL = 30 * time.Minute

// ContactCollector wraps ContactStore with an in-memory "seen" cache
// to avoid redundant UPSERT queries on every message.
type ContactCollector struct {
	store ContactStore
	seen  cache.Cache[bool]
}

// NewContactCollector creates a new collector backed by the given store and cache.
func NewContactCollector(s ContactStore, c cache.Cache[bool]) *ContactCollector {
	return &ContactCollector{store: s, seen: c}
}

// EnsureContact creates or refreshes a contact entry, skipping DB if recently seen.
// contactType: "user" (individual sender), "group" (group chat entity), or "topic" (forum topic).
// Pass empty threadID/threadType for base contacts (DM, group root).
func (c *ContactCollector) EnsureContact(ctx context.Context, channelType, channelInstance, senderID, userID, displayName, username, peerKind, contactType, threadID, threadType string) {
	// Cache key must include every dimension the underlying DB unique constraint
	// uses, otherwise dedup skips legitimate upserts:
	//   - tenantID: fixes cross-tenant leak (same sender in tenant A vs B)
	//   - channelInstance: fixes collision when two bots in the same tenant share
	//     overlapping sender ID spaces (e.g. two Telegram bot tokens with users
	//     who happen to have the same Telegram user_id)
	//   - threadID: different threads/topics track separate contacts
	// Zero UUID (Desktop / single-tenant) keeps legacy dedup semantics intact.
	tid := TenantIDFromContext(ctx)
	key := tid.String() + ":" + channelType + ":" + channelInstance + ":" + senderID + ":" + threadID
	if _, ok := c.seen.Get(ctx, key); ok {
		return
	}
	if contactType == "" {
		contactType = "user"
	}
	if err := c.store.UpsertContact(ctx, channelType, channelInstance, senderID, userID, displayName, username, peerKind, contactType, threadID, threadType); err != nil {
		slog.Warn("contact_collector.upsert_failed",
			"error", err,
			"tenant_id", tid,
			"channel", channelType,
			"instance", channelInstance,
			"sender", senderID,
		)
		return
	}
	c.seen.Set(ctx, key, true, contactSeenTTL)
}

// ResolveTenantUserID delegates to the underlying ContactStore.
func (c *ContactCollector) ResolveTenantUserID(ctx context.Context, channelType, senderID string) (string, error) {
	return c.store.ResolveTenantUserID(ctx, channelType, senderID)
}

// UpsertContactForce writes the contact unconditionally, bypassing the seen
// cache. Used when the caller knows new information (e.g. a freshly-resolved
// WhatsApp group name) and needs to refresh an existing row that was upserted
// earlier with an empty display_name. After writing, the seen-cache entry is
// refreshed so subsequent calls within the TTL window still dedupe.
func (c *ContactCollector) UpsertContactForce(ctx context.Context, channelType, channelInstance, senderID, userID, displayName, username, peerKind, contactType, threadID, threadType string) {
	if contactType == "" {
		contactType = "user"
	}
	if err := c.store.UpsertContact(ctx, channelType, channelInstance, senderID, userID, displayName, username, peerKind, contactType, threadID, threadType); err != nil {
		slog.Warn("contact_collector.upsert_force_failed",
			"error", err,
			"tenant_id", TenantIDFromContext(ctx),
			"channel", channelType,
			"instance", channelInstance,
			"sender", senderID,
		)
		return
	}
	tid := TenantIDFromContext(ctx)
	key := tid.String() + ":" + channelType + ":" + channelInstance + ":" + senderID + ":" + threadID
	c.seen.Set(ctx, key, true, contactSeenTTL)
}
