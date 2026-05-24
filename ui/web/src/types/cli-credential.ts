export interface SecureCLIBinary {
  id: string;
  binary_name: string;
  binary_path?: string;
  description: string;
  deny_args: string[];
  deny_verbose: string[];
  timeout_seconds: number;
  tips: string;
  is_global: boolean;
  enabled: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
  /** Env variable names only (no values); from API for edit form */
  env_keys?: string[];
  /**
   * Agent grants summary for row chips (Phase 4 API field).
   * Absent on older API versions — capability-probe: skip rendering if undefined.
   */
  agent_grants_summary?: AgentGrantSummary[];
}

export interface CLIPresetEnvVar {
  name: string;
  desc: string;
  is_file?: boolean;
  optional?: boolean;
}

export interface CLIPreset {
  binary_name: string;
  description: string;
  env_vars: CLIPresetEnvVar[];
  deny_args: string[];
  deny_verbose: string[];
  timeout: number;
  tips: string;
}

export interface CLICredentialInput {
  preset?: string;
  binary_name: string;
  binary_path?: string;
  description?: string;
  deny_args?: string[];
  deny_verbose?: string[];
  timeout_seconds?: number;
  tips?: string;
  is_global?: boolean;
  enabled?: boolean;
  env?: Record<string, string>;
}

/** Per-agent grant with optional setting overrides */
export interface CLIAgentGrant {
  id: string;
  binary_id: string;
  agent_id: string;
  /**
   * Scopes the grant to a specific inbound chat (e.g. WhatsApp group JID).
   * null = grant applies to every chat for this agent (the "default" grant).
   * A non-null chat_id is more specific and wins over the default at lookup time.
   */
  chat_id?: string | null;
  deny_args: string[] | null;
  deny_verbose: string[] | null;
  timeout_seconds: number | null;
  tips: string | null;
  enabled: boolean;
  /** Whether this grant has an env override (keys present, values encrypted) */
  env_set?: boolean;
  /** Env variable names only (no values); populated when env_set=true */
  env_keys?: string[];
  created_at: string;
  updated_at: string;
}

export interface CLIAgentGrantInput {
  agent_id: string;
  /**
   * Per-grant chat scope. Semantics:
   * - absent / undefined → on create: defaults to all-chats; on update: leave unchanged
   * - null               → on update: clear the scope (revert to all-chats default)
   * - empty string ""    → coerced to null server-side
   * - non-empty string   → scope grant to that chat_id (e.g. WhatsApp group JID)
   */
  chat_id?: string | null;
  deny_args?: string[] | null;
  deny_verbose?: string[] | null;
  timeout_seconds?: number | null;
  tips?: string | null;
  enabled?: boolean;
  /**
   * env_vars semantics — 3-state, all three distinct behaviors (Finding #15):
   *
   * - **absent / undefined** → keep existing env override (omit from request payload)
   * - **null**               → clear override; grant falls back to binary-level defaults
   * - **`{}` (empty map)**   → treated as clear (same as null) — wipes the override
   * - **`{K: V, ...}`**      → replace the entire env override with this map
   *
   * Backend: internal/http/secure_cli_agent_grants.go handleUpdate (3-state env_vars branch).
   * Keys must match ^[A-Z_][A-Z0-9_]*$ and must not be on the denylist.
   */
  env_vars?: Record<string, string> | null;
}

/** Summary of a single grant shown in the table row chips (Phase 4 API field). */
export interface AgentGrantSummary {
  grant_id: string;
  agent_id: string;
  agent_key: string;
  name: string;
  enabled: boolean;
  env_set: boolean;
}
