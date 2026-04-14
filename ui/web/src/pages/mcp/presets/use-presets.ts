import { useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import i18next from "i18next";
import { useHttp } from "@/hooks/use-ws";
import { queryKeys } from "@/lib/query-keys";
import { toast } from "@/stores/use-toast-store";
import type { MCPServerData } from "@/types/mcp";
import type { PresetMetadata } from "./types";

export function usePresets() {
  const http = useHttp();
  return useQuery({
    queryKey: queryKeys.mcp.presets,
    queryFn: async () => {
      const res = await http.get<{ presets: PresetMetadata[] }>("/v1/mcp/presets");
      return res.presets ?? [];
    },
    staleTime: 5 * 60_000,
  });
}

export function usePresetActions() {
  const http = useHttp();
  const queryClient = useQueryClient();

  const invalidate = useCallback(
    () => queryClient.invalidateQueries({ queryKey: queryKeys.mcp.all }),
    [queryClient],
  );

  const createFromPreset = useCallback(
    async (presetId: string, payload: Record<string, unknown>) => {
      try {
        const res = await http.post<MCPServerData>(
          `/v1/mcp/presets/${presetId}/servers`,
          payload,
        );
        await invalidate();
        toast.success(i18next.t("mcp:toast.created"));
        return res;
      } catch (err) {
        toast.error(
          i18next.t("mcp:toast.failedCreate"),
          err instanceof Error ? err.message : "",
        );
        throw err;
      }
    },
    [http, invalidate],
  );

  const updateFromPreset = useCallback(
    async (presetId: string, serverId: string, payload: Record<string, unknown>) => {
      try {
        await http.put(`/v1/mcp/presets/${presetId}/servers/${serverId}`, payload);
        await invalidate();
        toast.success(i18next.t("mcp:toast.updated"));
      } catch (err) {
        toast.error(
          i18next.t("mcp:toast.failedUpdate"),
          err instanceof Error ? err.message : "",
        );
        throw err;
      }
    },
    [http, invalidate],
  );

  return { createFromPreset, updateFromPreset };
}

/** Extract the preset id from a server row, if it was created via a preset. */
export function getServerPreset(server: MCPServerData | null | undefined): string | null {
  if (!server) return null;
  const settings = (server.settings as { preset?: string } | undefined) ?? undefined;
  return settings?.preset ?? null;
}
