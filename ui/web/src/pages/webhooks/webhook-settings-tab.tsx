import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { parseIpAllowlist, webhookUpdateSchema, type WebhookUpdateFormData } from "@/schemas/webhook.schema";
import type { WebhookData, WebhookUpdateInput } from "@/types/webhook";

interface Props {
  webhook: WebhookData;
  onSave: (data: WebhookUpdateInput) => Promise<void>;
}

export function WebhookSettingsTab({ webhook, onSave }: Props) {
  const { t } = useTranslation("webhooks");
  const [saving, setSaving] = useState(false);

  const defaults: WebhookUpdateFormData = {
    name: webhook.name,
    channel_id: webhook.channel_id ?? "",
    rate_limit_per_min: webhook.rate_limit_per_min,
    ip_allowlist_text: (webhook.ip_allowlist ?? []).join("\n"),
    require_hmac: webhook.require_hmac,
    localhost_only: webhook.localhost_only,
  };

  const {
    register,
    handleSubmit,
    watch,
    setValue,
    reset,
    formState: { errors, isDirty },
  } = useForm<WebhookUpdateFormData>({
    resolver: zodResolver(webhookUpdateSchema),
    defaultValues: defaults,
  });

  useEffect(() => {
    reset(defaults);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [webhook.id]);

  const requireHmac = watch("require_hmac");
  const localhostOnly = watch("localhost_only");

  const onValid = async (data: WebhookUpdateFormData) => {
    setSaving(true);
    try {
      const payload: WebhookUpdateInput = {
        name: data.name.trim(),
        rate_limit_per_min: data.rate_limit_per_min,
        ip_allowlist: parseIpAllowlist(data.ip_allowlist_text),
        require_hmac: data.require_hmac,
        localhost_only: data.localhost_only,
      };
      if (webhook.kind === "message") {
        payload.channel_id = data.channel_id || null;
      }
      await onSave(payload);
      reset(data);
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={handleSubmit(onValid)} className="space-y-4 max-w-2xl">
      <div className="grid gap-3 rounded-md border p-3 text-sm bg-muted/30">
        <div className="flex justify-between">
          <span className="text-muted-foreground">{t("settings.kind")}</span>
          <span className="font-mono">{webhook.kind}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-muted-foreground">{t("settings.secretPrefix")}</span>
          <code className="font-mono text-xs">{webhook.secret_prefix}…</code>
        </div>
        <div className="flex justify-between">
          <span className="text-muted-foreground">{t("settings.id")}</span>
          <code className="font-mono text-xs">{webhook.id}</code>
        </div>
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="wh-edit-name">{t("form.name")}</Label>
        <Input
          id="wh-edit-name"
          {...register("name")}
          className="text-base md:text-sm"
        />
        {errors.name && <p className="text-xs text-destructive">{errors.name.message}</p>}
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="wh-edit-rate">{t("form.rateLimit")}</Label>
        <Input
          id="wh-edit-rate"
          type="number"
          min="0"
          {...register("rate_limit_per_min", { valueAsNumber: true })}
          className="text-base md:text-sm"
        />
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="wh-edit-ips">{t("form.ipAllowlist")}</Label>
        <Textarea
          id="wh-edit-ips"
          {...register("ip_allowlist_text")}
          placeholder={"10.0.0.0/8\n192.168.1.42"}
          rows={3}
          className="text-base md:text-sm font-mono"
        />
        {errors.ip_allowlist_text && (
          <p className="text-xs text-destructive">{errors.ip_allowlist_text.message}</p>
        )}
      </div>

      <div className="flex items-center justify-between rounded-md border px-3 py-2">
        <div>
          <div className="text-sm font-medium">{t("form.requireHmac")}</div>
          <div className="text-xs text-muted-foreground">{t("form.requireHmacHint")}</div>
        </div>
        <Switch
          checked={requireHmac}
          onCheckedChange={(v) => setValue("require_hmac", v, { shouldDirty: true, shouldValidate: true })}
        />
      </div>

      <div className="flex items-center justify-between rounded-md border px-3 py-2">
        <div>
          <div className="text-sm font-medium">{t("form.localhostOnly")}</div>
          <div className="text-xs text-muted-foreground">{t("form.localhostOnlyHint")}</div>
        </div>
        <Switch
          checked={localhostOnly}
          onCheckedChange={(v) => setValue("localhost_only", v, { shouldDirty: true, shouldValidate: true })}
        />
      </div>

      <div className="flex items-center justify-end gap-2 pt-2">
        <Button type="button" variant="outline" onClick={() => reset(defaults)} disabled={!isDirty || saving}>
          {t("settings.discard")}
        </Button>
        <Button type="submit" disabled={!isDirty || saving}>
          {saving ? t("settings.saving") : t("settings.save")}
        </Button>
      </div>
    </form>
  );
}
