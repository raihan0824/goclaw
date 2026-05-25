package whatsapp

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
