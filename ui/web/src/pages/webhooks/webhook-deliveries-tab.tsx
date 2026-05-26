import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { EmptyState } from "@/components/shared/empty-state";
import { TableSkeleton } from "@/components/shared/loading-skeleton";
import { formatRelativeTime } from "@/lib/format";
import { useWebhookDeliveries } from "./hooks/use-webhook-deliveries";
import type { WebhookCallStatus } from "@/types/webhook";
import { Inbox } from "lucide-react";

const STATUS_OPTIONS: ("" | WebhookCallStatus)[] = [
  "",
  "queued",
  "running",
  "done",
  "failed",
  "dead",
];

function statusVariant(s: WebhookCallStatus): "default" | "secondary" | "destructive" {
  switch (s) {
    case "done":
      return "default";
    case "failed":
    case "dead":
      return "destructive";
    default:
      return "secondary";
  }
}

interface Props {
  webhookId: string;
}

export function WebhookDeliveriesTab({ webhookId }: Props) {
  const { t } = useTranslation("webhooks");
  const [status, setStatus] = useState<"" | WebhookCallStatus>("");
  const [offset, setOffset] = useState(0);
  const limit = 25;

  const { data, isLoading } = useWebhookDeliveries(webhookId, { status, limit, offset });
  const items = data?.items ?? [];
  const hasMore = data?.has_more ?? false;

  const onStatusChange = (v: string) => {
    setStatus(v as "" | WebhookCallStatus);
    setOffset(0);
  };

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <span className="text-sm text-muted-foreground">{t("deliveries.filterStatus")}</span>
        <Select value={status} onValueChange={onStatusChange}>
          <SelectTrigger className="w-40 text-base md:text-sm">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {STATUS_OPTIONS.map((s) => (
              <SelectItem key={s || "all"} value={s}>
                {s ? t(`status.${s}`) : t("deliveries.statusAll")}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {isLoading ? (
        <TableSkeleton rows={5} />
      ) : items.length === 0 ? (
        <EmptyState icon={Inbox} title={t("deliveries.empty")} />
      ) : (
        <div className="overflow-x-auto rounded-md border">
          <table className="w-full min-w-[700px] text-sm">
            <thead>
              <tr className="border-b bg-muted/50">
                <th className="px-3 py-2 text-left font-medium">{t("deliveries.columns.deliveryId")}</th>
                <th className="px-3 py-2 text-left font-medium">{t("deliveries.columns.status")}</th>
                <th className="px-3 py-2 text-left font-medium">{t("deliveries.columns.mode")}</th>
                <th className="px-3 py-2 text-left font-medium">{t("deliveries.columns.attempts")}</th>
                <th className="px-3 py-2 text-left font-medium">{t("deliveries.columns.callback")}</th>
                <th className="px-3 py-2 text-left font-medium">{t("deliveries.columns.lastError")}</th>
                <th className="px-3 py-2 text-left font-medium">{t("deliveries.columns.created")}</th>
                <th className="px-3 py-2 text-left font-medium">{t("deliveries.columns.completed")}</th>
              </tr>
            </thead>
            <tbody>
              {items.map((c) => (
                <tr key={c.id} className="border-b last:border-0 hover:bg-muted/30">
                  <td className="px-3 py-2 font-mono text-xs">{c.delivery_id.slice(0, 8)}...</td>
                  <td className="px-3 py-2">
                    <Badge variant={statusVariant(c.status)} className="text-xs">
                      {t(`status.${c.status}`)}
                    </Badge>
                  </td>
                  <td className="px-3 py-2">{c.mode}</td>
                  <td className="px-3 py-2 text-muted-foreground">{c.attempts}</td>
                  <td className="px-3 py-2 font-mono text-xs text-muted-foreground truncate max-w-[180px]" title={c.callback_url ?? ""}>
                    {c.callback_url ?? "—"}
                  </td>
                  <td className="px-3 py-2 text-destructive truncate max-w-[200px]" title={c.last_error ?? ""}>
                    {c.last_error ?? ""}
                  </td>
                  <td className="px-3 py-2 text-muted-foreground">{formatRelativeTime(c.created_at)}</td>
                  <td className="px-3 py-2 text-muted-foreground">
                    {c.completed_at ? formatRelativeTime(c.completed_at) : ""}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="flex items-center justify-between pt-1">
        <div className="text-xs text-muted-foreground">
          {t("deliveries.range", { from: offset + 1, to: offset + items.length })}
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={offset === 0}
            onClick={() => setOffset(Math.max(0, offset - limit))}
          >
            <ChevronLeft className="h-3.5 w-3.5" />
            {t("deliveries.prev")}
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={!hasMore}
            onClick={() => setOffset(offset + limit)}
          >
            {t("deliveries.next")}
            <ChevronRight className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>
    </div>
  );
}
