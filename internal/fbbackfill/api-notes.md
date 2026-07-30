# Graph API — Backfill endpoint notes

Captured during Phase 1 Task 1.0 verify-before-code. Do not trust without re-checking against a real Page response during integration testing.

## Endpoints

### `GET /v25.0/{page-id}/conversations`

- **Permission:** Page access token with `MESSAGING` task, plus app-level scopes `pages_messaging`, `pages_read_engagement`, `pages_manage_metadata`
- **Query params used by backfill:**
  - `fields=updated_time,message_count,participants`
  - `limit=100` (default 25; 100 appears safe in practice)
  - `platform=MESSENGER` (filter out Instagram threads if both are bound)
  - `after={cursor}` for pagination (cursor string from `paging.cursors.after`)
- **Response shape:**
  ```json
  {
    "data": [
      {
        "id": "t_12345",
        "updated_time": "2024-06-15T08:30:45+0000",
        "message_count": 42,
        "participants": {
          "data": [
            { "id": "<PSID>", "name": "Jane Doe", "email": "..." },
            { "id": "<PAGE_ID>", "name": "My Page" }
          ]
        }
      }
    ],
    "paging": {
      "cursors": { "after": "QVFIU...", "before": "QVFIU..." },
      "next": "https://graph.facebook.com/v25.0/{page-id}/conversations?...&after=QVFIU..."
    }
  }
  ```
- **Pagination:** Use `paging.cursors.after` (cleaner than parsing the `next` URL). When absent → end of list.
- **Notable:** "Time-based pagination is not available" per docs — we cannot filter by a date window on this endpoint. Backfill must walk the full list and filter client-side if needed.

### `GET /v25.0/{conversation-id}/messages`

- **Permission:** Same as conversations
- **Query params used by backfill:**
  - `fields=id,message,from,to,created_time,attachments`
  - `limit=100`
  - `after={cursor}` for pagination
- **Response shape:**
  ```json
  {
    "data": [
      {
        "id": "m_abc123",
        "message": "Hi, I'd like to order...",
        "created_time": "2024-06-15T08:30:45+0000",
        "from": { "id": "<PSID>", "name": "Jane Doe", "email": "..." },
        "to": { "data": [{ "id": "<PAGE_ID>", "name": "My Page" }] },
        "attachments": {
          "data": [
            { "id": "...", "mime_type": "image/jpeg", "name": "photo.jpg", "size": 123456,
              "file_url": "https://...", "image_data": { "width": 1024, "height": 768, "url": "..." } }
          ]
        }
      }
    ],
    "paging": { "cursors": { "after": "..." }, "next": "https://..." }
  }
  ```
- **Order:** Messages returned **newest-first (reverse chronological)**. Backfill reverses the slice before summarization so the LLM sees chronological order.
- **created_time:** ISO 8601 string with timezone offset — parse via `time.Parse(time.RFC3339, ...)` (Go's `time.RFC3339` accepts `+0000` and `Z`; if it fails for `+0000`, fall back to `"2006-01-02T15:04:05-0700"`).

## Rate limiting

### Platform-level error codes (generic)
- `#4` — App-level rate limit
- `#17` — User-level rate limit
- `#32` — Page-level rate limit
- `#613` — Request throttled

Retry after parsing `Retry-After` header or the `error_subcode` field if present.

### BUC (Business Use Case) header — preferred signal
Header: `X-Business-Use-Case-Usage`, value is JSON:
```json
{
  "<business-id-or-page-id>": [
    {
      "type": "pages",
      "call_count": 25,
      "total_cputime": 18,
      "total_time": 22,
      "estimated_time_to_regain_access": 0,
      "ads_api_access_tier": "standard_access"
    }
  ]
}
```

- Each field is a percentage (0–100) of the hourly quota consumed.
- Throttling begins when ANY of `call_count`, `total_cputime`, `total_time` reaches 100.
- BUC-specific error codes: `80001` for Pages.
- `estimated_time_to_regain_access` is in minutes — use as backoff when throttled.

### Pacing policy (our choice)

| BUC peak % | Action |
|-----------|--------|
| < 50 | No pause |
| 50–70 | 2s sleep between calls |
| 70–90 | 10s sleep |
| 90–99 | 60s sleep |
| ≥ 100 | Transition job to `paused`, schedule auto-resume at `estimated_time_to_regain_access` minutes (fallback 60 min) |

## Auth errors (non-retryable)
- `#190` — Token expired/revoked
- `#102` — Session invalid
- `#10` — Permission denied

When hit, backfill transitions job to `failed` with `last_error="Page Access Token expired"` — user must re-connect the channel.

## Open questions (defer to manual test with real Page)
- Does `participants.data` always list the PSID first, the Page second? Plan: filter by `id != pageID`.
- Are attachment URLs time-limited? Plan: don't store attachment URLs in summaries; just note attachment count/type in the summary text.
