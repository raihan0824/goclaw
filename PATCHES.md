# goclaw patches — `v3.12.0-patched.*`

Custom additions on top of upstream `v3.12.0`, focused on running a single
WhatsApp agent (`AIRA`) against Kubernetes and Moonshot Kimi Coding.

## Patches (in order)

### Image / runtime

- **kubectl + uvx in the `:full` image.** New `ENABLE_KUBECTL` build arg
  installs pinned `kubectl` and `uv`/`uvx` static musl binaries. Workflow
  flips it on only for the `full` variant.

### Secure CLI grants

- **Per-chat scope on `secure_cli_agent_grants`.** New nullable `chat_id`
  column. Same agent can use a different `KUBECONFIG` per WhatsApp group;
  resolution prefers chat-specific grant over the NULL "agent default".
- **`__FILE_` env-key convention.** Lets admins paste multi-line file
  contents (kubeconfig YAML, service-account JSON, PEM bundles) into the
  grant env. At exec time the value is materialized to a 0600 temp file
  under a 0700 dir; `KUBECONFIG=<path>` injected; dir wiped after the
  child exits. Sandbox exec rejects file env vars.
- **Drag-and-drop file upload UI** for both grant env section and
  Add-CLI-credential dialog (when preset declares `is_file: true`).

### Kimi Coding provider

- **`kimi_coding` provider** with reusable `WithExtraHeaders` on
  `OpenAIProvider` (always sends `User-Agent: claude-code/0.1.0`).
- **Temperature lock** — kimi_coding joins the `skipTemp` branch so the
  upstream's "only temperature=1 allowed" doesn't 400.
- **`reasoning_content` always present** on assistant tool-call messages
  for kimi_coding (Kimi enables thinking server-side; replay needs the
  field even when empty).

### WhatsApp per-chat behavior

- **`silent_chats` / `mention_required_chats` / `auto_respond_chats`**
  overrides on the WhatsApp channel config. Same number, same agent —
  different reply behavior per group.
- **Silent groups now observe.** Messages persist to the session and fire
  `session.completed` (so episodic summaries still run) — agent skips
  the LLM call and outbound. Cross-chat recall works because memory is
  keyed by `(agent_id, user_id)`.

### Webhooks

- **Full Lite parity.** Removed 4 edition gates so the Lite/SQLite build
  can create `kind=message` webhooks, toggle `localhost_only` freely, and
  mount `POST /v1/webhooks/message`. Channel delivery from external HMAC-
  signed posts now works on the desktop/single-user edition.
- **`GET /v1/webhooks/{id}/calls`** — new admin endpoint exposing rows
  from the `webhook_calls` table for the new UI deliveries tab. Tenant-
  scoped 404, omits heavy `request_payload`/`response`/`lease_token`,
  paginates via a `limit+1` peek (`has_more`).
- **Webhooks management page** under `/webhooks` (admin-only, "Send"
  icon). Covers list, create with copy-once secret + HMAC key + POST URL
  + curl example, rotate, revoke, permanent delete (two-step: must be
  revoked first), edit settings, and a delivery-history tab with status
  filter and offset pagination. `kind=message` create lists existing
  channel instances via the existing channels API. Full i18n in en/vi/zh.
- **`DELETE /v1/webhooks/{id}?purge=true`** — hard-delete endpoint for
  revoked webhooks. Returns 409 if the row is still active (forces a
  two-step flow); cascades `webhook_calls` via the existing FK.
- **Named webhook URLs** — every webhook now has a dedicated URL that
  embeds its name: `POST /v1/webhooks/{name}/message` and
  `POST /v1/webhooks/{name}/llm`. The legacy unnamed variants still
  work for back-compat. The auth middleware verifies the resolved
  webhook's name matches the URL segment; mismatch → 401. The UI shows
  the new URL on the create + rotate dialog and on the settings tab.

### WhatsApp group names

- **Auto-fetch group names** via cached `whatsmeow.GetGroupInfo` (24h TTL,
  5-min negative cache). Names flow into:
  - `metadata[chat_title]` → agent's system prompt identity line
    (`group chat "AIRA - Cluster OpenRouter"`)
  - `channel_contacts.display_name` via force-upsert (visible in
    Contacts page + picker dropdowns)
- **Manual `group_aliases` map** on the channel config — override or
  supply names when WhatsApp can't fetch them. Row-based key-value
  editor in the UI; backfilled into contacts on channel start.
- **Group roster in system prompt** — every group inbound prepends a
  JID→name dictionary built from contacts + aliases, so the agent can
  translate JIDs it encounters in memory recall.
- **`sessions_list` tool enriched with `chat_name`** — each session
  entry includes the resolved display name so the LLM doesn't have to
  apply lookups in its head.

## Final images

| Image | Tag |
|---|---|
| Backend | `dekaregistry.cloudeka.id/cloudeka-system/goclaw:v3.12.0-patched.v14` |
| Web UI | `dekaregistry.cloudeka.id/cloudeka-system/goclaw-web:v3.12.0-patched.v11` |

Roll the backend pod to `v14` and (if you use the standalone web
container) the web pod to `v11`. No DB migration outside what upstream
v3.12.0 already brings.
