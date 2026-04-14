# Lark MCP Preset

Connect Goclaw agents to your Lark (Feishu) workspace with a one-form setup.
Agents can read messages, docs, calendars, and contacts via the official
`@larksuiteoapi/lark-mcp` package.

## Prerequisites

- Admin role on the Goclaw tenant
- Admin access to a Lark Developer Console app (or permission to create one)
- Goclaw image with Node.js available — `:latest`, `:otel`, or `:full` all
  include Node 22 + a pre-warmed copy of `@larksuiteoapi/lark-mcp`. The `:base`
  image does **not** include Node; use a larger variant if you plan to use
  preset-based MCP servers.

## 1. Create a Lark app

1. Go to the [Lark Developer Console](https://open.larksuite.com/app) (or
   [Feishu Open Platform](https://open.feishu.cn/app) for China tenants).
2. Click **Create App** → **Custom App**.
3. Note the **App ID** (starts with `cli_…`) and **App Secret** — you'll paste
   them into Goclaw.
4. Under **Permissions & Scopes**, enable the scopes you need. A reasonable
   starter set for the default tool preset:
   - `im:message`, `im:chat`
   - `docx:document:readonly`, `wiki:wiki:readonly`
   - `calendar:calendar:readonly`
   - `contact:contact.base:readonly`
5. Publish the app version. Self-built apps are only usable by your own tenant,
   which is what you want for internal Goclaw integration.

## 2. Add the preset in Goclaw

1. Go to `/mcp` (e.g. https://agent.lemondigital.vn/mcp).
2. Click **Add Server** → **From Preset**.
3. Pick the **Lark** card.
4. Fill:
   - **Display Name** (optional) — anything, e.g. `Lark — Prod`.
   - **App ID** — paste from step 1 (`cli_…`).
   - **App Secret** — paste from step 1.
   - **Domain**:
     - `Lark (International / Vietnam)` → `https://open.larksuite.com`
     - `Feishu (China)` → `https://open.feishu.cn`
   - **Token Mode** — leave on **Tenant** (app-level). User OAuth is disabled
     in v1.
   - **Tool Presets** — pick at least one. `Default` covers IM + calendar +
     docs + contacts. Pick narrower presets to reduce the total tool surface
     (Goclaw's agent loop performs better with fewer tools).
   - **Timeout** — default 90 s works for most tenants. Increase if the first
     connect is slow on cold caches.
5. Click **Test Connection**. Within 30 s you should see a green checkmark
   with the discovered tool count. If it fails, see Troubleshooting below.
6. Click **Create**. The server appears in the list with a **lark** badge.
7. Grant the server to one or more agents via the grants dialog, same as any
   other MCP server.

## 3. Editing

Click the pencil icon on a Lark row → the Lark preset form reopens with the
previous values. The App Secret field is intentionally empty on edit — leave
it blank to keep the existing secret, or type a new value to rotate.

## 4. Security model

- **App Secret** is encrypted at rest via `GOCLAW_ENCRYPTION_KEY` (AES-256-GCM).
- It is delivered to the spawned `npx` subprocess via the `LARK_APP_SECRET`
  env variable — **never** via command-line args, so it does not appear in
  `ps aux` / `/proc/*/cmdline`.
- `GET /v1/mcp/servers` responses redact `api_key` and mask
  `env.LARK_APP_SECRET` to `***` when the server was created via a preset.
- MCP exports (`GET /v1/mcp/export`) exclude `preset_config` secrets because
  they live in the `api_key` column, which export already encrypts.
- Tenant isolation: presets are tenant-scoped like any other MCP server —
  tenant A cannot read or modify tenant B's Lark configuration.

## 5. Troubleshooting

| Symptom | Cause / Fix |
|---|---|
| `ENOENT: npx` or `node: command not found` in server status | Running the `:base` image without Node. Use `:latest` / `:otel` / `:full`, or install Node manually in a derived image. |
| Test Connection times out after 30 s | Node is downloading `@larksuiteoapi/lark-mcp` from npm on first call. Official Goclaw images pre-install it — ensure you're on a recent image. Otherwise, bump Timeout to 180 s and retry. |
| `401 Unauthorized` from Lark | App Secret is wrong, or the app is not published/activated. |
| `permission denied` for specific tools | The app in Lark Developer Console is missing the required scope. Add the scope, re-publish, and retry. |
| `tenant not found` | Wrong Domain. Lark Vietnam uses the international domain (`open.larksuite.com`); Feishu is China-only. |
| Server stays "Connecting…" forever | Check Goclaw logs for `mcp.server.connect_failed`. Common cause: network egress blocked to `open.larksuite.com`. |
| Agent fails to invoke a tool with "tool not registered" | Either the agent doesn't have a grant, or `tool_allow`/`tool_deny` filters block it, or the server is in search-mode (>40 total MCP tools) and the tool needs to be activated via `mcp_tool_search`. |

## 6. Deleting

Click the trash icon → confirm by typing the display name. This terminates
the `npx` subprocess, removes all agent grants, and purges the encrypted
secret from the DB.

## 7. Known limitations (v1)

- **User OAuth mode** is not supported yet — only `tenant_access_token`.
- **`streamable` transport** is not exposed — only `stdio` (Lark MCP's stdio
  transport is more robust for multi-user servers).
- **Rate limiting** relies on `@larksuiteoapi/lark-mcp`'s built-in throttling;
  Goclaw does not add client-side rate limiting on top.

## 8. For developers — adding new presets

The preset architecture is extensible. See `docs/mcp-presets-developer.md`
(TODO) and the Lark reference implementation at
`internal/mcp/presets/lark.go` + `ui/web/src/pages/mcp/presets/lark/`.
