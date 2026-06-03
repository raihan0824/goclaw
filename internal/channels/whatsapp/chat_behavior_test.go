package whatsapp

// Resolution tests for per-chat behavior overrides.
// Covers: silent-chats short-circuit, mention-required override, auto-respond
// override, channel-wide default fallback, and override priority ordering.

import (
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/channels"
	"github.com/nextlevelbuilder/goclaw/internal/config"
)

// boolPtr returns a *bool — saves callers from `b := true; &b` boilerplate.
func boolPtr(b bool) *bool { return &b }

// newChannel constructs a minimally-populated Channel for resolver tests.
// Most Channel fields stay zero-valued; only c.config matters here.
func newChannel(cfg config.WhatsAppConfig) *Channel {
	return &Channel{config: cfg}
}

func TestIsSilentChat(t *testing.T) {
	ch := newChannel(config.WhatsAppConfig{
		SilentChats: []string{"silent-A@g.us", "silent-B@g.us"},
	})
	cases := map[string]bool{
		"silent-A@g.us": true,
		"silent-B@g.us": true,
		"loud-Z@g.us":   false,
		"":              false,
	}
	for chat, want := range cases {
		if got := ch.isSilentChat(chat); got != want {
			t.Errorf("isSilentChat(%q) = %v, want %v", chat, got, want)
		}
	}
}

func TestRequireMentionFor_ChannelWideDefault(t *testing.T) {
	t.Run("RequireMention nil → false", func(t *testing.T) {
		ch := newChannel(config.WhatsAppConfig{})
		if got := ch.requireMentionFor("any@g.us"); got {
			t.Errorf("requireMentionFor with nil default = true, want false")
		}
	})
	t.Run("RequireMention true → true", func(t *testing.T) {
		ch := newChannel(config.WhatsAppConfig{RequireMention: boolPtr(true)})
		if got := ch.requireMentionFor("any@g.us"); !got {
			t.Errorf("requireMentionFor with true default = false, want true")
		}
	})
	t.Run("RequireMention false → false", func(t *testing.T) {
		ch := newChannel(config.WhatsAppConfig{RequireMention: boolPtr(false)})
		if got := ch.requireMentionFor("any@g.us"); got {
			t.Errorf("requireMentionFor with false default = true, want false")
		}
	})
}

func TestRequireMentionFor_MentionRequiredOverride(t *testing.T) {
	ch := newChannel(config.WhatsAppConfig{
		RequireMention:       boolPtr(false), // default = always respond
		MentionRequiredChats: []string{"strict@g.us"},
	})
	if !ch.requireMentionFor("strict@g.us") {
		t.Error("listed chat should require mention even when channel default is false")
	}
	if ch.requireMentionFor("other@g.us") {
		t.Error("unlisted chat should follow channel default (false)")
	}
}

func TestRequireMentionFor_AutoRespondOverride(t *testing.T) {
	ch := newChannel(config.WhatsAppConfig{
		RequireMention:   boolPtr(true), // default = require mention
		AutoRespondChats: []string{"casual@g.us"},
	})
	if ch.requireMentionFor("casual@g.us") {
		t.Error("auto-respond chat should NOT require mention even when channel default is true")
	}
	if !ch.requireMentionFor("other@g.us") {
		t.Error("unlisted chat should follow channel default (true)")
	}
}

// TestRequireMentionFor_PriorityMentionRequiredWins documents resolution order
// when the same chat appears in both lists — MentionRequiredChats wins because
// it's checked first. This is a deterministic tie-break, not a recommended
// configuration; the UI should prevent putting a chat in both.
func TestRequireMentionFor_PriorityMentionRequiredWins(t *testing.T) {
	ch := newChannel(config.WhatsAppConfig{
		RequireMention:       boolPtr(false),
		MentionRequiredChats: []string{"both@g.us"},
		AutoRespondChats:     []string{"both@g.us"},
	})
	if !ch.requireMentionFor("both@g.us") {
		t.Error("MentionRequiredChats should win when a chat is in both lists")
	}
}

// --- agentForChat ---

// newChannelWithAgent builds a Channel for agent-resolution tests. The
// channel-bound default agent_id is set on the embedded BaseChannel via
// SetAgentID so AgentID() returns it (matching production wiring).
func newChannelWithAgent(defaultAgent string, cfg config.WhatsAppConfig) *Channel {
	ch := &Channel{
		BaseChannel: channels.NewBaseChannel("test-wa", nil, nil),
		config:      cfg,
	}
	ch.SetAgentID(defaultAgent)
	return ch
}

func TestAgentForChat_NoOverride_ReturnsDefault(t *testing.T) {
	ch := newChannelWithAgent("default-agent", config.WhatsAppConfig{})
	if got := ch.agentForChat("any@g.us"); got != "default-agent" {
		t.Errorf("no override: got %q, want default-agent", got)
	}
}

func TestAgentForChat_OverrideHit_ReturnsCustomAgent(t *testing.T) {
	ch := newChannelWithAgent("default-agent", config.WhatsAppConfig{
		GroupAgentOverrides: map[string]string{
			"ops@g.us":   "ops-agent",
			"sales@g.us": "sales-agent",
		},
	})
	cases := map[string]string{
		"ops@g.us":     "ops-agent",
		"sales@g.us":   "sales-agent",
		"other@g.us":   "default-agent", // miss → fall back
		"5551234@s.whatsapp.net": "default-agent", // DM → fall back
	}
	for chat, want := range cases {
		if got := ch.agentForChat(chat); got != want {
			t.Errorf("agentForChat(%q) = %q, want %q", chat, got, want)
		}
	}
}

func TestAgentForChat_EmptyOverride_IsIgnored(t *testing.T) {
	// Empty string value in the map should be treated as "no override",
	// not as "route to no agent". Guards against UI bugs that send blank
	// values when the user clears a row but doesn't delete it.
	ch := newChannelWithAgent("default-agent", config.WhatsAppConfig{
		GroupAgentOverrides: map[string]string{
			"empty@g.us": "",
		},
	})
	if got := ch.agentForChat("empty@g.us"); got != "default-agent" {
		t.Errorf("empty override: got %q, want default-agent fallback", got)
	}
}

func TestAgentForChat_NilMap_IsSafe(t *testing.T) {
	// Nil map (typical zero-value config) must not panic.
	ch := newChannelWithAgent("default-agent", config.WhatsAppConfig{
		GroupAgentOverrides: nil,
	})
	if got := ch.agentForChat("any@g.us"); got != "default-agent" {
		t.Errorf("nil map: got %q, want default-agent", got)
	}
}
