import { z } from "zod";

const cidrOrIp = /^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$/;

export function parseIpAllowlist(text: string): string[] {
  return text
    .split(/\r?\n/)
    .map((s) => s.trim())
    .filter(Boolean);
}

const ipAllowlistField = z
  .string()
  .refine(
    (s) => parseIpAllowlist(s).every((line) => cidrOrIp.test(line)),
    { message: "Each line must be an IPv4 address or CIDR block (e.g. 10.0.0.0/8)" },
  );

export const webhookCreateSchema = z.object({
  name: z.string().min(1, "Required").max(100),
  kind: z.enum(["llm", "message"]),
  agent_id: z
    .string()
    .optional()
    .refine(
      (v) => !v || /^[0-9a-fA-F-]{36}$/.test(v),
      { message: "Must be a UUID" },
    ),
  channel_id: z
    .string()
    .optional()
    .refine(
      (v) => !v || /^[0-9a-fA-F-]{36}$/.test(v),
      { message: "Must be a UUID" },
    ),
  rate_limit_per_min: z.number().int().min(0).max(100000),
  ip_allowlist_text: ipAllowlistField,
  require_hmac: z.boolean(),
  localhost_only: z.boolean(),
});
export type WebhookCreateFormData = z.infer<typeof webhookCreateSchema>;

export const webhookUpdateSchema = z.object({
  name: z.string().min(1).max(100),
  channel_id: z
    .string()
    .optional()
    .refine(
      (v) => !v || /^[0-9a-fA-F-]{36}$/.test(v),
      { message: "Must be a UUID" },
    ),
  rate_limit_per_min: z.number().int().min(0).max(100000),
  ip_allowlist_text: ipAllowlistField,
  require_hmac: z.boolean(),
  localhost_only: z.boolean(),
});
export type WebhookUpdateFormData = z.infer<typeof webhookUpdateSchema>;
