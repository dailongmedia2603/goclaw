// TypeScript types mirroring the Go BackfillState struct in
// internal/fbbackfill/state.go. Keep the JSON field names in sync —
// these are the wire format used by fb_backfill.status RPC and the
// fb_backfill.progress WS event payload.

export type JobStatus =
  | "pending"
  | "running"
  | "paused"
  | "completed"
  | "failed"
  | "cancelled";

export interface BackfillState {
  version: number;
  status: JobStatus;
  started_at?: string | null;
  finished_at?: string | null;
  updated_at: string;
  last_error?: string;
  conversations_total: number;
  conversations_done: number;
  conversations_skipped: number;
  messages_ingested: number;
  episodics_created: number;
  conversation_cursor?: string;
  current_convo_id?: string;
  message_cursor?: string;
  max_conversations: number;
  skip_existing: boolean;
  force_recreate: boolean;
  triggered_by: string;
}

export interface StartOpts {
  maxConversations?: number;
  skipExisting?: boolean;
  forceRecreate?: boolean;
  triggeredBy?: string;
}

export interface StatusResponse {
  state: BackfillState | null;
}
