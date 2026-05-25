package whatsapp

// Unit tests for the in-memory group-name TTL cache.
// We test the cache in isolation; the whatsmeow GetGroupInfo call site is
// integration-only (would need a connected WhatsApp client + real group).

import (
	"testing"
	"time"
)

func TestGroupNameCache_MissReturnsNotFresh(t *testing.T) {
	c := newGroupNameCache()
	if name, fresh := c.get("120363@g.us"); name != "" || fresh {
		t.Errorf("cold get = (%q, %v), want (empty, false)", name, fresh)
	}
}

func TestGroupNameCache_SetThenGetFresh(t *testing.T) {
	c := newGroupNameCache()
	c.set("120363@g.us", "Engineering Team")

	name, fresh := c.get("120363@g.us")
	if !fresh {
		t.Error("freshly-set entry must be fresh")
	}
	if name != "Engineering Team" {
		t.Errorf("name = %q, want %q", name, "Engineering Team")
	}
}

func TestGroupNameCache_ExpiredEntryStaleButReturnsName(t *testing.T) {
	c := newGroupNameCache()
	// Inject an expired entry directly.
	c.entries["120363@g.us"] = groupNameEntry{
		name:     "Old Name",
		cachedAt: time.Now().Add(-2 * groupNameTTL),
	}

	name, fresh := c.get("120363@g.us")
	if fresh {
		t.Error("expired entry must be stale")
	}
	if name != "Old Name" {
		t.Errorf("name = %q, want %q (stale name still returned)", name, "Old Name")
	}
}

func TestGroupNameCache_FailedEntryUsesShorterTTL(t *testing.T) {
	c := newGroupNameCache()
	c.setFailed("120363@g.us")

	// Within the negative TTL window — entry is fresh (empty but fresh).
	name, fresh := c.get("120363@g.us")
	if !fresh {
		t.Error("just-failed entry must be fresh within negative TTL")
	}
	if name != "" {
		t.Errorf("failed entry name = %q, want empty", name)
	}

	// Force expiration past negative TTL.
	c.entries["120363@g.us"] = groupNameEntry{
		cachedAt: time.Now().Add(-2 * groupNameNegativeTTL),
		failed:   true,
	}
	if _, fresh := c.get("120363@g.us"); fresh {
		t.Error("expired failed entry must NOT be fresh — caller should retry")
	}
}

// TestGroupNameCache_SetOverwritesFailed verifies that a successful lookup
// after a failure flips the entry back to the long TTL.
func TestGroupNameCache_SetOverwritesFailed(t *testing.T) {
	c := newGroupNameCache()
	c.setFailed("120363@g.us")
	c.set("120363@g.us", "Recovered Name")

	name, fresh := c.get("120363@g.us")
	if !fresh {
		t.Error("entry must be fresh after successful set")
	}
	if name != "Recovered Name" {
		t.Errorf("name = %q, want %q", name, "Recovered Name")
	}
	if got := c.entries["120363@g.us"].failed; got {
		t.Error("entry.failed must be cleared after successful set")
	}
}
