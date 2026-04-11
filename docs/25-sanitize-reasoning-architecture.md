# Sanitize Pipeline & Reasoning Separation

## Overview

Assistant responses pass through a 10-step pipeline in [internal/agent/sanitize.go](../internal/agent/sanitize.go) before being saved to session and sent to users. Reasoning/thinking content is **separated from text content at the provider layer**, not via text-based heuristics.

## Pipeline Steps

`SanitizeAssistantContent()` applies these transformers in order:

1. **stripGarbledToolXML** — broken XML tool calls (DeepSeek, Minimax, GLM)
2. **stripDowngradedToolCallText** — `[Tool Call: ...]` text blocks
3. **stripBareToolCallText** — bare `func_name(args=...)` text
4. **stripThinkingTags** — `<think>/<thinking>/<thought>/<antThinking>` XML blocks (structural)
5. **stripFinalTags** — keep content inside `<final>...</final>`
6. **stripEchoedSystemMessages** — `[System Message]` blocks
7. **collapseConsecutiveDuplicateBlocks**
8. **stripMediaPaths** — `MEDIA:/path` references
9. **stripLeadingBlankLines**
10. **replaceInlineLaTeX** + **stripToolResultJSON**

## Reasoning Content Separation

Reasoning is **not** separated by sniffing text output. Each provider exposes it as a distinct field/event, and the agent loop routes it through a separate channel:

| Provider | Reasoning field | File |
|---|---|---|
| Anthropic | `thinking_delta` event (`content_block_delta`) | [anthropic_stream.go:84-107](../internal/providers/anthropic_stream.go) |
| OpenAI-compat (DeepSeek, Kimi, Qwen) | `delta.reasoning_content` | [openai.go:186-197](../internal/providers/openai.go) |
| Claude CLI | `block.Thinking` | [claude_cli_parse.go](../internal/providers/claude_cli_parse.go) |
| DashScope | `resp.Thinking` | [dashscope.go](../internal/providers/dashscope.go) |

Each provider emits two distinct `StreamChunk` types:
- `StreamChunk{Content: ...}` → `protocol.ChatEventChunk` → `rc.streamBuffer` → channel answer lane
- `StreamChunk{Thinking: ...}` → `protocol.ChatEventThinking` → `rc.thinkingBuffer` → channel reasoning lane

The channel layer ([internal/channels/events.go](../internal/channels/events.go)) maintains two separate stream messages: the reasoning lane (edited with `formatReasoningPreview`) and the answer lane (edited with the raw buffer).

## Fallback: `<think>` XML Tags

For providers that don't natively separate reasoning (DeepSeek-via-OpenRouter, Qwen local, Ollama), the channel layer applies `SplitThinkTags` on streaming chunks to extract `<think>...</think>` content and route it to the reasoning lane. This is a **structural marker** — not a heuristic — and is covered by [internal/channels/think_tag_parser_test.go](../internal/channels/think_tag_parser_test.go).

## What Was Removed (2026-04-11)

Two text-based heuristics that tried to strip a plain-text `Reasoning:` prefix were removed:

- `stripPlainTextReasoning` (final pipeline) — `internal/agent/sanitize.go`
- `sanitizeStreamBuffer` (streaming path) — `internal/channels/events.go`

### Why

Both heuristics had a broken bullet-list detection: after a blank line, any line starting with `-`, `*`, `•`, or whitespace-indented was treated as "reasoning continuation" and stripped. Bullet-list answers (common in Vietnamese assistant replies) were drained to empty, causing the `"..."` fallback in `loop_finalize.go` and producing apparent bot failures.

Since every supported provider already exposes reasoning via a dedicated protocol field, the text heuristic was a redundant defensive layer that caused more bugs than it prevented.

### What Replaced It

Nothing. The streaming buffer now forwards directly to the channel, and the final pipeline relies on structural `<think>` XML tag stripping. If an LLM still emits `Reasoning:` as a prose header, the text appears verbatim to the user — a minor cosmetic issue, not a silent drop.

## Defensive Helper

`sanitizeWithEmptyGuard(name, content, fn)` wraps a transformer so that if it drains non-empty input to empty, it logs a warning (`sanitize.drained_to_empty`) and returns the original. Use this to guard any future transformer that is NOT expected to legitimately empty the content.

See [internal/agent/sanitize.go](../internal/agent/sanitize.go) and `TestSanitizeWithEmptyGuard` in [internal/agent/sanitize_pipeline_test.go](../internal/agent/sanitize_pipeline_test.go).

## References

- Plan: [plans/260410-remove-reasoning-text-heuristic/](../plans/260410-remove-reasoning-text-heuristic/)
- [Anthropic streaming messages API](https://docs.anthropic.com/en/api/messages-streaming)
- [DeepSeek reasoning model API](https://api-docs.deepseek.com/guides/reasoning_model)
