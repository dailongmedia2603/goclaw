import { useCallback, useEffect, useState } from "react";
import { useWs } from "@/hooks/use-ws";
import { useWsEvent } from "@/hooks/use-ws-event";
import type { BackfillState, StartOpts, StatusResponse } from "./types";

// useFbBackfill encapsulates the WS RPC + event subscription for one
// channel instance. Returns the latest state and the five control
// actions. Re-renders on progress/completed/failed events.
export function useFbBackfill(channelInstanceId: string) {
  const ws = useWs();
  const [state, setState] = useState<BackfillState | null>(null);
  const [loading, setLoading] = useState(true);

  // Initial load — RPC fb_backfill.status.
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    ws.call<StatusResponse>("fb_backfill.status", { channelInstanceId })
      .then((r) => {
        if (!cancelled) setState(r?.state ?? null);
      })
      .catch(() => {
        if (!cancelled) setState(null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [ws, channelInstanceId]);

  // Event subscriptions — filter by instanceId in payload because the
  // WS stream carries events for all tenant instances.
  const updateFromEvent = useCallback(
    (payload: unknown) => {
      const p = payload as { instanceId?: string; state?: BackfillState } | null;
      if (!p || p.instanceId !== channelInstanceId) return;
      if (p.state) setState(p.state);
    },
    [channelInstanceId],
  );
  useWsEvent("fb_backfill.progress", updateFromEvent);
  useWsEvent("fb_backfill.completed", updateFromEvent);
  useWsEvent("fb_backfill.started", (payload) => {
    const p = payload as { instanceId?: string } | null;
    if (p?.instanceId !== channelInstanceId) return;
    // Refetch on started so we see the fresh state immediately.
    ws.call<StatusResponse>("fb_backfill.status", { channelInstanceId })
      .then((r) => setState(r?.state ?? null))
      .catch(() => {});
  });
  useWsEvent("fb_backfill.paused", updateFromEvent);
  useWsEvent("fb_backfill.resumed", updateFromEvent);
  useWsEvent("fb_backfill.failed", (payload) => {
    const p = payload as { instanceId?: string; error?: string } | null;
    if (p?.instanceId !== channelInstanceId) return;
    // Refetch after failure to pick up last_error.
    ws.call<StatusResponse>("fb_backfill.status", { channelInstanceId })
      .then((r) => setState(r?.state ?? null))
      .catch(() => {});
  });

  // Actions.
  const start = useCallback(
    async (opts: StartOpts = {}) => {
      const r = await ws.call<{ status: string; state?: BackfillState }>(
        "fb_backfill.start",
        {
          channelInstanceId,
          maxConversations: opts.maxConversations,
          skipExisting: opts.skipExisting,
          forceRecreate: opts.forceRecreate,
          triggeredBy: opts.triggeredBy ?? "manual",
        },
      );
      if (r?.state) setState(r.state);
    },
    [ws, channelInstanceId],
  );

  const pause = useCallback(
    () => ws.call("fb_backfill.pause", { channelInstanceId }).catch(() => {}),
    [ws, channelInstanceId],
  );
  const resume = useCallback(
    () => ws.call("fb_backfill.resume", { channelInstanceId }).catch(() => {}),
    [ws, channelInstanceId],
  );
  const cancel = useCallback(
    () => ws.call("fb_backfill.cancel", { channelInstanceId }).catch(() => {}),
    [ws, channelInstanceId],
  );
  const retry = useCallback(
    () => ws.call("fb_backfill.retry", { channelInstanceId }).catch(() => {}),
    [ws, channelInstanceId],
  );

  return { state, loading, start, pause, resume, cancel, retry };
}
