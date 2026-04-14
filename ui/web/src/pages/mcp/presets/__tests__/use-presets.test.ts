import { describe, it, expect } from "vitest";
import { getServerPreset } from "../use-presets";
import type { MCPServerData } from "@/types/mcp";

function makeServer(overrides: Partial<MCPServerData> = {}): MCPServerData {
  return {
    id: "00000000-0000-0000-0000-000000000000",
    name: "x",
    display_name: "",
    transport: "stdio",
    command: "npx",
    args: null,
    url: "",
    headers: null,
    env: null,
    tool_prefix: "",
    timeout_sec: 60,
    enabled: true,
    created_by: "test",
    created_at: "",
    updated_at: "",
    ...overrides,
  };
}

describe("getServerPreset", () => {
  it("returns null for plain generic servers", () => {
    expect(getServerPreset(makeServer())).toBeNull();
  });

  it("returns null when settings is absent", () => {
    expect(getServerPreset(makeServer({ settings: undefined }))).toBeNull();
  });

  it("returns null for preset-less settings blob (e.g. only require_user_credentials)", () => {
    const server = makeServer({ settings: { require_user_credentials: true } });
    expect(getServerPreset(server)).toBeNull();
  });

  it("returns the preset id when settings.preset is set", () => {
    const server = makeServer({ settings: { preset: "lark" } });
    expect(getServerPreset(server)).toBe("lark");
  });

  it("returns a future preset id unchanged (forward-compat)", () => {
    const server = makeServer({ settings: { preset: "github" } });
    expect(getServerPreset(server)).toBe("github");
  });

  it("is nil-safe", () => {
    expect(getServerPreset(null)).toBeNull();
    expect(getServerPreset(undefined)).toBeNull();
  });
});
