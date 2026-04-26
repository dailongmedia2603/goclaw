import { useCallback } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import i18next from "i18next";
import { useWs } from "@/hooks/use-ws";
import { useAuthStore } from "@/stores/use-auth-store";
import { toast } from "@/stores/use-toast-store";

// --- Types mirroring internal/channels/fbcloak/types.go ---

export type CredentialStatus = "active" | "expired" | "checkpoint" | "disabled";

export interface FBCloakCredential {
  id: string;
  tenantId: string;
  fanpageId: string;
  fanpageName: string;
  userAgent: string;
  viewportW: number;
  viewportH: number;
  timezone: string;
  status: CredentialStatus;
  lastLoginAt?: string;
  lastCheckAt?: string;
  createdAt: string;
  updatedAt: string;
}

export type JobStatus = "ok" | "partial" | "fail" | "killed";

export interface WorkingHours {
  start: string;
  end: string;
  tz: string;
}

export interface FBCloakJob {
  id: string;
  tenantId: string;
  credentialId: string;
  name: string;
  templateId?: string;
  targetMinIdleSec: number;
  targetMaxIdleSec: number;
  dailyCap: number;
  workingHours: WorkingHours;
  cronExpr: string;
  enabled: boolean;
  dryRun: boolean;
  useScannerFallback: boolean;
  nextRunAt?: string;
  lastRunAt?: string;
  lastRunStatus?: JobStatus;
  createdAt: string;
  updatedAt: string;
}

export type SendStatus = "sent" | "dry_run" | "skipped" | "failed";

export interface FBCloakSendLog {
  id: string;
  tenantId: string;
  jobId: string;
  credentialId: string;
  fanpageId: string;
  conversationId: string;
  recipientPsid?: string;
  recipientName?: string;
  lastInboundAt?: string;
  messageText: string;
  status: SendStatus;
  skipReason?: string;
  error?: string;
  screenshotPre?: string;
  screenshotPost?: string;
  sentAt: string;
}

// --- Query keys ---

const QK = {
  credentials: ["fbcloak", "credentials"] as const,
  jobs: ["fbcloak", "jobs"] as const,
  log: (filters: Record<string, unknown>) => ["fbcloak", "log", filters] as const,
};

// --- Credentials ---

export function useFBCloakCredentials() {
  const ws = useWs();
  const connected = useAuthStore((s) => s.connected);
  return useQuery({
    queryKey: QK.credentials,
    queryFn: async () => {
      const res = await ws.call<{ credentials: FBCloakCredential[] }>("fbcloak.credentials.list");
      return res.credentials ?? [];
    },
    enabled: connected,
    staleTime: 30_000,
  });
}

export function useAddCredential() {
  const ws = useWs();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: {
      fanpageId: string;
      fanpageName: string;
      cookies: string;
      proxyUrl?: string;
      userAgent?: string;
      viewportW?: number;
      viewportH?: number;
      timezone?: string;
    }) => ws.call<{ credential: FBCloakCredential }>("fbcloak.credentials.add", input),
    onSuccess: () => qc.invalidateQueries({ queryKey: QK.credentials }),
    onError: (err) =>
      toast.error(
        i18next.t("fbcloak:errors.saveFailed", { detail: err instanceof Error ? err.message : "" }),
      ),
  });
}

export function useTestCredential() {
  const ws = useWs();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => ws.call<{ result: { ok: boolean; status: string } }>("fbcloak.credentials.test", { id }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QK.credentials }),
  });
}

export function useDeleteCredential() {
  const ws = useWs();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => ws.call<{ ok: boolean }>("fbcloak.credentials.delete", { id }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QK.credentials }),
    onError: (err) =>
      toast.error(
        i18next.t("fbcloak:errors.deleteFailed", { detail: err instanceof Error ? err.message : "" }),
      ),
  });
}

// --- Jobs ---

export function useFBCloakJobs() {
  const ws = useWs();
  const connected = useAuthStore((s) => s.connected);
  return useQuery({
    queryKey: QK.jobs,
    queryFn: async () => {
      const res = await ws.call<{ jobs: FBCloakJob[] }>("fbcloak.jobs.list");
      return res.jobs ?? [];
    },
    enabled: connected,
    staleTime: 30_000,
  });
}

export interface CreateJobInput {
  credentialId: string;
  name: string;
  cronExpr: string;
  targetMinIdleSec: number;
  targetMaxIdleSec: number;
  dailyCap: number;
  workingHours: WorkingHours;
  useScannerFallback: boolean;
}

export function useCreateJob() {
  const ws = useWs();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateJobInput) =>
      ws.call<{ job: FBCloakJob }>("fbcloak.jobs.create", input as unknown as Record<string, unknown>),
    onSuccess: () => qc.invalidateQueries({ queryKey: QK.jobs }),
    onError: (err) =>
      toast.error(
        i18next.t("fbcloak:errors.saveFailed", { detail: err instanceof Error ? err.message : "" }),
      ),
  });
}

export function useToggleJob() {
  const ws = useWs();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      ws.call<{ ok: boolean }>("fbcloak.jobs.toggle", { id, enabled }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QK.jobs }),
  });
}

export function useDeleteJob() {
  const ws = useWs();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => ws.call<{ ok: boolean }>("fbcloak.jobs.delete", { id }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QK.jobs }),
    onError: (err) =>
      toast.error(
        i18next.t("fbcloak:errors.deleteFailed", { detail: err instanceof Error ? err.message : "" }),
      ),
  });
}

export function useRunJobNow() {
  const ws = useWs();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => ws.call<{ status: JobStatus }>("fbcloak.jobs.run-now", { id }),
    onSuccess: () => qc.invalidateQueries({ queryKey: QK.jobs }),
    onError: (err) =>
      toast.error(
        i18next.t("fbcloak:errors.runNowFailed", { detail: err instanceof Error ? err.message : "" }),
      ),
  });
}

// --- Send log ---

export interface SendLogFilters {
  jobId?: string;
  status?: SendStatus;
  fromDate?: string; // RFC3339
  toDate?: string;
  limit?: number;
  offset?: number;
}

export function useFBCloakSendLog(filters: SendLogFilters) {
  const ws = useWs();
  const connected = useAuthStore((s) => s.connected);
  return useQuery({
    queryKey: QK.log(filters as Record<string, unknown>),
    queryFn: async () => {
      const res = await ws.call<{ logs: FBCloakSendLog[] }>(
        "fbcloak.log.list",
        filters as unknown as Record<string, unknown>,
      );
      return res.logs ?? [];
    },
    enabled: connected,
    staleTime: 15_000,
  });
}

// --- Phase 4: disclaimer + dual-mode router ---

export interface DisclaimerAck {
  tenantId: string;
  version: string;
  userId?: string;
  ackedAt: string;
}

export interface DisclaimerStatus {
  currentVersion: string;
  required: boolean;
  latest?: DisclaimerAck;
}

const DISCLAIMER_QK = ["fbcloak", "disclaimer"] as const;

export function useDisclaimerStatus() {
  const ws = useWs();
  const connected = useAuthStore((s) => s.connected);
  return useQuery({
    queryKey: DISCLAIMER_QK,
    queryFn: async () => {
      const res = await ws.call<{ status: DisclaimerStatus }>("fbcloak.disclaimer.status");
      return res.status;
    },
    enabled: connected,
    staleTime: 60_000,
  });
}

export function useAckDisclaimer() {
  const ws = useWs();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (version?: string) =>
      ws.call<{ ok: boolean }>("fbcloak.disclaimer.ack", version ? { version } : {}),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: DISCLAIMER_QK });
      qc.invalidateQueries({ queryKey: QK.jobs });
    },
    onError: (err) =>
      toast.error(
        i18next.t("fbcloak:errors.saveFailed", { detail: err instanceof Error ? err.message : "" }),
      ),
  });
}

export type FBProactiveChannel = "api" | "cloak";
export type FBProactiveTag = "response" | "human_agent" | "";

export interface FBProactiveResult {
  channel: FBProactiveChannel;
  tag: FBProactiveTag;
  sendLogId?: string;
}

export function useSendProactive() {
  const ws = useWs();
  return useMutation({
    mutationFn: (input: { fanpageId: string; recipientPsid: string; message: string }) =>
      ws.call<{ result: FBProactiveResult }>("fbcloak.send-proactive", input),
  });
}

// useScreenshotPath — fetch the on-disk path; UI then signs via /v1/files/sign.
export function useScreenshotPath() {
  const ws = useWs();
  return useCallback(
    async (sendLogId: string, kind: "pre" | "post") => {
      const res = await ws.call<{ path: string }>("fbcloak.log.screenshot", { sendLogId, kind });
      return res.path;
    },
    [ws],
  );
}
