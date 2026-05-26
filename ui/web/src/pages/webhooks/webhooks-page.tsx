import { lazy, Suspense, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Send, Plus, RefreshCw, Ban, Trash2, ArrowLeft, RefreshCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { SearchInput } from "@/components/shared/search-input";
import { TableSkeleton } from "@/components/shared/loading-skeleton";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { useMinLoading } from "@/hooks/use-min-loading";
import { ROUTES } from "@/lib/constants";
import { formatRelativeTime } from "@/lib/format";
import {
  useWebhook,
  useWebhooks,
  useUpdateWebhook,
  useRotateWebhook,
} from "./hooks/use-webhooks";
import type { WebhookData, WebhookCreateResponse, WebhookKind, WebhookRotateResponse } from "@/types/webhook";

const WebhookCreateDialog = lazy(() =>
  import("./webhook-create-dialog").then((m) => ({ default: m.WebhookCreateDialog })),
);
const WebhookSecretDialog = lazy(() =>
  import("./webhook-secret-dialog").then((m) => ({ default: m.WebhookSecretDialog })),
);
const WebhookSettingsTab = lazy(() =>
  import("./webhook-settings-tab").then((m) => ({ default: m.WebhookSettingsTab })),
);
const WebhookDeliveriesTab = lazy(() =>
  import("./webhook-deliveries-tab").then((m) => ({ default: m.WebhookDeliveriesTab })),
);

function statusBadge(wh: WebhookData, label: { active: string; revoked: string }) {
  if (wh.revoked) return { label: label.revoked, variant: "destructive" as const };
  return { label: label.active, variant: "default" as const };
}

export function WebhooksPage() {
  const { id: detailId } = useParams<{ id: string }>();
  if (detailId) return <WebhookDetail id={detailId} />;
  return <WebhookList />;
}

function WebhookList() {
  const { t } = useTranslation("webhooks");
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const { webhooks, loading, refresh, createWebhook, revokeWebhook, purgeWebhook } = useWebhooks();
  const spinning = useMinLoading(loading);
  const [search, setSearch] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [actionTarget, setActionTarget] = useState<WebhookData | null>(null);
  const [acting, setActing] = useState(false);
  const [secretShown, setSecretShown] = useState<{
    secret: string;
    hmac?: string;
    title: string;
    description: string;
    kind: WebhookKind;
  } | null>(null);

  const filtered = webhooks.filter(
    (w) =>
      w.name.toLowerCase().includes(search.toLowerCase()) ||
      w.secret_prefix.toLowerCase().includes(search.toLowerCase()),
  );

  const handleCreate = async (input: Parameters<typeof createWebhook>[0]) => {
    const res: WebhookCreateResponse = await createWebhook(input);
    setCreateOpen(false);
    setSecretShown({
      secret: res.secret,
      hmac: res.hmac_signing_key,
      title: t("created.title"),
      description: t("created.description"),
      kind: res.kind,
    });
  };

  const handleAction = async () => {
    if (!actionTarget) return;
    setActing(true);
    try {
      if (actionTarget.revoked) {
        await purgeWebhook(actionTarget.id);
      } else {
        await revokeWebhook(actionTarget.id);
      }
      setActionTarget(null);
    } finally {
      setActing(false);
    }
  };

  return (
    <div className="p-4 sm:p-6 pb-10">
      <PageHeader
        title={t("title")}
        description={t("description")}
        actions={
          <div className="flex gap-2">
            <Button size="sm" onClick={() => setCreateOpen(true)} className="gap-1">
              <Plus className="h-3.5 w-3.5" /> {t("addWebhook")}
            </Button>
            <Button variant="outline" size="sm" onClick={refresh} disabled={spinning} className="gap-1">
              <RefreshCw className={spinning ? "animate-spin h-3.5 w-3.5" : "h-3.5 w-3.5"} />
              {tc("refresh")}
            </Button>
          </div>
        }
      />

      <div className="mt-4">
        <SearchInput value={search} onChange={setSearch} placeholder={t("searchPlaceholder")} className="max-w-sm" />
      </div>

      <div className="mt-4">
        {loading && webhooks.length === 0 ? (
          <TableSkeleton rows={4} />
        ) : filtered.length === 0 ? (
          <EmptyState icon={Send} title={t("emptyTitle")} description={t("emptyDescription")} />
        ) : (
          <div className="overflow-x-auto rounded-md border">
            <table className="w-full min-w-[600px] text-sm">
              <thead>
                <tr className="border-b bg-muted/50">
                  <th className="px-4 py-3 text-left font-medium">{t("columns.name")}</th>
                  <th className="px-4 py-3 text-left font-medium">{t("columns.kind")}</th>
                  <th className="px-4 py-3 text-left font-medium">{t("columns.status")}</th>
                  <th className="px-4 py-3 text-left font-medium">{t("columns.lastUsed")}</th>
                  <th className="px-4 py-3 text-right font-medium">{t("columns.actions")}</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((wh) => {
                  const sb = statusBadge(wh, { active: t("status.active"), revoked: t("status.revoked") });
                  return (
                    <tr
                      key={wh.id}
                      className="border-b last:border-0 hover:bg-muted/30 cursor-pointer"
                      onClick={() => navigate(`${ROUTES.WEBHOOKS}/${wh.id}`)}
                    >
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <Send className="h-4 w-4 text-muted-foreground shrink-0" />
                          <div>
                            <div className={`font-medium ${wh.revoked ? "line-through text-muted-foreground" : ""}`}>
                              {wh.name}
                            </div>
                            <code className="text-xs-plus text-muted-foreground font-mono">
                              {wh.secret_prefix}…
                            </code>
                          </div>
                        </div>
                      </td>
                      <td className="px-4 py-3">
                        <Badge variant="secondary" className="text-xs font-mono px-1.5 py-0">
                          {wh.kind}
                        </Badge>
                      </td>
                      <td className="px-4 py-3">
                        <Badge variant={sb.variant} className="text-xs">
                          {sb.label}
                        </Badge>
                      </td>
                      <td className="px-4 py-3 text-muted-foreground">
                        {wh.last_used_at ? formatRelativeTime(wh.last_used_at) : t("neverUsed")}
                      </td>
                      <td className="px-4 py-3 text-right">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={(e) => {
                            e.stopPropagation();
                            setActionTarget(wh);
                          }}
                          className="text-destructive hover:text-destructive"
                          title={wh.revoked ? t("delete.title") : t("revoke.title")}
                        >
                          {wh.revoked ? (
                            <Trash2 className="h-3.5 w-3.5" />
                          ) : (
                            <Ban className="h-3.5 w-3.5" />
                          )}
                        </Button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <Suspense fallback={null}>
        {createOpen && (
          <WebhookCreateDialog open={createOpen} onOpenChange={setCreateOpen} onCreate={handleCreate} />
        )}
        {secretShown && (
          <WebhookSecretDialog
            open={!!secretShown}
            onOpenChange={(v) => !v && setSecretShown(null)}
            title={secretShown.title}
            description={secretShown.description}
            secret={secretShown.secret}
            hmacSigningKey={secretShown.hmac}
            kind={secretShown.kind}
          />
        )}
      </Suspense>

      <ConfirmDialog
        open={!!actionTarget}
        onOpenChange={(v) => !v && setActionTarget(null)}
        title={actionTarget?.revoked ? t("delete.title") : t("revoke.title")}
        description={
          actionTarget?.revoked
            ? t("delete.description", { name: actionTarget?.name ?? "" })
            : t("revoke.description", { name: actionTarget?.name ?? "" })
        }
        confirmLabel={
          actionTarget?.revoked ? t("delete.confirmLabel") : t("revoke.confirmLabel")
        }
        variant="destructive"
        onConfirm={handleAction}
        loading={acting}
      />
    </div>
  );
}

function WebhookDetail({ id }: { id: string }) {
  const { t } = useTranslation("webhooks");
  const { t: tc } = useTranslation("common");
  const navigate = useNavigate();
  const { data: webhook, isLoading } = useWebhook(id);
  const updateWebhook = useUpdateWebhook(id);
  const rotateWebhook = useRotateWebhook(id);
  const [rotateOpen, setRotateOpen] = useState(false);
  const [rotating, setRotating] = useState(false);
  const [rotatedSecret, setRotatedSecret] = useState<WebhookRotateResponse | null>(null);

  if (isLoading) {
    return (
      <div className="p-4 sm:p-6">
        <TableSkeleton rows={6} />
      </div>
    );
  }
  if (!webhook) {
    return (
      <div className="p-4 sm:p-6">
        <Button variant="ghost" size="sm" onClick={() => navigate(ROUTES.WEBHOOKS)} className="gap-1">
          <ArrowLeft className="h-3.5 w-3.5" /> {tc("back")}
        </Button>
        <EmptyState icon={Send} title={t("notFound")} />
      </div>
    );
  }

  const sb = statusBadge(webhook, { active: t("status.active"), revoked: t("status.revoked") });

  const handleRotate = async () => {
    setRotating(true);
    try {
      const res = await rotateWebhook();
      setRotateOpen(false);
      if (res) setRotatedSecret(res);
    } finally {
      setRotating(false);
    }
  };

  return (
    <div className="p-4 sm:p-6 pb-10 space-y-4">
      <Button variant="ghost" size="sm" onClick={() => navigate(ROUTES.WEBHOOKS)} className="gap-1">
        <ArrowLeft className="h-3.5 w-3.5" /> {tc("back")}
      </Button>

      <PageHeader
        title={webhook.name}
        description={
          <span className="flex items-center gap-2 flex-wrap">
            <Badge variant="secondary" className="text-xs font-mono px-1.5 py-0">{webhook.kind}</Badge>
            <Badge variant={sb.variant} className="text-xs">{sb.label}</Badge>
            <code className="text-xs text-muted-foreground font-mono">{webhook.secret_prefix}…</code>
          </span>
        }
        actions={
          !webhook.revoked && (
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={() => setRotateOpen(true)} className="gap-1">
                <RefreshCcw className="h-3.5 w-3.5" /> {t("actions.rotate")}
              </Button>
            </div>
          )
        }
      />

      <Tabs defaultValue="settings">
        <TabsList>
          <TabsTrigger value="settings">{t("tabs.settings")}</TabsTrigger>
          <TabsTrigger value="deliveries">{t("tabs.deliveries")}</TabsTrigger>
        </TabsList>

        <TabsContent value="settings" className="pt-4">
          <Suspense fallback={<TableSkeleton rows={4} />}>
            <WebhookSettingsTab webhook={webhook} onSave={updateWebhook} />
          </Suspense>
        </TabsContent>

        <TabsContent value="deliveries" className="pt-4">
          <Suspense fallback={<TableSkeleton rows={4} />}>
            <WebhookDeliveriesTab webhookId={id} />
          </Suspense>
        </TabsContent>
      </Tabs>

      <ConfirmDialog
        open={rotateOpen}
        onOpenChange={setRotateOpen}
        title={t("rotate.title")}
        description={t("rotate.description")}
        confirmLabel={t("actions.rotate")}
        onConfirm={handleRotate}
        loading={rotating}
      />

      <Suspense fallback={null}>
        {rotatedSecret && (
          <WebhookSecretDialog
            open={!!rotatedSecret}
            onOpenChange={(v) => !v && setRotatedSecret(null)}
            title={t("rotated.title")}
            description={t("rotated.description")}
            secret={rotatedSecret.secret}
            hmacSigningKey={rotatedSecret.hmac_signing_key}
            kind={webhook.kind}
          />
        )}
      </Suspense>
    </div>
  );
}
