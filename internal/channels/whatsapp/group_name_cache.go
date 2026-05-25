package whatsapp

// Group-name resolution with TTL cache.
//
// whatsmeow's GetGroupInfo hits the WhatsApp servers, which is way too slow
// to do per inbound message. We keep a per-Channel map keyed by group JID
// (string form) with a configurable TTL — typical group rename frequency is
// "almost never", so a long TTL is fine. Cache misses fall back to whatsmeow,
// and any failure (no network, not authenticated, etc.) is silently cached as
// the empty string for a short refresh window so we don't hammer the server
// during a transient outage.

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/types"
)

const (
	groupNameTTL        = 24 * time.Hour      // happy-path: refresh once a day
	groupNameNegativeTTL = 5 * time.Minute    // sad-path: short retry window on failure
)

type groupNameEntry struct {
	name      string
	cachedAt  time.Time
	failed    bool // true → use shorter TTL before retrying
}

// groupNameCache is a simple TTL map. Safe for concurrent use.
type groupNameCache struct {
	mu      sync.Mutex
	entries map[string]groupNameEntry
}

func newGroupNameCache() *groupNameCache {
	return &groupNameCache{entries: make(map[string]groupNameEntry)}
}

// get returns the cached name and whether the entry is still fresh.
func (c *groupNameCache) get(jid string) (name string, fresh bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[jid]
	if !ok {
		return "", false
	}
	ttl := groupNameTTL
	if e.failed {
		ttl = groupNameNegativeTTL
	}
	if time.Since(e.cachedAt) > ttl {
		return e.name, false
	}
	return e.name, true
}

// set stores a successful lookup.
func (c *groupNameCache) set(jid, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[jid] = groupNameEntry{name: name, cachedAt: time.Now()}
}

// setFailed marks a JID as failed so we don't retry immediately.
func (c *groupNameCache) setFailed(jid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[jid] = groupNameEntry{cachedAt: time.Now(), failed: true}
}

// resolveGroupName returns the display name of a WhatsApp group, fetching
// from whatsmeow when the local cache is cold or stale. Empty string is a
// valid (non-error) return — group either has no name or we couldn't fetch
// one yet; callers should treat empty as "unknown" and fall back to the JID.
//
// Manual GroupAliases on the channel config win — admins can override the
// resolved name (or supply one when whatsmeow can't fetch it at all).
func (c *Channel) resolveGroupName(ctx context.Context, chatJID types.JID) string {
	if chatJID.Server != types.GroupServer {
		return ""
	}
	key := chatJID.String()

	// Manual override wins — simple map lookup.
	if name, ok := c.config.GroupAliases[key]; ok && name != "" {
		return name
	}

	if c.groupNames == nil {
		// Defensive: New() should always initialise this; recover if not.
		c.groupNames = newGroupNameCache()
	}
	if name, fresh := c.groupNames.get(key); fresh {
		return name
	}

	if c.client == nil || !c.client.IsConnected() {
		// Don't make network calls when offline. Keep any stale name we have,
		// otherwise mark a short negative cache so we don't retry every
		// message during an outage.
		if name, _ := c.groupNames.get(key); name != "" {
			return name
		}
		c.groupNames.setFailed(key)
		return ""
	}

	info, err := c.client.GetGroupInfo(ctx, chatJID)
	if err != nil || info == nil {
		// Surface so admins can debug why the picker dropdown shows bare JIDs.
		// Most common cause: client just connected and hasn't synced groups yet,
		// or transient network. The 5min negative TTL means at most one log
		// line per group per 5min during an outage.
		slog.Warn("whatsapp: resolve group name failed",
			"chat_jid", key,
			"error", err,
		)
		c.groupNames.setFailed(key)
		return ""
	}
	c.groupNames.set(key, info.Name)
	slog.Debug("whatsapp: resolved group name", "chat_jid", key, "name", info.Name)
	return info.Name
}
