import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Copy, Check } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  secret: string;
  hmacSigningKey?: string;
}

export function WebhookSecretDialog({
  open,
  onOpenChange,
  title,
  description,
  secret,
  hmacSigningKey,
}: Props) {
  const { t } = useTranslation("webhooks");
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const [acknowledged, setAcknowledged] = useState(false);

  const copy = async (value: string, field: string) => {
    await navigator.clipboard.writeText(value);
    setCopiedField(field);
    setTimeout(() => setCopiedField(null), 2000);
  };

  const handleClose = (next: boolean) => {
    if (!next) {
      setAcknowledged(false);
      setCopiedField(null);
    }
    onOpenChange(next);
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="max-sm:inset-0 max-sm:translate-x-0 max-sm:translate-y-0 sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <div>
            <div className="text-xs text-muted-foreground mb-1">{t("secret.tokenLabel")}</div>
            <div className="flex items-center gap-2">
              <code className="flex-1 overflow-x-auto rounded bg-muted px-3 py-2 text-base md:text-sm font-mono break-all">
                {secret}
              </code>
              <Button
                variant="outline"
                size="sm"
                onClick={() => copy(secret, "secret")}
                className="gap-1 shrink-0"
              >
                {copiedField === "secret" ? (
                  <Check className="h-3.5 w-3.5" />
                ) : (
                  <Copy className="h-3.5 w-3.5" />
                )}
                {copiedField === "secret" ? t("secret.copied") : t("secret.copy")}
              </Button>
            </div>
          </div>

          {hmacSigningKey && (
            <div>
              <div className="text-xs text-muted-foreground mb-1">
                {t("secret.hmacLabel")}
              </div>
              <div className="flex items-center gap-2">
                <code className="flex-1 overflow-x-auto rounded bg-muted px-3 py-2 text-base md:text-sm font-mono break-all">
                  {hmacSigningKey}
                </code>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => copy(hmacSigningKey, "hmac")}
                  className="gap-1 shrink-0"
                >
                  {copiedField === "hmac" ? (
                    <Check className="h-3.5 w-3.5" />
                  ) : (
                    <Copy className="h-3.5 w-3.5" />
                  )}
                  {copiedField === "hmac" ? t("secret.copied") : t("secret.copy")}
                </Button>
              </div>
            </div>
          )}

          <label className="flex items-center gap-2 text-sm pt-2">
            <input
              type="checkbox"
              checked={acknowledged}
              onChange={(e) => setAcknowledged(e.target.checked)}
              className="h-4 w-4 rounded border-input"
            />
            <span>{t("secret.acknowledge")}</span>
          </label>
        </div>

        <DialogFooter>
          <Button onClick={() => handleClose(false)} disabled={!acknowledged}>
            {t("secret.done")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
