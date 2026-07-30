import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { AlertCircle, CheckCircle2, Clock, Loader2, PauseCircle, PlayCircle, RotateCw, XCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useFbBackfill } from "./use-fb-backfill";
import type { BackfillState, JobStatus } from "./types";

// FbBackfillPanel renders controls + progress for one channel instance's
// backfill job. Mounted conditionally in channel-detail-page.tsx when
// channel_type === "facebook". When autoStart is true and no prior
// backfill state exists, the panel triggers Start once on first mount —
// this is how the "backfill_on_create" create-dialog checkbox gets wired
// without editing the shared create dialog.
export function FbBackfillPanel({
  channelInstanceId,
  autoStart = false,
}: {
  channelInstanceId: string;
  autoStart?: boolean;
}) {
  const { t } = useTranslation("channels");
  const { state, loading, start, pause, resume, cancel, retry } = useFbBackfill(channelInstanceId);
  const [busy, setBusy] = useState(false);
  const autoTriggered = useRef(false);

  // One-shot auto-start when the panel first sees state=null and the
  // channel was created with backfill_on_create=true.
  useEffect(() => {
    if (!autoStart || loading || autoTriggered.current) return;
    if (state !== null) return; // a run already exists — do not double-trigger
    autoTriggered.current = true;
    start({ maxConversations: 500, skipExisting: true, triggeredBy: "auto_on_create" }).catch(() => {});
  }, [autoStart, loading, state, start]);

  const wrap = (fn: () => Promise<unknown>) => async () => {
    setBusy(true);
    try {
      await fn();
    } finally {
      setBusy(false);
    }
  };

  const handleCancel = async () => {
    if (!window.confirm(t("fbBackfill.confirmCancel"))) return;
    await wrap(cancel)();
  };
  const handleResync = async () => {
    if (!window.confirm(t("fbBackfill.confirmResync"))) return;
    await wrap(() => start({ forceRecreate: true, triggeredBy: "manual" }))();
  };

  const statusLabel = useMemo(() => {
    const s = (state?.status ?? "none") as JobStatus | "none";
    return t(`fbBackfill.status.${s}`);
  }, [state?.status, t]);

  return (
    <section className="rounded-lg border bg-card p-4 space-y-3">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h3 className="font-medium text-sm flex items-center gap-2">
            <StatusIcon status={state?.status} />
            {t("fbBackfill.title")}
            <span className="text-xs text-muted-foreground font-normal">
              ({statusLabel})
            </span>
          </h3>
          <p className="text-xs text-muted-foreground mt-1">
            {t("fbBackfill.description")}
          </p>
        </div>
      </div>

      {loading ? (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
          <span>...</span>
        </div>
      ) : (
        <>
          {state && <ProgressView state={state} />}

          {state?.last_error && (
            <div className="flex items-start gap-2 rounded-md border border-destructive/30 bg-destructive/5 p-2 text-xs text-destructive">
              <AlertCircle className="h-3.5 w-3.5 mt-0.5 shrink-0" />
              <span>{state.last_error}</span>
            </div>
          )}

          <div className="flex flex-wrap gap-2">
            {(!state || state.status === "completed" || state.status === "cancelled") && (
              <Button
                size="sm"
                disabled={busy}
                onClick={wrap(() => start({ maxConversations: 500, skipExisting: true, triggeredBy: "manual" }))}
              >
                <PlayCircle className="h-3.5 w-3.5 mr-1.5" />
                {t("fbBackfill.startButton")}
              </Button>
            )}
            {state?.status === "running" && (
              <>
                <Button size="sm" variant="outline" disabled={busy} onClick={wrap(pause)}>
                  <PauseCircle className="h-3.5 w-3.5 mr-1.5" />
                  {t("fbBackfill.pause")}
                </Button>
                <Button size="sm" variant="destructive" disabled={busy} onClick={handleCancel}>
                  <XCircle className="h-3.5 w-3.5 mr-1.5" />
                  {t("fbBackfill.cancel")}
                </Button>
              </>
            )}
            {state?.status === "paused" && (
              <>
                <Button size="sm" disabled={busy} onClick={wrap(resume)}>
                  <PlayCircle className="h-3.5 w-3.5 mr-1.5" />
                  {t("fbBackfill.resume")}
                </Button>
                <Button size="sm" variant="destructive" disabled={busy} onClick={handleCancel}>
                  <XCircle className="h-3.5 w-3.5 mr-1.5" />
                  {t("fbBackfill.cancel")}
                </Button>
              </>
            )}
            {state?.status === "failed" && (
              <Button size="sm" disabled={busy} onClick={wrap(retry)}>
                <RotateCw className="h-3.5 w-3.5 mr-1.5" />
                {t("fbBackfill.retry")}
              </Button>
            )}
            {state?.status === "completed" && (
              <Button size="sm" variant="outline" disabled={busy} onClick={handleResync}>
                <RotateCw className="h-3.5 w-3.5 mr-1.5" />
                {t("fbBackfill.resync")}
              </Button>
            )}
          </div>
        </>
      )}
    </section>
  );
}

function ProgressView({ state }: { state: BackfillState }) {
  const { t } = useTranslation("channels");
  const total = Math.max(state.conversations_total, 1);
  const pct = Math.min(100, (state.conversations_done / total) * 100);
  const isFailed = state.status === "failed";
  const isRunning = state.status === "running";

  return (
    <div className="space-y-1.5">
      <div className="h-1.5 rounded-full bg-muted overflow-hidden">
        <div
          className={
            isFailed
              ? "h-full bg-destructive transition-all"
              : isRunning
              ? "h-full bg-primary transition-all"
              : "h-full bg-muted-foreground/60 transition-all"
          }
          style={{ width: `${pct}%` }}
        />
      </div>
      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span>
          {t("fbBackfill.stats", {
            convosDone: state.conversations_done,
            convosTotal: state.conversations_total,
            msgs: state.messages_ingested,
            episodics: state.episodics_created,
          })}
        </span>
        <span>{pct.toFixed(0)}%</span>
      </div>
      {state.conversations_skipped > 0 && (
        <div className="text-xs text-muted-foreground">
          {t("fbBackfill.skippedStats", { skipped: state.conversations_skipped })}
        </div>
      )}
    </div>
  );
}

function StatusIcon({ status }: { status?: JobStatus }) {
  switch (status) {
    case "running":
      return <Loader2 className="h-3.5 w-3.5 animate-spin text-primary" />;
    case "paused":
      return <PauseCircle className="h-3.5 w-3.5 text-muted-foreground" />;
    case "completed":
      return <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />;
    case "failed":
      return <AlertCircle className="h-3.5 w-3.5 text-destructive" />;
    case "cancelled":
      return <XCircle className="h-3.5 w-3.5 text-muted-foreground" />;
    case "pending":
      return <Clock className="h-3.5 w-3.5 text-muted-foreground" />;
    default:
      return <Clock className="h-3.5 w-3.5 text-muted-foreground" />;
  }
}
