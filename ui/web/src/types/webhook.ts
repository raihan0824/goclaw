export type WebhookKind = "llm" | "message";
export type WebhookCallStatus = "queued" | "running" | "done" | "failed" | "dead";
export type WebhookCallMode = "sync" | "async";

export interface WebhookData {
  id: string;
  tenant_id: string;
  agent_id?: string | null;
  name: string;
  kind: WebhookKind;
  secret_prefix: string;
  scopes: string[];
  channel_id?: string | null;
  rate_limit_per_min: number;
  ip_allowlist: string[];
  require_hmac: boolean;
  localhost_only: boolean;
  revoked: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
  last_used_at?: string | null;
}

export interface WebhookCreateInput {
  name: string;
  kind: WebhookKind;
  agent_id?: string;
  channel_id?: string;
  scopes?: string[];
  rate_limit_per_min?: number;
  ip_allowlist?: string[];
  require_hmac?: boolean;
  localhost_only?: boolean;
}

export interface WebhookCreateResponse extends WebhookData {
  secret: string;
  hmac_signing_key: string;
}

export interface WebhookUpdateInput {
  name?: string;
  scopes?: string[];
  channel_id?: string | null;
  rate_limit_per_min?: number;
  ip_allowlist?: string[];
  require_hmac?: boolean;
  localhost_only?: boolean;
}

export interface WebhookRotateResponse {
  id: string;
  secret: string;
  hmac_signing_key: string;
  secret_prefix: string;
}

export interface WebhookCall {
  id: string;
  delivery_id: string;
  status: WebhookCallStatus;
  mode: WebhookCallMode;
  attempts: number;
  callback_url?: string | null;
  last_error?: string | null;
  next_attempt_at?: string | null;
  started_at?: string | null;
  completed_at?: string | null;
  created_at: string;
}

export interface WebhookCallsPage {
  items: WebhookCall[];
  limit: number;
  offset: number;
  has_more: boolean;
}
