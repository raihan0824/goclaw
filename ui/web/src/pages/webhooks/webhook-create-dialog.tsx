import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { Send } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useChannelInstances } from "@/pages/channels/hooks/use-channel-instances";
import { parseIpAllowlist, webhookCreateSchema, type WebhookCreateFormData } from "@/schemas/webhook.schema";
import type { WebhookCreateInput } from "@/types/webhook";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreate: (input: WebhookCreateInput) => Promise<void>;
}

export function WebhookCreateDialog({ open, onOpenChange, onCreate }: Props) {
  const { t } = useTranslation("webhooks");
  const { t: tc } = useTranslation("common");
  const { instances: channelInstances } = useChannelInstances({});
  const [saving, setSaving] = useState(false);

  const {
    register,
    handleSubmit,
    watch,
    setValue,
    reset,
    formState: { errors },
  } = useForm<WebhookCreateFormData>({
    resolver: zodResolver(webhookCreateSchema),
    defaultValues: {
      name: "",
      kind: "llm",
      agent_id: "",
      channel_id: "",
      rate_limit_per_min: 0,
      ip_allowlist_text: "",
      require_hmac: true,
      localhost_only: false,
    },
  });

  const kind = watch("kind");
  const requireHmac = watch("require_hmac");
  const localhostOnly = watch("localhost_only");
  const channelId = watch("channel_id");

  const onValid = async (data: WebhookCreateFormData) => {
    setSaving(true);
    try {
      const input: WebhookCreateInput = {
        name: data.name.trim(),
        kind: data.kind,
        rate_limit_per_min: data.rate_limit_per_min,
        ip_allowlist: parseIpAllowlist(data.ip_allowlist_text),
        require_hmac: data.require_hmac,
        localhost_only: data.localhost_only,
      };
      if (data.agent_id) input.agent_id = data.agent_id;
      if (data.kind === "message" && data.channel_id) input.channel_id = data.channel_id;
      await onCreate(input);
      reset();
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-sm:inset-0 max-sm:translate-x-0 max-sm:translate-y-0 sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Send className="h-5 w-5" />
            {t("form.title")}
          </DialogTitle>
          <DialogDescription>{t("form.description")}</DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit(onValid)} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="wh-name">{t("form.name")}</Label>
            <Input
              id="wh-name"
              {...register("name")}
              placeholder={t("form.namePlaceholder")}
              className="text-base md:text-sm"
            />
            {errors.name && <p className="text-xs text-destructive">{errors.name.message}</p>}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="wh-kind">{t("form.kind")}</Label>
            <Select
              value={kind}
              onValueChange={(v) => setValue("kind", v as "llm" | "message", { shouldValidate: true })}
            >
              <SelectTrigger id="wh-kind" className="text-base md:text-sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="llm">{t("kind.llm")}</SelectItem>
                <SelectItem value="message">{t("kind.message")}</SelectItem>
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">{t(`kindHint.${kind}`)}</p>
          </div>

          {kind === "message" && (
            <div className="space-y-1.5">
              <Label htmlFor="wh-channel">{t("form.channelId")}</Label>
              <Select
                value={channelId ?? ""}
                onValueChange={(v) =>
                  setValue("channel_id", v || "", { shouldValidate: true })
                }
              >
                <SelectTrigger id="wh-channel" className="text-base md:text-sm">
                  <SelectValue placeholder={t("form.channelIdPlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  {channelInstances.length === 0 ? (
                    <SelectItem value="__none__" disabled>
                      {t("form.noChannels")}
                    </SelectItem>
                  ) : (
                    channelInstances.map((c) => (
                      <SelectItem key={c.id} value={c.id}>
                        {c.display_name || c.name}{" "}
                        <span className="text-muted-foreground">({c.channel_type})</span>
                      </SelectItem>
                    ))
                  )}
                </SelectContent>
              </Select>
            </div>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="wh-agent">{t("form.agentId")}</Label>
            <Input
              id="wh-agent"
              {...register("agent_id")}
              placeholder={t("form.agentIdPlaceholder")}
              className="text-base md:text-sm font-mono"
            />
            {errors.agent_id && (
              <p className="text-xs text-destructive">{errors.agent_id.message}</p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="wh-rate">{t("form.rateLimit")}</Label>
            <Input
              id="wh-rate"
              type="number"
              min="0"
              {...register("rate_limit_per_min", { valueAsNumber: true })}
              className="text-base md:text-sm"
            />
            <p className="text-xs text-muted-foreground">{t("form.rateLimitHint")}</p>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="wh-ips">{t("form.ipAllowlist")}</Label>
            <Textarea
              id="wh-ips"
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
              onCheckedChange={(v) => setValue("require_hmac", v, { shouldValidate: true })}
            />
          </div>

          <div className="flex items-center justify-between rounded-md border px-3 py-2">
            <div>
              <div className="text-sm font-medium">{t("form.localhostOnly")}</div>
              <div className="text-xs text-muted-foreground">{t("form.localhostOnlyHint")}</div>
            </div>
            <Switch
              checked={localhostOnly}
              onCheckedChange={(v) => setValue("localhost_only", v, { shouldValidate: true })}
            />
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              {tc("cancel")}
            </Button>
            <Button type="submit" disabled={saving}>
              {saving ? t("form.creating") : t("form.create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
