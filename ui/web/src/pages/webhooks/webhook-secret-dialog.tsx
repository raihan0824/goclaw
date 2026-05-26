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
import type { WebhookKind } from "@/types/webhook";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  secret: string;
  hmacSigningKey?: string;
  kind?: WebhookKind;
}

export function WebhookSecretDialog({
  open,
  onOpenChange,
  title,
  description,
  secret,
  hmacSigningKey,
  kind,
}: Props) {
  const { t } = useTranslation("webhooks");
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const [acknowledged, setAcknowledged] = useState(false);

  const url = kind ? `${window.location.origin}/v1/webhooks/${kind}` : "";
  const curlExample = kind
    ? buildCurl(url, kind, secret)
    : "";

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
      <DialogContent className="max-sm:inset-0 max-sm:translate-x-0 max-sm:translate-y-0 sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>

        <div className="space-y-3 max-h-[60vh] overflow-y-auto">
          {url && (
            <Field
              label={t("secret.urlLabel")}
              value={url}
              fieldKey="url"
              copiedField={copiedField}
              onCopy={copy}
              copyLabel={t("secret.copy")}
              copiedLabel={t("secret.copied")}
            />
          )}

          <Field
            label={t("secret.tokenLabel")}
            value={secret}
            fieldKey="secret"
            copiedField={copiedField}
            onCopy={copy}
            copyLabel={t("secret.copy")}
            copiedLabel={t("secret.copied")}
          />

          {hmacSigningKey && (
            <Field
              label={t("secret.hmacLabel")}
              value={hmacSigningKey}
              fieldKey="hmac"
              copiedField={copiedField}
              onCopy={copy}
              copyLabel={t("secret.copy")}
              copiedLabel={t("secret.copied")}
            />
          )}

          {curlExample && (
            <div>
              <div className="text-xs text-muted-foreground mb-1">{t("secret.curlLabel")}</div>
              <div className="relative">
                <pre className="overflow-x-auto rounded bg-muted px-3 py-2 text-xs font-mono whitespace-pre-wrap break-all">
                  {curlExample}
                </pre>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => copy(curlExample, "curl")}
                  className="gap-1 absolute right-1 top-1"
                >
                  {copiedField === "curl" ? (
                    <Check className="h-3.5 w-3.5" />
                  ) : (
                    <Copy className="h-3.5 w-3.5" />
                  )}
                  {copiedField === "curl" ? t("secret.copied") : t("secret.copy")}
                </Button>
              </div>
              <p className="text-xs text-muted-foreground mt-1">{t("secret.hmacHint")}</p>
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

function Field({
  label,
  value,
  fieldKey,
  copiedField,
  onCopy,
  copyLabel,
  copiedLabel,
}: {
  label: string;
  value: string;
  fieldKey: string;
  copiedField: string | null;
  onCopy: (v: string, k: string) => void;
  copyLabel: string;
  copiedLabel: string;
}) {
  const isCopied = copiedField === fieldKey;
  return (
    <div>
      <div className="text-xs text-muted-foreground mb-1">{label}</div>
      <div className="flex items-center gap-2">
        <code className="flex-1 overflow-x-auto rounded bg-muted px-3 py-2 text-base md:text-sm font-mono break-all">
          {value}
        </code>
        <Button
          variant="outline"
          size="sm"
          onClick={() => onCopy(value, fieldKey)}
          className="gap-1 shrink-0"
        >
          {isCopied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
          {isCopied ? copiedLabel : copyLabel}
        </Button>
      </div>
    </div>
  );
}

function buildCurl(url: string, kind: WebhookKind, token: string): string {
  if (kind === "llm") {
    return `curl -X POST '${url}' \\
  -H 'Authorization: Bearer ${token}' \\
  -H 'Content-Type: application/json' \\
  -d '{"input":"hello","mode":"sync"}'`;
  }
  // message
  return `curl -X POST '${url}' \\
  -H 'Authorization: Bearer ${token}' \\
  -H 'Content-Type: application/json' \\
  -d '{"channel_name":"<channel>","chat_id":"<id>","content":"hello"}'`;
}
