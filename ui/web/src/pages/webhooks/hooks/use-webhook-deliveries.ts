import { useQuery } from "@tanstack/react-query";
import { useHttp } from "@/hooks/use-ws";
import { queryKeys } from "@/lib/query-keys";
import type { WebhookCallsPage, WebhookCallStatus } from "@/types/webhook";

interface Params {
  status?: WebhookCallStatus | "";
  limit?: number;
  offset?: number;
}

export function useWebhookDeliveries(id: string | undefined, params: Params) {
  const http = useHttp();
  const status = params.status ?? "";
  const limit = params.limit ?? 25;
  const offset = params.offset ?? 0;

  return useQuery({
    queryKey: id ? queryKeys.webhooks.deliveries(id, { status, limit, offset }) : ["webhooks", "deliveries", "noop"],
    queryFn: () => {
      const q: Record<string, string> = {
        limit: String(limit),
        offset: String(offset),
      };
      if (status) q.status = status;
      return http.get<WebhookCallsPage>(`/v1/webhooks/${id}/calls`, q);
    },
    enabled: !!id,
    staleTime: 10_000,
  });
}
