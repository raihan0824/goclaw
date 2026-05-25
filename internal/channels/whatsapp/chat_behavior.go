package whatsapp

import (
	"context"
	"strings"
)

// Per-chat group-behavior resolution.
//
// Three optional list overrides on Channel.config refine the channel-wide
// RequireMention setting per chat JID:
//
//   SilentChats          → never reply, even when @mentioned. History is still
//                          recorded so the agent has context if asked elsewhere.
//   MentionRequiredChats → behave as if RequireMention=true for this chat.
//   AutoRespondChats     → behave as if RequireMention=false for this chat.
//
// Resolution order at inbound time:
//   1. isSilentChat → drop with history recording, return early
//   2. requireMentionFor → mention-required vs auto-respond per-chat
//      overrides win over the channel-wide default

// buildGroupRoster returns a plain-text JID→name dictionary suitable for
// injection into the agent's system prompt. Sources, in precedence order:
//
//  1. Admin-configured GroupAliases (manual overrides — highest priority)
//  2. Auto-fetched names from the contacts table (display_name on
//     contact_type='group' rows)
//  3. The current chat's just-resolved name (covers groups not yet in
//     either of the above)
//
// Returns empty string when nothing is known.
func (c *Channel) buildGroupRoster(ctx context.Context, currentName, currentJID string) string {
	merged := make(map[string]string, len(c.config.GroupAliases)+8)

	// Layer 2: contacts table (auto-fetched names). Lower priority — manual
	// aliases override these. Failure to fetch is non-fatal: just skip.
	if cc := c.ContactCollector(); cc != nil {
		if contacts, err := cc.ListGroupContacts(ctx, c.Type(), c.Name()); err == nil {
			for _, ct := range contacts {
				if ct.DisplayName == nil || *ct.DisplayName == "" {
					continue
				}
				merged[ct.SenderID] = *ct.DisplayName
			}
		}
	}

	// Layer 1: admin GroupAliases (overrides contacts). Highest priority.
	for jid, name := range c.config.GroupAliases {
		if jid == "" || name == "" {
			continue
		}
		merged[jid] = name
	}

	// Layer 3: current chat's resolved name (only if not already known).
	if currentJID != "" && currentName != "" {
		if _, has := merged[currentJID]; !has {
			merged[currentJID] = currentName
		}
	}

	if len(merged) == 0 {
		return ""
	}
	jids := make([]string, 0, len(merged))
	for jid := range merged {
		jids = append(jids, jid)
	}
	sortStringsAscii(jids)

	var b strings.Builder
	b.WriteString("Known WhatsApp groups (use these names instead of raw JIDs when the user asks or when summarising cross-group recall):\n")
	for _, jid := range jids {
		b.WriteString("  ")
		b.WriteString(jid)
		b.WriteString(" = ")
		b.WriteString(merged[jid])
		b.WriteByte('\n')
	}
	return b.String()
}

// sortStringsAscii sorts a string slice in place (lexicographic).
// Kept as a thin helper so this package doesn't have to import sort for one call.
func sortStringsAscii(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j-1] > ss[j]; j-- {
			ss[j-1], ss[j] = ss[j], ss[j-1]
		}
	}
}

// isSilentChat reports whether chatID is on the silent-chats override list.
func (c *Channel) isSilentChat(chatID string) bool {
	for _, id := range c.config.SilentChats {
		if id == chatID {
			return true
		}
	}
	return false
}

// requireMentionFor returns the effective RequireMention value for a chat.
// Per-chat overrides win; otherwise the channel-wide RequireMention default
// (nil = false) is returned.
func (c *Channel) requireMentionFor(chatID string) bool {
	for _, id := range c.config.MentionRequiredChats {
		if id == chatID {
			return true
		}
	}
	for _, id := range c.config.AutoRespondChats {
		if id == chatID {
			return false
		}
	}
	return c.config.RequireMention != nil && *c.config.RequireMention
}
