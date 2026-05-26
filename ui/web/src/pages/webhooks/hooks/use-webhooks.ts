import { useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import i18next from "i18next";
import { useHttp } from "@/hooks/use-ws";
import { queryKeys } from "@/lib/query-keys";
import { toast } from "@/stores/use-toast-store";
import type {
  WebhookData,
  WebhookCreateInput,
  WebhookCreateResponse,
  WebhookUpdateInput,
  WebhookRotateResponse,
} from "@/types/webhook";

export function useWebhooks() {
  const http = useHttp();
  const queryClient = useQueryClient();

  const { data: webhooks = [], isLoading: loading } = useQuery({
    queryKey: queryKeys.webhooks.all,
    queryFn: () => http.get<WebhookData[]>("/v1/webhooks"),
    staleTime: 60_000,
  });

  const invalidate = useCallback(
    () => queryClient.invalidateQueries({ queryKey: queryKeys.webhooks.all }),
    [queryClient],
  );

  const createWebhook = useCallback(
    async (data: WebhookCreateInput): Promise<WebhookCreateResponse> => {
      try {
        const res = await http.post<WebhookCreateResponse>("/v1/webhooks", data);
        await invalidate();
        toast.success(i18next.t("webhooks:toast.created"));
        return res;
      } catch (err) {
        toast.error(i18next.t("webhooks:toast.failedCreate"), err instanceof Error ? err.message : "");
        throw err;
      }
    },
    [http, invalidate],
  );

  const revokeWebhook = useCallback(
    async (id: string) => {
      try {
        await http.delete(`/v1/webhooks/${id}`);
        await invalidate();
        toast.success(i18next.t("webhooks:toast.revoked"));
      } catch (err) {
        toast.error(i18next.t("webhooks:toast.failedRevoke"), err instanceof Error ? err.message : "");
        throw err;
      }
    },
    [http, invalidate],
  );

  return { webhooks, loading, refresh: invalidate, createWebhook, revokeWebhook };
}

export function useWebhook(id: string | undefined) {
  const http = useHttp();
  return useQuery({
    queryKey: id ? queryKeys.webhooks.detail(id) : ["webhooks", "noop"],
    queryFn: () => http.get<WebhookData>(`/v1/webhooks/${id}`),
    enabled: !!id,
    staleTime: 30_000,
  });
}

export function useUpdateWebhook(id: string | undefined) {
  const http = useHttp();
  const queryClient = useQueryClient();
  return useCallback(
    async (data: WebhookUpdateInput) => {
      if (!id) return;
      try {
        await http.patch(`/v1/webhooks/${id}`, data);
        await Promise.all([
          queryClient.invalidateQueries({ queryKey: queryKeys.webhooks.all }),
          queryClient.invalidateQueries({ queryKey: queryKeys.webhooks.detail(id) }),
        ]);
        toast.success(i18next.t("webhooks:toast.updated"));
      } catch (err) {
        toast.error(i18next.t("webhooks:toast.failedUpdate"), err instanceof Error ? err.message : "");
        throw err;
      }
    },
    [http, id, queryClient],
  );
}

export function useRotateWebhook(id: string | undefined) {
  const http = useHttp();
  const queryClient = useQueryClient();
  return useCallback(async (): Promise<WebhookRotateResponse | undefined> => {
    if (!id) return undefined;
    try {
      const res = await http.post<WebhookRotateResponse>(`/v1/webhooks/${id}/rotate`, {});
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.webhooks.all }),
        queryClient.invalidateQueries({ queryKey: queryKeys.webhooks.detail(id) }),
      ]);
      toast.success(i18next.t("webhooks:toast.rotated"));
      return res;
    } catch (err) {
      toast.error(i18next.t("webhooks:toast.failedRotate"), err instanceof Error ? err.message : "");
      throw err;
    }
  }, [http, id, queryClient]);
}
